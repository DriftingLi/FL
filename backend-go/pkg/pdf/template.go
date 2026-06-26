// Package pdf 实现 PDF 报告生成
// 本文件：报告模板生成器，将 EvaluationDetailResponse 渲染为完整版 PDF 报告
// 6 段落结构：封面 + 评估基本信息 + 核心部件状态 + 残值估算与定价 + 评估结论与建议 + 免责声明
package pdf

import (
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/valuation/model"
)

// 排版常量（单位：mm / pt）
const (
	pageMargin     = 15.0  // 页边距
	pageWidth      = 210.0 // A4 宽度
	pageHeight     = 297.0 // A4 高度
	contentWidth   = pageWidth - 2*pageMargin
	titleSize      = 18.0  // 封面大标题
	h1Size         = 14.0  // 段落标题
	h2Size         = 12.0  // 二级标题
	bodySize       = 10.0  // 正文字号
	smallSize      = 9.0   // 表格内小字
	lineHeight     = 6.0   // 正文行高
	tableRowHeight = 7.0   // 表格行高
)

// 评估机构名称（硬编码，按用户要求）
const orgName = "和润天下人工智能科技有限公司"

// Generator 报告生成器
type Generator struct {
	outputDir string // PDF 输出目录
}

// NewGenerator 构造报告生成器
// outputDir: 报告 PDF 文件输出目录
func NewGenerator(outputDir string) *Generator {
	return &Generator{outputDir: outputDir}
}

// GenerateReport 生成评估报告 PDF
// report: 评估详情（包含输入参数、计算结果）
// items:  部件状态明细
// weights: 加权权重（用于计算过程展示）
// 返回生成文件的绝对路径
func (g *Generator) GenerateReport(report *model.EvaluationDetailResponse, items []model.EvaluationItemDTO, weights model.CalcWeights) (string, error) {
	// 1. 创建 PDF 文档
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(true, pageMargin)

	// 2. 加载中文字体
	if err := ensureFontLoaded(pdf); err != nil {
		return "", err
	}

	// 3. 渲染封面页
	pdf.AddPage()
	g.renderCover(pdf, report)

	// 4. 第 2 页：一、评估基本信息
	pdf.AddPage()
	g.renderBasicInfo(pdf, report)

	// 5. 第 3 页：二、核心部件状态
	pdf.AddPage()
	g.renderPartItems(pdf, items)

	// 6. 第 4 页：三、残值估算与定价（方法说明 + 计算过程 + 置信结果 + 雷达图）
	pdf.AddPage()
	g.renderValuationMethod(pdf, report, weights)
	g.renderFinalResult(pdf, report)
	g.renderRadarSection(pdf, report)

	// 7. 第 5 页：四、评估结论与建议（综合等级 + 处置建议 + 风险提示）
	pdf.AddPage()
	g.renderConclusion(pdf, report)

	// 8. 第 6 页：五、免责声明
	pdf.AddPage()
	g.renderDisclaimer(pdf)

	// 9. 确定输出路径
	filename := fmt.Sprintf("evaluation_report_%d_%s.pdf",
		report.ID, time.Now().Format("20060102150405"))
	outputPath := joinPath(g.outputDir, filename)

	// 10. 落盘
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return "", fmt.Errorf("保存 PDF 失败: %w", err)
	}
	return outputPath, nil
}

// joinPath 拼接输出路径（避免与 path 包重名）
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

// ========== 段落渲染 ==========

