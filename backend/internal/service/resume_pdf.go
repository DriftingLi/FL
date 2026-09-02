// Package service 在线简历 PDF 渲染（spec #484 决定：单份打码口径）。
// 由简历卡结构化字段实时渲染，不落盘、不预生成缓存快照；姓名沿用简历库打码规则
// （MaskRealName）、剔除电话/微信、现居地展示到市、不含工作照与证书原图。
// 渲染复用共享 pdfutil（内嵌 simhei 字体，与残值域 PDF 资产同源）。
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"

	"forklift-training/internal/model"
	"forklift-training/internal/pdfutil"
)

// ResumePDFRenderer 在线简历 PDF 渲染器（无状态：每次调用返回新字节，不落盘）。
type ResumePDFRenderer struct{}

// NewResumePDFRenderer 构造在线简历渲染器。
func NewResumePDFRenderer() *ResumePDFRenderer {
	return &ResumePDFRenderer{}
}

// RenderResumePDF 由简历卡实时渲染打码在线简历 PDF，返回二进制内容。
// 口径（spec #484 / 子票 #485）：
//   - 姓名打码（MaskRealName 规则：张*丰）
//   - 不含 contact_phone / wechat 明文
//   - 现居地截断到市（两段「省/市」或直辖市一段）
//   - 工作经历、持证只出名称，不出证书原图；无工作照（photos）
//   - 意向地区/薪资/经验/到岗/用工性质/自我介绍保留
//
// 压缩开关 testCompress=false 时关闭 PDF 压缩，便于契约测试做字节级文本断言。
func (g *ResumePDFRenderer) RenderResumePDF(m *model.JobCard, testCompress bool) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pdfutil.PageMarginMm, pdfutil.PageMarginMm, pdfutil.PageMarginMm)
	pdf.SetAutoPageBreak(true, pdfutil.PageMarginMm)
	if !testCompress {
		pdf.SetCompression(false)
	}
	if err := pdfutil.EnsureFontLoaded(pdf); err != nil {
		return nil, err
	}

	pdf.AddPage()
	g.renderHeader(pdf, m)
	g.renderBasicInfo(pdf, m)
	g.renderSections(pdf, m)

	buf := &bytes.Buffer{}
	if err := pdf.Output(buf); err != nil {
		return nil, fmt.Errorf("生成在线简历 PDF 失败: %w", err)
	}
	return buf.Bytes(), nil
}

const (
	resumeContentW = pdfutil.PageWidthMm - 2*pdfutil.PageMarginMm
	resumeLineH    = 5.6 // 正文行高（mm）
	resumeH1Pt     = 12.5
	resumeBodyPt   = 10
	resumeLabelPt  = 9
)

// rgb 三元组（在线简历配色，中性商务风）
type resumeRGB [3]int

var (
	resumeText    = resumeRGB{15, 23, 42}    // 主文字
	resumeTextSub = resumeRGB{71, 85, 105}   // 次级文字
	resumeTextLbl = resumeRGB{100, 116, 139} // 标签文字
	resumeBorder  = resumeRGB{203, 213, 225} // 边框
	resumeAccent  = resumeRGB{30, 64, 175}   // 强调（主色）
)

// renderHeader 页头：姓名（打码）+ 一句话定位（期望岗位/意向地区）。
func (g *ResumePDFRenderer) renderHeader(pdf *gofpdf.Fpdf, m *model.JobCard) {
	masked := MaskRealName(m.RealName)
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", 20)
	pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
	pdf.SetXY(pdfutil.PageMarginMm, pdfutil.PageMarginMm+2)
	pdf.CellFormat(resumeContentW, 10, masked, "", 1, "L", false, 0, "")

	sub := g.headerLine(m)
	if sub != "" {
		pdf.SetFont(pdfutil.FontSimHei, "", 10)
		pdf.SetTextColor(resumeTextSub[0], resumeTextSub[1], resumeTextSub[2])
		pdf.CellFormat(resumeContentW, 6, sub, "", 1, "L", false, 0, "")
	}
	// 分隔线
	y := pdf.GetY() + 2
	pdf.SetDrawColor(resumeBorder[0], resumeBorder[1], resumeBorder[2])
	pdf.SetLineWidth(0.4)
	pdf.Line(pdfutil.PageMarginMm, y, pdfutil.PageWidthMm-pdfutil.PageMarginMm, y)
	pdf.SetY(y + 4)
}

