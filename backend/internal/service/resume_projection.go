// 本文件：简历卡投影模块（ADR-0053 §4）——打码规则与**三处投影的字段清单**同文件。
//
// 为什么并排放：这三处投影的口径**有意不同**（词表已区分），但「新增一个简历字段时要在三处
// 各决定一次（露 / 不露 / 怎么露）」这件事没法靠记忆。把三份清单摆在一起，改动时一眼可见。
//
//  1. desensitize   企业浏览用脱敏卡（L2）：姓名打码、无现居地、无电话微信、无上传 PDF、无证件原图
//  2. toJobCardDTO  本人与已授权方用明文卡：全字段（授权门禁在 contact_authz.go）
//  3. resumePDFView 打码版在线简历 PDF：姓名打码、无电话微信、现居地到市、无工作照与证件原图
//     （版式与字体留在 resume_pdf.go，本文件只给**取值**）
//
// 打码规则三件（都只有一份实现，三处共用）：姓名打码 / 持证去图 / 现居地截断到市。
// 刻意**不做**字段级声明表或 DSL——那会让三个不同形状的投影退化为表配置（本仓已否过同类硬声明化）。
package service

import (
	"encoding/json"
	"strings"
	"time"

	"forklift-training/internal/model"
)

// ===== 打码规则（单份实现） =====

// MaskRealName 真实姓名打码：1 字→*，2 字→首字+*，≥3 字→首字+中间*+尾字。
func MaskRealName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return ""
	}
	rs := []rune(s)
	n := len(rs)
	if n == 1 {
		return "*"
	}
	if n == 2 {
		return string(rs[0]) + "*"
	}
	return string(rs[0]) + strings.Repeat("*", n-2) + string(rs[n-1])
}

// resumeCertSafeKeys 脱敏/打码面**允许保留**的持证键（白名单）。
//
// 为什么是白名单：持证是 JSON 结构，此前用的是「删掉已知的 image_urls / imageUrls」——
// 将来多一个带图或带隐私的键（备注、审核状态、原件扫描件…）就会静默跟着露出去。
// 改成白名单后，新键的默认归属是「不露面」：要么进这张表（并想清楚为什么安全），要么不出去。
var resumeCertSafeKeys = []string{"credential_id", "cert_no", "expire_date"}

// maskCertifications 持证去图：按白名单**重建**每个条目，只保留已知安全键。
// 入参为脏数据（非法 JSON）时返回空数组（fail-closed，不留半份原始内容）。
func maskCertifications(raw model.JSONB) []map[string]any {
	var certs []map[string]any
	if err := json.Unmarshal([]byte(raw), &certs); err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(certs))
	for _, cert := range certs {
		safe := make(map[string]any, len(resumeCertSafeKeys))
		for _, key := range resumeCertSafeKeys {
			if v, ok := cert[key]; ok {
				safe[key] = v
			}
		}
		out = append(out, safe)
	}
	return out
}