// renderCover 渲染封面页
func (g *Generator) renderCover(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse) {
	// 大标题
	pdf.SetFont(FontSimHeiBold, "B", titleSize)
	pdf.SetXY(pageMargin, 60)
	pdf.CellFormat(contentWidth, 15, "叉车残值评估报告", "", 1, "C", false, 0, "")

	// 英文副标题
	pdf.SetFont(FontSimHei, "", h1Size)
	pdf.SetXY(pageMargin, 85)
	pdf.CellFormat(contentWidth, 10, "Forklift Residual Value Evaluation Report", "", 1, "C", false, 0, "")

	// 报告编号
	pdf.SetFont(FontSimHei, "", h2Size)
	pdf.SetXY(pageMargin, 115)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("报告编号：EV-%06d", r.ID), "", 1, "C", false, 0, "")

	// 生成时间
	pdf.SetXY(pageMargin, 127)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("生成时间：%s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "C", false, 0, "")

	// 评估机构（新增）
	pdf.SetXY(pageMargin, 139)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("评估机构：%s", orgName), "", 1, "C", false, 0, "")

	// 叉车类型
	pdf.SetXY(pageMargin, 151)
	ftType := "电动叉车"
	if r.ForkliftType == model.ForkliftTypeCombustion {
		ftType = "内燃叉车"
	}
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("叉车类型：%s", ftType), "", 1, "C", false, 0, "")

	// 报告有效期声明（新增）
	pdf.SetFont(FontSimHei, "", bodySize)
	pdf.SetXY(pageMargin, 220)
	pdf.CellFormat(contentWidth, 6, "报告有效期声明", "", 1, "C", false, 0, "")
	pdf.SetXY(pageMargin, 232)
	pdf.MultiCell(contentWidth, lineHeight,
		"本报告有效期为自生成之日起 6 个月，逾期后请重新评估。\n"+
			"在有效期内，本报告可作为叉车残值评估的参考依据。",
		"", "C", false)

	// 底部提示
	pdf.SetFont(FontSimHei, "", smallSize)
	pdf.SetXY(pageMargin, 270)
	pdf.CellFormat(contentWidth, 6, "本报告由系统自动生成，仅供参考", "", 1, "C", false, 0, "")
}

// renderBasicInfo 渲染一、评估基本信息（车辆信息）
func (g *Generator) renderBasicInfo(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse) {
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "一、评估基本信息", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(FontSimHei, "", bodySize)
	ftType := "电动叉车"
	if r.ForkliftType == model.ForkliftTypeCombustion {
		ftType = "内燃叉车"
	}

	// 燃料类型：仅内燃叉车有意义
	fuel := "-"
	if r.FuelType != "" {
		fuel = string(r.FuelType)
	}

	// 车辆信息键值对
	rows := [][2]string{
		{"叉车类型", ftType},
		{"品牌", r.Brand},
		{"型号", defaultIfEmpty(r.Model, "-")},
		{"原始购买价格", fmt.Sprintf("%.2f 万元", r.OriginalPrice)},
		{"购置年份", fmt.Sprintf("%d 年", r.PurchaseYear)},
		{"成交年份", fmt.Sprintf("%d 年", r.SaleYear)},
		{"使用年限", fmt.Sprintf("%d 年", r.SaleYear-r.PurchaseYear)},
		{"累计使用小时", fmt.Sprintf("%d 小时", r.UsageHours)},
		{"使用工况", string(r.WorkCondition)},
		{"燃料类型", fuel},
		{"能否正常行驶", boolToYesNo(r.CanDrive)},
		{"液压功能是否正常", boolToYesNo(r.HydraulicOk)},
	}

	for _, row := range rows {
		pdf.SetFont(FontSimHei, "", bodySize)
		pdf.CellFormat(50, lineHeight, row[0]+"：", "", 0, "L", false, 0, "")
		pdf.SetFont(FontSimHei, "", bodySize)
		pdf.CellFormat(contentWidth-50, lineHeight, row[1], "", 1, "L", false, 0, "")
	}
}

