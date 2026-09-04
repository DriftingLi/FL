// Package model 定义全部 GORM 数据模型，与 migrations/000001_init.up.sql 一一对应。
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONB 是 PostgreSQL JSONB 字段的 Go 映射，支持任意 JSON 值。
type JSONB json.RawMessage

// Scan 实现 sql.Scanner。
func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
		return nil
	case string:
		*j = JSONB(v)
		return nil
	}
	return errors.New("JSONB.Scan: 不支持的类型")
}

// Value 实现 driver.Valuer。
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// MarshalJSON 实现 json.Marshaler。
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSONB.UnmarshalJSON: 指针为空")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// ===== 1. HRWAI 账号(统一用户表) =====

// HrwaiUser 统一用户表,合并原 student 与 valuation_users 两表。
// 三套登录鉴权(培训学员端 / 残值评估 / AI 助手)共用此表与主体系 JWT。
// admin / tutor 账号仍保留独立表,不并入此表。
type HrwaiUser struct {
	ID        int    `gorm:"column:id;primaryKey" json:"id"`
	UID       int64  `gorm:"column:uid" json:"uid,string"`
	Account   string `gorm:"column:account;uniqueIndex" json:"account"`
	Username  string `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string `gorm:"column:password" json:"-"`
	AvatarURL string `gorm:"column:avatar_url" json:"avatar_url"`
	Phone     string `gorm:"column:phone" json:"phone"`
	Email     string `gorm:"column:email" json:"email,omitempty"`
	Company   string `gorm:"column:company" json:"company,omitempty"`
	// WechatOpenID 微信开放平台 openid（微信扫码登录绑定用，框架字段）。
	WechatOpenID string `gorm:"column:wechat_openid" json:"wechat_openid,omitempty"`
	// WechatUnionID 微信开放平台 unionid（多应用互通，框架字段）。
	WechatUnionID string `gorm:"column:wechat_unionid" json:"wechat_unionid,omitempty"`
	// CurrentCredentialID 当前目标证件（单选上下文，NULL 表示未预筛选）。
	CurrentCredentialID *int      `gorm:"column:current_credential_id" json:"current_credential_id,omitempty"`
	PointsBalance       int       `gorm:"column:points_balance;default:0" json:"points_balance"`
	Status              int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
}

func (HrwaiUser) TableName() string { return "hrwai_users" }

// ===== 2. 管理员 =====

type Admin struct {
	AdminID   int       `gorm:"column:admin_id;primaryKey" json:"admin_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Admin) TableName() string { return "admin" }

// ===== 3. 导师 =====

type Tutor struct {
	TutorID   int       `gorm:"column:tutor_id;primaryKey" json:"tutor_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Tutor) TableName() string { return "tutor" }

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

// ===== 4. 课程 =====

type Course struct {
	CourseID    int    `gorm:"column:course_id;primaryKey" json:"course_id"`
	Name        string `gorm:"column:name" json:"name"`
	Description string `gorm:"column:description" json:"description"`
	CoverImage  string `gorm:"column:cover_image" json:"cover_image"`
	Duration    int    `gorm:"column:duration;default:0" json:"duration"`
	// CredentialID 所属目标证件（顶层分区，单归属）。
	CredentialID *int `gorm:"column:credential_id" json:"credential_id,omitempty"`
	// SpecialtyID 专业方向（目录一级节点）。
	SpecialtyID *int `gorm:"column:specialty_id" json:"specialty_id,omitempty"`
	// LevelID 课程等级（入门/进阶/专项/认证）。
	LevelID *int `gorm:"column:level_id" json:"level_id,omitempty"`
	// TheoryHours 理论学时。
	TheoryHours int `gorm:"column:theory_hours;default:0" json:"theory_hours"`
	// PracticeHours 实操学时。
	PracticeHours int `gorm:"column:practice_hours;default:0" json:"practice_hours"`
	// CertificateTemplateID 关联证书模板（有效期取模板 validity_days）。
	CertificateTemplateID *int `gorm:"column:certificate_template_id" json:"certificate_template_id,omitempty"`
	// SortOrder 课程排序值（所属专业方向+课程等级层级内生效，越小越靠前）。
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	IsHot       bool      `gorm:"column:is_hot;default:false" json:"is_hot"`
	IsFeatured  bool      `gorm:"column:is_featured;default:false" json:"is_featured"`
	PointsPrice *int      `gorm:"column:points_price" json:"points_price,omitempty"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Course) TableName() string { return "course" }

// ===== 4.5 专业方向（课程目录一级节点） =====

type Specialty struct {
	SpecialtyID int       `gorm:"column:specialty_id;primaryKey" json:"specialty_id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Specialty) TableName() string { return "specialty" }

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

// ===== 4.6 课程等级 =====

type CourseLevel struct {
	LevelID     int       `gorm:"column:level_id;primaryKey" json:"level_id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CourseLevel) TableName() string { return "course_level" }

// ===== 4.6.1 目标证件（target credential） =====

// Credential 目标证件，学员报考的外部持证目标（与证书模板区分）。
// category: special_operation 特种作业上岗证 / skill_level 职业技能等级；
// level: 仅 skill_level 类填 1-5（5 初级→1 高级），特种作业为 NULL。
type Credential struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	Category    string    `gorm:"column:category" json:"category"`
	Level       *int      `gorm:"column:level" json:"level,omitempty"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Credential) TableName() string { return "credential" }

// ===== 4.7 证书模板 =====

// CertificateTemplate 证书模板，validity_days 为证书有效期（天）。
type CertificateTemplate struct {
	ID           int       `gorm:"column:id;primaryKey" json:"id"`
	Code         string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name         string    `gorm:"column:name" json:"name"`
	Description  string    `gorm:"column:description" json:"description"`
	ValidityDays int       `gorm:"column:validity_days;default:365" json:"validity_days"`
	TemplateURL  string    `gorm:"column:template_url" json:"template_url"`
	Status       int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CertificateTemplate) TableName() string { return "certificate_template" }

// ===== 4.8 课程前置课程（多对多） =====

type CoursePrerequisite struct {
	CourseID             int       `gorm:"column:course_id;primaryKey" json:"course_id"`
	PrerequisiteCourseID int       `gorm:"column:prerequisite_course_id;primaryKey" json:"prerequisite_course_id"`
	CreatedAt            time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CoursePrerequisite) TableName() string { return "course_prerequisite" }

// ===== 5. 章节 =====

type Chapter struct {
	ChapterID   int    `gorm:"column:chapter_id;primaryKey" json:"chapter_id"`
	CourseID    int    `gorm:"column:course_id" json:"course_id"`
	Title       string `gorm:"column:title" json:"title"`
	Content     string `gorm:"column:content" json:"content"`
	ContentType string `gorm:"column:content_type;default:text" json:"content_type"`
	FileURL     string `gorm:"column:file_url" json:"file_url"`
	// SlideUrls PPT 转图后的幻灯片 URL 列表（JSON 数组字符串）。
	// 为空表示未生成；由 SlideRenderer.Render 生成后写入。
	SlideUrls   string    `gorm:"column:slide_urls" json:"slide_urls"`
	Description string    `gorm:"column:description" json:"description"`
	Duration    int       `gorm:"column:duration;default:0" json:"duration"`
	OrderNum    int       `gorm:"column:order_num;default:0" json:"order_num"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Chapter) TableName() string { return "chapter" }

// ===== 6. 章节文件 =====

type ChapterFile struct {
	FileID      int       `gorm:"column:file_id;primaryKey" json:"file_id"`
	ChapterID   *int      `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	ContentType string    `gorm:"column:content_type;default:document" json:"content_type"`
	FileSize    int64     `gorm:"column:file_size;default:0" json:"file_size"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ChapterFile) TableName() string { return "chapter_file" }

// ===== 7. 学习记录 =====

type StudyRecord struct {
	RecordID      int        `gorm:"column:record_id;primaryKey" json:"record_id"`
	StudentID     int        `gorm:"column:student_id" json:"student_id"`
	CourseID      int        `gorm:"column:course_id" json:"course_id"`
	ChapterID     *int       `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	StudyDuration int        `gorm:"column:study_duration;default:0" json:"study_duration"`
	Progress      float64    `gorm:"column:progress;type:numeric(5,2);default:0" json:"progress"`
	StudyDate     time.Time  `gorm:"column:study_date" json:"study_date"`
	VideoPosition int        `gorm:"column:video_position;default:0" json:"video_position"`
	LastChapterID *int       `gorm:"column:last_chapter_id" json:"last_chapter_id,omitempty"`
	LastStudiedAt *time.Time `gorm:"column:last_studied_at" json:"last_studied_at,omitempty"`
}

func (StudyRecord) TableName() string { return "study_record" }

// ===== 9. AI 生成记录 =====

type AIGenerationLog struct {
	LogID          int       `gorm:"column:log_id;primaryKey" json:"log_id"`
	UserID         int       `gorm:"column:user_id" json:"user_id"`
	UserType       string    `gorm:"column:user_type" json:"user_type"`
	GenerationType string    `gorm:"column:generation_type" json:"generation_type"`
	InputParams    JSONB     `gorm:"column:input_params;type:jsonb" json:"input_params,omitempty"`
	OutputResult   string    `gorm:"column:output_result" json:"output_result"`
	Status         int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIGenerationLog) TableName() string { return "ai_generation_log" }

// ===== 11. 题目 =====

type Question struct {
	ID              int    `gorm:"column:id;primaryKey" json:"id"`
	Type            string `gorm:"column:type" json:"type"`
	Content         string `gorm:"column:content" json:"content"`
	Options         JSONB  `gorm:"column:options;type:jsonb" json:"options,omitempty"`
	Answer          string `gorm:"column:answer" json:"answer"`
	Explanation     string `gorm:"column:explanation" json:"explanation"`
	AIExplanation   string `gorm:"column:ai_explanation" json:"ai_explanation,omitempty"`
	ImageURL        string `gorm:"column:image_url" json:"image_url"`
	ReferenceAnswer string `gorm:"column:reference_answer" json:"reference_answer"`
	ScoringCriteria string `gorm:"column:scoring_criteria" json:"scoring_criteria"`
	Score           int    `gorm:"column:score;default:0" json:"score"`
	// CredentialID 所属目标证件（顶层分区，单归属）。
	CredentialID  *int      `gorm:"column:credential_id" json:"credential_id,omitempty"`
	Status        string    `gorm:"column:status;default:draft" json:"status"`
	RejectReason  string    `gorm:"column:reject_reason" json:"reject_reason"`
	CreatedBy     *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedByType string    `gorm:"column:created_by_type;default:tutor" json:"created_by_type"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Question) TableName() string { return "question" }

// ===== 11.5 题库标签 =====

// QuestionTag 题库标签（法规/结构/液压/电气/制动/故障诊断/应急等考点模块）。
type QuestionTag struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	IsSourceTag bool      `gorm:"column:is_source_tag;default:false" json:"is_source_tag"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (QuestionTag) TableName() string { return "question_tag" }

// QuestionTagRelation 题目-标签关联（多对多）。
type QuestionTagRelation struct {
	QuestionID int       `gorm:"column:question_id;primaryKey" json:"question_id"`
	TagID      int       `gorm:"column:tag_id;primaryKey" json:"tag_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionTagRelation) TableName() string { return "question_tag_relation" }

// ===== 15. 题库练习记录 =====

type QuestionPracticeRecord struct {
	ID           int       `gorm:"column:id;primaryKey" json:"id"`
	StudentID    int       `gorm:"column:student_id" json:"student_id"`
	QuestionID   int       `gorm:"column:question_id" json:"question_id"`
	IsCorrect    bool      `gorm:"column:is_correct;default:false" json:"is_correct"`
	PracticeType string    `gorm:"column:practice_type;default:free" json:"practice_type"`
	UserAnswer   string    `gorm:"column:user_answer" json:"user_answer"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionPracticeRecord) TableName() string { return "question_practice_record" }

// ===== 17. 错题记录 =====

type WrongQuestion struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	StudentID   int       `gorm:"column:student_id" json:"student_id"`
	QuestionID  int       `gorm:"column:question_id" json:"question_id"`
	WrongCount  int       `gorm:"column:wrong_count;default:1" json:"wrong_count"`
	LastWrongAt time.Time `gorm:"column:last_wrong_at" json:"last_wrong_at"`
	IsRemoved   bool      `gorm:"column:is_removed;default:false" json:"is_removed"`
	IsRedone    bool      `gorm:"column:is_redone;default:false" json:"is_redone"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (WrongQuestion) TableName() string { return "wrong_question" }

// ===== 18. 模拟考试 =====

type MockExam struct {
	ID            int        `gorm:"column:id;primaryKey" json:"id"`
	StudentID     int        `gorm:"column:student_id" json:"student_id"`
	QuestionIDs   JSONB      `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	Answers       JSONB      `gorm:"column:answers;type:jsonb" json:"answers,omitempty"`
	StartTime     *time.Time `gorm:"column:start_time" json:"start_time,omitempty"`
	SubmitTime    *time.Time `gorm:"column:submit_time" json:"submit_time,omitempty"`
	RemainingTime int        `gorm:"column:remaining_time;default:0" json:"remaining_time"`
	Duration      int        `gorm:"column:duration;default:90" json:"duration"`
	Score         *float64   `gorm:"column:score;type:numeric(5,2)" json:"score,omitempty"`
	Status        string     `gorm:"column:status;default:not_started" json:"status"`
	Result        JSONB      `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	PaperID       *int       `gorm:"column:paper_id" json:"paper_id,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (MockExam) TableName() string { return "mock_exam" }

// RealExamPaper 真题套卷（导入工具按真题源文件生成，credential 单归属分区）。
type RealExamPaper struct {
	PaperID         int       `gorm:"column:paper_id;primaryKey" json:"paper_id"`
	CredentialID    int       `gorm:"column:credential_id" json:"credential_id"`
	Title           string    `gorm:"column:title" json:"title"`
	Year            *int      `gorm:"column:year" json:"year,omitempty"`
	Source          *string   `gorm:"column:source" json:"source,omitempty"`
	DurationMinutes int       `gorm:"column:duration_minutes;default:90" json:"duration_minutes"`
	LevelID         *int      `gorm:"column:level_id" json:"level_id,omitempty"`
	SourceRef       string    `gorm:"column:source_ref" json:"source_ref"`
	QuestionCount   int       `gorm:"column:question_count;default:0" json:"question_count"`
	Status          int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RealExamPaper) TableName() string { return "real_exam_paper" }

// RealExamPaperQuestion 真题卷-题目关联（order_num 维持卷内题序）。
type RealExamPaperQuestion struct {
	PaperID    int `gorm:"column:paper_id;primaryKey" json:"paper_id"`
	QuestionID int `gorm:"column:question_id;primaryKey" json:"question_id"`
	OrderNum   int `gorm:"column:order_num;default:0" json:"order_num"`
}

func (RealExamPaperQuestion) TableName() string { return "real_exam_paper_question" }

// ===== 18.5 练习进度（顺序练习断点续练） =====

type PracticeProgress struct {
	ID           int    `gorm:"column:id;primaryKey" json:"id"`
	StudentID    int    `gorm:"column:student_id" json:"student_id"`
	PracticeMode string `gorm:"column:practice_mode" json:"practice_mode"`
	// CredentialID 进度归属的证件分区（#414）：仅顺序练习携带（唯一键 (student, mode, credential)），
	// 标签/按卷练习保持 NULL（partial 唯一索引兜底，NULL 不判重），未预筛选学员亦为 NULL。
	CredentialID *int      `gorm:"column:credential_id" json:"credential_id,omitempty"`
	QuestionIDs  JSONB     `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	CurrentIndex int       `gorm:"column:current_index;default:0" json:"current_index"`
	Total        int       `gorm:"column:total;default:0" json:"total"`
	AnswersState JSONB     `gorm:"column:answers_state;type:jsonb" json:"answers_state"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PracticeProgress) TableName() string { return "practice_progress" }

// ===== 19. 异步任务 =====

type AsyncTask struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	TaskType  string    `gorm:"column:task_type" json:"task_type"`
	Status    string    `gorm:"column:status;default:pending" json:"status"`
	Payload   JSONB     `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Result    JSONB     `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	Error     string    `gorm:"column:error" json:"error"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AsyncTask) TableName() string { return "async_task" }

// ===== 20. 内容精选（公司动态 / 行业新闻 等） =====

type FeaturedContent struct {
	ContentID   int        `gorm:"column:content_id;primaryKey" json:"content_id"`
	Title       string     `gorm:"column:title" json:"title"`
	Summary     string     `gorm:"column:summary" json:"summary"`
	CoverImage  string     `gorm:"column:cover_image" json:"cover_image"`
	Content     string     `gorm:"column:content" json:"content"`
	Category    string     `gorm:"column:category;default:industry" json:"category"`
	Source      string     `gorm:"column:source" json:"source"`
	Status      int16      `gorm:"column:status;default:0" json:"status"`
	ViewCount   int        `gorm:"column:view_count;default:0" json:"view_count"`
	SortOrder   int        `gorm:"column:sort_order;default:0" json:"sort_order"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (FeaturedContent) TableName() string { return "featured_content" }

// ===== 21. 系统设置 =====

// SystemSetting 系统设置表（key-value 结构，承载 AI 等模块的动态配置）。
type SystemSetting struct {
	Key         string    `gorm:"column:key;primaryKey" json:"key"`
	Value       string    `gorm:"column:value" json:"value"`
	Description string    `gorm:"column:description" json:"description"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }

// ===== 22. AI 多配置 =====

// AIConfig AI 服务配置（多套命名配置，可绑定到不同 AI 功能）。
type AIConfig struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;uniqueIndex" json:"name"`
	APIKey      string    `gorm:"column:api_key" json:"api_key"`
	BaseURL     string    `gorm:"column:base_url" json:"base_url"`
	Model       string    `gorm:"column:model" json:"model"`
	Description string    `gorm:"column:description" json:"description"`
	IsActive    bool      `gorm:"column:is_active" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIConfig) TableName() string { return "ai_configs" }

// AIFeatureBinding AI 功能-配置绑定。
// 单绑定功能（如简答题评分）在应用层限制每个 feature_key 只能有一条记录；
// 多绑定功能（如 AI 助手）允许一个 feature_key 绑定多个 config_id（DB 通过 (feature_key, config_id) 复合唯一约束保证不重复）。
type AIFeatureBinding struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FeatureKey string    `gorm:"column:feature_key" json:"feature_key"`
	ConfigID   int       `gorm:"column:config_id" json:"config_id"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIFeatureBinding) TableName() string { return "ai_feature_bindings" }

// ===== 23. AI 助手模块 =====

// AIChatSession AI 助手会话（归属 valuation_users）。
type AIChatSession struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Title      string    `gorm:"column:title" json:"title"`
	ModelName  string    `gorm:"column:model_name" json:"model_name"`
	FeatureKey string    `gorm:"column:feature_key" json:"feature_key"` // 所属功能（旧数据为 ai_assistant）
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIChatSession) TableName() string { return "ai_chat_sessions" }