// headerLine 头部定位行（期望岗位 · 意向地区 · 期望薪资）。
func (g *ResumePDFRenderer) headerLine(m *model.JobCard) string {
	parts := make([]string, 0, 3)
	if m.ExpectedPositionExtra != "" {
		parts = append(parts, m.ExpectedPositionExtra)
	} else if m.ExpectedPositionID != nil {
		parts = append(parts, "期望岗位已选")
	}
	if regs := resumeRegions(m.ExpectedRegions); len(regs) > 0 {
		parts = append(parts, "意向："+strings.Join(regs, "、"))
	}
	if m.SalaryNegotiable {
		parts = append(parts, "薪资面议")
	} else if m.SalaryMin != nil || m.SalaryMax != nil {
		lo, hi := "-", "-"
		if m.SalaryMin != nil {
			lo = fmt.Sprintf("%d", *m.SalaryMin)
		}
		if m.SalaryMax != nil {
			hi = fmt.Sprintf("%d", *m.SalaryMax)
		}
		parts = append(parts, fmt.Sprintf("期望薪资 %s-%s", lo, hi))
	}
	return strings.Join(parts, "  |  ")
}

// renderBasicInfo 基本信息区（无电话/微信；现居地截断到市）。
func (g *ResumePDFRenderer) renderBasicInfo(pdf *gofpdf.Fpdf, m *model.JobCard) {
	g.sectionTitle(pdf, "基本信息")
	rows := [][2]string{
		{"现居地", cityLevelRegion(m.Region)},
		{"工作年限", fmt.Sprintf("%d 年", m.ExperienceYears)},
		{"到岗时间", availableLabel(m.AvailableIn)},
		{"用工性质", jobNatureLabel(m.JobNature)},
	}
	for _, r := range rows {
		if r[1] == "" || r[1] == "-" {
			continue
		}
		g.labelValueRow(pdf, r[0], r[1])
	}
	pdf.Ln(2)
}

// renderSections 分段：自我介绍 / 工作经历 / 持证（只出名称）。
func (g *ResumePDFRenderer) renderSections(pdf *gofpdf.Fpdf, m *model.JobCard) {
	if strings.TrimSpace(m.SelfIntro) != "" {
		g.sectionTitle(pdf, "自我介绍")
		pdf.SetFont(pdfutil.FontSimHei, "", resumeBodyPt)
		pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
		pdf.MultiCell(resumeContentW, resumeLineH, strings.TrimSpace(m.SelfIntro), "", "L", false)
		pdf.Ln(2)
	}

	// 工作经历
	var exps []map[string]any
	if err := json.Unmarshal(m.ResumeExperiences, &exps); err == nil && len(exps) > 0 {
		g.sectionTitle(pdf, "工作经历")
		for _, e := range exps {
			company, _ := e["company"].(string)
			role, _ := e["role"].(string)
			startM, _ := e["start_month"].(string)
			endM, _ := e["end_month"].(string)
			desc, _ := e["desc"].(string)
			title := strings.TrimSpace(company)
			if role != "" {
				if title != "" {
					title += " · " + role
				} else {
					title = role
				}
			}
			if title == "" {
				continue
			}
			period := ""
			if startM != "" || endM != "" {
				period = fmt.Sprintf("（%s ~ %s）", orDash(startM), orDash(endM))
			}
			pdf.SetFont(pdfutil.FontSimHeiBold, "B", resumeBodyPt)
			pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
			pdf.MultiCell(resumeContentW, resumeLineH, title+period, "", "L", false)
			if desc != "" {
				pdf.SetFont(pdfutil.FontSimHei, "", resumeBodyPt-0.5)
				pdf.SetTextColor(resumeTextSub[0], resumeTextSub[1], resumeTextSub[2])
				pdf.MultiCell(resumeContentW, resumeLineH, desc, "", "L", false)
			}
			pdf.Ln(1.5)
		}
	}

	// 持证（只出名称，不出原图）
	var certs []map[string]any
	if err := json.Unmarshal(m.ResumeCertifications, &certs); err == nil && len(certs) > 0 {
		g.sectionTitle(pdf, "持证情况")
		for _, c := range certs {
			certNo, _ := c["cert_no"].(string)
			expire, _ := c["expire_date"].(string)
			line := "持证"
			if certNo != "" {
				line += "（证书编号 " + certNo + "）"
			}
			if expire != "" {
				line += " 有效期至 " + expire
			}
			pdf.SetFont(pdfutil.FontSimHei, "", resumeBodyPt)
			pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
			pdf.CellFormat(resumeContentW, resumeLineH, "• "+line, "", 1, "L", false, 0, "")
		}
	}
}