// renderPartItems 渲染二、核心部件状态（状态+得分表格）
func (g *Generator) renderPartItems(pdf *gofpdf.Fpdf, items []model.EvaluationItemDTO) {
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "二、核心部件状态", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	if len(items) == 0 {
		pdf.SetFont(FontSimHei, "", bodySize)
		pdf.CellFormat(contentWidth, lineHeight, "（无部件状态数据）", "", 1, "L", false, 0, "")
		return
	}

	// 按类别分组（保持出现顺序）
	groups := make(map[string][]model.EvaluationItemDTO)
	order := []string{}
	for _, it := range items {
		if _, ok := groups[it.CategoryCode]; !ok {
			order = append(order, it.CategoryCode)
		}
		groups[it.CategoryCode] = append(groups[it.CategoryCode], it)
	}

	// 类别级得分汇总表
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "类别得分汇总", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	// 汇总表表头
	sumHeaders := []string{"类别", "条目数", "平均得分", "状态"}
	sumWidths := []float64{60.0, 30.0, 40.0, contentWidth - 60.0 - 30.0 - 40.0}
	drawTableHeader(pdf, sumHeaders, sumWidths)

	// 状态文本映射
	statusText := map[model.ItemStatus]string{
		model.ItemStatusNormal:      "正常",
		model.ItemStatusSlightWear:  "轻微磨损",
		model.ItemStatusNeedRepair:  "需维修",
		model.ItemStatusNeedReplace: "需更换",
	}

	// 汇总表数据行
	for _, code := range order {
		group := groups[code]
		categoryName := ""
		if len(group) > 0 {
			categoryName = group[0].CategoryName
		}
		// 计算类别平均得分
		sumScore := 0.0
		worstStatus := model.ItemStatusNormal
		statusPriority := map[model.ItemStatus]int{
			model.ItemStatusNormal:      0,
			model.ItemStatusSlightWear:  1,
			model.ItemStatusNeedRepair:  2,
			model.ItemStatusNeedReplace: 3,
		}
		for _, it := range group {
			sumScore += it.Score
			if statusPriority[it.Status] > statusPriority[worstStatus] {
				worstStatus = it.Status
			}
		}
		avgScore := sumScore / float64(len(group))

		row := []string{
			categoryName,
			fmt.Sprintf("%d", len(group)),
			fmt.Sprintf("%.2f", avgScore),
			statusText[worstStatus],
		}
		drawTableRow(pdf, row, sumWidths)
	}

	pdf.Ln(4)

	// 部件明细表
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "部件状态明细", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	headers := []string{"类别", "条目", "状态", "评分"}
	widths := []float64{30.0, 60.0, 45.0, contentWidth - 30.0 - 60.0 - 45.0}
	drawTableHeader(pdf, headers, widths)

	for _, code := range order {
		group := groups[code]
		categoryName := ""
		if len(group) > 0 {
			categoryName = group[0].CategoryName
		}
		for i, it := range group {
			cat := ""
			if i == 0 {
				cat = categoryName
			}
			row := []string{
				cat,
				it.ItemName,
				statusText[it.Status],
				fmt.Sprintf("%.2f", it.Score),
			}
			drawTableRow(pdf, row, widths)
		}
	}
}

