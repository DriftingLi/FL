// Package pdf 实现 PDF 报告生成
// 本文件:简洁版叉车残值评估报告,3 页 A4 布局。
//   - 第 1 页:封面(渐变蓝带 + Logo + 报告元信息卡片)
//   - 第 2 页:评估基本信息 + 评估结果摘要(Hero 卡片 + 置信区间 + 雷达图/维度进度条)
//   - 第 3 页:评估结论 + 处置建议 + 风险提示 + 免责声明
//
// 设计稿: .trae/design-exports/简洁版评估报告.html
package pdf

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/valuation/model"
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

// Generator 报告生成器
type Generator struct {
	outputDir string
}

// NewGenerator 构造报告生成器
func NewGenerator(outputDir string) *Generator {
	return &Generator{outputDir: outputDir}
}

// GenerateReport 生成 3 页简洁版评估报告 PDF。
// 入参 r 含评估详情(含输入字段与计算结果);dimensionScores 为 5 维评分;suggestions 为处置建议文本列表。
func (g *Generator) GenerateReport(r *model.EvaluationDetail, dimensionScores map[string]float64, suggestions []string) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	// 关闭自动分页,由 3 个 render 方法自行控制 AddPage
	pdf.SetAutoPageBreak(false, pageMargin)
	if err := ensureFontLoaded(pdf); err != nil {
		return "", err
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

	filename := fmt.Sprintf("evaluation_report_%d_%s.pdf",
		r.ID, time.Now().Format("20060102150405"))
	outputPath := joinPath(g.outputDir, filename)
	if g.outputDir != "" {
		if err := os.MkdirAll(g.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("创建 PDF 输出目录失败: %w", err)
		}
	}
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return "", fmt.Errorf("保存 PDF 失败: %w", err)
	}
	return outputPath, nil
}

// joinPath 拼接输出路径
func joinPath(dir, filename string) string {
	if dir == "" {
		return filename
	}
	last := dir[len(dir)-1]
	if last == '/' || last == '\\' {
		return dir + filename
	}
	return dir + "/" + filename
}

// =====================================================
// 第 1 页:封面
// =====================================================

func (g *Generator) renderCover(pdf *gofpdf.Fpdf, r *model.EvaluationDetail) {
	// 顶部蓝色渐变条(5mm)
	drawHGradientBar(pdf, 0, 0, pageWidth, 5, primary, primaryLite, primary)

	// Logo 区域
	logoSize := 22.0
	cx := pageWidth / 2
	logoY := 40.0
	pdf.SetFillColor(primary[0], primary[1], primary[2])
	pdf.RoundedRect(cx-logoSize/2, logoY, logoSize, logoSize, 3.5, "1234", "F")
	drawForkliftIcon(pdf, cx, logoY+logoSize/2)

	// 公司中文名
	pdf.SetFont(FontSimHeiBold, "B", 15)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(pageMargin, logoY+logoSize+4)
	pdf.CellFormat(contentWidth, 6, orgName, "", 1, "C", false, 0, "")
	// 公司英文名
	pdf.SetFont(FontSimHei, "", 10.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageMargin, logoY+logoSize+12)
	pdf.CellFormat(contentWidth, 5, orgNameEN, "", 1, "C", false, 0, "")

	// 装饰线
	dividerY := 95.0
	drawCenterFadeBar(pdf, cx, dividerY, 30, 0.6, primary)

	// 大标题
	pdf.SetFont(FontSimHeiBold, "B", 28)
	pdf.SetTextColor(text[0], text[1], text[2])
	pdf.SetXY(pageMargin, dividerY+10)
	pdf.CellFormat(contentWidth, 14, "叉车残值评估报告", "", 1, "C", false, 0, "")
	// 英文副标题
	pdf.SetFont(FontSimHei, "", 12)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(pageMargin, dividerY+26)
	pdf.CellFormat(contentWidth, 6, reportTitleEN, "", 1, "C", false, 0, "")

	// 报告元信息卡片
	metaW := 108.0
	metaH := 58.0
	metaX := (pageWidth - metaW) / 2
	metaY := 172.0
	pdf.SetFillColor(bgMuted[0], bgMuted[1], bgMuted[2])
	pdf.SetDrawColor(border[0], border[1], border[2])
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(metaX, metaY, metaW, metaH, 3, "1234", "FD")

	// 三行
	drawCoverMetaRow(pdf, metaX+10, metaY+9, metaW-20, "报告编号",
		fmt.Sprintf("EV-%06d", r.ID), true, text, textLabel)
	drawCoverMetaRow(pdf, metaX+10, metaY+26, metaW-20, "生成日期",
		time.Now().Format("2006-01-02"), false, text, textLabel)
	drawCoverMetaRow(pdf, metaX+10, metaY+43, metaW-20, "叉车类型",
		coverVehicleLabel(r), false, primary, textLabel)

	// 装饰线
	drawCenterFadeBar(pdf, cx, 244, 30, 0.6, primary)

	// 底部提示
	pdf.SetFont(FontSimHei, "", 10.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageMargin, 250)
	pdf.CellFormat(contentWidth, 5, "本报告由系统自动生成,仅供参考", "", 1, "C", false, 0, "")
	pdf.SetXY(pageMargin, 257)
	pdf.CellFormat(contentWidth, 5, "本报告有效期为自生成之日起 6 个月", "", 1, "C", false, 0, "")

	// 底部蓝色渐变条(3mm)
	drawHGradientBar(pdf, 0, pageHeight-3, pageWidth, 3, primary, primaryLite, primary)
}

