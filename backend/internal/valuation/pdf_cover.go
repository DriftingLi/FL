// PDF 报告生成（原 internal/valuation/pdf，#1514 波 5 并回域包）。
// 本文件:第 1 页封面(渐变蓝带 + 叉车图标 + 报告元信息卡片 + 车辆标签)。
// 报告总览与入口见 pdf_template.go。
package valuation

import (
	"fmt"
	"time"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/pdfutil"
)

// 第 1 页:封面

func (g *PDFGenerator) renderCover(pdf *gofpdf.Fpdf, r *EvaluationDetail) {
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
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 15)
	pdf.SetTextColor(primary[0], primary[1], primary[2])
	pdf.SetXY(pageMargin, logoY+logoSize+4)
	pdf.CellFormat(contentWidth, 6, orgName, "", 1, "C", false, 0, "")
	// 公司英文名
	pdf.SetFont(pdfutil.FontSimHei, "", 10.5)
	pdf.SetTextColor(textLite[0], textLite[1], textLite[2])
	pdf.SetXY(pageMargin, logoY+logoSize+12)
	pdf.CellFormat(contentWidth, 5, orgNameEN, "", 1, "C", false, 0, "")

	// 装饰线
	dividerY := 95.0
	drawCenterFadeBar(pdf, cx, dividerY, 30, 0.6, primary)

	// 大标题
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 28)
	pdf.SetTextColor(text[0], text[1], text[2])
	pdf.SetXY(pageMargin, dividerY+10)
	pdf.CellFormat(contentWidth, 14, "叉车残值评估报告", "", 1, "C", false, 0, "")
	// 英文副标题
	pdf.SetFont(pdfutil.FontSimHei, "", 12)
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
	pdf.SetFont(pdfutil.FontSimHei, "", 10.5)
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
func coverVehicleLabel(r *EvaluationDetail) string {
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
	pdf.SetFont(pdfutil.FontSimHei, "", 11.5)
	labelW := pdf.GetStringWidth(label)
	if valueBold {
		pdf.SetFont(pdfutil.FontSimHeiBold, "B", 13)
	} else {
		pdf.SetFont(pdfutil.FontSimHei, "", 13)
	}
	valueW := pdf.GetStringWidth(value)
	totalW := labelW + gap + valueW
	startX := x
	if totalW < w {
		startX = x + (w-totalW)/2
	}

	pdf.SetFont(pdfutil.FontSimHei, "", 11.5)
	pdf.SetTextColor(lc[0], lc[1], lc[2])
	pdf.SetXY(startX, y)
	pdf.CellFormat(labelW, 6, label, "", 0, "L", false, 0, "")
	if valueBold {
		pdf.SetFont(pdfutil.FontSimHeiBold, "B", 13)
	} else {
		pdf.SetFont(pdfutil.FontSimHei, "", 13)
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
