// Package handler 实现 HTTP 处理器
// 本文件：报告流程协调器——评估报告与电池 RUL 报告的生成/下载/再生成单点实现。
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"forklift-training/internal/storage"
	"forklift-training/pkg/response"
)

// ReportCoordinator 报告流程协调器（评估与电池 RUL 报告共用）：
// 加载 → 重建派生 → 生成 → 上传 → 回写 → 302/响应。
// loader / generator / writer 三个 adapter 槽位：生产为 pgx 仓储与 PDF 生成器，
// 测试注入内存替身（两个 loader adapter 证明 seam 是真实的）。
// 并发去重：同 ID 的再生成经 singleflight 合并，不产生重复上传/孤儿 PDF。
type ReportCoordinator[T any] struct {
	logger  *zap.Logger
	storage storage.Storage
	// keyPrefix 上传 key 前缀（reports/evaluation_report_ / reports/battery_report_）
	keyPrefix   string
	notFoundMsg string
	sf          singleflight.Group

	loader    func(ctx context.Context, id int64) (*T, error)
	generator func(ctx context.Context, rec *T) ([]byte, error)
	pathOf    func(rec *T) string
	writer    func(ctx context.Context, id int64, url string) error
}

// Generate 处理 POST <prefix>/:id/report：重新加载记录 → 生成 PDF → 上传 → 回写路径。
func (c *ReportCoordinator[T]) Generate(ginCtx *gin.Context) {
	id, err := strconv.ParseInt(ginCtx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ginCtx, "id 必须为整数")
		return
	}

	rec, err := c.loader(ginCtx.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(ginCtx, c.notFoundMsg)
			return
		}
		c.logger.Error("查询记录失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(ginCtx, "查询记录失败")
		return
	}

	ctx, cancel := context.WithTimeout(ginCtx.Request.Context(), 60*time.Second)
	defer cancel()
	pdfURL, size, err := c.generateAndUpload(ctx, id, rec)
	if err != nil {
		c.logger.Error("生成报告失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(ginCtx, "生成报告失败: "+err.Error())
		return
	}

	response.Success(ginCtx, gin.H{
		"evaluation_id": id,
		"pdf_url":       pdfURL,
		"file_size":     size,
	})
}

// Download 处理 GET <prefix>/:id/report：URL 有效则 302；否则并发安全地再生成并回写后 302。
// 并发语义：加载→检查→再生成整体放入 singleflight（同 ID 只产生一份 PDF）；
// 晚到的请求在 fn 重跑时发现路径已回写，短路直接返回既有 URL。
func (c *ReportCoordinator[T]) Download(ginCtx *gin.Context) {
	id, err := strconv.ParseInt(ginCtx.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(ginCtx, "id 必须为整数")
		return
	}

	ctx, cancel := context.WithTimeout(ginCtx.Request.Context(), 60*time.Second)
	defer cancel()
	pdfURL, err := c.resolveDownloadURL(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(ginCtx, c.notFoundMsg)
			return
		}
		c.logger.Error("查询记录失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(ginCtx, "查询记录失败")
		return
	}

	// 302 重定向到公开访问 URL（浏览器直连下载）
	ginCtx.Redirect(http.StatusFound, pdfURL)
}

// resolveDownloadURL 在 singleflight 内完成 加载 → 检查 → 再生成（原子，无孤儿 PDF）。
func (c *ReportCoordinator[T]) resolveDownloadURL(ctx context.Context, id int64) (string, error) {
	v, err, _ := c.sf.Do(fmt.Sprintf("dl:%d", id), func() (any, error) {
		rec, loadErr := c.loader(ctx, id)
		if loadErr != nil {
			return "", loadErr
		}
		// 已有有效 URL 直接返回（并发/晚到请求的短路路径）
		if url := c.pathOf(rec); url != "" && c.storageExists(ctx, url) {
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

// generateAndUpload 生成 PDF 并上传，返回 URL 与文件大小（Generate 响应需要 size）。
// 新 PDF 上传并回写成功后删除旧 PDF（避免存储累积历史报告文件；删除失败仅告警不影响主流程）。
func (c *ReportCoordinator[T]) generateAndUpload(ctx context.Context, id int64, rec *T) (string, int, error) {
	pdfBytes, err := c.generator(ctx, rec)
	if err != nil {
		return "", 0, err
	}
	key := fmt.Sprintf("%s%d_%s.pdf", c.keyPrefix, id, time.Now().Format("20060102150405"))
	url, err := c.storage.Save(ctx, key, pdfBytes, "application/pdf")
	if err != nil {
		return "", 0, err
	}
	if err := c.writer(ctx, id, url); err != nil {
		c.logger.Warn("回写报告路径失败", zap.Error(err), zap.Int64("id", id))
	}
	// 清理旧 PDF（新文件已上传成功，删除旧文件失败不影响可用性）
	if oldURL := c.pathOf(rec); oldURL != "" && oldURL != url {
		if err := c.storage.Delete(ctx, oldURL); err != nil {
			c.logger.Warn("删除旧报告失败", zap.Error(err), zap.Int64("id", id), zap.String("old_url", oldURL))
		}
	}
	return url, len(pdfBytes), nil
}

// storageExists 检查存储中文件是否存在；出错按不存在处理（重新生成，保持既有语义）。
func (c *ReportCoordinator[T]) storageExists(ctx context.Context, url string) bool {
	exists, err := c.storage.Exists(ctx, url)
	if err != nil {
		c.logger.Warn("检查存储文件失败，按不存在处理", zap.String("url", url), zap.Error(err))
		return false
	}
	return exists
}
