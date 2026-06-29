// Package pdf 实现 PDF 报告生成
// 本文件：4 维雷达图渲染，直接使用 gofpdf 矢量原语绘制（Line/Polygon/Circle）
// 维度顺序：时间衰减（含品牌/强度修正）、车况、市场、残值率（顺时针从顶部开始）
package pdf

import (
	"fmt"
	"math"

	"github.com/jung-kurt/gofpdf"
)

// radarDimensionOrder 4 维度的固定展示顺序（顺时针从顶部开始，间隔 90°）
// 重构说明：从原 5 维（时间/使用强度/品牌/车况/市场）合并为 4 维
//   - 品牌系数与使用强度系数已并入时间衰减（Kt_adj = Kt^(Kh/Kb)）
//   - 新增"残值率"维度直觀展示最终保值水平
var radarDimensionOrder = []string{
	"时间衰减", // 顶部 -90°
	"车况",   // 右侧 0°
	"市场",   // 底部 90°
	"残值率",  // 左侧 -180° / 180°
}

// radarMaxValue 雷达图最大刻度值
// 时间衰减与车况、市场范围通常 0.3~1.15；残值率 ≤ 1.0；统一设 1.2 为满刻度
const radarMaxValue = 1.2

// drawRadarChart 在 PDF 上绘制 4 维雷达图
// pdf: gofpdf 实例
// cx, cy: 雷达图中心坐标（mm）
// radius: 雷达图半径（mm）
// dimensionScores: 维度名 → 评分（实际 K 系数值或残值率，如 0.74/1.10/1.05 等）
func drawRadarChart(pdf *gofpdf.Fpdf, cx, cy, radius float64, dimensionScores map[string]float64) {
	// 1. 计算每个维度的角度（弧度），从顶部 -90° 开始顺时针，每维间隔 90°
	angles := make([]float64, len(radarDimensionOrder))
	for i := range radarDimensionOrder {
		angles[i] = (-90.0 + float64(i)*90.0) * math.Pi / 180.0
	}

	// 2. 绘制同心网格(4 圈:25% / 50% / 75% / 100%)
	gridLevels := []float64{0.3, 0.6, 0.9, 1.2}
	pdf.SetDrawColor(226, 232, 240) // #E2E8F0 浅灰
	pdf.SetLineWidth(0.2)
	for i, level := range gridLevels {
		ratio := level / radarMaxValue
		r := radius * ratio
		points := make([]gofpdf.PointType, len(radarDimensionOrder))
		for j := range radarDimensionOrder {
			x := cx + r*math.Cos(angles[j])
			y := cy + r*math.Sin(angles[j])
			points[j] = gofpdf.PointType{X: x, Y: y}
		}
		// 最外圈用稍深色,模拟设计稿 100% 边框
		if i == len(gridLevels)-1 {
			pdf.SetDrawColor(203, 213, 225) // #CBD5E1
		}
		pdf.Polygon(points, "D")
		// 恢复内圈浅色
		if i == len(gridLevels)-1 {
			pdf.SetDrawColor(226, 232, 240)
		}
	}

	// 3. 绘制轴线（从中心到外圈）
	pdf.SetDrawColor(200, 200, 200)
	pdf.SetLineWidth(0.3)
	for i := range radarDimensionOrder {
		x := cx + radius*math.Cos(angles[i])
		y := cy + radius*math.Sin(angles[i])
		pdf.Line(cx, cy, x, y)
	}

	// 4. 绘制刻度标签（在顶部轴线上标注刻度值）
	pdf.SetFont(FontSimHei, "", 7.0)
	pdf.SetTextColor(150, 150, 150)
	for _, level := range gridLevels {
		ratio := level / radarMaxValue
		r := radius * ratio
		// 在顶部轴线（-90° 方向）标注刻度
		x := cx + r*math.Cos(angles[0])
		y := cy + r*math.Sin(angles[0])
		pdf.SetXY(x+1, y-2)
		pdf.CellFormat(10, 3, fmt.Sprintf("%.2f", level), "", 0, "L", false, 0, "")
	}

	// 5. 绘制数据多边形(浅蓝填充 + 深蓝描边,匹配设计稿)
	// 评分值按 value/radarMaxValue 归一化到 0~1 范围用于绘图
	dataPoints := make([]gofpdf.PointType, len(radarDimensionOrder))
	for i, dimName := range radarDimensionOrder {
		value := dimensionScores[dimName]
		if value < 0 {
			value = 0
		}
		if value > radarMaxValue {
			value = radarMaxValue
		}
		ratio := value / radarMaxValue
		r := radius * ratio
		x := cx + r*math.Cos(angles[i])
		y := cy + r*math.Sin(angles[i])
		dataPoints[i] = gofpdf.PointType{X: x, Y: y}
	}
	// 填充色:近似 rgba(30,64,175,0.15) 在白底上的效果 → (220, 230, 250)
	pdf.SetFillColor(220, 230, 250)
	// 描边色:#1E40AF = (30, 64, 175)
	pdf.SetDrawColor(30, 64, 175)
	pdf.SetLineWidth(0.6)
	pdf.Polygon(dataPoints, "DF") // DF = 描边 + 填充

	// 6. 绘制数据点(每个顶点画实心圆点)
	pdf.SetFillColor(30, 64, 175)
	for _, p := range dataPoints {
		pdf.Circle(p.X, p.Y, 1.2, "F") // 半径 1.2mm 的实心圆点
	}

	// 7. 绘制维度标签（在外圈外侧，根据角度精确定位）
	pdf.SetFont(FontSimHei, "", 8.5)
	pdf.SetTextColor(71, 85, 105) // #475569
	for i, dimName := range radarDimensionOrder {
		// 标签位置：外圈外侧 8mm
		labelR := radius + 8.0
		x := cx + labelR*math.Cos(angles[i])
		y := cy + labelR*math.Sin(angles[i])

		// 评分值
		value := dimensionScores[dimName]
		label := fmt.Sprintf("%s %.2f", dimName, value)

		// 根据角度精确调整标签位置和对齐方式
		angleDeg := -90.0 + float64(i)*90.0

		// 标签宽度估算（每个字符约 3mm，9pt 字体）
		labelWidth := float64(len([]rune(label))) * 3.0
		labelHeight := 4.0

		switch {
		case angleDeg == -90:
			// 顶部：标签在上方，水平居中
			pdf.SetXY(x-labelWidth/2, y-labelHeight-1)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "C", false, 0, "")
		case angleDeg == 90:
			// 底部：标签在下方，水平居中
			pdf.SetXY(x-labelWidth/2, y+1)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "C", false, 0, "")
		case angleDeg == 0:
			// 右侧：标签在右侧，左对齐
			pdf.SetXY(x+1, y-labelHeight/2)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "L", false, 0, "")
		default:
			// 左侧（angleDeg == 180 或 -180）：标签在左侧，右对齐
			pdf.SetXY(x-labelWidth-1, y-labelHeight/2)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "R", false, 0, "")
		}
	}

	// 恢复默认颜色
	pdf.SetTextColor(0, 0, 0)
}