// coverVehicleLabel 封面"叉车类型"展示:车型 + 品牌(取品牌短名)
// 例如:"电动叉车 / 合力 (HELI)"
func coverVehicleLabel(r *model.EvaluationDetail) string {
	if r == nil {
		return "-"
	}
	if r.VehicleType == "" && r.Brand == "" {
		return "-"
	}
	if r.Brand == "" {
		return r.VehicleType
	}
	if r.VehicleType == "" {
		return r.Brand
	}
	return r.VehicleType + " / " + r.Brand
}

func drawCoverMetaRow(pdf *gofpdf.Fpdf, x, y, w float64, label, value string, valueBold bool, vc, lc rgb) {
	gap := 3.0
	pdf.SetFont(FontSimHei, "", 11.5)
	labelW := pdf.GetStringWidth(label)
	if valueBold {
		pdf.SetFont(FontSimHeiBold, "B", 13)
	} else {
		pdf.SetFont(FontSimHei, "", 13)
	}
	valueW := pdf.GetStringWidth(value)
	totalW := labelW + gap + valueW
	startX := x
	if totalW < w {
		startX = x + (w-totalW)/2
	}

	pdf.SetFont(FontSimHei, "", 11.5)
	pdf.SetTextColor(lc[0], lc[1], lc[2])
	pdf.SetXY(startX, y)
	pdf.CellFormat(labelW, 6, label, "", 0, "L", false, 0, "")
	if valueBold {
		pdf.SetFont(FontSimHeiBold, "B", 13)
	} else {
		pdf.SetFont(FontSimHei, "", 13)
	}
	pdf.SetTextColor(vc[0], vc[1], vc[2])
	pdf.SetXY(startX+labelW+gap, y)
	pdf.CellFormat(valueW, 6, value, "", 0, "L", false, 0, "")
}

// drawForkliftIcon 在中心 (cx, cy) 处绘制简化的叉车图标
func drawForkliftIcon(pdf *gofpdf.Fpdf, cx, cy float64) {
	pdf.SetFillColor(255, 255, 255)
	w := 12.0
	// 货箱(底层,大矩形)
	pdf.RoundedRect(cx-w/2, cy+1, w, 3, 0.6, "1234", "F")
	// 主体(中层)
	pdf.RoundedRect(cx-w*0.38, cy-2, w*0.76, 3, 0.6, "1234", "F")
	// 顶(上层)
	pdf.RoundedRect(cx-w*0.28, cy-5, w*0.56, 3, 0.6, "1234", "F")
	// 轮子
	pdf.SetFillColor(primary[0], primary[1], primary[2])
	pdf.Circle(cx-w*0.34, cy+4.5, 0.9, "F")
	pdf.Circle(cx+w*0.34, cy+4.5, 0.9, "F")
}