// sectionTitle 区块标题（带左侧强调条）。
func (g *ResumePDFRenderer) sectionTitle(pdf *gofpdf.Fpdf, title string) {
	pdf.Ln(1)
	y := pdf.GetY()
	pdf.SetFillColor(resumeAccent[0], resumeAccent[1], resumeAccent[2])
	pdf.Rect(pdfutil.PageMarginMm, y, 1.2, 6, "F")
	pdf.SetFont(pdfutil.FontSimHeiBold, "B", resumeH1Pt)
	pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
	pdf.SetX(pdfutil.PageMarginMm + 4)
	pdf.CellFormat(resumeContentW-4, 7, title, "", 1, "L", false, 0, "")
	pdf.Ln(1.5)
}

// labelValueRow 标签-值行。
func (g *ResumePDFRenderer) labelValueRow(pdf *gofpdf.Fpdf, label, value string) {
	pdf.SetFont(pdfutil.FontSimHei, "", resumeLabelPt)
	pdf.SetTextColor(resumeTextLbl[0], resumeTextLbl[1], resumeTextLbl[2])
	pdf.CellFormat(26, resumeLineH, label, "", 0, "L", false, 0, "")
	pdf.SetFont(pdfutil.FontSimHei, "", resumeBodyPt)
	pdf.SetTextColor(resumeText[0], resumeText[1], resumeText[2])
	pdf.CellFormat(resumeContentW-26, resumeLineH, value, "", 1, "L", false, 0, "")
}

// resumeRegions 解析 expected_regions JSONB 为字符串数组（兼容历史格式）。
func resumeRegions(raw model.JSONB) []string {
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, r := range arr {
		if v := strings.TrimSpace(r); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// cityLevelRegion 现居地截断到市（#485 打码口径 / #486 数据契约）：
//   - 两段「省/市」保留两段
//   - 直辖市一段保留一段
//   - 历史无分隔串（如「江苏苏州精确地址123号」）按省/市字典拆分到市
func cityLevelRegion(region string) string {
	r := strings.TrimSpace(region)
	if r == "" {
		return "-"
	}
	parts := strings.Split(r, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[:2], "/")
	}
	// 无分隔：直辖市整段保留
	if regionMunicipalities[r] {
		return r
	}
	// 尝试按省名 + 市名拆分（如「江苏苏州精确地址123号」→「江苏/苏州」）
	if prov, city := SplitRegionNoSeparator(r); city != "" {
		return prov + "/" + city
	}
	return r
}

func availableLabel(v string) string {
	m := map[string]string{"immediate": "随时", "1w": "1周内", "2w": "2周内", "1m": "1月内"}
	if s, ok := m[v]; ok {
		return s
	}
	if v == "" {
		return "-"
	}
	return v
}

func jobNatureLabel(v string) string {
	m := map[string]string{"fulltime": "全职", "parttime": "兼职", "contract": "合同"}
	if s, ok := m[v]; ok {
		return s
	}
	if v == "" {
		return "-"
	}
	return v
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
