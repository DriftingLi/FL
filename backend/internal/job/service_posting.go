// Package job 招聘域：职位（job posting，spec #449 T2 #451）。
// 企业供给侧表达「我在招什么人」：职位名（岗位字典 + 可选自由细化）+ 地区/薪资/经验要求/职位描述
// + 所需证件（单一「要求」档，可空多选，#2066），open/closed 二态，按发布新鲜度排序。
// 学员侧只见 open 且未被强制下架的职位；closed/强制下架职位企业自己仍能看到历史。
package job

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/pkg/paging"
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
	// ErrJobCredentialInvalid 所需证件里有非正数或字典里不存在的 id（#2066）。
	// 「不传 / 空数组」不算非法——两者都表意「不限证件」，与字段缺省等价。
	ErrJobCredentialInvalid = errors.New("所需证件不存在")
)

// Service 职位服务。
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService 创建职位服务。
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
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
	// RequiredCredentialIDs 所需证件（#2066）：可空多选，空 / 缺省 = 不限。
	// 编辑语义与其它字段一致——整组替换（空数组即清空）；重复 id 去重，非法 id → 400。
	RequiredCredentialIDs []int `json:"required_credential_ids"`
}

// JobPostingDTO 职位展示对象（企业侧与学员侧共用，字段按角色裁剪由 handler 决定）。
type JobPostingDTO struct {
	ID            int    `json:"id"`
	RecruiterID   int    `json:"recruiter_id"`
	Title         string `json:"title"`
	PositionID    *int   `json:"position_id,omitempty" extensions:"x-optional"`
	PositionName  string `json:"position_name,omitempty" extensions:"x-optional"`
	Region        string `json:"region"`
	SalaryMin     *int   `json:"salary_min,omitempty" extensions:"x-optional"`
	SalaryMax     *int   `json:"salary_max,omitempty" extensions:"x-optional"`
	SalaryText    string `json:"salary_text"`
	ExperienceReq string `json:"experience_req"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	ForcedOffline bool   `json:"forced_offline"`
	OfflineReason string `json:"offline_reason,omitempty" extensions:"x-optional"`
	PublishedAt   string `json:"published_at"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	// 企业信息（学员侧可见，不含电话/邮箱/信用代码）
	CompanyName   string `json:"company_name,omitempty" extensions:"x-optional"`
	BusinessScope string `json:"business_scope,omitempty" extensions:"x-optional"`
	ContactName   string `json:"contact_name,omitempty" extensions:"x-optional"`
	// 学员视角投递状态（#488）：none 可投递 / applied 已投递 / not_hired 未录用（30 天冷却中）
	ApplyState   string `json:"apply_state,omitempty" extensions:"x-optional"`
	CooldownDays int    `json:"cooldown_days,omitempty" extensions:"x-optional"` // not_hired 时距可再投天数（向上取整）
	// RequiredCredentials 所需证件（#2066）：空集 = 不限，恒发 []（不因无要求而缺席）。
	// 键在场是为了让「这个职位不限证件」与「这个字段没实现」在消费端可区分。
	RequiredCredentials []JobCredentialDTO `json:"required_credentials" nullability:"nonnil"`
}