// AIChatMessage AI 助手会话消息。
type AIChatMessage struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionID int       `gorm:"column:session_id" json:"session_id"`
	Role      string    `gorm:"column:role" json:"role"` // 'user' | 'assistant' | 'system'
	Content   string    `gorm:"column:content" json:"content"`
	Images    string    `gorm:"column:images" json:"-"` // JSON 数组字符串，用户消息附带的图片 URL
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

// AIUserModel 用户自定义 AI 模型配置（openai 兼容格式，归属 valuation_users）。
type AIUserModel struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Name      string    `gorm:"column:name" json:"name"`
	APIKey    string    `gorm:"column:api_key" json:"api_key"`
	BaseURL   string    `gorm:"column:base_url" json:"base_url"`
	Model     string    `gorm:"column:model" json:"model"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIUserModel) TableName() string { return "ai_user_models" }

// ===== 24. 论坛模块 =====

// ForumTopic 论坛主题。
//
// 两个正交维度中，chapter_id 只服务讨论帖：
//   - category=discussion + chapter_id IS NULL = 综合讨论区
//   - category=discussion + chapter_id 非空    = 章节讨论区
//   - category=question   + chapter_id IS NULL = 全局问答
//   - category=question   + chapter_id 非空    = 非法组合
//
// ⚠️ 判类别看 category，判区域看 chapter_id，两者不可互相替代：scope=general 的定义
// 就是 chapter_id IS NULL，而问答帖的 chapter_id 同样为 NULL，故列表查询必须让
// category 与 scope 共存在同一条 WHERE 里，否则问答帖会整片灌进讨论 Tab。
//
// ⚠️ 上述非法组合在数据库层由 CHECK 兜底，但这些约束**只存在于迁移 SQL（000004）**：
// 测试库由 AutoMigrate 建表、不执行 migrations/，因此两条 CHECK（值域 chk_forum_topics_category
// 与非法组合 chk_forum_topics_question_no_chapter）契约测试都覆盖不到，别误以为测试守住了它们。
// 行为层由 service 的校验守住：非法类别 400、问答帖带 chapter_id>0 返回 400。
type ForumTopic struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChapterID       *int       `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	Category        string     `gorm:"column:category;not null;default:discussion" json:"category"` // 'discussion' | 'question'
	UserID          int        `gorm:"column:user_id" json:"user_id"`
	Title           string     `gorm:"column:title" json:"title"`
	Content         string     `gorm:"column:content" json:"content"`
	Images          JSONB      `gorm:"column:images;type:jsonb" json:"images"`
	ViewCount       int        `gorm:"column:view_count;default:0" json:"view_count"`
	ReplyCount      int        `gorm:"column:reply_count;default:0" json:"reply_count"`
	LikesCount      int        `gorm:"column:likes_count;default:0" json:"likes_count"`
	AcceptedReplyID *int64     `gorm:"column:accepted_reply_id" json:"accepted_reply_id,omitempty"`
	SolvedAt        *time.Time `gorm:"column:solved_at" json:"solved_at,omitempty"`
	LastReplyAt     *time.Time `gorm:"column:last_reply_at" json:"last_reply_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ForumTopic) TableName() string { return "forum_topics" }

// ForumReply 论坛回复。
type ForumReply struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TopicID    int64     `gorm:"column:topic_id" json:"topic_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	ParentID   *int64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Content    string    `gorm:"column:content" json:"content"`
	Images     JSONB     `gorm:"column:images;type:jsonb" json:"images"`
	LikesCount int       `gorm:"column:likes_count;default:0" json:"likes_count"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReply) TableName() string { return "forum_replies" }

// ForumTopicLike 论坛主题点赞（topic_id+user_id 唯一约束保证幂等，ADR-0018）。
type ForumTopicLike struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TopicID   int64     `gorm:"column:topic_id" json:"topic_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumTopicLike) TableName() string { return "forum_topic_like" }