// maskCertificationsJSON maskCertifications 的 JSONB 形态（脱敏卡字段是 JSON 直出）。
func maskCertificationsJSON(raw model.JSONB) model.JSONB {
	b, err := json.Marshal(maskCertifications(raw))
	if err != nil {
		return model.JSONB([]byte("[]"))
	}
	return model.JSONB(b)
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
	// #486：直辖市一段式——即使存量出现「北京市/东城区」等两段，也只保留一段（区不入 PDF）
	if regionMunicipalities[parts[0]] {
		return parts[0]
	}
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

// ===== 投影 1：企业浏览用脱敏卡（L2） =====
//
// 三格同为 JSONArray，表态却**两样**（recruit_service.go 里那两条 nullability tag），差别在
// 投影有没有把它重建过：
//   - expected_regions / resume_experiences 只加了一道 `len(x)==0 → "[]"` 的守卫，列里存着
//     4 字节的 JSON `null` 时原样透传 ⇒ 保持 nullable。
//   - resume_certifications 走 maskCertificationsJSON：解码成 []map 再 make(...,0,n) 重编，
//     解码失败与解出 null 都落到 `[]` ⇒ 任何列内容下都不是 null，所以 nonnil。
//
// 判据 5 跑一次成功出口分不出这两种（两种都能量到 `[]`）——非 null 来自代码还是来自列默认值，
// 只能读这里。也是「改判前先读投影」这条纪律的出处。
//
// 字段清单：姓名（打码）/ 期望岗位 / 意向地区 / 薪资 / 到岗 / 用工性质 / 年限 / 自述 /
// 工作经历 / 持证（去图）/ 更新时间。
// 刻意**不含**：真实姓名、现居地、电话、微信、上传 PDF、工作照、证件原图。
func desensitize(m *model.JobCard) RecruitResumeCard {
	masked := MaskRealName(m.RealName)
	certsRaw := maskCertificationsJSON(m.ResumeCertifications)
	// expected_regions / experiences 保持原样（无敏感字段）
	expRegions := m.ExpectedRegions
	if len(expRegions) == 0 {
		expRegions = model.JSONB([]byte("[]"))
	}
	exps := m.ResumeExperiences
	if len(exps) == 0 {
		exps = model.JSONB([]byte("[]"))
	}
	return RecruitResumeCard{
		UserID:                m.UserID,
		RealName:              masked,
		RealNameMasked:        masked,
		ExpectedPositionID:    m.ExpectedPositionID,
		ExpectedPositionExtra: m.ExpectedPositionExtra,
		ExpectedRegions:       JSONArray(expRegions),
		SalaryMin:             m.SalaryMin,
		SalaryMax:             m.SalaryMax,
		SalaryNegotiable:      m.SalaryNegotiable,
		AvailableIn:           m.AvailableIn,
		JobNature:             m.JobNature,
		ExperienceYears:       m.ExperienceYears,
		SelfIntro:             m.SelfIntro,
		ResumeExperiences:     JSONArray(exps),
		ResumeCertifications:  JSONArray(certsRaw),
		UpdatedAt:             m.UpdatedAt.Format(time.RFC3339),
	}
}

// ===== 投影 2：本人 / 已授权方用明文卡 =====
//
// 字段清单：全字段（含真实姓名、现居地、电话、微信、上传 PDF、工作照、证件含原图）。
// 门禁不在本函数：谁能拿到它由 contact_authz.go 决定（本人恒可、企业需有效授权）。
func toJobCardDTO(m *model.JobCard) JobCardDTO {
	return JobCardDTO{
		UserID:                m.UserID,
		RealName:              m.RealName,
		ContactPhone:          m.ContactPhone,
		Wechat:                m.Wechat,
		Region:                m.Region,
		ExpectedPositionID:    m.ExpectedPositionID,
		ExpectedPositionExtra: m.ExpectedPositionExtra,
		ExpectedRegions:       JSONArray(m.ExpectedRegions),
		SalaryMin:             m.SalaryMin,
		SalaryMax:             m.SalaryMax,
		SalaryNegotiable:      m.SalaryNegotiable,
		AvailableIn:           m.AvailableIn,
		JobNature:             m.JobNature,
		ExperienceYears:       m.ExperienceYears,
		SelfIntro:             m.SelfIntro,
		ResumeExperiences:     JSONArray(m.ResumeExperiences),
		ResumeCertifications:  JSONArray(m.ResumeCertifications),
		ResumeFileURL:         m.ResumeFileURL,
		Photos:                JSONArray(m.Photos),
		Visibility:            m.Visibility,
		CreatedAt:             m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             m.UpdatedAt.Format(time.RFC3339),
	}
}

// ===== 投影 3：打码版在线简历 PDF =====

// resumePDFView 打码版 PDF 的**取值**（版式在 resume_pdf.go）。
//
// 字段清单：姓名（打码）/ 期望岗位 / 意向地区 / 薪资 / 现居地（到市）/ 年限 / 到岗 / 用工性质 /
// 自述 / 工作经历 / 持证（去图，只出编号与有效期）。
// 刻意**不含**：电话、微信、工作照、上传 PDF、证件原图——所以「PDF 不会泄露联系方式」
// 是结构上成立的，而不是靠渲染函数记得别去读。
type resumePDFView struct {
	RealNameMasked   string
	PositionExtra    string
	PositionPicked   bool // 只选了字典项（无自由文本）时为 true
	Regions          []string
	SalaryNegotiable bool
	SalaryMin        *int
	SalaryMax        *int
	RegionCity       string
	ExperienceYears  int
	AvailableIn      string
	JobNature        string
	SelfIntro        string
	Experiences      []map[string]any
	Certifications   []map[string]any
}

// resumePDFViewOf 由简历卡投影出打码版 PDF 的取值（打码规则与脱敏卡共用同一份实现）。
func resumePDFViewOf(m *model.JobCard) resumePDFView {
	var exps []map[string]any
	if err := json.Unmarshal(m.ResumeExperiences, &exps); err != nil {
		exps = nil
	}
	return resumePDFView{
		RealNameMasked:   MaskRealName(m.RealName),
		PositionExtra:    m.ExpectedPositionExtra,
		PositionPicked:   m.ExpectedPositionExtra == "" && m.ExpectedPositionID != nil,
		Regions:          resumeRegions(m.ExpectedRegions),
		SalaryNegotiable: m.SalaryNegotiable,
		SalaryMin:        m.SalaryMin,
		SalaryMax:        m.SalaryMax,
		RegionCity:       cityLevelRegion(m.Region),
		ExperienceYears:  m.ExperienceYears,
		AvailableIn:      m.AvailableIn,
		JobNature:        m.JobNature,
		SelfIntro:        m.SelfIntro,
		Experiences:      exps,
		Certifications:   maskCertifications(m.ResumeCertifications),
	}
}
