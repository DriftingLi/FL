package pdfutil

import (
	"bytes"
	"testing"
)

// TestEmbeddedFontIsCompiledIn 锁定共享字体确实通过 //go:embed 编译进二进制。
// 该断言原先在 valuation/pdf 包内（随该包第二份嵌入删除而迁到字体唯一来源处，
// ADR-0056 §12 / #1105）：字体只有一份，因此校验也只有一处。
func TestEmbeddedFontIsCompiledIn(t *testing.T) {
	if len(embeddedFont) == 0 {
		t.Fatal("内嵌字体字节为空，//go:embed 未生效")
	}
	if len(embeddedFont) < 1_000_000 {
		t.Errorf("内嵌字体过小 (%d 字节)，可能不是完整的 TTF 文件", len(embeddedFont))
	}
	if !(bytes.HasPrefix(embeddedFont, []byte{0x00, 0x01, 0x00, 0x00}) ||
		bytes.HasPrefix(embeddedFont, []byte("true")) ||
		bytes.HasPrefix(embeddedFont, []byte("ttcf"))) {
		t.Errorf("内嵌字体不是合法 TTF 头: % x", embeddedFont[:4])
	}
	t.Logf("内嵌字体大小: %d 字节", len(embeddedFont))
}
