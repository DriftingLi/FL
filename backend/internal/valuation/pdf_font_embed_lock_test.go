// 估值域字体嵌入的第二份禁令（ADR-0056 §12 / #1105）。
//
// 字体唯一来源是 internal/pdfutil 的那一份 //go:embed：估值域内不得出现 fonts/ 目录，
// 任何生产文件也不得自己声明 //go:embed 指令。
// #1514 波 5 把 pdf/ 并回域包后，本锁的扫描面由「pdf 子包」扩到**整个域目录**（更强，不是放宽）。
package valuation

import (
	"os"
	"strings"
	"testing"
)

// TestFontEmbedSingleSourceLock 是第二份字体嵌入的仓库断言（ADR-0056 §12 / #1105）：
// 估值域字体必须来自 internal/pdfutil 的唯一一份 //go:embed。
func TestFontEmbedSingleSourceLock(t *testing.T) {
	if _, err := os.Stat("fonts"); err == nil {
		t.Fatalf("internal/valuation/fonts/ 必须不存在：字体唯一来源是 internal/pdfutil")
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
		// 只认**行首**的指令形态：域目录里现在有十几个文件，按子串判会让任何一句
		// 「见 //go:embed」的说明文字把这条锁变成假红；真指令必然独占一行。
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//go:embed") {
				t.Errorf("%s 不得声明 //go:embed：字体唯一来源是 internal/pdfutil.EnsureFontLoaded", e.Name())
				break
			}
		}
	}
}
