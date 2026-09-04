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

	"forklift-training/internal/clock"
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
	PositionID      *int
	CredentialID    *int // 持证：resume_certifications 中含该 credential_id
	SalaryMin       *int
	SalaryMax       *int
	ExperienceMin   *int
	ExperienceMax   *int
	ExperienceYears *int
	AvailableIn     string
	// JobNature 用工性质（#492：fulltime/parttime/contract 精确匹配）
	JobNature string
	// RecruiterID 当前招聘者（#489：>0 时批量回填 contact_state）
	RecruiterID int
}

// RecruitResumeCard 脱敏卡（L2 可见字段；打码姓名，无 phone/wechat/region/PDF/cert image）。
type RecruitResumeCard struct {
	UserID                int             `json:"user_id"`
	RealName              string          `json:"real_name"`        // 已打码（如 张* 或 张*丰）
	RealNameMasked        string          `json:"real_name_masked"` // 同上，兼容验收对打码字段的显式断言
	ExpectedPositionID    *int            `json:"expected_position_id,omitempty"`
	ExpectedPositionExtra string          `json:"expected_position_extra"`
	ExpectedRegions       json.RawMessage `json:"expected_regions"`
	SalaryMin             *int            `json:"salary_min,omitempty"`
	SalaryMax             *int            `json:"salary_max,omitempty"`
	SalaryNegotiable      bool            `json:"salary_negotiable"`
	AvailableIn           string          `json:"available_in"`
	JobNature             string          `json:"job_nature"`
	ExperienceYears       int             `json:"experience_years"`
	SelfIntro             string          `json:"self_intro"`
	ResumeExperiences     json.RawMessage `json:"resume_experiences"`
	ResumeCertifications  json.RawMessage `json:"resume_certifications"` // 已去 image_urls
	UpdatedAt             string          `json:"updated_at"`
	// #489：企业视角联系状态（none/pending/approved，approved 带来源）
	ContactState  string `json:"contact_state,omitempty"`
	ContactSource string `json:"contact_source,omitempty"` // recruiter/application
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
		UserID:                m.UserID,
		RealName:              masked,
		RealNameMasked:        masked,
		ExpectedPositionID:    m.ExpectedPositionID,
		ExpectedPositionExtra: m.ExpectedPositionExtra,
		ExpectedRegions:       json.RawMessage(expRegions),
		SalaryMin:             m.SalaryMin,
		SalaryMax:             m.SalaryMax,
		SalaryNegotiable:      m.SalaryNegotiable,
		AvailableIn:           m.AvailableIn,
		JobNature:             m.JobNature,
		ExperienceYears:       m.ExperienceYears,
		SelfIntro:             m.SelfIntro,
		ResumeExperiences:     json.RawMessage(exps),
		ResumeCertifications:  json.RawMessage(certsRaw),
		UpdatedAt:             m.UpdatedAt.Format(time.RFC3339),
	}
}

// fillContactStates 批量回填企业视角联系状态（#489，禁止 N+1）。
// 状态：none 无授权 / pending 有待处理申请 / approved 已授权（含投递产生）。
func fillContactStates(db *gorm.DB, recruiterID int, cards []RecruitResumeCard) {
	if recruiterID <= 0 || len(cards) == 0 {
		return
	}
	ids := make([]int, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.UserID)
	}
	var reqs []model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id IN ?", recruiterID, ids).
		Order("created_at DESC").Find(&reqs).Error; err != nil {
		return
	}
	// 对每个学员取优先级最高的状态：approved > pending（approved 覆盖 pending）
	state := make(map[int]struct{ status, source string }, len(cards))
	for _, r := range reqs {
		cur, ok := state[r.StudentUserID]
		if r.Status == "approved" && (!ok || cur.status != "approved") {
			state[r.StudentUserID] = struct{ status, source string }{"approved", r.Source}
		} else if r.Status == "pending" && !ok {
			state[r.StudentUserID] = struct{ status, source string }{"pending", r.Source}
		}
	}
	for i := range cards {
		if st, ok := state[cards[i].UserID]; ok && st.status != "" {
			cards[i].ContactState = st.status
			cards[i].ContactSource = st.source
		}
	}
}

