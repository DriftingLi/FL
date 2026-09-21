// 悬空回收 sweep 单点的行为测试（ADR-0062 票5）：seam 是 sweep 自己的 interface
// （orphanSweepConfig 的注入槽），与 orphan_sweep.go 的既有形状同层（ADR-0050 §3 同包 internal seam）。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"forklift-training/internal/storage"
)

// staleFileURL 命名符合 <name>_<毫秒时间戳>.<ext> 契约的超时文件。
const staleFileURL = "/static/uploads/images/forum/orphan_1000000000000.webp"

// TestOrphanSweepAbortsOnEmptyReferenceSet 存储里有超时文件、引用集却为空 ⇒ 整轮不清理。
// 健康库里「一个引用都没有」儿不成比例，拿它当合法输入等于拿最坏情况当缺省（票5 的兜底闸）。
func TestOrphanSweepAbortsOnEmptyReferenceSet(t *testing.T) {
	var deleted []string
	cleaned := runOrphanSweep(context.Background(), orphanSweepConfig{
		domain: "test",
		ttl:    time.Hour,
		list: func(context.Context) ([]storage.FileInfo, error) {
			return []storage.FileInfo{{URL: staleFileURL, LastModified: time.Now().Add(-2 * time.Hour)}}, nil
		},
		referenced: func() (map[string]bool, error) { return map[string]bool{}, nil },
		keyOf:      func(u string) string { return u },
		deleteFile: func(_ context.Context, u string) error {
			deleted = append(deleted, u)
			return nil
		},
	})
	if cleaned != 0 || len(deleted) != 0 {
		t.Fatalf("引用集为空时不得删除任何文件: cleaned=%d deleted=%v", cleaned, deleted)
	}
}

// TestOrphanSweepAbortsWhenReferenceLookupFails 引用集「查不动」必须报 error 并整轮放弃。
// provider 这里带回**部分结果 + error**（主题查成功、回复查失败的真实形状）——只有 error
// 通道能区分「确实只有这些引用」与「还有一半没查到」，空集兜底闸拦不住这一形（ADR-0062 票5）。
func TestOrphanSweepAbortsWhenReferenceLookupFails(t *testing.T) {
	var deleted []string
	cleaned := runOrphanSweep(context.Background(), orphanSweepConfig{
		domain: "test",
		ttl:    time.Hour,
		list: func(context.Context) ([]storage.FileInfo, error) {
			return []storage.FileInfo{{URL: staleFileURL, LastModified: time.Now().Add(-2 * time.Hour)}}, nil
		},
		referenced: func() (map[string]bool, error) {
			return map[string]bool{"images/forum/other.webp": true}, errors.New("查询回复引用失败")
		},
		keyOf: func(u string) string { return u },
		deleteFile: func(_ context.Context, u string) error {
			deleted = append(deleted, u)
			return nil
		},
	})
	if cleaned != 0 || len(deleted) != 0 {
		t.Fatalf("引用集查询失败时不得删除任何文件: cleaned=%d deleted=%v", cleaned, deleted)
	}
}
