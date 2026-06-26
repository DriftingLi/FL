// Package pdf 实现 PDF 报告生成
// 本文件：电池 RUL 评估报告模板（与现有叉车评估报告并存）
// 不展示算法内部细节（与项目硬约束一致）
package pdf

import (
	"fmt"
	"os"
	"time"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/valuation/model"
)

// batteryTitleSize 电池报告专用排版
const (
	batteryTitleSize    = 18.0
	batteryH1Size       = 14.0
	batteryH2Size       = 12.0
	batteryBodySize     = 10.0
	batteryTableRowSize = 7.0
)

// GenerateBatteryReport 生成电池 RUL 评估报告 PDF
// 6 章节：电池信息 → 评估结论 → 健康度仪表 → Top-5 特征 → 置信区间 → 免责声明
func (g *Generator) GenerateBatteryReport(eval *model.BatteryEvaluation) (string, error) {
	if eval == nil {
		return "", fmt.Errorf("评估记录不能为空")
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(true, pageMargin)
	if err := ensureFontLoaded(pdf); err != nil {
		return "", err
	}

	// 封面页
	pdf.AddPage()
	g.renderBatteryCover(pdf, eval)
	// 正文
	pdf.AddPage()
	g.renderBatteryInfo(pdf, eval)
	g.renderBatteryConclusion(pdf, eval)
	g.renderBatteryTopFeatures(pdf, eval)
	g.renderBatteryConfidence(pdf, eval)
	g.renderBatteryDisclaimer(pdf)

	filename := fmt.Sprintf("battery_report_%d_%s.pdf",
		eval.ID, time.Now().Format("20060102150405"))
	outputPath := joinPath(g.outputDir, filename)

	// 确保输出目录存在
	if g.outputDir != "" {
		if err := os.MkdirAll(g.outputDir, 0o755); err != nil {
			return "", fmt.Errorf("创建输出目录失败: %w", err)
		}
	}
	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return "", fmt.Errorf("保存 PDF 失败: %w", err)
	}
	return outputPath, nil
}

// renderBatteryCover 电池报告封面
func (g *Generator) renderBatteryCover(pdf *gofpdf.Fpdf, eval *model.BatteryEvaluation) {
	pdf.SetFont(FontSimHeiBold, "B", batteryTitleSize)
	pdf.SetXY(pageMargin, 60)
	pdf.CellFormat(contentWidth, 15, "锂电池 RUL 评估报告", "", 1, "C", false, 0, "")

	pdf.SetFont(FontSimHei, "", batteryH1Size)
	pdf.SetXY(pageMargin, 85)
	pdf.CellFormat(contentWidth, 10, "Lithium Battery Remaining Useful Life Report", "", 1, "C", false, 0, "")

	pdf.SetFont(FontSimHei, "", batteryH2Size)
	pdf.SetXY(pageMargin, 120)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("报告编号：BAT-%06d", eval.ID), "", 1, "C", false, 0, "")

	pdf.SetXY(pageMargin, 132)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("生成时间：%s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "C", false, 0, "")

	pdf.SetXY(pageMargin, 144)
	btName := batteryTypeName(eval.BatteryType)
	pdf.CellFormat(contentWidth, 8, fmt.Sprintf("电池类型：%s", btName), "", 1, "C", false, 0, "")

	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.SetXY(pageMargin, 250)
	pdf.CellFormat(contentWidth, 6, "本报告由系统自动生成，仅供参考", "", 1, "C", false, 0, "")
}