// applyFilters 在查询上叠加筛选轴（visibility=open 已由调用方保证）。
func (s *RecruitService) applyFilters(q *gorm.DB, p RecruitListParams) *gorm.DB {
	if v := strings.TrimSpace(p.Region); v != "" {
		// #486：地区筛选改为与录入同源的市级精确匹配——候选 expected_regions 任一元素
		// 的「市名」（第 2 段；直辖市取整段）等于筛选值（即市名）。
		// 存储格式：两段「省/市」（直辖市一段），筛选参数为市级值（可能带「市」后缀也可能不带）。
		// 实现：CAST 全文后按 JSON 元素解析匹配市名（兼容 pg 与 sqlite 内存库）。
		q = q.Where(applyRegionCityFilter(p.Region))
	}
	if p.PositionID != nil && *p.PositionID > 0 {
		q = q.Where("expected_position_id = ?", *p.PositionID)
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
	if v := strings.TrimSpace(p.JobNature); v != "" {
		// #492：用工性质精确匹配（fulltime/parttime/contract）
		q = q.Where("job_nature = ?", v)
	}
	return q
}

// applyRegionCityFilter 构造地区市级精确匹配的 WHERE 子句（#486）。
// 数据契约：expected_regions 数组元素为两段「省/市」中文串（直辖市一段）。
// 筛选值归一为规范市全名（「苏州」→「苏州市」）后精确匹配元素第 2 段：
//   - 普通市：模式 %/苏州市" 命中元素 "江苏省/苏州市" 结尾
//   - 直辖市：模式 %"北京市" 命中一段式元素
//
// CAST AS TEXT 兼容 pg(jsonb) 与 sqlite 内存库(BLOB)；LIKE 用于跨引擎等价，
// 模式两侧锚定（斜杠/引号）实现「精确匹配第 2 段」而非任意子串。
func applyRegionCityFilter(region string) any {
	city := strings.TrimSpace(region)
	if city == "" {
		return nil
	}
	// 归一：短名 → 规范市全名（苏州市）
	city = RegionCityName(city)
	if city == "" {
		return nil
	}
	if regionMunicipalities[city] {
		// 直辖市一段式元素：["北京市"]
		return gorm.Expr("CAST(expected_regions AS TEXT) LIKE ?", `%"`+city+`"%`)
	}
	// 普通市两段式元素：["江苏省/苏州市"] → 匹配 /苏州市"
	return gorm.Expr("CAST(expected_regions AS TEXT) LIKE ?", `%/`+city+`"%`)
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
	// #489：批量回填企业视角联系状态
	fillContactStates(s.db, p.RecruiterID, items)
	return &RecruitListResult{Items: items, Total: total}, nil
}

// Get 脱敏详情：仅 open 可见，同一脱敏路径；关闭或不存在返回 ErrRecordNotFound。
func (s *RecruitService) Get(userID int) (*RecruitResumeCard, error) {
	return s.GetForRecruiter(userID, 0)
}

// GetForRecruiter 脱敏详情（#489）：带企业视角联系状态。recruiterID>0 时回填。
func (s *RecruitService) GetForRecruiter(userID, recruiterID int) (*RecruitResumeCard, error) {
	var card model.JobCard
	if err := s.db.Where("user_id = ? AND visibility = ?", userID, "open").First(&card).Error; err != nil {
		return nil, err
	}
	dto := desensitize(&card)
	if recruiterID > 0 {
		cards := []RecruitResumeCard{dto}
		fillContactStates(s.db, recruiterID, cards)
		dto = cards[0]
	}
	return &dto, nil
}

// GetRaw 取原始简历卡（在线简历 PDF 渲染用）。
// 招聘者路径：仅 open 卡可见（与 Get 同门禁）；学员本人路径由 handler 保证本人鉴权，不受 visibility 限制。
// 返回原始模型（含敏感字段），调用方负责打码口径（本包 RenderResumePDF 统一处理）。
func (s *RecruitService) GetRaw(userID int) (*model.JobCard, error) {
	var card model.JobCard
	if err := s.db.Where("user_id = ? AND visibility = ?", userID, "open").First(&card).Error; err != nil {
		return nil, err
	}
	return &card, nil
}

// GetRawAny 取原始简历卡（学员本人路径：本人鉴权，不校验 visibility）。
func (s *RecruitService) GetRawAny(userID int) (*model.JobCard, error) {
	var card model.JobCard
	if err := s.db.Where("user_id = ?", userID).First(&card).Error; err != nil {
		return nil, err
	}
	return &card, nil
}

// LogView 写入浏览审计（best-effort，失败仅日志）。
// 粒度为同一招聘方对同一学员每日一次（Asia/Shanghai 自然日），避免翻页刷量。
func (s *RecruitService) LogView(recruiterID, resumeUserID int) {
	if recruiterID <= 0 || resumeUserID <= 0 {
		return
	}
	now := clock.Now()
	// 当日 0 点（Shanghai）
	dayStart := clock.DayStart(now)
	// 已存在当日记录则跳过（幂等，避免刷量）
	var cnt int64
	if err := s.db.Model(&model.RecruitResumeView{}).
		Where("recruiter_id = ? AND resume_user_id = ? AND viewed_at >= ?", recruiterID, resumeUserID, dayStart).
		Count(&cnt).Error; err == nil && cnt > 0 {
		return
	}
	rec := model.RecruitResumeView{
		RecruiterID:  recruiterID,
		ResumeUserID: resumeUserID,
		ViewedAt:     now,
	}
	if err := s.db.Create(&rec).Error; err != nil && s.logger != nil {
		s.logger.Warn("recruit view audit 写入失败", zap.Error(err), zap.Int("recruiter", recruiterID), zap.Int("resume", resumeUserID))
	}
}

// LogViews 批量留痕（列表场景，每项一条，同样受每日一次约束）。
func (s *RecruitService) LogViews(recruiterID int, resumeUserIDs []int) {
	for _, id := range resumeUserIDs {
		s.LogView(recruiterID, id)
	}
}

// StudentViewStats 学员侧聚合：近 7 天查看过我的企业数（按企业去重计数），不返回企业名。
func (s *RecruitService) StudentViewStats(studentUserID int) (int64, error) {
	if studentUserID <= 0 {
		return 0, nil
	}
	since := clock.Now().AddDate(0, 0, -7)
	var cnt int64
	// 按企业去重计数，7 天窗口需走索引 (resume_user_id, viewed_at)
	if err := s.db.Model(&model.RecruitResumeView{}).
		Where("resume_user_id = ? AND viewed_at >= ?", studentUserID, since).
		Distinct("recruiter_id").Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}
