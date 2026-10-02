// Package contribution 资料投稿域的对外 DTO（ADR-0070 域包形态：DTO 与实现分层，形状锁见 dto_shape_test.go）。
package contribution

// ContributionAuthor 投稿作者信息（展示名 = 昵称；匿名投稿不展示）。
type ContributionAuthor struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Anonymous bool   `json:"anonymous"`
}

// ContributionFileDTO 投稿文件对象。
type ContributionFileDTO struct {
	// FileID 暂存文件尚未落库：key 不存在（omitempty）→ x-optional。
	FileID      int64  `json:"file_id,omitempty" extensions:"x-optional"`
	FileName    string `json:"file_name"`
	FileURL     string `json:"file_url"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// ContributionItemDTO 投稿对象。
type ContributionItemDTO struct {
	ID             int64                 `json:"id"`
	CredentialID   int                   `json:"credential_id"`
	Title          string                `json:"title"`
	Intro          string                `json:"intro"`
	Status         string                `json:"status"`
	IsAnonymous    bool                  `json:"is_anonymous"`
	DownloadsCount int                   `json:"downloads_count"`
	RejectReason   string                `json:"reject_reason,omitempty" extensions:"x-optional"`
	Files          []ContributionFileDTO `json:"files,omitempty" extensions:"x-optional" nullability:"nonnil"`
	// Author 的 omitempty 对结构体取值**无效**（encoding/json 不省略零值结构体）：key 恒在，
	// 生成物按必填渲染是正确的，前端手写的 author? 属过时宽容。
	Author    ContributionAuthor `json:"author,omitempty"`
	CreatedAt string             `json:"created_at"`
}

// ContributionPageResult 分页结果。
type ContributionPageResult struct {
	Items    []ContributionItemDTO `json:"items" nullability:"nonnil"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// CreateContributionInput 创建投稿入参。
type CreateContributionInput struct {
	UserID       int
	CredentialID int
	Title        string
	Intro        string
	IsAnonymous  bool
	Files        []ContributionFileDTO
	// FileURLs 直接传文件 URL 列表（已上传暂存）时使用（后端二次校验归属）。
	FileURLs []string
}

// ListPublicInput 公开广场列表入参。
type ListPublicInput struct {
	CredentialID int
	Sort         string // latest / hot
	Page         int
	PageSize     int
}

// DownloadResult 下载结果。
type DownloadResult struct {
	IsNew       bool `json:"is_new"`       // 是否新增一次计数（重复点击=false）
	TierAwarded int  `json:"tier_awarded"` // 本次触发的达阶奖励（0=未触发）
}

// ContributionReportItemDTO 举报条目（管理端队列）。
type ContributionReportItemDTO struct {
	ID                int64  `json:"id"`
	ReporterID        int    `json:"reporter_id"`
	ContributionID    int64  `json:"contribution_id"`
	ContributionTitle string `json:"contribution_title"`
	Reason            string `json:"reason"`
	Status            int16  `json:"status"` // 0 待处理 / 1 已处理
	CreatedAt         string `json:"created_at"`
}

// ContributionReportPageResult 举报分页结果。
type ContributionReportPageResult struct {
	Items    []ContributionReportItemDTO `json:"items" nullability:"nonnil"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
}