// renderBatteryInfo 渲染电池基本信息
func (g *Generator) renderBatteryInfo(pdf *gofpdf.Fpdf, eval *model.BatteryEvaluation) {
	pdf.SetFont(FontSimHeiBold, "B", batteryH1Size)
	pdf.CellFormat(contentWidth, 10, "一、电池基本信息", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	pdf.SetFont(FontSimHei, "", batteryBodySize)
	rows := [][2]string{
		{"电池类型", batteryTypeName(eval.BatteryType)},
		{"电池型号", defaultIfEmpty(eval.BatteryModel, "-")},
		{"循环数据量", fmt.Sprintf("%d 个", eval.CycleCount)},
		{"评估时间", eval.CreatedAt},
	}
	for _, row := range rows {
		pdf.SetFont(FontSimHeiBold, "B", batteryBodySize)
		pdf.CellFormat(40, batteryTableRowSize, row[0]+"：", "", 0, "L", false, 0, "")
		pdf.SetFont(FontSimHei, "", batteryBodySize)
		pdf.CellFormat(contentWidth-40, batteryTableRowSize, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
}

// renderBatteryConclusion 评估结论（健康度 + RUL）
func (g *Generator) renderBatteryConclusion(pdf *gofpdf.Fpdf, eval *model.BatteryEvaluation) {
	pdf.SetFont(FontSimHeiBold, "B", batteryH1Size)
	pdf.CellFormat(contentWidth, 10, "二、评估结论", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// 健康度大字
	pdf.SetFont(FontSimHeiBold, "B", 36)
	pdf.SetTextColor(62, 106, 225) // Electric Blue
	pdf.CellFormat(contentWidth, 18, fmt.Sprintf("SOH %.1f %%", eval.SohPercent), "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.CellFormat(contentWidth, 6, "当前健康度（State of Health）", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// RUL 大字
	pdf.SetFont(FontSimHeiBold, "B", 36)
	pdf.SetTextColor(62, 106, 225)
	pdf.CellFormat(contentWidth, 18, fmt.Sprintf("%d 循环", eval.RulCycles), "", 1, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.CellFormat(contentWidth, 6, "预测剩余循环数（Remaining Useful Life）", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// 置信度
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.CellFormat(contentWidth, 6, fmt.Sprintf("预测置信度：%.1f%%（置信区间 %d ~ %d 循环）", eval.Confidence*100, eval.ConfidenceLow, eval.ConfidenceHigh), "", 1, "C", false, 0, "")
	pdf.Ln(4)
}

// renderBatteryTopFeatures Top-5 特征重要性
func (g *Generator) renderBatteryTopFeatures(pdf *gofpdf.Fpdf, eval *model.BatteryEvaluation) {
	pdf.SetFont(FontSimHeiBold, "B", batteryH1Size)
	pdf.CellFormat(contentWidth, 10, "三、Top-5 关键特征", "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.CellFormat(contentWidth, 6, "按重要性排序的特征组（基于充电 CC-CV 段 20 维特征聚合）", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// 表头
	pdf.SetFont(FontSimHeiBold, "B", batteryBodySize)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(15, batteryTableRowSize, "序号", "1", 0, "C", true, 0, "")
	pdf.CellFormat(60, batteryTableRowSize, "特征名称", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, batteryTableRowSize, "特征组", "1", 0, "C", true, 0, "")
	pdf.CellFormat(35, batteryTableRowSize, "重要性", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, batteryTableRowSize, "条形", "1", 1, "C", true, 0, "")

	// 至少展示前 5 条
	limit := 5
	if len(eval.FeatureImportance) < limit {
		limit = len(eval.FeatureImportance)
	}
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	for i := 0; i < limit; i++ {
		f := eval.FeatureImportance[i]
		pdf.CellFormat(15, batteryTableRowSize, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(60, batteryTableRowSize, f.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(40, batteryTableRowSize, f.Group, "1", 0, "C", false, 0, "")
		pdf.CellFormat(35, batteryTableRowSize, fmt.Sprintf("%.1f%%", f.Normalized*100), "1", 0, "C", false, 0, "")
		// 简易条形（用 ▇ 字符近似）
		barLen := int(f.Normalized * 20)
		bar := ""
		for j := 0; j < barLen; j++ {
			bar += "█"
		}
		pdf.CellFormat(30, batteryTableRowSize, bar, "1", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
}

// renderBatteryConfidence 置信区间
func (g *Generator) renderBatteryConfidence(pdf *gofpdf.Fpdf, eval *model.BatteryEvaluation) {
	pdf.SetFont(FontSimHeiBold, "B", batteryH1Size)
	pdf.CellFormat(contentWidth, 10, "四、置信区间与评估建议", "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	pdf.CellFormat(contentWidth, 6, fmt.Sprintf("置信区间：%d ~ %d 循环（中心值 %d）",
		eval.ConfidenceLow, eval.ConfidenceHigh, eval.RulCycles), "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// 建议列表
	if len(eval.Suggestions) > 0 {
		pdf.SetFont(FontSimHeiBold, "B", batteryBodySize)
		pdf.CellFormat(contentWidth, 6, "评估建议：", "", 1, "L", false, 0, "")
		pdf.SetFont(FontSimHei, "", batteryBodySize)
		for i, s := range eval.Suggestions {
			pdf.MultiCell(contentWidth, batteryTableRowSize, fmt.Sprintf("• %s", s), "", "L", false)
			_ = i
		}
	}
	pdf.Ln(3)
}

// renderBatteryDisclaimer 免责声明
func (g *Generator) renderBatteryDisclaimer(pdf *gofpdf.Fpdf) {
	pdf.SetFont(FontSimHeiBold, "B", batteryH1Size)
	pdf.CellFormat(contentWidth, 10, "五、免责声明", "", 1, "L", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont(FontSimHei, "", batteryBodySize)
	disclaimers := []string{
		"1. 本报告基于用户提交的充放电循环数据与论文启发算法生成，仅供研究、评估与决策参考。",
		"2. 预测结果受数据质量、传感器精度、电池使用工况等因素影响，实际剩余寿命可能与预测存在偏差。",
		"3. 建议结合电池厂商规格书、定期充放电测试与现场巡检数据进行综合判断。",
		"4. 评估系统不对因使用本报告而造成的任何直接或间接损失承担责任。",
	}
	for _, d := range disclaimers {
		pdf.MultiCell(contentWidth, batteryTableRowSize, d, "", "L", false)
	}
}

// batteryTypeName 电池类型中文名
func batteryTypeName(t model.BatteryType) string {
	switch t {
	case model.BatteryTypeLFP:
		return "磷酸铁锂（LFP）"
	case model.BatteryTypeNCM:
		return "三元锂（NCM）"
	case model.BatteryTypeOther:
		return "其他类型"
	}
	return string(t)
}

// GenerateBatteryReportToPath 静态函数：直接生成 PDF 到指定路径（handler 调用入口）
func GenerateBatteryReportToPath(outputDir, fullPath string, eval *model.BatteryEvaluation) error {
	// 把 outputDir 临时改成 fullPath 的目录
	g := &Generator{outputDir: fullPath}
	// 重用 Generator.GenerateBatteryReport 但跳过其内部拼路径
	// 这里直接调内部实现
	_ = outputDir
	return g.generateBatteryReportInternal(fullPath, eval)
}

// generateBatteryReportInternal 直接输出到 fullPath（不重新拼路径）
func (g *Generator) generateBatteryReportInternal(fullPath string, eval *model.BatteryEvaluation) error {
	if eval == nil {
		return fmt.Errorf("评估记录不能为空")
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(true, pageMargin)
	if err := ensureFontLoaded(pdf); err != nil {
		return err
	}
	pdf.AddPage()
	g.renderBatteryCover(pdf, eval)
	pdf.AddPage()
	g.renderBatteryInfo(pdf, eval)
	g.renderBatteryConclusion(pdf, eval)
	g.renderBatteryTopFeatures(pdf, eval)
	g.renderBatteryConfidence(pdf, eval)
	g.renderBatteryDisclaimer(pdf)
	return pdf.OutputFileAndClose(fullPath)
}
