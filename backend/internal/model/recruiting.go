// 招聘域（CONTEXT.md「企业招聘者」「简历卡」「联系方式交换」「职位」「投递」「职位举报」）。
package model

import "time"

// ===== 3.5 企业招聘者（第四角色，邀约制独立表） =====

// RecruiterUser 企业招聘者账号，独立于 hrwai_users / admin / tutor。
// 邀约制：仅管理员创建，企业信息字段全部必填；支持禁用位。
type RecruiterUser struct {
	ID            int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username      string `gorm:"column:username;uniqueIndex" json:"username"`
	Password      string `gorm:"column:password" json:"-"`
	CompanyName   string `gorm:"column:company_name" json:"company_name"`
	CreditCode    string `gorm:"column:credit_code" json:"credit_code"`
	BusinessScope string `gorm:"column:business_scope" json:"business_scope"`
	ContactName   string `gorm:"column:contact_name" json:"contact_name"`
	ContactPhone  string `gorm:"column:contact_phone" json:"contact_phone"`
	ContactEmail  string `gorm:"column:contact_email" json:"contact_email"`
	// Wechat 企业微信（#487：可空；学员侧交换授权 approved 后透出）
	Wechat    string    `gorm:"column:wechat;default:''" json:"wechat"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RecruiterUser) TableName() string { return "recruiter_users" }

// Position 岗位字典（spec 问题4：岗位与专业方向解绑，管理员配置）。
// 职位发布与简历「期望岗位」都从该字典选取；与培训域专业方向（specialty）完全解耦。
type Position struct {
	PositionID  int       `gorm:"column:position_id;primaryKey" json:"position_id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Position) TableName() string { return "positions" }

// ===== 29. 简历卡 =====
type JobCard struct {
	UserID                int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	RealName              string    `gorm:"column:real_name;default:''" json:"real_name"`
	ContactPhone          string    `gorm:"column:contact_phone;default:''" json:"contact_phone"`
	Wechat                string    `gorm:"column:wechat;default:''" json:"wechat"`
	Region                string    `gorm:"column:region;default:''" json:"region"`
	ExpectedPositionID    *int      `gorm:"column:expected_position_id" json:"expected_position_id,omitempty"`
	ExpectedPositionExtra string    `gorm:"column:expected_position_extra;default:''" json:"expected_position_extra"`
	ExpectedRegions       JSONB     `gorm:"column:expected_regions;type:jsonb;default:'[]'" json:"expected_regions"`
	SalaryMin             *int      `gorm:"column:salary_min" json:"salary_min,omitempty"`
	SalaryMax             *int      `gorm:"column:salary_max" json:"salary_max,omitempty"`
	SalaryNegotiable      bool      `gorm:"column:salary_negotiable;default:false" json:"salary_negotiable"`
	AvailableIn           string    `gorm:"column:available_in;default:''" json:"available_in"`
	JobNature             string    `gorm:"column:job_nature;default:''" json:"job_nature"`
	ExperienceYears       int       `gorm:"column:experience_years;default:0" json:"experience_years"`
	SelfIntro             string    `gorm:"column:self_intro;default:''" json:"self_intro"`
	ResumeExperiences     JSONB     `gorm:"column:resume_experiences;type:jsonb;default:'[]'" json:"resume_experiences"`
	ResumeCertifications  JSONB     `gorm:"column:resume_certifications;type:jsonb;default:'[]'" json:"resume_certifications"`
	ResumeFileURL         string    `gorm:"column:resume_file_url;default:''" json:"resume_file_url"`
	Photos                JSONB     `gorm:"column:photos;type:jsonb;default:'[]'" json:"photos"`
	Visibility            string    `gorm:"column:visibility;default:hidden" json:"visibility"`
	CreatedAt             time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (JobCard) TableName() string { return "job_cards" }

// ===== 29.1 招聘端简历浏览审计（L2 留痕） =====
type RecruitResumeView struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RecruiterID  int       `gorm:"column:recruiter_id" json:"recruiter_id"`
	ResumeUserID int       `gorm:"column:resume_user_id" json:"resume_user_id"`
	ViewedAt     time.Time `gorm:"column:viewed_at" json:"viewed_at"`
}

func (RecruitResumeView) TableName() string { return "recruit_resume_views" }

// ===== 29.2 联系方式交换申请（L3 闭环，#375） =====
type ContactRequest struct {
	ID            int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RecruiterID   int    `gorm:"column:recruiter_id;index" json:"recruiter_id"`
	StudentUserID int    `gorm:"column:student_user_id;index" json:"student_user_id"`
	Message       string `gorm:"column:message" json:"message"`
	Status        string `gorm:"column:status;default:pending" json:"status"` // pending/approved/rejected/expired/revoked
	// Source 授权来源（spec #449 决定 1）：recruiter 企业发起 / application 投递产生。
	// 明文载体仍然只有联系方式交换一个；投递产生的授权同样经 GetContact 判定。
	Source    string     `gorm:"column:source;default:recruiter" json:"source"` // recruiter/application
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DecidedAt *time.Time `gorm:"column:decided_at" json:"decided_at,omitempty"`
	ExpiresAt time.Time  `gorm:"column:expires_at" json:"expires_at"`
}

func (ContactRequest) TableName() string { return "contact_requests" }

// ===== 招聘域：职位（job posting，spec #449） =====

// JobPosting 企业发布的职位（先发后审：open/closed 二态，可被管理员强制下架）。
// 职位名不叫「岗位」——本系统「岗位」已是专业方向的历史别名（术语登记见 CONTEXT.md）。
type JobPosting struct {
	ID          int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RecruiterID int    `gorm:"column:recruiter_id;index" json:"recruiter_id"`
	Title       string `gorm:"column:title" json:"title"`
	// PositionID 岗位字典（问题4：与专业方向解绑）；业务层必填；库层可空（字典项删除置空不级联）。
	PositionID    *int   `gorm:"column:position_id" json:"position_id,omitempty"`
	Region        string `gorm:"column:region;default:''" json:"region"`
	SalaryMin     *int   `gorm:"column:salary_min" json:"salary_min,omitempty"`
	SalaryMax     *int   `gorm:"column:salary_max" json:"salary_max,omitempty"`
	SalaryText    string `gorm:"column:salary_text;default:''" json:"salary_text"` // 薪资自由文本（面议等）
	ExperienceReq string `gorm:"column:experience_req;default:''" json:"experience_req"`
	Description   string `gorm:"column:description;default:''" json:"description"`
	Status        string `gorm:"column:status;default:open" json:"status"` // open/closed
	// ForcedOffline 被管理员强制下架（原因见 OfflineReason）：学员侧不可见，企业不能自行重新上架。
	ForcedOffline bool      `gorm:"column:forced_offline;default:false" json:"forced_offline"`
	OfflineReason string    `gorm:"column:offline_reason;default:''" json:"offline_reason,omitempty"`
	PublishedAt   time.Time `gorm:"column:published_at" json:"published_at"` // 首次发布时间（新鲜度排序）
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (JobPosting) TableName() string { return "job_postings" }

// JobApplication 学员对职位的投递（三态 applied/rejected/withdrawn）。
// 冗余 recruiter_id（授权落地与企业维度查询都需要它，避免每次 join 职位）。
type JobApplication struct {
	ID            int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobPostingID  int    `gorm:"column:job_posting_id;index" json:"job_posting_id"`
	RecruiterID   int    `gorm:"column:recruiter_id;index" json:"recruiter_id"`
	StudentUserID int    `gorm:"column:student_user_id;index" json:"student_user_id"`
	Status        string `gorm:"column:status;default:applied" json:"status"` // applied/rejected/withdrawn
	// ResumeUpdatedAt 投递那一刻的简历更新时间（版本指针，不落快照；内容永远读最新）。
	ResumeUpdatedAt time.Time  `gorm:"column:resume_updated_at" json:"resume_updated_at"`
	EmployerViewAt  *time.Time `gorm:"column:employer_viewed_at" json:"employer_viewed_at,omitempty"` // 企业打开投递详情即记录已读
	RejectedAt      *time.Time `gorm:"column:rejected_at" json:"rejected_at,omitempty"`
	WithdrawnAt     *time.Time `gorm:"column:withdrawn_at" json:"withdrawn_at,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (JobApplication) TableName() string { return "job_applications" }

// JobReport 学员对职位的举报（招聘域自有存储，不挂论坛举报表）。
// 同一学员对同一职位唯一；重复举报被合并而非堆叠。
type JobReport struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	JobPostingID  int        `gorm:"column:job_posting_id;index" json:"job_posting_id"`
	StudentUserID int        `gorm:"column:student_user_id;index" json:"student_user_id"`
	Reason        string     `gorm:"column:reason;default:''" json:"reason"`
	Status        string     `gorm:"column:status;default:pending" json:"status"` // pending/handled
	HandledAt     *time.Time `gorm:"column:handled_at" json:"handled_at,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (JobReport) TableName() string { return "job_reports" }