// drawHGradientBar 水平三色渐变条(左 → 中 → 右)
func drawHGradientBar(pdf *gofpdf.Fpdf, x, y, w, h float64, left, mid, right rgb) {
	// gofpdf 的 LinearGradient 仅支持两色,用 2 段拼接模拟三色
	half := w / 2
	pdf.LinearGradient(x, y, half, h,
		left[0], left[1], left[2],
		mid[0], mid[1], mid[2],
		0, 0, 1, 0)
	pdf.LinearGradient(x+half, y, w-half, h,
		mid[0], mid[1], mid[2],
		right[0], right[1], right[2],
		0, 0, 1, 0)
}

// drawCenterFadeBar 居中淡出装饰线
func drawCenterFadeBar(pdf *gofpdf.Fpdf, cx, y, w, h float64, c rgb) {
	strips := 16
	stripW := w / float64(strips)
	for i := 0; i < strips; i++ {
		t := float64(i) / float64(strips-1)
		alpha := 1 - 2*absF(t-0.5)
		if alpha < 0 {
			alpha = 0
		}
		rr := lerp(255, c[0], alpha)
		gg := lerp(255, c[1], alpha)
		bb := lerp(255, c[2], alpha)
		pdf.SetFillColor(rr, gg, bb)
		pdf.Rect(cx-w/2+float64(i)*stripW, y, stripW+0.1, h, "F")
	}
}

