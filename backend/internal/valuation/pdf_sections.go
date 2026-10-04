// PDF 报告生成（原 internal/valuation/pdf，#1514 波 5 并回域包）。
// 本文件:第 2 页(页眉/页脚/章节标题、基本信息表与状态徽章、价值主卡与 Hero 统计格、
// 置信区间条)。
// 报告总览与入口见 pdf_template.go。
package valuation

import (
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/pdfutil"
)

// 第 2 页:评估基本信息 + 评估结果摘要

func (g *PDFGenerator) renderBasicInfoAndSummary(pdf *gofpdf.Fpdf, r *EvaluationDetail, dimensionScores []DimensionScore) {
	drawPageHeader(pdf, r)

	// 评估基本信息
	infoHeaderY := 35.0
	infoTableY := 45.0
	drawSectionHeader(pdf, "评估基本信息", pageMargin, infoHeaderY)
	infoTableH := drawBasicInfoTable(pdf, r, pageMargin, infoTableY, contentWidth)

	// 评估结果摘要(与上一段表格保持间距,避免标题触及表格)
	summaryY := infoTableY + infoTableH + 6.0
	drawSectionHeader(pdf, "评估结果摘要", pageMargin, summaryY)

	// Hero 卡片
	heroH := 42.0
	drawValueHero(pdf, pageMargin, summaryY+10, contentWidth, heroH, r)

	// 置信区间条
	confY := summaryY + 10 + heroH + 4
	drawConfidenceBar(pdf, pageMargin, confY, contentWidth, r)

	// 雷达图 + 维度评分明细
	radarY := confY + 22
	drawRadarAndDimensions(pdf, pageMargin, radarY, contentWidth, dimensionScores)

	// 页脚
	drawPageFooter(pdf, 2, 3)
}

func drawPageHeader(pdf *gofpdf.Fpdf, r *EvaluationDetail) {
	y := 18.0
	// 左:报告名
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 14)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(pageMargin, y)
	pdf.CellFormat(120, 6, "叉车残值评估报告", "", 0, "L", false, 0, "")
	// 副行
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageMargin, y+6.5)
	pdf.CellFormat(120, 4, fmt.Sprintf("报告编号: EV-%06d  |  生成日期: %s",
		r.ID, time.Now().Format("2006-01-02")), "", 0, "L", false, 0, "")

	// 右:公司名
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageWidth-pageMargin-70, y+3)
	pdf.CellFormat(70, 5, orgName, "", 0, "R", false, 0, "")

	// 底部分隔线
	pdf.SetDrawColor(primary[0], primary[1], primary[2])
	pdf.SetLineWidth(0.6)
	pdf.Line(pageMargin, y+13, pageWidth-pageMargin, y+13)
}

func drawPageFooter(pdf *gofpdf.Fpdf, page, total int) {
	y := pageHeight - pageMargin + 5
	pdf.SetDrawColor(borderLite[0], borderLite[1], borderLite[2])
	pdf.SetLineWidth(0.2)
	pdf.Line(pageMargin, y-5, pageWidth-pageMargin, y-5)
	pdf.SetFont(pdfutil.FontSimHei, "", 8.5)
	pdf.SetTextColor(textPale[0], textPale[1], textPale[2])
	pdf.SetXY(pageMargin, y-3)
	pdf.CellFormat(80, 4, orgName, "", 0, "L", false, 0, "")
	pdf.SetXY(pageWidth-pageMargin-40, y-3)
	pdf.CellFormat(40, 4, fmt.Sprintf("第 %d 页 / 共 %d 页", page, total), "", 0, "R", false, 0, "")
}

func drawSectionHeader(pdf *gofpdf.Fpdf, title string, x, y float64) {
	// 蓝色细竖条
	pdf.SetFillColor(primary[0], primary[1], primary[2])
	pdf.RoundedRect(x, y+1.5, 1.4, 6.5, 0.7, "1234", "F")
	// 标题(与下方表格内容左对齐)
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", h1Pt)
	pdf.SetTextColor(text[0], text[1], text[2])
	pdf.SetXY(x+3.5, y)
	pdf.CellFormat(150, 7, title, "", 0, "L", false, 0, "")
	// 底部分隔线
	pdf.SetDrawColor(border[0], border[1], border[2])
	pdf.SetLineWidth(0.2)
	pdf.Line(x, y+8, x+contentWidth, y+8)
}

