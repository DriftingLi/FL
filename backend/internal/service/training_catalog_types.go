// Package service 培训目录 typed surface：专业方向/课程等级/证书模板/题库标签的
// Input 与 DTO 结构体（JSON 字段名与前端契约保持完全一致）。
package service

// SpecialtyInput 专业方向创建/更新入参。
// 更新语义（与旧 map 接口一致）：Code/Name 为空表示不改动；指针字段为 nil 表示不改动。
type SpecialtyInput struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
	Status      *int16  `json:"status"`
}

// SpecialtyDict 专业方向字典（列表/创建/更新返回）。
// 字段声明顺序与旧 map 字典的 JSON 键序（按键排序）一致，保证契约字节级不变。
type SpecialtyDict struct {
	Code        string `json:"code"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	SpecialtyID int    `json:"specialty_id"`
	Status      int16  `json:"status"`
}

// LevelInput 课程等级创建/更新入参。
type LevelInput struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
	Status      *int16  `json:"status"`
}

// LevelDict 课程等级字典。
type LevelDict struct {
	Code        string `json:"code"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
	LevelID     int    `json:"level_id"`
	Name        string `json:"name"`
	SortOrder   int    `json:"sort_order"`
	Status      int16  `json:"status"`
}

// CertificateTemplateInput 证书模板创建/更新入参。
type CertificateTemplateInput struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	ValidityDays *int    `json:"validity_days"`
	TemplateURL  *string `json:"template_url"`
	Status       *int16  `json:"status"`
}

// CertificateTemplateDict 证书模板字典。
type CertificateTemplateDict struct {
	Code         string `json:"code"`
	CreatedAt    string `json:"created_at"`
	Description  string `json:"description"`
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       int16  `json:"status"`
	TemplateURL  string `json:"template_url"`
	UpdatedAt    string `json:"updated_at"`
	ValidityDays int    `json:"validity_days"`
}

// QuestionTagInput 题库标签创建/更新入参。
type QuestionTagInput struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
	Status      *int16  `json:"status"`
}

// QuestionTagDict 题库标签字典。
// QuestionCount 仅列表返回（含 0 计数），创建/更新返回时省略（旧 map 契约同）。
type QuestionTagDict struct {
	Code          string `json:"code"`
	CreatedAt     string `json:"created_at"`
	Description   string `json:"description"`
	ID            int    `json:"id"`
	Name          string `json:"name"`
	QuestionCount *int64 `json:"question_count,omitempty"`
	SortOrder     int    `json:"sort_order"`
	Status        int16  `json:"status"`
	UpdatedAt     string `json:"updated_at"`
}

// QuestionTagRef 题目-标签关联摘要（id/code/name/sort_order/status，无时间戳等扩展字段）。
type QuestionTagRef struct {
	Code      string `json:"code"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Status    int16  `json:"status"`
}

// inputString 解引用 *string，nil 返回空串。
func inputString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// inputInt 解引用 *int，nil 返回 def。
func inputInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// inputInt16 解引用 *int16，nil 返回 def。
func inputInt16(p *int16, def int16) int16 {
	if p == nil {
		return def
	}
	return *p
}
