package pdf

import (
	"os"
	"strings"
	"testing"
)

// TestFontEmbedSingleSourceLock 是第二份字体嵌入的仓库断言（ADR-0056 §12 / #1105）：
// 估值域字体必须来自 internal/pdfutil 的唯一一份 //go:embed，
// 本包内不得再出现 fonts/ 目录，也不得自己声明 go:embed。
func TestFontEmbedSingleSourceLock(t *testing.T) {
	if _, err := os.Stat("fonts"); err == nil {
		t.Fatalf("internal/valuation/pdf/fonts/ 必须不存在：字体唯一来源是 internal/pdfutil")
	} else if !os.IsNotExist(err) {
		t.Fatalf("检查 fonts 目录失败: %v", err)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取包目录失败: %v", err)
	}
	for _, e := range entries {
		// 只扫非测试源码：//go:embed 是源码声明，测试文件里的字面量不算
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", e.Name(), err)
		}
		if strings.Contains(string(b), "go:embed") {
			t.Errorf("%s 不得声明 //go:embed：字体唯一来源是 internal/pdfutil.EnsureFontLoaded", e.Name())
		}
	}
}
