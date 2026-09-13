package credentialscope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec #940 片六：登记表的两条契约。
//
// prior art：internal/authz/codegen_test.go（生成物字节全等）、authz_coverage_lock_test.go
// （覆盖锁：新增调用点必须登记）。

func TestFrontendCredentialScopeTSInSync(t *testing.T) {
	want, err := RenderFrontendTS()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	path := filepath.Join("..", "..", "..", "frontend", "src", "config", "credentialScope.ts")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取生成物 %s 失败（应先运行 cd backend && go run ./cmd/gen-credscope）: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("生成物与登记表不同步：请 cd backend && go run ./cmd/gen-credscope")
	}
}

// TestCredentialScopeCoverageLock 覆盖锁：前端源码里出现的每一处 opt-out 都必须登记，
// 登记的每一处也必须在源码里真的存在（防止「登记表成了历史垃圾」）。
func TestCredentialScopeCoverageLock(t *testing.T) {
	srcDir := filepath.Join("..", "..", "..", "frontend", "src")
	found, err := ScanOptOutFiles(srcDir)
	if err != nil {
		t.Fatalf("扫描前端源码失败: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("一处 opt-out 都没扫到 —— 扫描面失效（判据字面量改了？目录变了？）")
	}
	registered := map[string]bool{}
	for _, f := range RegisteredFiles() {
		registered[f] = true
	}
	foundSet := map[string]bool{}
	for _, f := range found {
		foundSet[f] = true
	}
	var missing, stale []string
	for _, f := range found {
		if !registered[f] {
			missing = append(missing, f)
		}
	}
	for _, f := range RegisteredFiles() {
		if !foundSet[f] {
			stale = append(stale, f)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("以下文件用了 %s 但没登记（请加进 registry.go 并重跑 go run ./cmd/gen-credscope）:\n  %s",
			Marker, strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Fatalf("以下登记项在前端源码里找不到对应的 %s（条目已过期，请从 registry.go 删除并重跑生成）:\n  %s",
			Marker, strings.Join(stale, "\n  "))
	}
}

// TestOptOutsShape 登记表形状：字段完整、无重复文件、理由不是占位文本。
func TestOptOutsShape(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range OptOuts {
		if o.File == "" || o.Reason == "" {
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
