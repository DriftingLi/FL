// Package service 招聘域：职位（job posting，spec #449 T2 #451）。
// 企业供给侧表达「我在招什么人」：职位名（岗位字典 + 可选自由细化）+ 地区/薪资/经验要求/职位描述
// + 地区/薪资/经验要求/职位描述，open/closed 二态，按发布新鲜度排序。
// 学员侧只见 open 且未被强制下架的职位；closed/强制下架职位企业自己仍能看到历史。
package service

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
)

// 职位域业务错误哨兵（ADR-0024）：handler 以 errors.Is 映射状态码，不做字符串比对。
var (
	// ErrJobNotFound 职位不存在（或不属于该招聘者）。
	ErrJobNotFound = errors.New("职位不存在")
	// ErrJobNotYours 不能操作他人的职位。
	ErrJobNotYours = errors.New("无权操作该职位")
	// ErrJobActiveLimit 单企业活跃职位数触顶（默认 50，宽松值只防误操作）。
	ErrJobActiveLimit = errors.New("活跃职位数已达上限（50 个），请先下架部分职位")
	// ErrJobForcedOffline 被管理员强制下架的职位不能自行重新上架。
	ErrJobForcedOffline = errors.New("该职位已被强制下架，不能自行重新上架，请联系平台处理")
	// ErrJobPositionRequired 岗位字典在业务层必填（问题4：与专业方向解绑）。
	ErrJobPositionRequired = errors.New("岗位不能为空")
)

// JobPostingService 职位服务。
type JobPostingService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewJobPostingService 创建职位服务。
func NewJobPostingService(db *gorm.DB, logger *zap.Logger) *JobPostingService {
	return &JobPostingService{db: db, logger: logger}
}

// maxActiveJobs 单企业活跃职位上限（宽松值，只防误操作不防人）。
const maxActiveJobs = 50

// JobPostingInput 职位创建/编辑入参。
type JobPostingInput struct {
	Title         string `json:"title"`
	PositionID    *int   `json:"position_id"`
	Region        string `json:"region"`
	SalaryMin     *int   `json:"salary_min"`
	SalaryMax     *int   `json:"salary_max"`
	SalaryText    string `json:"salary_text"`
	ExperienceReq string `json:"experience_req"`
	Description   string `json:"description"`
}

// JobPostingDTO 职位展示对象（企业侧与学员侧共用，字段按角色裁剪由 handler 决定）。
type JobPostingDTO struct {
	ID            int    `json:"id"`
	RecruiterID   int    `json:"recruiter_id"`
	Title         string `json:"title"`
	PositionID    *int   `json:"position_id,omitempty"`
	PositionName  string `json:"position_name,omitempty"`
	Region        string `json:"region"`
	SalaryMin     *int   `json:"salary_min,omitempty"`
	SalaryMax     *int   `json:"salary_max,omitempty"`
	SalaryText    string `json:"salary_text"`
	ExperienceReq string `json:"experience_req"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	ForcedOffline bool   `json:"forced_offline"`
	OfflineReason string `json:"offline_reason,omitempty"`
	PublishedAt   string `json:"published_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	// 企业信息（学员侧可见，不含电话/邮箱/信用代码）
	CompanyName   string `json:"company_name,omitempty"`
	BusinessScope string `json:"business_scope,omitempty"`
	ContactName   string `json:"contact_name,omitempty"`
	// 学员视角投递状态（#488）：none 可投递 / applied 已投递 / not_hired 未录用（30 天冷却中）
	ApplyState   string `json:"apply_state,omitempty"`
	CooldownDays int    `json:"cooldown_days,omitempty"` // not_hired 时距可再投天数（向上取整）
}

// validate 校验职位入参（岗位字典业务层必填）。
func validateJobPostingInput(in *JobPostingInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return errors.New("职位名不能为空")
	}
	if len([]rune(in.Title)) > 100 {
		return errors.New("职位名不能超过 100 字")
	}
	if in.PositionID == nil || *in.PositionID <= 0 {
		return ErrJobPositionRequired
	}
	if len([]rune(in.Description)) > 5000 {
		return errors.New("职位描述不能超过 5000 字")
	}
	if in.SalaryMin != nil && in.SalaryMax != nil && *in.SalaryMin > *in.SalaryMax {
		return errors.New("薪资区间下限不能高于上限")
	}
	return nil
}

// toDTO 转换 DB 行为 DTO，带企业信息（学员侧可见字段）。
func (s *JobPostingService) toDTO(m *model.JobPosting) JobPostingDTO {
	dto := JobPostingDTO{
		ID:            m.ID,
		RecruiterID:   m.RecruiterID,
		Title:         m.Title,
		PositionID:    m.PositionID,
		Region:        m.Region,
		SalaryMin:     m.SalaryMin,
		SalaryMax:     m.SalaryMax,
		SalaryText:    m.SalaryText,
		ExperienceReq: m.ExperienceReq,
		Description:   m.Description,
		Status:        m.Status,
		ForcedOffline: m.ForcedOffline,
		OfflineReason: m.OfflineReason,
		PublishedAt:   m.PublishedAt.Format(time.RFC3339),
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     m.UpdatedAt.Format(time.RFC3339),
	}
	if m.PositionID != nil {
		var pos model.Position
		if err := s.db.First(&pos, *m.PositionID).Error; err == nil {
			dto.PositionName = pos.Name
		}
	}
	var rec model.RecruiterUser
	if err := s.db.First(&rec, m.RecruiterID).Error; err == nil {
		dto.CompanyName = rec.CompanyName
		dto.BusinessScope = rec.BusinessScope
		dto.ContactName = rec.ContactName
	}
	return dto
}