// basicInfoRow 评估基本信息表的一行
type basicInfoRow struct {
	label1, value1 string
	label2, value2 string
	badge1, badge2 bool // value 是否渲染为状态徽章
	span1          bool // value1 是否跨 3 列(独占该行)
}

// drawBasicInfoTable 评估基本信息表(2 列布局,共 8 行)
// 展示重构后的全部输入字段（已移除 brand_type）:
//   - 品牌 / 车型
//   - 系列 / 吨位
//   - 配置类型 / 门架类型
//   - 门架高度 / 出厂年份
//   - 成交年份 / 累计使用小时
//   - 原厂漆 / 车况评级
//   - 区域 / 维保记录
//   - 车牌 / 登记证
func drawBasicInfoTable(pdf *gofpdf.Fpdf, r *EvaluationDetail, x, y, w float64) float64 {
	rowH := 6.5
	colLabelW := w * 0.18
	colValueW := w * 0.32
	colTotal := colLabelW + colValueW // 1/2 列宽

	rows := []basicInfoRow{
		{label1: "品牌", value1: defaultIfEmpty(r.Brand, "-"), label2: "车型", value2: defaultIfEmpty(r.VehicleType, "-")},
		{label1: "系列", value1: defaultIfEmpty(r.Series, "-"), label2: "吨位", value2: fmt.Sprintf("%.1f 吨", r.Tonnage)},
		{label1: "配置类型", value1: defaultIfEmpty(r.ConfigType, "-"), label2: "门架类型", value2: defaultIfEmpty(r.MastType, "-")},
		{label1: "门架高度", value1: fmt.Sprintf("%d mm", r.MastHeightMM), label2: "出厂年份", value2: fmt.Sprintf("%d 年", r.FactoryYear)},
		{label1: "成交年份", value1: fmt.Sprintf("%d 年", r.SaleYear), label2: "累计使用小时", value2: fmt.Sprintf("%d 小时", r.UsageHours)},
		{label1: "原厂漆", value1: statusText(r.OriginalPaint), badge1: true, label2: "车况评级", value2: defaultIfEmpty(r.ConditionRating, "-")},
		{label1: "区域", value1: formatRegion(r.Province, r.City), label2: "维保记录", value2: statusText(r.HasMaintenanceRecords), badge2: true},
		{label1: "车牌", value1: statusText(r.HasLicensePlate), badge1: true, label2: "登记证", value2: statusText(r.HasRegistrationCertificate), badge2: true},
	}

	for i, row := range rows {
		ry := y + float64(i)*float64(rowH)
		// 偶数行浅底
		if i%2 == 0 {
			pdf.SetFillColor(bgMuted[0], bgMuted[1], bgMuted[2])
			pdf.Rect(x, ry, w, float64(rowH), "F")
		}
		// 第一组 label
		pdf.SetFont(pdfutil.FontSimHei, "", 10)
		pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
		pdf.SetXY(x+3, ry+1.5)
		pdf.CellFormat(colLabelW-3, float64(rowH)-1.5, row.label1, "", 0, "L", false, 0, "")
		// 第一组 value
		if row.badge1 {
			drawStatusBadge(pdf, x+colLabelW+3, ry+1.5, colValueW-6, row.value1)
		} else {
			pdf.SetFont(pdfutil.FontSimHei, "", 10)
			pdf.SetTextColor(text[0], text[1], text[2])
			pdf.SetXY(x+colLabelW+3, ry+1.5)
			pdf.CellFormat(colValueW-3, float64(rowH)-1.5, row.value1, "", 0, "L", false, 0, "")
		}
		// 第二组(非跨列)
		if !row.span1 {
			pdf.SetFont(pdfutil.FontSimHei, "", 10)
			pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
			pdf.SetXY(x+colTotal+3, ry+1.5)
			pdf.CellFormat(colLabelW-3, float64(rowH)-1.5, row.label2, "", 0, "L", false, 0, "")
			if row.badge2 {
				drawStatusBadge(pdf, x+colTotal+colLabelW+3, ry+1.5, colValueW-6, row.value2)
			} else {
				pdf.SetFont(pdfutil.FontSimHei, "", 10)
				pdf.SetTextColor(text[0], text[1], text[2])
				pdf.SetXY(x+colTotal+colLabelW+3, ry+1.5)
				pdf.CellFormat(colValueW-3, float64(rowH)-1.5, row.value2, "", 0, "L", false, 0, "")
			}
		}
	}
	// 表格外框 + 内部网格
	pdf.SetDrawColor(border[0], border[1], border[2])
	pdf.SetLineWidth(0.2)
	totalH := float64(rowH) * float64(len(rows))
	pdf.Rect(x, y, w, totalH, "D")
	// 列分隔(只画两条,把表分 4 列)
	pdf.Line(x+colTotal, y, x+colTotal, y+totalH)
	// 行分隔
	for i := 1; i < len(rows); i++ {
		pdf.Line(x, y+float64(i)*float64(rowH), x+w, y+float64(i)*float64(rowH))
	}
	return totalH
}

