// PDF 报告生成（原 internal/valuation/pdf，#1514 波 5 并回域包）。
// 本文件:第 3 页(维度进度条、系数说明、评级卡、处置建议、风险提示、免责声明)
// 与文本小助手(yuanToWan / defaultIfEmpty)。
// 报告总览与入口见 pdf_template.go。
package valuation

import (
	"fmt"
	"math"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/pdfutil"
)

// drawRadarAndDimensions 雷达图 + 维度进度条(并排)
func drawRadarAndDimensions(pdf *gofpdf.Fpdf, x, y, w float64, dimensionScores []DimensionScore) {
	// 维度分 → 按标签取值（顺序固定为 DimensionLabels）
	scoreByLabel := dimensionScoreByLabel(dimensionScores)
	// 左侧雷达图
	radarW := 70.0
	dimX := x + radarW + 4
	dimW := w - radarW - 4

	// 标题
	pdf.SetFont(pdfutil.FontSimHei, "", 10.5)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(radarW, 4, "综合评分雷达图", "", 0, "L", false, 0, "")
	pdf.SetXY(dimX, y)
	pdf.CellFormat(dimW, 4, "维度评分明细", "", 0, "L", false, 0, "")

	// 雷达图(整体下移,避免标题与标签重合)
	if len(dimensionScores) > 0 {
		rcx := x + radarW/2
		rcy := y + 44
		drawRadarChart(pdf, rcx, rcy, 18, dimensionScores)
	} else {
		pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
		pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
		pdf.SetXY(x+radarW/2-15, y+30)
		pdf.CellFormat(30, 4, "(无数据)", "", 0, "C", false, 0, "")
	}

	// 维度进度条
	dimY0 := y + 8
	rowH := 6.5
	barH := 1.8
	labelW := 22.0
	valW := 14.0
	barW := dimW - labelW - valW

	for i, dim := range DimensionLabels {
		v := scoreByLabel[dim]
		rowY := dimY0 + float64(i)*rowH
		// 维度名
		pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
		pdf.SetTextColor(textSub[0], textSub[1], textSub[2])
		pdf.SetXY(dimX, rowY+0.5)
		pdf.CellFormat(labelW, 3, dim, "", 0, "L", false, 0, "")
		// 进度条背景
		barX := dimX + labelW
		barY := rowY + 0.9
		pdf.SetFillColor(border[0], border[1], border[2])
		pdf.RoundedRect(barX, barY, barW, barH, 0.9, "1234", "F")
		// 进度条填充(以雷达图满刻度 1.3 为基准归一化)
		fillRatio := v / radarMaxValue
		if fillRatio > 1.0 {
			fillRatio = 1.0
		}
		if fillRatio < 0 {
			fillRatio = 0
		}
		fillW := barW * fillRatio
		fillColor := dimensionBarColor(v)
		if fillW > 0.5 {
			pdf.SetFillColor(fillColor[0], fillColor[1], fillColor[2])
			pdf.RoundedRect(barX, barY, fillW, barH, 0.9, "1234", "F")
		}
		// 数值
		pdf.SetFont(pdfutil.FontSimHeiBold, "B", 9.5)
		pdf.SetTextColor(fillColor[0], fillColor[1], fillColor[2])
		pdf.SetXY(dimX+labelW+barW+1, rowY+0.5)
		pdf.CellFormat(valW, 3, fmt.Sprintf("%.2f", v), "", 0, "R", false, 0, "")
	}
}

// dimensionBarColor 根据值返回进度条颜色
func dimensionBarColor(v float64) rgb {
	switch {
	case v >= 1.0:
		return success
	case v >= 0.7:
		return primary
	case v >= 0.4:
		return info
	default:
		return warningDk
	}
}

// 第 3 页:评估结论 + 处置建议 + 风险提示 + 免责声明

// renderCoefficientsAndConclusion 渲染第 3 页
// 顺序:评估结论 → 处置建议 → 风险提示 → 免责声明
func (g *PDFGenerator) renderCoefficientsAndConclusion(pdf *gofpdf.Fpdf, r *EvaluationDetail, suggestions []string) {
	drawPageHeader(pdf, r)

	// 评估结论:车况评级 + 估算残值双卡
	conclusionY := 38.0
	drawSectionHeader(pdf, "评估结论", pageMargin, conclusionY)
	rate := 0.0
	if r.OriginalPrice > 0 {
		rate = r.EstimatedValue / r.OriginalPrice * 100
	}
	grade, _ := gradeFromRate(rate)
	cardY := conclusionY + 10
	drawGradeCards(pdf, pageMargin, cardY, contentWidth, grade, rate, r.EstimatedValue)

	// 处置建议
	suggs := suggestions
	if len(suggs) == 0 {
		suggs = []string{"暂无建议"}
	}
	if len(suggs) > 6 {
		suggs = suggs[:6]
	}
	suggY := cardY + 30
	suggH := drawRecommendations(pdf, pageMargin, suggY, contentWidth, suggs)

	// 风险提示
	riskY := suggY + suggH + 6
	riskH := drawRiskWarnings(pdf, pageMargin, riskY, contentWidth)

	// 免责声明
	disY := riskY + riskH + 8
	drawDisclaimer(pdf, pageMargin, disY, contentWidth)

	// 页脚
	drawPageFooter(pdf, 3, 3)
}

