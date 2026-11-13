// 课程域 typed 契约（ADR-0009 收尾，spec #166 T5；P2 波 3b-1 随课程域搬进 internal/course）：
// 取代 map[string]any 输入输出；指针字段区分「未携带」与「零值」，
// JSON key 与旧 map 逐字一致（shape-lock 由既有契约测试兜底）。
package course

import "encoding/json"

// CourseInput 课程创建/编辑请求体。
// 前置课程切片 nil = 未携带（编辑时保留现状）；[] = 显式清空。
// points_price（#2447）三种形态：≥1 的整数落值；显式 null = 清空（免费）；未携带 = 保留现状
// （新建未携带即免费）。0/负数/非整数拒绝——「免费」不写 0：000002 迁移的列约束是
// points_price IS NULL OR points_price > 0，免费的落点是 NULL。
type CourseInput struct {
	Name                  *string `json:"name"`
	Description           *string `json:"description"`
	CoverImage            *string `json:"cover_image"`
	Duration              *int    `json:"duration"`
	Status                *int16  `json:"status"`
	SortOrder             *int    `json:"sort_order"`
	CredentialID          *int    `json:"credential_id"`
	SpecialtyID           *int    `json:"specialty_id"`
	LevelID               *int    `json:"level_id"`
	CertificateTemplateID *int    `json:"certificate_template_id"`
	TheoryHours           *int    `json:"theory_hours"`
	PracticeHours         *int    `json:"practice_hours"`
	PrerequisiteCourseIDs []int   `json:"prerequisite_course_ids"`
	IsHot                 *bool   `json:"is_hot"`
	IsFeatured            *bool   `json:"is_featured"`
	PointsPrice           *int    `json:"points_price"`
	// pointsPriceCleared 请求体显式携带了 points_price: null（清空为免费）。标准 *int 解码
	// 分不开「显式 null」与「缺键」（两者都落 nil），这两个语义（清空 / 保留现状）只能靠
	// 本标志分开——见下方 UnmarshalJSON。
	pointsPriceCleared bool
}

// UnmarshalJSON 实现 json.Unmarshaler：解码全部走标准语义，只额外记 points_price 的
// 「显式清空」标志（见结构体注释）。
func (in *CourseInput) UnmarshalJSON(b []byte) error {
	type wire CourseInput // 无方法的影子类型，避免递归回本方法
	var w wire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*in = CourseInput(w)
	if in.PointsPrice != nil {
		return nil
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		return err
	}
	raw, ok := keys["points_price"]
	in.pointsPriceCleared = ok && string(raw) == "null"
	return nil
}

// ChapterInput 章节创建/编辑请求体。
type ChapterInput struct {
	Title       *string `json:"title"`
	Content     *string `json:"content"`
	Duration    *int    `json:"duration"`
	OrderNum    *int    `json:"order_num"`
	Description *string `json:"description"`
}

// DeleteCourseResult 删除课程结果（原 map{"course_id"} 的 typed 形态）。
type DeleteCourseResult struct {
	CourseID int `json:"course_id"`
}

// DeleteChapterResult 删除章节结果。
type DeleteChapterResult struct {
	ChapterID int `json:"chapter_id"`
}
