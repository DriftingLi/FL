// Package report 报告流程协调器：评估报告与电池 RUL 报告的生成/下载/再生成单点实现。
// 与 gin 解耦：方法接受 context.Context、返回纯值，HTTP 翻译留在 handler 层薄壳。
package report

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"forklift-training/internal/storage"
)

// Spec 报告规格：报告类型间差异的全部收敛点（评估 vs 电池）。
// loader / generator / writer 三个 adapter 槽位：生产为 pgx 仓储与 PDF 生成器，
// 测试注入内存替身（两个 loader adapter 证明 seam 是真实的）。
type Spec[T any] struct {
	// Loader 按 ID 加载记录（记录不存在时返回 pgx.ErrNoRows，handler 据此 404）。
	Loader func(ctx context.Context, id int64) (*T, error)
	// PathOf 读取记录中的报告 URL（缺失/失效时触发再生成）。
	PathOf func(rec *T) string
	// Writer 回写报告 URL（失败仅告警，不影响主流程）。
	Writer func(ctx context.Context, id int64, url string) error
	// Prepare 业务 fallback：生成前重建派生数据 / 补齐建议。
	// 详情端点与报告生成共用同一实现（评估与电池各自单点，不再两处复制）。
	Prepare func(ctx context.Context, rec *T)
	// Render 渲染 PDF 字节（Prepare 之后调用）。
	Render func(ctx context.Context, rec *T) ([]byte, error)
	// KeyPrefix 上传 key 前缀（reports/evaluation_report_ / reports/battery_report_）。
	KeyPrefix string
	// Logger 告警日志输出。
	Logger *zap.Logger
	// Storage 上传/删除/探测文件。
	Storage storage.Storage
}

// Coordinator 报告流程协调器：加载 → 业务 fallback → 生成 → 上传 → 回写 → 清理旧 PDF。
// 并发去重：同 ID 的再生成经 singleflight 合并，不产生重复上传/孤儿 PDF。
type Coordinator[T any] struct {
	spec Spec[T]
	sf   singleflight.Group
}

// New 构造协调器。
func New[T any](spec Spec[T]) *Coordinator[T] {
	return &Coordinator[T]{spec: spec}
}

// GenerateResult 生成成功的结果（handler 翻译为 HTTP 响应）。
type GenerateResult struct {
	ID       int64
	PDFURL   string
	FileSize int
}

// Generate 处理 POST <prefix>/:id/report：重新加载记录 → 生成 PDF → 上传 → 回写路径。
func (c *Coordinator[T]) Generate(ctx context.Context, id int64) (GenerateResult, error) {
	rec, err := c.spec.Loader(ctx, id)
	if err != nil {
		return GenerateResult{}, err
	}
	url, size, err := c.generateAndUpload(ctx, id, rec)
	if err != nil {
		return GenerateResult{}, err
	}
	return GenerateResult{ID: id, PDFURL: url, FileSize: size}, nil
}

// DownloadURL 处理 GET <prefix>/:id/report：URL 有效则直接返回；否则并发安全地
// 再生成并回写后返回。
// 并发语义：加载→检查→再生成整体放入 singleflight（同 ID 只产生一份 PDF）；
// 晚到的请求在 fn 重跑时发现路径已回写，短路直接返回既有 URL。
func (c *Coordinator[T]) DownloadURL(ctx context.Context, id int64) (string, error) {
	v, err, _ := c.sf.Do(fmt.Sprintf("dl:%d", id), func() (any, error) {
		rec, loadErr := c.spec.Loader(ctx, id)
		if loadErr != nil {
			return "", loadErr
		}
		// 已有有效 URL 直接返回（并发/晚到请求的短路路径）
		if url := c.spec.PathOf(rec); url != "" && c.storageExists(ctx, url) {
			return url, nil
		}
		url, _, genErr := c.generateAndUpload(ctx, id, rec)
		return url, genErr
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// generateAndUpload 业务 fallback → 生成 PDF → 上传 → 回写 → 清理旧 PDF，返回 URL 与文件大小。
// 新 PDF 上传并回写成功后删除旧 PDF（避免存储累积历史报告文件；删除失败仅告警不影响主流程）。
func (c *Coordinator[T]) generateAndUpload(ctx context.Context, id int64, rec *T) (string, int, error) {
	if c.spec.Prepare != nil {
		c.spec.Prepare(ctx, rec)
	}
	pdfBytes, err := c.spec.Render(ctx, rec)
	if err != nil {
		return "", 0, err
	}
	key := fmt.Sprintf("%s%d_%s.pdf", c.spec.KeyPrefix, id, time.Now().Format("20060102150405"))
	url, err := c.spec.Storage.Save(ctx, key, pdfBytes, "application/pdf")
	if err != nil {
		return "", 0, err
	}
	if err := c.spec.Writer(ctx, id, url); err != nil {
		c.spec.Logger.Warn("回写报告路径失败", zap.Error(err), zap.Int64("id", id))
	}
	// 清理旧 PDF（新文件已上传成功，删除旧文件失败不影响可用性）
	if oldURL := c.spec.PathOf(rec); oldURL != "" && oldURL != url {
		if err := c.spec.Storage.Delete(ctx, oldURL); err != nil {
			c.spec.Logger.Warn("删除旧报告失败", zap.Error(err), zap.Int64("id", id), zap.String("old_url", oldURL))
		}
	}
	return url, len(pdfBytes), nil
}

// storageExists 检查存储中文件是否存在；出错按不存在处理（重新生成，保持既有语义）。
func (c *Coordinator[T]) storageExists(ctx context.Context, url string) bool {
	exists, err := c.spec.Storage.Exists(ctx, url)
	if err != nil {
		c.spec.Logger.Warn("检查存储文件失败，按不存在处理", zap.String("url", url), zap.Error(err))
		return false
	}
	return exists
}
