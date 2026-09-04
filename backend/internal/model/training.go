// 培训域：课程目录（目标证件 → 专业方向 → 课程等级 → 课程）与学习内容（CONTEXT.md「培训领域」）。
package model

import "time"

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
