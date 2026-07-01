// Package pdf 实现 PDF 报告生成
// 本文件：5 维雷达图渲染，直接使用 gofpdf 矢量原语绘制（Line/Polygon/Circle）
// 维度顺序：出厂时间、使用强度、品牌价值、市场需求、车辆情况（顺时针从顶部开始）
package pdf

import (
	"fmt"
	"math"

	"github.com/jung-kurt/gofpdf"
)

// radarDimensionOrder 5 维度的固定展示顺序（顺时针从顶部开始，间隔 72°）
// 5 维独立展示，各维度值钳制到 [0, 1]，与前端雷达图 max=1 对齐
var radarDimensionOrder = []string{
	"出厂时间", // 顶部 -90°
	"使用强度", // -18°
	"品牌价值", // 54°
	"市场需求", // 126°
	"车辆情况", // 198° / -162°
}

// radarMaxValue 雷达图最大刻度值
// 5 维值均钳制到 [0, 1]，满刻度 1.0 对应 100%
const radarMaxValue = 1.0

// drawRadarChart 在 PDF 上绘制 5 维雷达图
// pdf: gofpdf 实例
// cx, cy: 雷达图中心坐标（mm）
// radius: 雷达图半径（mm）
// dimensionScores: 维度名 → 评分（K 系数值，已钳制到 [0,1]）
func drawRadarChart(pdf *gofpdf.Fpdf, cx, cy, radius float64, dimensionScores map[string]float64) {
	// 1. 计算每个维度的角度（弧度），从顶部 -90° 开始顺时针，每维间隔 72°
	angles := make([]float64, len(radarDimensionOrder))
	for i := range radarDimensionOrder {
		angles[i] = (-90.0 + float64(i)*72.0) * math.Pi / 180.0
	}

	// 2. 绘制同心网格(4 圈:25% / 50% / 75% / 100%)
	gridLevels := []float64{0.25, 0.5, 0.75, 1.0}
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
		// 标签位置：外圈外侧 3mm
		labelR := radius + 3.0
		x := cx + labelR*math.Cos(angles[i])
		y := cy + labelR*math.Sin(angles[i])

		// 雷达图标签只显示维度名(数值已在右侧进度条展示)
		label := dimName

		// 标签宽度按实际字体测量
		labelWidth := pdf.GetStringWidth(label) + 1.0
		labelHeight := 4.0

		// 根据 cos/sin 分量决定标签对齐方式（通用，适配任意维度数）
		cosA := math.Cos(angles[i])
		sinA := math.Sin(angles[i])
		switch {
		case cosA >= 0.5:
			// 右侧：标签在右侧，左对齐
			pdf.SetXY(x+1, y-labelHeight/2)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "L", false, 0, "")
		case cosA <= -0.5:
			// 左侧：标签在左侧，右对齐
			pdf.SetXY(x-labelWidth-1, y-labelHeight/2)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "R", false, 0, "")
		case sinA < 0:
			// 顶部：标签在上方，水平居中
			pdf.SetXY(x-labelWidth/2, y-labelHeight-1)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "C", false, 0, "")
		default:
			// 底部：标签在下方，水平居中
			pdf.SetXY(x-labelWidth/2, y+1)
			pdf.CellFormat(labelWidth, labelHeight, label, "", 0, "C", false, 0, "")
		}
	}

	// 恢复默认颜色
	pdf.SetTextColor(0, 0, 0)
}
