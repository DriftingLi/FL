// 本文件：悬空文件回收 sweep 单点（ADR-0027 C2）——论坛图片与投稿文件两份 ~100 行同算法复制
// （扫描循环 + key 提取 + TTL 判定）收成一个 module；两个业务域各自只剩
// 「前缀 + 引用集 provider + TTL」薄配置。
// 时间戳知识归位存储 seam：TTL 判定用 storage.FileInfo.LastModified（存储侧原生元数据），
// 不再反向解析文件名内嵌时间戳。
package filestore

import (
	"context"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/storage"
)

// OrphanSweepConfig 悬空回收域配置（ADR-0027 C2：两域薄配置）。
type OrphanSweepConfig struct {
	// Domain 日志域前缀（"forum_image" / "contribution"）。
	Domain string
	// TTL 悬空回收门槛（超过该时长未被引用才删）。
	TTL time.Duration
	// List 按域前缀列出文件（携带存储侧原生 LastModified）。
	List func(ctx context.Context) ([]storage.FileInfo, error)
	// Referenced 收集全量引用集（与 keyOf 同归一化口径），差集判定用。
	// 必须报 error：「查不到谁在引用」不得被当成「没人引用」——失败与空集在 interface 上
	// 不同形（ADR-0062 票5）。
	Referenced func() (map[string]bool, error)
	// KeyOf URL → 存储对象 key；提取失败返回空串（跳过该文件）。
	KeyOf func(url string) string
	// DeleteFile 按 URL 删除（ctx 取消语义贯穿）。
	DeleteFile func(ctx context.Context, url string) error
	// Logger 尽力而为日志（可为 nil）。
	Logger *zap.Logger
}

// RunOrphanSweep 悬空回收单点实现：list(prefix) 与全量引用集差集，
// 仅删除 LastModified 超过 ttl 且未被引用的文件。
// 返回清理数（尽力而为，存储错误不中断不误删）；ctx 取消语义贯穿（优雅退出及时让路）。
func RunOrphanSweep(ctx context.Context, cfg OrphanSweepConfig) int {
	if ctx == nil {
		ctx = context.Background()
	}
	stored, err := cfg.List(ctx)
	if err != nil {
		if cfg.Logger != nil {
			cfg.Logger.Warn("["+cfg.Domain+"] ListWithInfo 失败", zap.Error(err))
		}
		// 是取消导致或存储不可用：本轮不清理，避免误判
		return 0
	}
	if len(stored) == 0 {
		return 0
	}
	referenced, err := cfg.Referenced()
	if err != nil {
		// 引用集不完整 ⇒ 无从判断谁还在被引用，与 list 失败同口径整轮放弃（票5）。
		if cfg.Logger != nil {
			cfg.Logger.Warn("["+cfg.Domain+"] 收集引用集失败", zap.Error(err))
		}
		return 0
	}
	if len(referenced) == 0 {
		// 存储非空而引用集为空：健康库里「没有任何文件被引用」儿不成比例，判为异常而非
		// 「全都悬空」——兜底闸与 list 失败同口径，整轮不清理（ADR-0062 票5）。
		if cfg.Logger != nil {
			cfg.Logger.Warn("[" + cfg.Domain + "] 引用集为空，本轮不清理")
		}
		return 0
	}
	cleaned := 0
	cutoff := time.Now().Add(-cfg.TTL)
	for _, f := range stored {
		// 优雅退出：检查取消
		select {
		case <-ctx.Done():
			return cleaned
		default:
		}
		key := cfg.KeyOf(f.URL)
		if key == "" || referenced[key] {
			continue
		}
		// 元数据缺失（LastModified 零值）无法判定 TTL，保守保留
		if f.LastModified.IsZero() || !f.LastModified.Before(cutoff) {
			continue
		}
		if err := cfg.DeleteFile(ctx, f.URL); err == nil {
			cleaned++
		} else if ctx.Err() != nil {
			// 取消导致失败，立即返回
			return cleaned
		}
	}
	return cleaned
}