// Create 企业发布职位（recruiterID 即企业，账号即企业）。
func (s *JobPostingService) Create(recruiterID int, in *JobPostingInput) (*JobPostingDTO, error) {
	if err := validateJobPostingInput(in); err != nil {
		return nil, err
	}
	// 活跃职位上限：status=open 且未被强制下架的职位数
	var active int64
	if err := s.db.Model(&model.JobPosting{}).Where("recruiter_id = ? AND status = ? AND forced_offline = ?", recruiterID, "open", false).Count(&active).Error; err != nil {
		return nil, err
	}
	if active >= maxActiveJobs {
		return nil, ErrJobActiveLimit
	}
	now := clock.Now()
	m := model.JobPosting{
		RecruiterID:   recruiterID,
		Title:         strings.TrimSpace(in.Title),
		PositionID:    in.PositionID,
		Region:        strings.TrimSpace(in.Region),
		SalaryMin:     in.SalaryMin,
		SalaryMax:     in.SalaryMax,
		SalaryText:    strings.TrimSpace(in.SalaryText),
		ExperienceReq: strings.TrimSpace(in.ExperienceReq),
		Description:   strings.TrimSpace(in.Description),
		Status:        "open",
		PublishedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.Create(&m).Error; err != nil {
		return nil, err
	}
	dto := s.toDTO(&m)
	return &dto, nil
}

// Update 企业编辑自己的职位（title/岗位/地区/薪资/经验/描述；状态走 ToggleStatus）。
func (s *JobPostingService) Update(recruiterID, jobID int, in *JobPostingInput) (*JobPostingDTO, error) {
	if err := validateJobPostingInput(in); err != nil {
		return nil, err
	}
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if m.RecruiterID != recruiterID {
		return nil, ErrJobNotYours
	}
	updates := map[string]any{
		"title":          strings.TrimSpace(in.Title),
		"position_id":    in.PositionID,
		"region":         strings.TrimSpace(in.Region),
		"salary_min":     in.SalaryMin,
		"salary_max":     in.SalaryMax,
		"salary_text":    strings.TrimSpace(in.SalaryText),
		"experience_req": strings.TrimSpace(in.ExperienceReq),
		"description":    strings.TrimSpace(in.Description),
		"updated_at":     clock.Now(),
	}
	if err := s.db.Model(&model.JobPosting{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&m, jobID).Error
	dto := s.toDTO(&m)
	return &dto, nil
}

// ToggleStatus 企业上架/下架自己的职位（open<->closed；强制下架职位不能自行重新上架）。
func (s *JobPostingService) ToggleStatus(recruiterID, jobID int) (*JobPostingDTO, error) {
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if m.RecruiterID != recruiterID {
		return nil, ErrJobNotYours
	}
	if m.ForcedOffline && m.Status == "open" {
		return nil, ErrJobForcedOffline
	}
	next := "closed"
	if m.Status == "closed" {
		if m.ForcedOffline {
			return nil, ErrJobForcedOffline
		}
		next = "open"
		// 重新上架也要受活跃上限约束
		var active int64
		if err := s.db.Model(&model.JobPosting{}).Where("recruiter_id = ? AND status = ? AND forced_offline = ?", recruiterID, "open", false).Count(&active).Error; err != nil {
			return nil, err
		}
		if active >= maxActiveJobs {
			return nil, ErrJobActiveLimit
		}
	}
	if err := s.db.Model(&model.JobPosting{}).Where("id = ?", jobID).Update("status", next).Error; err != nil {
		return nil, err
	}
	_ = s.db.First(&m, jobID).Error
	dto := s.toDTO(&m)
	return &dto, nil
}

// GetForStudent 学员职位详情（#488）：仅 open 且未强制下架，回填学员视角投递状态。
func (s *JobPostingService) GetForStudent(studentUserID, jobID int) (*JobPostingDTO, error) {
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if m.Status != "open" || m.ForcedOffline {
		return nil, ErrJobNotFound
	}
	dto := s.toDTO(&m)
	if studentUserID > 0 {
		list := []JobPostingDTO{dto}
		s.fillApplyStates(studentUserID, list)
		dto = list[0]
	}
	return &dto, nil
}

// JobListParams 职位列表筛选参数（学员侧与企业管理侧共用）。
type JobListParams struct {
	Page       int
	PageSize   int
	PositionID *int
	Region     string
	SalaryMin  *int
	SalaryMax  *int
	Experience string
	// StudentUserID 学员视角（#488）：>0 时列表项回填 apply_state/cooldown_days。
	StudentUserID int
	// MineOnly 只看自己的职位（企业管理侧）。
	MineOnly bool
	// RecruiterID 按企业过滤（管理端巡检用；>0 时生效）。
	RecruiterID int
	// All 管理端巡检：不过滤状态（含 closed/强制下架），仅按需企业筛。
	All bool
	// IncludeHidden 是否包含 closed/强制下架（企业侧看历史；学员侧恒 false）。
	IncludeHidden bool
}

// JobListResult 职位分页列表。
type JobListResult struct {
	Items []JobPostingDTO `json:"items"`
	Total int64           `json:"total"`
}

// List 职位列表。学员侧：只见 open 且未强制下架，按新鲜度排序；
// 企业侧（MineOnly）：含 closed/强制下架历史，同样按新鲜度排序。
func (s *JobPostingService) List(recruiterID int, p JobListParams) (*JobListResult, error) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 || p.PageSize > 50 {
		p.PageSize = 20
	}
	q := s.db.Model(&model.JobPosting{})
	if p.MineOnly {
		q = q.Where("recruiter_id = ?", recruiterID)
	} else if p.All {
		// 管理端巡检：全量（含 closed/强制下架），可按企业筛
		if p.RecruiterID > 0 {
			q = q.Where("recruiter_id = ?", p.RecruiterID)
		}
	} else if p.RecruiterID > 0 {
		q = q.Where("recruiter_id = ?", p.RecruiterID)
	} else {
		q = q.Where("status = ? AND forced_offline = ?", "open", false)
	}
	if p.PositionID != nil {
		q = q.Where("position_id = ?", *p.PositionID)
	}
	if p.Region != "" {
		q = q.Where("region LIKE ?", "%"+p.Region+"%")
	}
	if p.SalaryMin != nil {
		q = q.Where("salary_max IS NULL OR salary_max >= ?", *p.SalaryMin)
	}
	if p.SalaryMax != nil {
		q = q.Where("salary_min IS NULL OR salary_min <= ?", *p.SalaryMax)
	}
	if p.Experience != "" {
		q = q.Where("experience_req LIKE ?", "%"+p.Experience+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.JobPosting
	if err := q.Order("published_at DESC, id DESC").Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]JobPostingDTO, 0, len(rows))
	for i := range rows {
		items = append(items, s.toDTO(&rows[i]))
	}
	// #488：学员视角批量回填投递状态
	s.fillApplyStates(p.StudentUserID, items)
	return &JobListResult{Items: items, Total: total}, nil
}

// fillApplyStates 批量回填学员视角投递状态（#488：单次批量 join，禁止 N+1）。
// 入参 dtos 为学员可见的职位列表；对每个职位批量查该学员的投递记录判定状态。
func (s *JobPostingService) fillApplyStates(studentUserID int, dtos []JobPostingDTO) {
	if studentUserID <= 0 || len(dtos) == 0 {
		return
	}
	ids := make([]int, 0, len(dtos))
	for _, d := range dtos {
		ids = append(ids, d.ID)
	}
	// 批量 join：该学员对这些职位的最近投递（applied 或 rejected-with-cooldown）
	var apps []model.JobApplication
	if err := s.db.Where("job_posting_id IN ? AND student_user_id = ?", ids, studentUserID).
		Order("created_at DESC").Find(&apps).Error; err != nil {
		return
	}
	byJob := make(map[int][]model.JobApplication, len(dtos))
	for _, a := range apps {
		byJob[a.JobPostingID] = append(byJob[a.JobPostingID], a)
	}
	now := clock.Now()
	for i := range dtos {
		appList := byJob[dtos[i].ID]
		if len(appList) == 0 {
			continue
		}
		// 取最新的 applied 或 rejected 判定
		for _, a := range appList {
			if a.Status == "applied" {
				dtos[i].ApplyState = "applied"
				break
			}
			if a.Status == "rejected" && a.RejectedAt != nil {
				// 冷却 30 天（沿用既有规则），冷却期内 not_hired，期满恢复可投
				remaining := 30*24*time.Hour - now.Sub(*a.RejectedAt)
				if remaining > 0 {
					dtos[i].ApplyState = "not_hired"
					dtos[i].CooldownDays = int(remaining.Hours()/24) + 1
				}
				break
			}
		}
	}
}

// Get 职位详情。recruiterID>0 表示企业侧（可看自己的 closed/强制下架历史），
// recruiterID=0 表示学员侧（仅 open 且未强制下架）。
func (s *JobPostingService) Get(recruiterID, jobID int) (*JobPostingDTO, error) {
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if recruiterID > 0 {
		if m.RecruiterID != recruiterID {
			return nil, ErrJobNotYours
		}
	} else if m.Status != "open" || m.ForcedOffline {
		return nil, ErrJobNotFound
	}
	dto := s.toDTO(&m)
	return &dto, nil
}