// JobCredentialDTO 职位所需证件的简述（id + 名称）：职位卡据此渲染「需持 X 证」标签，
// 完整字典项（编码/类别/排序）由证件字典接口提供，这里只给渲染所需的最小集合。
type JobCredentialDTO struct {
	CredentialID int    `json:"credential_id"`
	Name         string `json:"name"`
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

// validRequiredCredentialIDs 校验并归一 required_credential_ids（#2066）：
// 空 / 缺省 → (nil, nil)，与「不限证件」等价；重复 id 去重保序；
// 非正数或字典里不存在的 id → ErrJobCredentialInvalid（HTTP 面 400）。
func (s *Service) validRequiredCredentialIDs(ids []int) ([]int, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[int]bool, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrJobCredentialInvalid
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	var found int64
	if err := s.db.Model(&model.Credential{}).Where("id IN ?", out).Count(&found).Error; err != nil {
		return nil, err
	}
	if int(found) != len(out) {
		return nil, ErrJobCredentialInvalid
	}
	return out, nil
}

// replaceJobCredentials 整组替换某职位的要求集合（空集 = 清空 ⇒ 该职位「不限证件」）。
// 与职位行同事务提交：写了一半的集合会让「需持 X 证」标签与筛选各说一套。
func replaceJobCredentials(tx *gorm.DB, jobID int, credIDs []int) error {
	if err := tx.Where("job_posting_id = ?", jobID).Delete(&model.JobCredentialRequirement{}).Error; err != nil {
		return err
	}
	if len(credIDs) == 0 {
		return nil
	}
	now := clock.Now()
	rows := make([]model.JobCredentialRequirement, 0, len(credIDs))
	for _, id := range credIDs {
		rows = append(rows, model.JobCredentialRequirement{JobPostingID: jobID, CredentialID: id, CreatedAt: now})
	}
	return tx.Create(&rows).Error
}

// fillRequiredCredentials 批量回填所需证件（单次查询，禁止 N+1）：
// 列表路径一次查全页，单行路径经 withCredentials 复用同一实现。
// 查不动即上抛 error（ADR-0062 票6：咽掉错误会让列表静默回成「不限证件」）。
func (s *Service) fillRequiredCredentials(dtos []JobPostingDTO) error {
	if len(dtos) == 0 {
		return nil
	}
	jobIDs := make([]int, 0, len(dtos))
	for i := range dtos {
		jobIDs = append(jobIDs, dtos[i].ID)
	}
	var rows []struct {
		JobPostingID int
		CredentialID int
		Name         string
	}
	if err := s.db.Table("job_credential_requirements AS requirement").
		Select("requirement.job_posting_id, requirement.credential_id, credential.name").
		Joins("JOIN credential ON credential.id = requirement.credential_id").
		Where("requirement.job_posting_id IN ?", jobIDs).
		Order("requirement.job_posting_id, credential.sort_order, credential.id").
		Scan(&rows).Error; err != nil {
		return err
	}
	byJob := make(map[int][]JobCredentialDTO, len(rows))
	for _, r := range rows {
		byJob[r.JobPostingID] = append(byJob[r.JobPostingID], JobCredentialDTO{CredentialID: r.CredentialID, Name: r.Name})
	}
	for i := range dtos {
		if list, ok := byJob[dtos[i].ID]; ok {
			dtos[i].RequiredCredentials = list
		}
	}
	return nil
}

// withCredentials 单行 DTO 的证件回填（走与列表同一个批量实现，只喂一行）。
func (s *Service) withCredentials(dto *JobPostingDTO) error {
	list := []JobPostingDTO{*dto}
	if err := s.fillRequiredCredentials(list); err != nil {
		return err
	}
	*dto = list[0]
	return nil
}

// toDTO 转换 DB 行为 DTO，带企业信息（学员侧可见字段）。
func (s *Service) toDTO(m *model.JobPosting) JobPostingDTO {
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
		// 起手就是空集：本函数只给形状，内容由 fillRequiredCredentials 批量补
		// （列表路径禁止逐行查，见该函数注释）。
		RequiredCredentials: []JobCredentialDTO{},
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
func (s *Service) Create(recruiterID int, in *JobPostingInput) (*JobPostingDTO, error) {
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
	credIDs, err := s.validRequiredCredentialIDs(in.RequiredCredentialIDs)
	if err != nil {
		return nil, err
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
	// 职位行与要求集合同事务：只写一半会让筛选命中一个没有标签的职位。
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		return replaceJobCredentials(tx, m.ID, credIDs)
	}); err != nil {
		return nil, err
	}
	dto := s.toDTO(&m)
	if err := s.withCredentials(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

// Update 企业编辑自己的职位（title/岗位/地区/薪资/经验/描述；状态走 ToggleStatus）。
func (s *Service) Update(recruiterID, jobID int, in *JobPostingInput) (*JobPostingDTO, error) {
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
	credIDs, err := s.validRequiredCredentialIDs(in.RequiredCredentialIDs)
	if err != nil {
		return nil, err
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
	// 编辑语义与其它字段一致：要求集合整组替换（空数组即清空 ⇒ 不限）。
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.JobPosting{}).Where("id = ?", jobID).Updates(updates).Error; err != nil {
			return err
		}
		return replaceJobCredentials(tx, jobID, credIDs)
	}); err != nil {
		return nil, err
	}
	_ = s.db.First(&m, jobID).Error
	dto := s.toDTO(&m)
	if err := s.withCredentials(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

// ToggleStatus 企业上架/下架自己的职位（open<->closed；强制下架职位不能自行重新上架）。
func (s *Service) ToggleStatus(recruiterID, jobID int) (*JobPostingDTO, error) {
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
	if err := s.withCredentials(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}

// jobStatusOpen 职位「在招」状态值（库层默认值也是它）。
const jobStatusOpen = "open"

// GetForStudent 学员职位详情（#488）：仅 open 且未强制下架，回填学员视角投递状态。
// jobVisibleToStudent 学员可见性谓词：open 且未被强制下架（Get 与 GetForStudent 共用）。
func jobVisibleToStudent(m *model.JobPosting) bool {
	return m.Status == jobStatusOpen && !m.ForcedOffline
}

// VisibleToStudentScope 学员可见性谓词的**查询侧形态**：open 且未被强制下架。
//
// 与 jobVisibleToStudent 同一判据的两种形态（行内 / SQL）。跨域消费者（收藏的目标校验与快照回填）
// 按它过滤，条件只在本域改一次——否则「收藏里点得进去、详情 404」会静默复发（#2067 的失效口径
// 就是「不给坏链接」）。
func VisibleToStudentScope(q *gorm.DB) *gorm.DB {
	return q.Where("status = ? AND forced_offline = ?", jobStatusOpen, false)
}

func (s *Service) GetForStudent(studentUserID, jobID int) (*JobPostingDTO, error) {
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if !jobVisibleToStudent(&m) {
		return nil, ErrJobNotFound
	}
	list := []JobPostingDTO{s.toDTO(&m)}
	if err := s.fillRequiredCredentials(list); err != nil {
		return nil, err
	}
	if err := s.fillApplyStates(studentUserID, list); err != nil {
		return nil, err
	}
	return &list[0], nil
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
	// CredentialID 按所需证件收窄（#2066）：nil = 不过滤；命中任一要求即返回，
	// 无要求的职位不命中（空集表意「不限」，不等于「都符合」）。只做筛选，不做证件分区。
	CredentialID *int
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
	Items []JobPostingDTO `json:"items" nullability:"nonnil"`
	Total int64           `json:"total"`
}

// List 职位列表。学员侧：只见 open 且未强制下架，按新鲜度排序；
// 企业侧（MineOnly）：含 closed/强制下架历史，同样按新鲜度排序。
func (s *Service) List(recruiterID int, p JobListParams) (*JobListResult, error) {
	rows, total, _, _, err := paging.QueryWithMax[model.JobPosting](s.db, p.Page, p.PageSize, 20, 50,
		"published_at DESC, id DESC", func(q *gorm.DB) *gorm.DB {
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
			// 证件谓词走 internal/scope 的具名片段（ADR-0056 §2：调用点不手写 credential_id 比较，
			// 静态扫描锁住）：nil 语义为「不分区、看全部」，与本筛选的「不传不过滤」同义。
			if clause, args := scope.EntityOwnedByClause("requirement.credential_id", p.CredentialID); clause != "" {
				// 用 EXISTS 而不是 JOIN：JOIN 会把「多证件职位」变成多行、污染 total 与分页，
				// 也会把外表的 created_at 带进 SELECT *（SQLite 上直接 ambiguous column）。
				q = q.Where("EXISTS (SELECT 1 FROM job_credential_requirements requirement "+
					"WHERE requirement.job_posting_id = job_postings.id AND "+clause+")", args...)
			}
			return q
		})
	if err != nil {
		return nil, err
	}
	items := make([]JobPostingDTO, 0, len(rows))
	for i := range rows {
		items = append(items, s.toDTO(&rows[i]))
	}
	// #2066：所需证件整页一次查（与下面投递状态同一个批量口径，不逐行查）
	if err := s.fillRequiredCredentials(items); err != nil {
		return nil, err
	}
	// #488：学员视角批量回填投递状态
	if err := s.fillApplyStates(p.StudentUserID, items); err != nil {
		return nil, err
	}
	return &JobListResult{Items: items, Total: total}, nil
}

// fillApplyStates 批量回填学员视角投递状态（#488：单次批量 join，禁止 N+1）。
// 入参 dtos 为学员可见的职位列表；对每个职位批量查该学员的投递记录判定状态。
// 查不动即上抛 error（ADR-0062 票6）：咽掉错误会让整列表回成「可投递」（旧写法 `return` 一丢，
// 前端按钮全绿，点了才 400）。
func (s *Service) fillApplyStates(studentUserID int, dtos []JobPostingDTO) error {
	if studentUserID <= 0 || len(dtos) == 0 {
		return nil
	}
	ids := make([]int, 0, len(dtos))
	for _, d := range dtos {
		ids = append(ids, d.ID)
	}
	// 批量 join：该学员对这些职位的最近投递（applied 或 rejected-with-cooldown）
	var apps []model.JobApplication
	if err := s.db.Where("job_posting_id IN ? AND student_user_id = ?", ids, studentUserID).
		Order("created_at DESC").Find(&apps).Error; err != nil {
		return err
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
		// 只取最新一条投递判定（created_at DESC 首行）：withdrawn 后可立即重投（恢复可投状态），
		// 若回看更早的 applied 行会把「已撤回」错标成 applied（Standards 审查 #488 high）。
		latest := appList[0]
		switch latest.Status {
		case "applied":
			dtos[i].ApplyState = "applied"
		case "rejected":
			// 冷却 30 天（沿用既有规则），冷却期内 not_hired，期满恢复可投
			if latest.RejectedAt != nil {
				remaining := 30*24*time.Hour - now.Sub(*latest.RejectedAt)
				if remaining > 0 {
					dtos[i].ApplyState = "not_hired"
					dtos[i].CooldownDays = int(remaining.Hours()/24) + 1
				}
			}
		case "withdrawn":
			// 撤回后可立即重投：不设状态（none）
		}
	}
	return nil
}

// Get 职位详情。recruiterID>0 表示企业侧（可看自己的 closed/强制下架历史），
// recruiterID=0 表示学员侧（仅 open 且未强制下架）。
func (s *Service) Get(recruiterID, jobID int) (*JobPostingDTO, error) {
	var m model.JobPosting
	if err := s.db.First(&m, jobID).Error; err != nil {
		return nil, ErrJobNotFound
	}
	if recruiterID > 0 {
		if m.RecruiterID != recruiterID {
			return nil, ErrJobNotYours
		}
	} else if !jobVisibleToStudent(&m) {
		return nil, ErrJobNotFound
	}
	dto := s.toDTO(&m)
	if err := s.withCredentials(&dto); err != nil {
		return nil, err
	}
	return &dto, nil
}
