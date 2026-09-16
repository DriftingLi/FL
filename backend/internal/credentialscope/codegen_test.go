package credentialscope

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"forklift-training/internal/codegen"
)

// spec #940 片六：登记表的两条契约。
//
// prior art：internal/authz/codegen_test.go（生成物字节全等）、authz_coverage_lock_test.go
// （覆盖锁：新增调用点必须登记）。

func TestFrontendCredentialScopeTSInSync(t *testing.T) {
	codegen.AssertInSync(t, FrontendCredentialScopeGen)
}

// TestCredentialScopeCoverageLock 覆盖锁（**按出现次数**）：前端源码里每一处 opt-out 都必须
// 登记，登记的每一处也必须在源码里真的存在，且次数一致 —— 同一文件里再加一处例外同样报红。
func TestCredentialScopeCoverageLock(t *testing.T) {
	srcDir := filepath.Join("..", "..", "..", "frontend", "src")
	found, err := ScanOptOutCounts(srcDir)
	if err != nil {
		t.Fatalf("扫描前端源码失败: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("一处 opt-out 都没扫到 —— 扫描面失效（判据字面量改了？目录变了？）")
	}
	registered := RegisteredCounts()

	diff := func(a, b map[string]int) []string {
		var out []string
		for f, n := range a {
			if bn, ok := b[f]; !ok {
				out = append(out, f+"（源码 "+strconv.Itoa(n)+" 处，未登记）")
			} else if bn != n {
				out = append(out, f+"（源码 "+strconv.Itoa(n)+" 处，登记 "+strconv.Itoa(bn)+" 处）")
			}
		}
		sort.Strings(out)
		return out
	}

	if missing := diff(found, registered); len(missing) > 0 {
		t.Fatalf("以下文件用了 %s 但登记不符（改 registry.go 的 Count 并重跑 go run ./cmd/gen-credscope）:\n  %s",
			Marker, strings.Join(missing, "\n  "))
	}
	if stale := diff(registered, found); len(stale) > 0 {
		t.Fatalf("以下登记项在前端源码里找不到对应的 %s（条目已过期，请从 registry.go 删除并重跑生成）:\n  %s",
			Marker, strings.Join(stale, "\n  "))
	}
}

// TestOptOutsShape 登记表形状：字段完整、无重复文件、次数为正、理由不是占位文本。
func TestOptOutsShape(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range OptOuts {
		if o.File == "" || o.Reason == "" || o.Count <= 0 {
			t.Fatalf("登记项不完整: %+v", o)
		}
		if seen[o.File] {
			t.Fatalf("同一文件重复登记: %s", o.File)
		}
		seen[o.File] = true
		if len([]rune(o.Reason)) < 8 {
			t.Fatalf("理由太短，不构成「域理由」: %+v", o)
		}
	}
}
