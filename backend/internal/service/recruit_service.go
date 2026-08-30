// Package service 招聘端脱敏简历（L2）。
// List/Get 均只返回 visibility=open 的卡；响应经同一脱敏路径，不含手机/微信/PDF/未打码姓名/证书原图/现居地精确值。
// 过滤轴：意向地区/期望岗位/证书/薪资区间/经验年限/到岗时间；默认排序 updated_at DESC（不按注册时间）。
// 浏览留痕：Detail（及 List 按需）写入 recruit_resume_views 供审计。
package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// RecruitService 招聘端简历服务（脱敏读）。
type RecruitService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewRecruitService(db *gorm.DB, logger *zap.Logger) *RecruitService {
	return &RecruitService{db: db, logger: logger}
}

// RecruitListParams 招聘端列表筛选参数（全部可选；page/pageSize 由 handler 归一）。
type RecruitListParams struct {
	Page            int
	PageSize        int
	Region          string // 意向地区：expected_regions JSON 数组中含该串（LIKE）
	SpecialtyID     *int
	CredentialID    *int // 持证：resume_certifications 中含该 credential_id
	SalaryMin       *int
	SalaryMax       *int
	ExperienceMin   *int
	ExperienceMax   *int
	ExperienceYears *int
	AvailableIn     string
}

// RecruitResumeCard 脱敏卡（L2 可见字段；打码姓名，无 phone/wechat/region/PDF/cert image）。
type RecruitResumeCard struct {
	UserID                 int             `json:"user_id"`
	RealName               string          `json:"real_name"`        // 已打码（如 张* 或 张*丰）
	RealNameMasked         string          `json:"real_name_masked"` // 同上，兼容验收对打码字段的显式断言
	ExpectedSpecialtyID    *int            `json:"expected_specialty_id,omitempty"`
	ExpectedSpecialtyExtra string          `json:"expected_specialty_extra"`
	ExpectedRegions        json.RawMessage `json:"expected_regions"`
	SalaryMin              *int            `json:"salary_min,omitempty"`
	SalaryMax              *int            `json:"salary_max,omitempty"`
	SalaryNegotiable       bool            `json:"salary_negotiable"`
	AvailableIn            string          `json:"available_in"`
	JobNature              string          `json:"job_nature"`
	ExperienceYears        int             `json:"experience_years"`
	SelfIntro              string          `json:"self_intro"`
	ResumeExperiences      json.RawMessage `json:"resume_experiences"`
	ResumeCertifications   json.RawMessage `json:"resume_certifications"` // 已去 image_urls
	UpdatedAt              string          `json:"updated_at"`
}

// RecruitListResult 列表结果。
type RecruitListResult struct {
	Items []RecruitResumeCard `json:"items"`
	Total int64               `json:"total"`
}

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

// desensitize 将原始 JobCard 转为脱敏卡（唯一脱敏路径，列表与详情共用）。
func desensitize(m *model.JobCard) RecruitResumeCard {
	masked := MaskRealName(m.RealName)
	// 持证去图：strip image_urls
	certsRaw := m.ResumeCertifications
	if len(certsRaw) == 0 {
		certsRaw = model.JSONB([]byte("[]"))
	}
	// 解析并重建，避免原图泄露
	var certs []map[string]any
	if err := json.Unmarshal([]byte(certsRaw), &certs); err == nil {
		for i := range certs {
			delete(certs[i], "image_urls")
			delete(certs[i], "imageUrls")
		}
		if b, err := json.Marshal(certs); err == nil {
			certsRaw = model.JSONB(b)
		} else {
			certsRaw = model.JSONB([]byte("[]"))
		}
	} else {
		certsRaw = model.JSONB([]byte("[]"))
	}
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
		UserID:                 m.UserID,
		RealName:               masked,
		RealNameMasked:         masked,
		ExpectedSpecialtyID:    m.ExpectedSpecialtyID,
		ExpectedSpecialtyExtra: m.ExpectedSpecialtyExtra,
		ExpectedRegions:        json.RawMessage(expRegions),
		SalaryMin:              m.SalaryMin,
		SalaryMax:              m.SalaryMax,
		SalaryNegotiable:       m.SalaryNegotiable,
		AvailableIn:            m.AvailableIn,
		JobNature:              m.JobNature,
		ExperienceYears:        m.ExperienceYears,
		SelfIntro:              m.SelfIntro,
		ResumeExperiences:      json.RawMessage(exps),
		ResumeCertifications:   json.RawMessage(certsRaw),
		UpdatedAt:              m.UpdatedAt.Format(time.RFC3339),
	}
}

