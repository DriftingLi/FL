package model

import "time"

// 章节审核状态机（#2498，spec C1）——**独立命名**，不与题库/投稿/资料三台状态机混称。
// 三态 draft / pending / published + reject_reason（驳回＝回 draft 记理由，无独立 rejected 态）。
const (
	ChapterReviewDraft     = "draft"
	ChapterReviewPending   = "pending"
	ChapterReviewPublished = "published"
)

// ChapterContentSlot 章节内容版本槽（#2498，spec C4）：working 草稿在 chapter 行上，
// 在审快照（slot=pending）与已发布槽（slot=published）各占一行——过审＝pending→published
// 原子改槽（旧版在改槽前保留可见，C5），驳回＝删 pending 行（弃快照）。
// 冻结内容＝正文 + 文件挂接清单（清单在 chapter_file_slot，C3 整章一个审核单元）。
type ChapterContentSlot struct {
	Slot        string    `gorm:"column:slot;primaryKey" json:"slot"`
	ChapterID   int       `gorm:"column:chapter_id;primaryKey" json:"chapter_id"`
	Content     string    `gorm:"column:content" json:"content"`
	ContentType string    `gorm:"column:content_type" json:"content_type"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
	SubmittedBy int       `gorm:"column:submitted_by" json:"submitted_by"`
	FrozenAt    time.Time `gorm:"column:frozen_at" json:"frozen_at"`
}

// TableName 指定表名。
func (ChapterContentSlot) TableName() string { return "chapter_content_slot" }

// ChapterFileSlot 章节文件挂接清单槽（#2498，spec C3/C4/C13）：提交审核时从
// live 文件读面（chapter_file 行 + tutor_file 挂接 + legacy 折叠，source 判别沿 #2497）
// 冻结一份清单；学员读 published 槽、导师编辑面仍读 live。在审快照与已发布槽引用的
// tutor_file 视同挂接中（C13）——库侧删除守卫查本表。
type ChapterFileSlot struct {
	Slot        string    `gorm:"column:slot;primaryKey" json:"slot"`
	ChapterID   int       `gorm:"column:chapter_id;primaryKey" json:"chapter_id"`
	Source      string    `gorm:"column:source;primaryKey" json:"source"`
	FileID      int       `gorm:"column:file_id;primaryKey" json:"file_id"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	ContentType string    `gorm:"column:content_type" json:"content_type"`
	FileSize    int64     `gorm:"column:file_size" json:"file_size"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名。
func (ChapterFileSlot) TableName() string { return "chapter_file_slot" }
