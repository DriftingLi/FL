// 资料投稿域（CONTEXT.md「资料投稿」）。
package model

import "time"

// ===== 30. 资料投稿域（#517 / ADR-0026，学员上传资料换积分） =====

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