// renderValuationMethod 渲染三、残值估算与定价（评估方法说明 + 残值计算过程）
func (g *Generator) renderValuationMethod(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse, w model.CalcWeights) {
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "三、残值估算与定价", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// ===== 评估方法说明 =====
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "评估方法说明", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont(FontSimHei, "", bodySize)
	pdf.MultiCell(contentWidth, lineHeight,
		"本评估采用两级加权模型，综合考虑时间折旧、使用强度、工况、品牌、车况、市场六大维度对叉车残值的影响。"+
			"其中车况维度采用类别级与条目级两级加权，精细反映各部件状态对整体残值的贡献。",
		"", "L", false)
	pdf.Ln(2)

	// 计算公式（使用 ASCII 下标，避免 simhei 字体不支持 Unicode 下标导致方框）
	pdf.SetFont(FontSimHeiBold, "B", bodySize)
	pdf.CellFormat(contentWidth, lineHeight, "计算公式：", "", 1, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", bodySize)
	pdf.MultiCell(contentWidth, lineHeight,
		"V = V0 × Kt × Kh × (w1·Kw + w2·Kb + w3·Kc + w4·Km)",
		"", "C", false)
	pdf.Ln(1)
	pdf.MultiCell(contentWidth, lineHeight,
		"其中：V 为估算残值，V0 为原始购买价格；\n"+
			"Kt 为时间系数，Kh 为使用强度系数；\n"+
			"Kw 为工况系数，Kb 为品牌系数，Kc 为车况系数，Km 为市场系数；\n"+
			"w1~w4 为对应权重，满足 w1+w2+w3+w4=1。",
		"", "L", false)
	pdf.Ln(3)

	// ===== 残值计算过程 =====
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "残值计算过程", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	// 参数取值表
	pdf.SetFont(FontSimHeiBold, "B", bodySize)
	pdf.CellFormat(contentWidth, lineHeight, "参数取值：", "", 1, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", bodySize)

	paramRows := [][2]string{
		{"原始价格 V0", fmt.Sprintf("%.2f 万元", r.OriginalPrice)},
		{"时间系数 Kt", fmt.Sprintf("%.4f", r.KTime)},
		{"使用强度系数 Kh", fmt.Sprintf("%.4f", r.KHours)},
		{"工况系数 Kw", fmt.Sprintf("%.4f", r.KWork)},
		{"品牌系数 Kb", fmt.Sprintf("%.4f", r.KBrand)},
		{"车况系数 Kc", fmt.Sprintf("%.4f", r.KCondition)},
		{"市场系数 Km", fmt.Sprintf("%.4f", r.KMarket)},
		{"加权权重 w1(工况)", fmt.Sprintf("%.2f", w.WWorkCondition)},
		{"加权权重 w2(品牌)", fmt.Sprintf("%.2f", w.WBrand)},
		{"加权权重 w3(车况)", fmt.Sprintf("%.2f", w.WCondition)},
		{"加权权重 w4(市场)", fmt.Sprintf("%.2f", w.WMarket)},
	}
	for _, row := range paramRows {
		pdf.CellFormat(60, lineHeight, row[0]+"：", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentWidth-60, lineHeight, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)

	// 计算步骤
	pdf.SetFont(FontSimHeiBold, "B", bodySize)
	pdf.CellFormat(contentWidth, lineHeight, "计算步骤：", "", 1, "L", false, 0, "")
	pdf.SetFont(FontSimHei, "", bodySize)

	// Σ(wi·Ki) 计算（使用 ASCII 下标避免方框）
	weightedSum := w.WWorkCondition*r.KWork + w.WBrand*r.KBrand + w.WCondition*r.KCondition + w.WMarket*r.KMarket
	pdf.MultiCell(contentWidth, lineHeight,
		fmt.Sprintf("Σ(wi·Ki) = %.2f×%.4f + %.2f×%.4f + %.2f×%.4f + %.2f×%.4f = %.4f",
			w.WWorkCondition, r.KWork, w.WBrand, r.KBrand, w.WCondition, r.KCondition, w.WMarket, r.KMarket, weightedSum),
		"", "L", false)
	pdf.Ln(1)

	// 最终残值计算
	pdf.MultiCell(contentWidth, lineHeight,
		fmt.Sprintf("V = %.2f × %.4f × %.4f × %.4f = %.2f 万元",
			r.OriginalPrice, r.KTime, r.KHours, weightedSum, r.EstimatedValue),
		"", "L", false)
}

// renderFinalResult 渲染残值估算结果（残值大字 + 95%置信水平 + 残值率）
func (g *Generator) renderFinalResult(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse) {
	pdf.Ln(6)
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "估算结果", "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// 残值大字号展示
	pdf.SetFont(FontSimHeiBold, "B", 22)
	pdf.SetTextColor(220, 50, 50)
	pdf.CellFormat(contentWidth, 16, fmt.Sprintf("估算残值：%.2f 万元", r.EstimatedValue), "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	pdf.Ln(4)
	pdf.SetFont(FontSimHei, "", bodySize)

	// 残值率：避免除零
	rate := 0.0
	if r.OriginalPrice > 0 {
		rate = r.EstimatedValue / r.OriginalPrice * 100
	}
	rows := [][2]string{
		{"置信水平", "95%"},
		{"置信区间下限", fmt.Sprintf("%.2f 万元", r.ConfidenceLow)},
		{"置信区间上限", fmt.Sprintf("%.2f 万元", r.ConfidenceHigh)},
		{"残值率（残值/原值）", fmt.Sprintf("%.1f%%", rate)},
	}
	for _, row := range rows {
		pdf.CellFormat(50, lineHeight, row[0]+"：", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentWidth-50, lineHeight, row[1], "", 1, "L", false, 0, "")
	}
}

// renderRadarSection 渲染维度评分雷达图（独占一页，避免标签触发自动分页散落）
func (g *Generator) renderRadarSection(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse) {
	// 雷达图独占一页：新建页面，避免上一节内容剩余空间不足导致标签散落
	pdf.AddPage()

	// 页面标题
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "维度评分雷达图", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// 若无维度评分数据，跳过绘图
	if len(r.DimensionScores) == 0 {
		pdf.SetFont(FontSimHei, "", bodySize)
		pdf.CellFormat(contentWidth, lineHeight, "（无维度评分数据）", "", 1, "L", false, 0, "")
		return
	}

	// 绘制期间禁用自动分页，防止标签 CellFormat 触发换页导致散落
	pdf.SetAutoPageBreak(false, 0)

	// 雷达图居中放置：水平居中，垂直在页面中部偏上
	// 页面可用高度 297 - 2*15 = 267mm，标题占用约 20mm，剩余 247mm
	// 雷达图直径 70mm + 标签外侧 8mm + 标签高度 4mm ≈ 90mm，居中放置
	cx := pageMargin + contentWidth/2
	cy := 120.0 // 雷达图中心 Y 坐标（页面中部偏上）
	radius := 35.0
	drawRadarChart(pdf, cx, cy, radius, r.DimensionScores)

	// 恢复自动分页
	pdf.SetAutoPageBreak(true, pageMargin)

	// 光标移到雷达图下方，为下一节做准备
	pdf.SetXY(pageMargin, cy+radius+25)
}

// renderConclusion 渲染四、评估结论与建议（综合等级 + 处置建议 + 风险提示）
func (g *Generator) renderConclusion(pdf *gofpdf.Fpdf, r *model.EvaluationDetailResponse) {
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "四、评估结论与建议", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// ===== 综合等级评定 =====
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "综合等级评定", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	// 计算残值率
	rate := 0.0
	if r.OriginalPrice > 0 {
		rate = r.EstimatedValue / r.OriginalPrice
	}
	gradeCN, gradeLetter, gradeDesc := gradeLevel(rate)

	pdf.SetFont(FontSimHei, "", bodySize)
	pdf.CellFormat(50, lineHeight, "残值率：", "", 0, "L", false, 0, "")
	pdf.CellFormat(contentWidth-50, lineHeight, fmt.Sprintf("%.1f%%", rate*100), "", 1, "L", false, 0, "")

	pdf.CellFormat(50, lineHeight, "综合等级：", "", 0, "L", false, 0, "")
	// 等级用红色突出
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.SetTextColor(220, 50, 50)
	pdf.CellFormat(contentWidth-50, lineHeight, fmt.Sprintf("%s级（%s）", gradeCN, gradeLetter), "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)

	pdf.SetFont(FontSimHei, "", bodySize)
	pdf.CellFormat(50, lineHeight, "等级说明：", "", 0, "L", false, 0, "")
	pdf.CellFormat(contentWidth-50, lineHeight, gradeDesc, "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// ===== 处置建议 =====
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "处置建议", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	if len(r.Suggestions) == 0 {
		pdf.SetFont(FontSimHei, "", bodySize)
		pdf.CellFormat(contentWidth, lineHeight, "（暂无建议）", "", 1, "L", false, 0, "")
	} else {
		pdf.SetFont(FontSimHei, "", bodySize)
		for i, s := range r.Suggestions {
			pdf.MultiCell(contentWidth, lineHeight, fmt.Sprintf("%d. %s", i+1, s), "", "L", false)
			pdf.Ln(1)
		}
	}
	pdf.Ln(3)

	// ===== 风险提示 =====
	pdf.SetFont(FontSimHeiBold, "B", h2Size)
	pdf.CellFormat(contentWidth, 8, "风险提示", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont(FontSimHei, "", bodySize)
	risks := []string{
		"1. 市场风险：二手叉车市场价格受供需关系、区域差异影响，存在波动，实际成交价可能偏离评估值。",
		"2. 评估风险：本评估基于评估时点车况，未来车况变化不在评估范围内，建议定期复评。",
		"3. 交易风险：实际成交价格受交易双方议价能力、设备运输成本、售后保障等因素影响。",
	}
	for _, risk := range risks {
		pdf.MultiCell(contentWidth, lineHeight, risk, "", "L", false)
		pdf.Ln(1)
	}
}

// renderDisclaimer 渲染五、免责声明
func (g *Generator) renderDisclaimer(pdf *gofpdf.Fpdf) {
	pdf.Ln(4)
	pdf.SetFont(FontSimHeiBold, "B", h1Size)
	pdf.CellFormat(contentWidth, 10, "五、免责声明", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(FontSimHei, "", smallSize)
	pdf.MultiCell(contentWidth, lineHeight,
		"1. 本报告基于系统评估模型与用户提交数据计算得出，仅作为残值评估的参考依据。\n\n"+
			"2. 实际成交价格受市场行情、交易双方议价能力、设备具体状况等因素影响，可能与本报告存在偏差。\n\n"+
			"3. 使用本报告进行商业决策所产生的任何后果，由使用方自行承担。\n\n"+
			"4. 叉车残值评估系统保留对本报告内容的最终解释权。",
		"", "L", false)
}

// ========== 表格辅助函数 ==========

// drawTableHeader 绘制表头（带灰色背景）
func drawTableHeader(pdf *gofpdf.Fpdf, headers []string, widths []float64) {
	pdf.SetFont(FontSimHeiBold, "B", bodySize)
	pdf.SetFillColor(230, 230, 230)
	for i, h := range headers {
		pdf.CellFormat(widths[i], tableRowHeight, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)
}

// drawTableRow 绘制数据行
func drawTableRow(pdf *gofpdf.Fpdf, row []string, widths []float64) {
	pdf.SetFont(FontSimHei, "", smallSize)
	for i, cell := range row {
		align := "L"
		if i == 2 || i == 3 {
			align = "C"
		}
		pdf.CellFormat(widths[i], tableRowHeight, cell, "1", 0, align, false, 0, "")
	}
	pdf.Ln(-1)
}

// ========== 工具函数 ==========

// boolToYesNo 布尔值 → 中文（"是"/"否"）
func boolToYesNo(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// defaultIfEmpty 空字符串回退
func defaultIfEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// gradeLevel 根据残值率评定等级
// rate: 残值率（0~1）
// 返回：(等级中文, 等级字母, 等级说明)
func gradeLevel(rate float64) (string, string, string) {
	switch {
	case rate >= 0.70:
		return "优", "A", "车况良好，保值率高，建议正常出售"
	case rate >= 0.50:
		return "良", "B", "车况尚可，保值率中等，建议适当整备后出售"
	case rate >= 0.30:
		return "中", "C", "车况一般，保值率偏低，建议维修后出售或折价处理"
	default:
		return "差", "D", "车况较差，保值率低，建议拆件出售或作为配件使用"
	}
}
