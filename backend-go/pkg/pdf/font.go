// Package pdf 实现 PDF 报告生成
// 本文件：字体加载器，通过 //go:embed 将 simhei.ttf 编译进二进制，
// 消除运行时对文件系统的依赖。
package pdf

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/jung-kurt/gofpdf"
)

// 字体常量
const (
	// FontSimHei 黑体（用于正文，支持中文）
	FontSimHei = "simhei"
	// FontSimHeiBold 黑体（用于标题，加粗样式）
	FontSimHeiBold = "simhei_b"
)

//go:embed fonts/simhei.ttf
var embeddedFont []byte

var (
	// regOnce 保证字体字节仅解析一次
	regOnce sync.Once
	// regErr 缓存解析过程中的错误
	regErr error
)

// ensureFontLoaded 确保字体已注册到给定的 Fpdf 实例。
// 字体字节通过 //go:embed 编译进二进制，无需运行时读盘。
func ensureFontLoaded(pdf *gofpdf.Fpdf) error {
	regOnce.Do(func() {
		if len(embeddedFont) == 0 {
			regErr = fmt.Errorf("内嵌字体字节为空")
		}
	})
	if regErr != nil {
		return fmt.Errorf("字体加载失败: %w", regErr)
	}

	// gofpdf 的字体注册是 per-instance 的，每个 Fpdf 实例需重新注册
	pdf.AddUTF8FontFromBytes(FontSimHei, "", embeddedFont)
	pdf.AddUTF8FontFromBytes(FontSimHeiBold, "B", embeddedFont)
	return nil
}

// fontFileName 返回内嵌字体的建议文件名（供调试/日志使用）
func fontFileName() string {
	return filepath.Base("fonts/simhei.ttf")
}