func lerp(a, b int, t float64) int {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return int(float64(a) + float64(b-a)*t)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// =====================================================
// 第 2 页:评估基本信息 + 评估结果摘要
// =====================================================

func (g *Generator) renderBasicInfoAndSummary(pdf *gofpdf.Fpdf, r *model.EvaluationDetail, dimensionScores map[string]float64) {
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

func drawPageHeader(pdf *gofpdf.Fpdf, r *model.EvaluationDetail) {
	y := 18.0
	// 左:报告名
	pdf.SetFont(FontSimHeiBold, "B", 14)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(pageMargin, y)
	pdf.CellFormat(120, 6, "叉车残值评估报告", "", 0, "L", false, 0, "")
	// 副行
	pdf.SetFont(FontSimHei, "", 9.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageMargin, y+6.5)
	pdf.CellFormat(120, 4, fmt.Sprintf("报告编号: EV-%06d  |  生成日期: %s",
		r.ID, time.Now().Format("2006-01-02")), "", 0, "L", false, 0, "")

	// 右:公司名
	pdf.SetFont(FontSimHei, "", 9.5)
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
	pdf.SetFont(FontSimHei, "", 8.5)
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
	pdf.SetFont(FontSimHeiBold, "B", h1Pt)
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
func drawBasicInfoTable(pdf *gofpdf.Fpdf, r *model.EvaluationDetail, x, y, w float64) float64 {
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
		pdf.SetFont(FontSimHei, "", 10)
		pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
		pdf.SetXY(x+3, ry+1.5)
		pdf.CellFormat(colLabelW-3, float64(rowH)-1.5, row.label1, "", 0, "L", false, 0, "")
		// 第一组 value
		if row.badge1 {
			drawStatusBadge(pdf, x+colLabelW+3, ry+1.5, colValueW-6, row.value1)
		} else {
			pdf.SetFont(FontSimHei, "", 10)
			pdf.SetTextColor(text[0], text[1], text[2])
			pdf.SetXY(x+colLabelW+3, ry+1.5)
			pdf.CellFormat(colValueW-3, float64(rowH)-1.5, row.value1, "", 0, "L", false, 0, "")
		}
		// 第二组(非跨列)
		if !row.span1 {
			pdf.SetFont(FontSimHei, "", 10)
			pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
			pdf.SetXY(x+colTotal+3, ry+1.5)
			pdf.CellFormat(colLabelW-3, float64(rowH)-1.5, row.label2, "", 0, "L", false, 0, "")
			if row.badge2 {
				drawStatusBadge(pdf, x+colTotal+colLabelW+3, ry+1.5, colValueW-6, row.value2)
			} else {
				pdf.SetFont(FontSimHei, "", 10)
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
	pdf.SetFont(FontSimHei, "", 9.5)
	pdf.SetTextColor(c[0], c[1], c[2])
	pdf.SetXY(x+8, y+1)
	pdf.CellFormat(w-10, h-1, text, "", 0, "L", false, 0, "")
}

// drawValueHero 渲染蓝色 Hero 卡片,显示估算残值
func drawValueHero(pdf *gofpdf.Fpdf, x, y, w, h float64, r *model.EvaluationDetail) {
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
	pdf.SetFont(FontSimHei, "", 9.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetAlpha(0.7, "Normal")
	pdf.SetXY(x+6, y+4)
	pdf.CellFormat(120, 4, "估算残值  RESIDUAL VALUE", "", 0, "L", false, 0, "")
	pdf.SetAlpha(1, "Normal")

	// 大数字行(￥ + 数值 + 万元)
	pdf.SetFont(FontSimHei, "", 14)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x+6, y+11)
	pdf.CellFormat(7, 11, "￥", "", 0, "L", false, 0, "")
	pdf.SetFont(FontSimHeiBold, "B", heroValuePt)
	pdf.SetXY(x+12, y+9)
	pdf.CellFormat(60, 14, fmt.Sprintf("%.2f", yuanToWan(r.EstimatedValue)), "", 0, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", 14)
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
	pdf.SetFont(FontSimHei, "", 8.5)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetAlpha(0.55, "Normal")
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 3.5, label, "", 0, "L", false, 0, "")
	pdf.SetAlpha(1, "Normal")
	pdf.SetFont(FontSimHeiBold, "B", 12)
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
func drawConfidenceBar(pdf *gofpdf.Fpdf, x, y, w float64, r *model.EvaluationDetail) {
	// 文字行
	pdf.SetFont(FontSimHei, "", 9.5)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(40, 4, "置信区间分布", "", 0, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", 9)
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
	pdf.SetFont(FontSimHei, "", 8.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(x, labelY)
	pdf.CellFormat(40, 3, "较低估值", "", 0, "L", false, 0, "")
	pdf.SetXY(x+w/2-15, labelY)
	pdf.CellFormat(30, 3, "最佳估值", "", 0, "C", false, 0, "")
	pdf.SetXY(x+w-40, labelY)
	pdf.CellFormat(40, 3, "较高估值", "", 0, "R", false, 0, "")
}

// drawRadarAndDimensions 雷达图 + 维度进度条(并排)
func drawRadarAndDimensions(pdf *gofpdf.Fpdf, x, y, w float64, dimensionScores map[string]float64) {
	// 左侧雷达图
	radarW := 70.0
	dimX := x + radarW + 4
	dimW := w - radarW - 4

	// 标题
	pdf.SetFont(FontSimHei, "", 10.5)
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
		pdf.SetFont(FontSimHei, "", 9.5)
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

	for i, dim := range radarDimensionOrder {
		v := dimensionScores[dim]
		rowY := dimY0 + float64(i)*rowH
		// 维度名
		pdf.SetFont(FontSimHei, "", 9.5)
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
		pdf.SetFont(FontSimHeiBold, "B", 9.5)
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

// =====================================================
// 第 3 页:评估结论 + 处置建议 + 风险提示 + 免责声明
// =====================================================

// renderCoefficientsAndConclusion 渲染第 3 页
// 顺序:评估结论 → 处置建议 → 风险提示 → 免责声明
func (g *Generator) renderCoefficientsAndConclusion(pdf *gofpdf.Fpdf, r *model.EvaluationDetail, suggestions []string) {
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
	pdf.SetFont(FontSimHei, "", 9)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y+3)
	pdf.CellFormat(cardW, 4, "综合等级评定", "", 0, "C", false, 0, "")
	pdf.SetFont(FontSimHeiBold, "B", 22)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(x, y+7)
	pdf.CellFormat(cardW, 10, grade.letter, "", 0, "C", false, 0, "")
	pdf.SetFont(FontSimHei, "", 10)
	pdf.SetTextColor(primaryLite[0], primaryLite[1], primaryLite[2])
	pdf.SetXY(x, y+18)
	pdf.CellFormat(cardW, 4, grade.cn, "", 0, "C", false, 0, "")

	// 估算残值卡(红色)
	ex := x + cardW + gap
	pdf.SetFillColor(errBg[0], errBg[1], errBg[2])
	pdf.SetDrawColor(errBord[0], errBord[1], errBord[2])
	pdf.SetLineWidth(0.3)
	pdf.RoundedRect(ex, y, estW, cardH, 2, "1234", "FD")
	pdf.SetFont(FontSimHei, "", 9)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(ex, y+3)
	pdf.CellFormat(estW, 4, "估算残值", "", 0, "C", false, 0, "")
	pdf.SetFont(FontSimHeiBold, "B", 22)
	pdf.SetTextColor(errColor[0], errColor[1], errColor[2])
	pdf.SetXY(ex, y+7)
	pdf.CellFormat(estW, 10, fmt.Sprintf("%.2f 万元", yuanToWan(estValue)), "", 0, "C", false, 0, "")
}

func drawRecommendations(pdf *gofpdf.Fpdf, x, y, w float64, suggs []string) float64 {
	// 标题
	pdf.SetFont(FontSimHeiBold, "B", 11)
	pdf.SetTextColor(textSub[0], textSub[1], textSub[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(80, 5, "处置建议", "", 0, "L", false, 0, "")

	// 列表(根据文字长度动态计算每行高度,避免换行重叠)
	rowY := y + 7
	pdf.SetFont(FontSimHei, "", 10)
	availW := w - 7
	for i, s := range suggs {
		sw := pdf.GetStringWidth(s)
		lines := math.Ceil(sw / availW)
		if lines < 1 {
			lines = 1
		}
		itemH := 4*lines + 2
		// 编号
		pdf.SetFont(FontSimHeiBold, "B", 10.5)
		pdf.SetTextColor(primary[0], primary[1], primary[2])
		pdf.SetXY(x, rowY)
		pdf.CellFormat(6, itemH, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
		// 文字
		pdf.SetFont(FontSimHei, "", 10)
		pdf.SetTextColor(textMuted[0], textMuted[1], textMuted[2])
		pdf.SetXY(x+7, rowY)
		pdf.MultiCell(availW, 4, s, "", "L", false)
		rowY += itemH
	}
	return rowY - y
}

func drawRiskWarnings(pdf *gofpdf.Fpdf, x, y, w float64) float64 {
	// 标题
	pdf.SetFont(FontSimHeiBold, "B", 11)
	pdf.SetTextColor(errColor[0], errColor[1], errColor[2])
	pdf.SetXY(x, y)
	pdf.CellFormat(80, 5, "风险提示", "", 0, "L", false, 0, "")

	risks := []string{
		"本报告基于当前采集的设备信息及市场数据生成,实际交易价格可能因地域差异、市场供需波动、设备实际状况等因素产生偏差。",
		"评估结果仅供参考,不构成任何形式的价格承诺或担保,买卖双方应结合实地验车结果进行最终定价。",
		"本报告有效期为自生成之日起 6 个月,逾期需重新评估。",
	}

	// 根据文字折行情况动态计算警示卡高度
	pdf.SetFont(FontSimHei, "", 9.5)
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
		pdf.SetFont(FontSimHei, "", 9.5)
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
	pdf.SetFont(FontSimHeiBold, "B", 10)
	pdf.SetTextColor(textLabel[0], textLabel[1], textLabel[2])
	pdf.SetXY(x, y+3)
	pdf.CellFormat(80, 4, "免责声明", "", 0, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", 9)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(x, y+8)
	pdf.MultiCell(w, 3.5,
		"本评估报告由"+orgName+"的叉车残值评估系统自动生成。报告中所有数据及结论均基于系统算法模型、历史市场数据及用户输入信息综合计算得出。本报告不构成任何投资、交易或法律建议,评估方不对因使用本报告而造成的任何直接或间接损失承担法律责任。报告使用者应结合专业人员的实地检测意见做出独立判断。未经本公司书面许可,不得将本报告用于商业宣传或作为法律依据。",
		"", "L", false)
}

// =====================================================
// 工具函数
// =====================================================

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
