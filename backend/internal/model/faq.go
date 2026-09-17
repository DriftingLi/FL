package model

import "time"

// ===== 帮助中心（FAQ，#1079）=====

// FaqCategory 帮助中心分类：学员端左侧分类的可排序 / 可停用清单。
// code 是稳定标识——管理端与日志引用它，不引用自增 id（改名不改引用）。
type FaqCategory struct {
	ID        int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Code      string `gorm:"column:code;size:64;uniqueIndex" json:"code"`
	Title     string `gorm:"column:title" json:"title"`
	SortOrder int    `gorm:"column:sort_order;default:0" json:"sort_order"`
	// 注意：**不能**写 gorm 的 `default:true`——GORM 见到零值 false 会省略该列、
	// 交给 DB 默认值，于是「停用」永远存不进去（写入静默变成 true）。默认值只留在
	// 迁移的 DDL 里当兜底，写路径一律显式落值。
	Enabled   bool      `gorm:"column:enabled" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (FaqCategory) TableName() string { return "faq_category" }

// Faq 帮助中心条目：**只有已发布 / 停用两态**，不做草稿工作流（#1079 明确不做）。
// answer 是纯文本、原样换行渲染——不引入 Markdown 的内容渲染口径。
type Faq struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CategoryID int    `gorm:"column:category_id" json:"category_id"`
	Question   string `gorm:"column:question" json:"question"`
	Answer     string `gorm:"column:answer" json:"answer"`
	SortOrder  int    `gorm:"column:sort_order;default:0" json:"sort_order"`
	// 同上：`default:true` 会让「未发布」存不进去。
	Published bool `gorm:"column:published" json:"published"`
	// #1099 验收探针（临时）：模型有列、迁移没有 —— 用于验证 migration-check 的单向列对账会红。
	ProbeMissing string    `gorm:"column:probe_missing_col" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Faq) TableName() string { return "faq" }
