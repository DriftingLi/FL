package model

import (
	"fmt"
	"testing"
)

// 模型登记表形状锁（ADR-0047 §6 / spec #933）：块名与模型都非空、模型不重复、
// AllModels 汇总与块声明一致。
//
// 为什么需要它：模型清单以前是 testutil 里的一张平表，漏加一个模型的症状是「测试里表不存在」
// 这类与改动无关的报错；现在清单在领域包里，形状由本测试兜住。
func TestModelRegistryShape(t *testing.T) {
	if len(ModelBlocks) == 0 {
		t.Fatal("模型登记表为空")
	}
	seen := map[string]bool{}
	count := 0
	for _, b := range ModelBlocks {
		if b.Name == "" {
			t.Fatal("存在未命名域块")
		}
		if len(b.Models) == 0 {
			t.Fatalf("域块 %q 没有任何模型", b.Name)
		}
		for _, m := range b.Models {
			name := fmt.Sprintf("%T", m)
			if seen[name] {
				t.Fatalf("模型重复登记: %s", name)
			}
			seen[name] = true
			count++
		}
	}
	if got := len(AllModels()); got != count {
		t.Fatalf("AllModels 汇总 %d 个模型，块声明共 %d 个", got, count)
	}
}