// formatRegion 拼接省份与城市(空值降级为 -)
func formatRegion(province, city string) string {
	if province == "" && city == "" {
		return "-"
	}
	if province == "" {
		return city
	}
	if city == "" {
		return province
	}
	return province + " " + city
}

// statusText 布尔 → 状态文本
func statusText(ok bool) string {
	if ok {
		return "正常"
	}
	return "异常"
}

// drawStatusBadge 渲染状态徽章(圆点 + 文字 + 浅色背景)
func drawStatusBadge(pdf *gofpdf.Fpdf, x, y, w float64, text string) {
	var c, bg rgb
	switch text {
	case "正常":
		c, bg = success, successBg
	case "异常":
		c, bg = errColor, errBg
	case "轻微磨损":
		c, bg = info, infoBg
	case "需维修":
		c, bg = warningDk, warningBg
	case "需更换":
		c, bg = errColor, errBg
	default:
		c, bg = textMuted, bgMuted
	}

	h := 5.0
	// 背景圆角矩形
	pdf.SetFillColor(bg[0], bg[1], bg[2])
	pdf.RoundedRect(x, y, w, h, 2.5, "1234", "F")
	// 圆点
	pdf.SetFillColor(c[0], c[1], c[2])
	pdf.Circle(x+4, y+h/2, 0.9, "F")
	// 文字
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	pdf.SetTextColor(c[0], c[1], c[2])
	pdf.SetXY(x+8, y+1)
	pdf.CellFormat(w-10, h-1, text, "", 0, "L", false, 0, "")
}

// drawValueHero 渲染蓝色 Hero 卡片,显示估算残值
func drawValueHero(pdf *gofpdf.Fpdf, x, y, w, h float64, r *EvaluationDetail) {
	// 渐变背景(深蓝 → 中蓝)
	pdf.LinearGradient(x, y, w, h,
		primaryDk[0], primaryDk[1], primaryDk[2],
		primaryMid[0], primaryMid[1], primaryMid[2],
		0, 0, 1, 0)
	// 圆角遮罩:叠加同色覆盖以让边缘看起来圆润(gofpdf 渐变无圆角,这里用半透明覆盖)
	pdf.SetAlpha(1, "Normal")
	// 装饰圆(右上,使用白色低透明)
	pdf.SetAlpha(0.06, "Normal")
	pdf.SetFillColor(255, 255, 255)
	pdf.Circle(x+w-12, y+8, 18, "F")
	pdf.SetAlpha(0.04, "Normal")
	pdf.Circle(x+w-55, y+h+2, 22, "F")
	pdf.SetAlpha(1, "Normal")

	// 标签
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetAlpha(0.7, "Normal")
	pdf.SetXY(x+6, y+4)
	pdf.CellFormat(120, 4, "估算残值  RESIDUAL VALUE", "", 0, "L", false, 0, "")
	pdf.SetAlpha(1, "Normal")

	// 大数字行(￥ + 数值 + 万元)
	pdf.SetFont(pdfutil.FontSimHei, "", 14)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x+6, y+11)
	pdf.CellFormat(7, 11, "￥", "", 0, "L", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", heroValuePt)
	pdf.SetXY(x+12, y+9)
	pdf.CellFormat(60, 14, fmt.Sprintf("%.2f", yuanToWan(r.EstimatedValue)), "", 0, "L", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHei, "", 14)
	pdf.SetXY(x+72, y+18)
	pdf.CellFormat(20, 7, "万元", "", 0, "L", false, 0, "")

	// 底部 3 项统计
	rate := 0.0
	if r.OriginalPrice > 0 {
		rate = r.EstimatedValue / r.OriginalPrice * 100
	}
	grade, _ := gradeFromRate(rate)
	statY := y + 27
	drawHeroStat(pdf, x+6, statY, 55, "残值率", fmt.Sprintf("%.1f%%", rate))
	drawHeroStat(pdf, x+62, statY, 70, "置信区间 (95%)",
		fmt.Sprintf("%.2f ~ %.2f 万元", yuanToWan(r.ConfidenceLow), yuanToWan(r.ConfidenceHigh)))
	drawHeroStat(pdf, x+133, statY, 45, "综合等级", fmt.Sprintf("%s级(%s)", grade.cn, grade.letter))
}

