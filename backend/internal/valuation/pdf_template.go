// PDF 报告生成（原 internal/valuation/pdf，#1514 波 5 并回域包）。
// 本文件:样式常量(颜色/字号/栅格)与报告生成入口 GenerateReport。
// 分部:封面 pdf_cover.go、页眉页脚与第 2 页各章节块 pdf_sections.go、
// 第 3 页结论与建议 pdf_conclusion.go、雷达图矢量绘制 pdf_radar.go、
// 电池报告 pdf_battery_template.go。
// 报告版式:3 页 A4。
//   - 第 1 页:封面(渐变蓝带 + Logo + 报告元信息卡片)
//   - 第 2 页:评估基本信息 + 评估结果摘要(Hero 卡片 + 置信区间 + 雷达图/维度进度条)
//   - 第 3 页:评估结论 + 处置建议 + 风险提示 + 免责声明
//
// 设计稿: .trae/design-exports/简洁版评估报告.html

package valuation

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/pdfutil"
)

// A4 排版常量
const (
	pageWidth    = 210.0
	pageHeight   = 297.0
	pageMargin   = 15.0
	contentWidth = pageWidth - 2*pageMargin // 180mm
)

// 排版尺寸 (mm / pt)
const (
	bodySizePt  = 9.0
	h1Pt        = 13.0 // 段落标题
	heroValuePt = 36.0 // Hero 卡片中的大数字
)

// 报告主体信息
const (
	orgName       = "和润天下人工智能科技有限公司"
	orgNameEN     = "HERUN TIANXIA AI TECHNOLOGY CO., LTD."
	reportTitleEN = "Forklift Residual Value Evaluation Report"
)

// rgb 设计色板三元组
type rgb [3]int

// 设计色板 (与设计稿 :root CSS 变量对齐)
var (
	// 主色 - 深蓝
	primary     = rgb{30, 64, 175}  // #1E40AF
	primaryLite = rgb{59, 130, 246} // #3B82F6
	primaryDk   = rgb{30, 58, 138}  // #1E3A8A
	primaryMid  = rgb{37, 99, 235}  // #2563EB

	// 中性文字
	text      = rgb{15, 23, 42}    // #0F172A
	textSub   = rgb{51, 65, 85}    // #334155
	textMuted = rgb{71, 85, 105}   // #475569
	textLabel = rgb{100, 116, 139} // #64748B
	textLite  = rgb{148, 163, 184} // #94A3B8
	textPale  = rgb{203, 213, 225} // #CBD5E1

	// 背景与边框
	bgMuted    = rgb{248, 250, 252} // #F8FAFC
	bgPrimary  = rgb{239, 246, 255} // #EFF6FF
	bgPrimary2 = rgb{219, 234, 254} // #DBEAFE
	border     = rgb{226, 232, 240} // #E2E8F0
	borderLite = rgb{241, 245, 249} // #F1F5F9

	// 语义色
	success     = rgb{22, 163, 74}   // #16A34A
	successBg   = rgb{240, 253, 244} // #F0FDF4
	info        = rgb{14, 165, 233}  // #0EA5E9
	infoBg      = rgb{240, 249, 255} // #F0F9FF
	warning     = rgb{245, 158, 11}  // #F59E0B
	warningDk   = rgb{217, 119, 6}   // #D97706
	warningBg   = rgb{255, 251, 235} // #FFFBEB
	warningBord = rgb{253, 230, 138} // #FDE68A
	warningText = rgb{146, 64, 14}   // #92400E
	errColor    = rgb{220, 38, 38}   // #DC2626
	errBg       = rgb{254, 242, 242} // #FEF2F2
	errBord     = rgb{254, 202, 202} // #FECACA

	// 等级颜色
	gradeA = rgb{22, 163, 74}
	gradeB = rgb{59, 130, 246}
	gradeC = rgb{245, 158, 11}
	gradeD = rgb{220, 38, 38}
)

// PDFGenerator 报告生成器（无状态：所有入口均返回 PDF 字节，不落盘）。
type PDFGenerator struct{}

// NewPDFGenerator 构造报告生成器。
func NewPDFGenerator() *PDFGenerator {
	return &PDFGenerator{}
}

// GenerateReport 生成 3 页简洁版评估报告 PDF，返回 PDF 二进制内容。
// 入参 r 含评估详情(含输入字段与计算结果);dimensionScores 为 5 维评分;suggestions 为处置建议文本列表。
func (g *PDFGenerator) GenerateReport(r *EvaluationDetail, dimensionScores []DimensionScore, suggestions []string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	// 关闭自动分页,由 3 个 render 方法自行控制 AddPage
	pdf.SetAutoPageBreak(false, pageMargin)
	if err := pdfutil.EnsureFontLoaded(pdf); err != nil {
		return nil, err
	}

	// 第 1 页:封面
	pdf.AddPage()
	g.renderCover(pdf, r)

	// 第 2 页:评估基本信息 + 评估结果摘要
	pdf.AddPage()
	g.renderBasicInfoAndSummary(pdf, r, dimensionScores)

	// 第 3 页:计算系数 + 车况评级 + 处置建议
	pdf.AddPage()
	g.renderCoefficientsAndConclusion(pdf, r, suggestions)

	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, fmt.Errorf("生成 PDF 失败: %w", err)
	}
	return buf.Bytes(), nil
}