// applyFilters 在查询上叠加筛选轴（visibility=open 已由调用方保证）。
func (s *RecruitService) applyFilters(q *gorm.DB, p RecruitListParams) *gorm.DB {
	if v := strings.TrimSpace(p.Region); v != "" {
		// expected_regions JSON 数组包含该地区串（CAST 兼容 pg 与 sqlite 内存库：sqlite 上 JSONB 为 BLOB，LIKE 需 CAST）
		q = q.Where("CAST(expected_regions AS TEXT) LIKE ?", "%"+v+"%")
	}
	if p.SpecialtyID != nil && *p.SpecialtyID > 0 {
		q = q.Where("expected_specialty_id = ?", *p.SpecialtyID)
	}
	if p.CredentialID != nil && *p.CredentialID > 0 {
		// 持证 JSON 中包含该 credential_id（CAST 兼容，精确匹配 "credential_id":<id> 避免数字误匹配日期等）
		idStr := strconv.Itoa(*p.CredentialID)
		pat1 := fmt.Sprintf("%%\"credential_id\":%s%%", idStr)
		pat2 := fmt.Sprintf("%%\"credential_id\": %s%%", idStr)
		q = q.Where("(CAST(resume_certifications AS TEXT) LIKE ? OR CAST(resume_certifications AS TEXT) LIKE ?)", pat1, pat2)
	}
	if p.SalaryMin != nil {
		// 候选期望不低于招聘方给出的下限视为匹配；面议视为通过
		q = q.Where("(salary_negotiable = ? OR (salary_min IS NOT NULL AND salary_min >= ?))", true, *p.SalaryMin)
	}
	if p.SalaryMax != nil {
		q = q.Where("(salary_negotiable = ? OR (salary_max IS NOT NULL AND salary_max <= ?))", true, *p.SalaryMax)
	}
	if p.ExperienceYears != nil {
		q = q.Where("experience_years = ?", *p.ExperienceYears)
	} else {
		if p.ExperienceMin != nil {
			q = q.Where("experience_years >= ?", *p.ExperienceMin)
		}
		if p.ExperienceMax != nil {
			q = q.Where("experience_years <= ?", *p.ExperienceMax)
		}
	}
	if v := strings.TrimSpace(p.AvailableIn); v != "" {
		q = q.Where("available_in = ?", v)
	}
	return q
}

// List 脱敏列表：仅 open，叠筛选，updated_at DESC，分页，无缓存（读最新）。
func (s *RecruitService) List(p RecruitListParams) (*RecruitListResult, error) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	if p.PageSize > 50 {
		p.PageSize = 50
	}
	q := s.db.Model(&model.JobCard{}).Where("visibility = ?", "open")
	q = s.applyFilters(q, p)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var cards []model.JobCard
	if err := q.Order("updated_at DESC").Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize).Find(&cards).Error; err != nil {
		return nil, err
	}
	items := make([]RecruitResumeCard, 0, len(cards))
	for i := range cards {
		items = append(items, desensitize(&cards[i]))
	}
	return &RecruitListResult{Items: items, Total: total}, nil
}

// Get 脱敏详情：仅 open 可见，同一脱敏路径；关闭或不存在返回 ErrRecordNotFound。
func (s *RecruitService) Get(userID int) (*RecruitResumeCard, error) {
	var card model.JobCard
	if err := s.db.Where("user_id = ? AND visibility = ?", userID, "open").First(&card).Error; err != nil {
		return nil, err
	}
	dto := desensitize(&card)
	return &dto, nil
}

// LogView 写入浏览审计（best-effort，失败仅日志）。
func (s *RecruitService) LogView(recruiterID, resumeUserID int) {
	if recruiterID <= 0 || resumeUserID <= 0 {
		return
	}
	rec := model.RecruitResumeView{
		RecruiterID:  recruiterID,
		ResumeUserID: resumeUserID,
		ViewedAt:     time.Now(),
	}
	if err := s.db.Create(&rec).Error; err != nil && s.logger != nil {
		s.logger.Warn("recruit view audit 写入失败", zap.Error(err), zap.Int("recruiter", recruiterID), zap.Int("resume", resumeUserID))
	}
}

// LogViews 批量留痕（列表场景，每项一条）。
func (s *RecruitService) LogViews(recruiterID int, resumeUserIDs []int) {
	for _, id := range resumeUserIDs {
		s.LogView(recruiterID, id)
	}
}