func drawHeroStat(pdf *gofpdf.Fpdf, x, y, w float64, label, value string) {
	pdf.SetFont(pdfutil.FontSimHei, "", 8.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetAlpha(0.55, "Normal")
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 3.5, label, "", 0, "L", false, 0, "")
	pdf.SetAlpha(1, "Normal")
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 12)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x, y+4)
	pdf.CellFormat(w, 5, value, "", 0, "L", false, 0, "")
}

// gradeInfo 等级信息
type gradeInfo struct {
	cn, letter, desc string
}

// gradeFromRate 根据残值率(百分比)计算等级
func gradeFromRate(rate float64) (gradeInfo, rgb) {
	switch {
	case rate >= 70:
		return gradeInfo{"优", "A", "车况良好,保值率高,建议正常出售"}, gradeA
	case rate >= 50:
		return gradeInfo{"良", "B", "车况尚可,保值率中等,建议适当整备后出售"}, gradeB
	case rate >= 30:
		return gradeInfo{"中", "C", "车况一般,保值率偏低,建议维修后出售或折价处理"}, gradeC
	default:
		return gradeInfo{"差", "D", "车况较差,保值率低,建议拆件出售或作为配件使用"}, gradeD
	}
}

// drawConfidenceBar 置信区间可视化
func drawConfidenceBar(pdf *gofpdf.Fpdf, x, y, w float64, r *EvaluationDetail) {
	// 文字行
	pdf.SetFont(pdfutil.FontSimHei, "", 9.5)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(40, 4, "置信区间分布", "", 0, "L", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHei, "", 9)
	pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
	pdf.SetXY(x+w-110, y)
	pdf.CellFormat(110, 4, fmt.Sprintf("%.2f 万元  ←  %.2f 万元  →  %.2f 万元",
		yuanToWan(r.ConfidenceLow), yuanToWan(r.EstimatedValue), yuanToWan(r.ConfidenceHigh)), "", 0, "R", false, 0, "")

	// 背景条
	barY := y + 6
	barH := 2.5
	pdf.SetFillColor(border[0], border[1], border[2])
	pdf.RoundedRect(x, barY, w, barH, 1.2, "1234", "F")
	// 填充(70% 宽,居中,橙→绿渐变)
	fillW := w * 0.70
	fillX := x + (w-fillW)/2
	pdf.LinearGradient(fillX, barY, fillW, barH,
		warning[0], warning[1], warning[2],
		success[0], success[1], success[2],
		0, 0, 1, 0)

	// 下方标签
	labelY := barY + barH + 1.5
	pdf.SetFont(pdfutil.FontSimHei, "", 8.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(x, labelY)
	pdf.CellFormat(40, 3, "较低估值", "", 0, "L", false, 0, "")
	pdf.SetXY(x+w/2-15, labelY)
	pdf.CellFormat(30, 3, "最佳估值", "", 0, "C", false, 0, "")
	pdf.SetXY(x+w-40, labelY)
	pdf.CellFormat(40, 3, "较高估值", "", 0, "R", false, 0, "")
}
