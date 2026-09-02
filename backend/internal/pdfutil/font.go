// Package pdfutil 共享 PDF 能力：内嵌中文字体加载与通用排版常量。
// 从残值域 PDF 绘制资产提取（spec #484 决定：复用并提取为共享能力），
// 培训域招聘侧在线简历渲染与残值域评估报告共用同一字体资源，消除重复内嵌。
package pdfutil

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/jung-kurt/gofpdf"
)

// 字体常量（与残值域 pdf.FontSimHei 同名同值，供两个域共用）
const (
	// FontSimHei 黑体（正文，支持中文）
	FontSimHei = "simhei"
	// FontSimHeiBold 黑体（标题，加粗样式）
	FontSimHeiBold = "simhei_b"
)

//go:embed fonts/simhei.ttf
var embeddedFont []byte

var (
	// regOnce 保证字体字节仅解析一次（gofpdf 的注册是 per-instance 的）
	regOnce sync.Once
	// regErr 缓存解析过程中的错误
	regErr error
)

// EnsureFontLoaded 确保字体已注册到给定的 Fpdf 实例（每个 Fpdf 实例需重新注册）。
// 字体字节通过 //go:embed 编译进二进制，无需运行时读盘。
func EnsureFontLoaded(pdf *gofpdf.Fpdf) error {
	regOnce.Do(func() {
		if len(embeddedFont) == 0 {
			regErr = fmt.Errorf("内嵌字体字节为空")
		}
	})
	if regErr != nil {
		return fmt.Errorf("字体加载失败: %w", regErr)
	}
	pdf.AddUTF8FontFromBytes(FontSimHei, "", embeddedFont)
	pdf.AddUTF8FontFromBytes(FontSimHeiBold, "B", embeddedFont)
	return nil
}

// A4 排版常量（与残值域 pdf 包一致，招聘侧在线简历共用同一套版式基线）
const (
	PageWidthMm  = 210.0
	PageHeightMm = 297.0
	PageMarginMm = 15.0
)