func drawGradeCards(pdf *gofpdf.Fpdf, x, y, w float64, grade gradeInfo, _, estValue float64) {
	cardH := 24.0
	gap := 4.0
	cardW := (w - gap) * 0.30
	estW := w - cardW - gap

	// 等级卡(蓝色)
	pdf.SetFillColor(bgPrimary[0], bgPrimary[1], bgPrimary[2])
	pdf.SetDrawColor(bgPrimary2[0], bgPrimary2[1], bgPrimary2[2])
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(x, y, cardW, cardH, 2, "1234", "FD")
	pdf.SetFont(pdfutil.FontSimHei, "", 9)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y+3)
	pdf.CellFormat(cardW, 4, "综合等级评定", "", 0, "C", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 22)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(x, y+7)
	pdf.CellFormat(cardW, 10, grade.letter, "", 0, "C", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHei, "", 10)
	pdf.SetTextColor(primaryLite[0], primaryLite[1], primaryLite[2])
	pdf.SetXY(x, y+18)
	pdf.CellFormat(cardW, 4, grade.cn, "", 0, "C", false, 0, "")

	// 估算残值卡(红色)
	ex := x + cardW + gap
	pdf.SetFillColor(errBg[0], errBg[1], errBg[2])
	pdf.SetDrawColor(errBord[0], errBord[1], errBord[2])
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(ex, y, estW, cardH, 2, "1234", "FD")
	pdf.SetFont(pdfutil.FontSimHei, "", 9)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(ex, y+3)
	pdf.CellFormat(estW, 4, "估算残值", "", 0, "C", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 22)
	pdf.SetTextColor(errColor[0], errColor[1], errColor[2])
	pdf.SetXY(ex, y+7)
	pdf.CellFormat(estW, 10, fmt.Sprintf("%.2f 万元", yuanToWan(estValue)), "", 0, "C", false, 0, "")
}

func drawRecommendations(pdf *gofpdf.Fpdf, x, y, w float64, suggs []string) float64 {
	// 标题
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 11)
	pdf.SetTextColor(textSub[0], textSub[1], textSub[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(80, 5, "处置建议", "", 0, "L", false, 0, "")

	// 列表(根据文字长度动态计算每行高度,避免换行重叠)
	rowY := y + 7
	pdf.SetFont(pdfutil.FontSimHei, "", 10)
	availW := w - 7
	for i, s := range suggs {
		sw := pdf.GetStringWidth(s)
		lines := math.Ceil(sw / availW)
		if lines < 1 {
			lines = 1
		}
		itemH := 4*lines + 2
		// 编号
		pdf.SetFont(pdfutil.FontSimHeiBold, "B", 10.5)
		pdf.SetTextColor(primary[0], primary[1], primary[2])
		pdf.SetXY(x, rowY)
		pdf.CellFormat(6, itemH, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
		// 文字
		pdf.SetFont(pdfutil.FontSimHei, "", 10)
		pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
		pdf.SetXY(x+7, rowY)
		pdf.MultiCell(availW, 4, s, "", "L", false)
		rowY += itemH
	}
	return rowY - y
}

func drawRiskWarnings(pdf *gofpdf.Fpdf, x, y, w float64) float64 {
	// 标题
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 11)
	pdf.SetTextColor(errColor[0], errColor[1], errColor[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(80, 5, "风险提示", "", 0, "L", false, 0, "")

	risks := []string{
		"本报告基于当前采集的设备信息及市场数据生成,实际交易价格可能因地域差异、市场供需波动、设备实际状况等因素产生偏差。",
		"评估结果仅供参考,不构成任何形式的价格承诺或担保,买卖双方应结合实地验车结果进行最终定价。",
		"本报告有效期为自生成之日起 6 个月,逾期需重新评估。",
	}

	// 根据文字折行情况动态计算警示卡高度
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	availTextW := w - 13
	itemHs := make([]float64, len(risks))
	totalTextH := 0.0
	for i, r := range risks {
		rw := pdf.GetStringWidth(r)
		lines := math.Ceil(rw / availTextW)
		if lines < 1 {
			lines = 1
		}
		itemHs[i] = 4*lines + 1.5
		totalTextH += itemHs[i]
	}

	// 黄色警示卡
	cardY := y + 7
	cardH := totalTextH + 4
	pdf.SetFillColor(warningBg[0], warningBg[1], warningBg[2])
	pdf.SetDrawColor(warningBord[0], warningBord[1], warningBord[2])
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(x, cardY, w, cardH, 1.5, "1234", "FD")
	rowY := cardY + 2
	for i, r := range risks {
		pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
		pdf.SetTextColor(warningText[0], warningText[1], warningText[2])
		pdf.SetXY(x+4, rowY)
		pdf.CellFormat(4, 4, "-", "", 0, "L", false, 0, "")
		pdf.SetXY(x+9, rowY)
		pdf.MultiCell(availTextW, 4, r, "", "L", false)
		rowY += itemHs[i]
	}
	return (cardY + cardH) - y
}

func drawDisclaimer(pdf *gofpdf.Fpdf, x, y, w float64) {
	pdf.SetDrawColor(border[0], border[1], border[2])
	pdf.SetLineWidth(0.2)
	pdf.Line(x, y, x+w, y)
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 10)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y+3)
	pdf.CellFormat(80, 4, "免责声明", "", 0, "L", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHei, "", 9)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(x, y+8)
	pdf.MultiCell(w, 3.5,
		"本评估报告由"+orgName+"的叉车残值评估系统自动生成。报告中所有数据及结论均基于系统算法模型、历史市场数据及用户输入信息综合计算得出。本报告不构成任何投资、交易或法律建议,评估方不对因使用本报告而造成的任何直接或间接损失承担法律责任。报告使用者应结合专业人员的实地检测意见做出独立判断。未经本公司书面许可,不得将本报告用于商业宣传或作为法律依据。",
		"", "L", false)
}

// 工具函数

func defaultIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// yuanToWan 元 → 万元（后端所有金额字段以元存储，展示时需转为万元）
func yuanToWan(v float64) float64 {
	return v / 10000
}