// ForumCheckIn 每日打卡记录（user_id + check_date 唯一，Asia/Shanghai 自然日，spec #268）。
type ForumCheckIn struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	CheckDate time.Time `gorm:"column:check_date;primaryKey;type:date" json:"check_date"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumCheckIn) TableName() string { return "forum_checkin" }

// ForumReplyLike 评论点赞（reply_id+user_id 唯一，与 ForumTopicLike 同构，spec #268）。
type ForumReplyLike struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ReplyID   int64     `gorm:"column:reply_id" json:"reply_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReplyLike) TableName() string { return "forum_reply_like" }

// ForumTopicView 用户帖子浏览去重（每日每帖一次，排除自帖，用于 daily_browse）
type ForumTopicView struct {
	UserID   int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	TopicID  int64     `gorm:"column:topic_id;primaryKey" json:"topic_id"`
	ViewedAt time.Time `gorm:"column:viewed_at" json:"viewed_at"`
	ViewDate string    `gorm:"column:view_date;primaryKey;type:date" json:"view_date"`
}

func (ForumTopicView) TableName() string { return "forum_topic_views" }

// Favorite 通用收藏：target_type + target_id 多态定位
// （course/chapter/question/featured/topic，ADR-0018；user+type+id 唯一）。
type Favorite struct {
	FavoriteID int64     `gorm:"column:favorite_id;primaryKey;autoIncrement" json:"favorite_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	TargetType string    `gorm:"column:target_type;size:20" json:"target_type"`
	TargetID   int       `gorm:"column:target_id" json:"target_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Favorite) TableName() string { return "favorite" }

// ForumReport 论坛举报：topic_id 与 reply_id 二选一；status 0 待处理 / 1 已处理（ADR-0018）。
type ForumReport struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ReporterID int       `gorm:"column:reporter_id" json:"reporter_id"`
	TopicID    *int64    `gorm:"column:topic_id" json:"topic_id,omitempty"`
	ReplyID    *int64    `gorm:"column:reply_id" json:"reply_id,omitempty"`
	Reason     string    `gorm:"column:reason" json:"reason"`
	Status     int16     `gorm:"column:status;default:0" json:"status"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReport) TableName() string { return "forum_report" }

// ===== 24.5 题目评论（刷题解析） =====

type QuestionComment struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	QuestionID int       `gorm:"column:question_id" json:"question_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Content    string    `gorm:"column:content" json:"content"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionComment) TableName() string { return "question_comment" }

// QuestionNote 题目笔记（每人每题一条，私有）。
type QuestionNote struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	QuestionID int       `gorm:"column:question_id" json:"question_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Content    string    `gorm:"column:content" json:"content"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (QuestionNote) TableName() string { return "question_note" }

// ===== 25. 资料修改审核 =====

// ProfileChangeRequest 用户资料（昵称/头像）修改审核请求。
type ProfileChangeRequest struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int        `gorm:"column:user_id" json:"user_id"`
	FieldType    string     `gorm:"column:field_type" json:"field_type"` // nickname / avatar
	OldValue     string     `gorm:"column:old_value" json:"old_value"`
	NewValue     string     `gorm:"column:new_value" json:"new_value"`
	Status       string     `gorm:"column:status;default:pending" json:"status"`
	RejectReason string     `gorm:"column:reject_reason" json:"reject_reason"`
	ReviewedBy   *int       `gorm:"column:reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ProfileChangeRequest) TableName() string { return "profile_change_requests" }

// ===== 26. 站内信通知 =====

// Notification 站内信通知（P0 通知基础设施，当前仅站内信渠道）。
// Payload 为结构化业务标记（JSONB，如资料审核 {"review_status":"approved"}），加性扩展，不参与人读文案。
type Notification struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int        `gorm:"column:user_id" json:"user_id"`
	Type      string     `gorm:"column:type;default:system" json:"type"`
	Title     string     `gorm:"column:title" json:"title"`
	Content   string     `gorm:"column:content" json:"content"`
	Link      string     `gorm:"column:link" json:"link"`
	Payload   JSONB      `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	IsRead    bool       `gorm:"column:is_read;default:false" json:"is_read"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	ReadAt    *time.Time `gorm:"column:read_at" json:"read_at,omitempty"`
}

func (Notification) TableName() string { return "notifications" }

// ===== 27. 审计日志 =====

// AuditLog 管理员/讲师关键操作审计日志。
type AuditLog struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ActorID   int       `gorm:"column:actor_id" json:"actor_id"`
	ActorRole string    `gorm:"column:actor_role" json:"actor_role"`
	ActorName string    `gorm:"column:actor_name" json:"actor_name"`
	Action    string    `gorm:"column:action" json:"action"`
	Path      string    `gorm:"column:path" json:"path"`
	Method    string    `gorm:"column:method" json:"method"`
	RequestID string    `gorm:"column:request_id" json:"request_id"`
	IP        string    `gorm:"column:ip" json:"ip"`
	Status    int       `gorm:"column:status" json:"status"`
	Detail    JSONB     `gorm:"column:detail;type:jsonb" json:"detail,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// ===== 28. 积分系统 =====

// PointsLedger 积分账本（不可变流水，delta>0 赚取、<0 消耗）。
type PointsLedger struct {
	ID        int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int        `gorm:"column:user_id" json:"user_id"`
	Delta     int        `gorm:"column:delta" json:"delta"`
	Reason    string     `gorm:"column:reason" json:"reason"`
	RefType   string     `gorm:"column:ref_type" json:"ref_type"`
	RefID     string     `gorm:"column:ref_id" json:"ref_id"`
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
}

func (PointsLedger) TableName() string { return "points_ledger" }

// PointsTaskConfig 积分任务配置（10 任务种子）。
type PointsTaskConfig struct {
	Code        string `gorm:"column:code;primaryKey" json:"code"`
	Title       string `gorm:"column:title" json:"title"`
	Group       string `gorm:"column:group" json:"group"`
	Points      int    `gorm:"column:points" json:"points"`
	DailyLimit  int    `gorm:"column:daily_limit" json:"daily_limit"`
	TotalLimit  *int   `gorm:"column:total_limit" json:"total_limit,omitempty"`
	EventType   string `gorm:"column:event_type" json:"event_type"`
	Description string `gorm:"column:description" json:"description"`
}

func (PointsTaskConfig) TableName() string { return "points_task_config" }

// PointsTaskClaim 任务领取幂等占坑。
type PointsTaskClaim struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	TaskCode  string    `gorm:"column:task_code" json:"task_code"`
	ClaimDate *string   `gorm:"column:claim_date;type:date" json:"claim_date,omitempty"` // YYYY-MM-DD，Asia/Shanghai（#409：对齐同域邻居 ForumCheckIn 的日期类型标注）
	RefID     *string   `gorm:"column:ref_id" json:"ref_id,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PointsTaskClaim) TableName() string { return "points_task_claim" }

// PointsUserProgress 用户任务进度快照。
type PointsUserProgress struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	TaskCode  string    `gorm:"column:task_code;primaryKey" json:"task_code"`
	Progress  int       `gorm:"column:progress" json:"progress"`
	Total     int       `gorm:"column:total" json:"total"`
	Status    string    `gorm:"column:status" json:"status"` // todo/claimable/claimed
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PointsUserProgress) TableName() string { return "points_user_progress" }

// PointsShopItem 积分商城（真题等，课程兑换走 course.points_price）。
type PointsShopItem struct {
	SKU     string `gorm:"column:sku;primaryKey" json:"sku"`
	Title   string `gorm:"column:title" json:"title"`
	Price   int    `gorm:"column:price" json:"price"`
	Stock   *int   `gorm:"column:stock" json:"stock,omitempty"`
	Enabled bool   `gorm:"column:enabled" json:"enabled"`
}

func (PointsShopItem) TableName() string { return "points_shop_item" }

// UserEntitlement 用户权益（课程/真题解锁）。
type UserEntitlement struct {
	UserID    int       `gorm:"column:user_id;primaryKey" json:"user_id"`
	SKU       string    `gorm:"column:sku;primaryKey" json:"sku"`
	RefID     string    `gorm:"column:ref_id;primaryKey" json:"ref_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserEntitlement) TableName() string { return "user_entitlement" }

// PointsEntryIdem 通用积分簿记幂等占坑（ADR-0023）：占坑行即「已处理」标记。
// 「一事件一分」与回收 settle 传确定性键（accepted_bonus:{topicID} / rollback:{topicID} /
// redeem:{ref} / ai_tokens:{requestID} 等），主键冲突 = 事件已处理。
type PointsEntryIdem struct {
	IdemKey   string    `gorm:"column:idem_key;primaryKey;size:128" json:"idem_key"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PointsEntryIdem) TableName() string { return "points_entry_idem" }

// ===== 29. 简历卡
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

// ===== 29.1 招聘端简历浏览审计（L2 留痕）
type RecruitResumeView struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RecruiterID  int       `gorm:"column:recruiter_id" json:"recruiter_id"`
	ResumeUserID int       `gorm:"column:resume_user_id" json:"resume_user_id"`
	ViewedAt     time.Time `gorm:"column:viewed_at" json:"viewed_at"`
}

func (RecruitResumeView) TableName() string { return "recruit_resume_views" }

// ===== 29.2 联系方式交换申请（L3 闭环，#375）
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

// ===== 招聘域：职位（job posting，spec #449）=====

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

// ===== 30. 资料投稿域（#517 / ADR-0026，学员上传资料换积分）=====

// UserContribution 投稿主行。状态机（见 CONTEXT.md「投稿审核」）：
// pending → approved / rejected；pending → withdrawn；approved → archived。
// rejected 不可恢复（重提 = 新建投稿新行）；approved 下架（archived）时追回投稿积分。
type UserContribution struct {
	ID           int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int    `gorm:"column:user_id;index" json:"user_id"`
	CredentialID int    `gorm:"column:credential_id;index" json:"credential_id"`
	Title        string `gorm:"column:title" json:"title"`
	Intro        string `gorm:"column:intro" json:"intro"`
	Status       string `gorm:"column:status;default:pending" json:"status"` // pending/approved/rejected/withdrawn/archived
	IsAnonymous  bool   `gorm:"column:is_anonymous;default:false" json:"is_anonymous"`
	// DownloadsCount 下载量反范式列（事实源为 contribution_download 唯一约束，同事务维护）。
	DownloadsCount int `gorm:"column:downloads_count;default:0" json:"downloads_count"`
	// RejectReason 驳回/下架原因（审核者填写；驳回必填）。
	RejectReason string `gorm:"column:reject_reason;default:''" json:"reject_reason"`
	// ReviewedBy 审核者（admin.id / tutor.tutor_id）；ReviewedAt 审核/下架时间。
	ReviewedBy *int       `gorm:"column:reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (UserContribution) TableName() string { return "user_contribution" }

// UserContributionFile 投稿文件行（1–5 个；服务层校验单文件 ≤20MB、合计 ≤50MB）。
type UserContributionFile struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ContributionID int64     `gorm:"column:contribution_id;index" json:"contribution_id"`
	FileURL        string    `gorm:"column:file_url" json:"file_url"`
	FileName       string    `gorm:"column:file_name" json:"file_name"`
	FileSize       int64     `gorm:"column:file_size;default:0" json:"file_size"`
	ContentType    string    `gorm:"column:content_type;default:document" json:"content_type"` // document/video/ppt/zip/other
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (UserContributionFile) TableName() string { return "user_contribution_file" }

// ContributionDownload 下载量事实源（每人每稿终身一次；作者本人下载不写入）。
type ContributionDownload struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         int       `gorm:"column:user_id;uniqueIndex:uq_contribution_download" json:"user_id"`
	ContributionID int64     `gorm:"column:contribution_id;uniqueIndex:uq_contribution_download" json:"contribution_id"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ContributionDownload) TableName() string { return "contribution_download" }

// ContributionReport 举报（同一学员对同一投稿唯一；重复举报合并）。照 job_reports 先例不挂论坛举报表。
type ContributionReport struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ReporterID     int       `gorm:"column:reporter_id;uniqueIndex:uq_contribution_report" json:"reporter_id"`
	ContributionID int64     `gorm:"column:contribution_id;uniqueIndex:uq_contribution_report" json:"contribution_id"`
	Reason         string    `gorm:"column:reason" json:"reason"`           // piracy/content_error/violation/stale
	Status         int16     `gorm:"column:status;default:0" json:"status"` // 0 待处理 / 1 已处理
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ContributionReport) TableName() string { return "contribution_report" }
