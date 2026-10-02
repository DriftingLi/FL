// 课程域 typed 契约（ADR-0009 收尾，spec #166 T5；P2 波 3b-1 随课程域搬进 internal/course）：
// 取代 map[string]any 输入输出；指针字段区分「未携带」与「零值」，
// JSON key 与旧 map 逐字一致（shape-lock 由既有契约测试兜底）。
package course

// CourseInput 课程创建/编辑请求体。
// 前置课程切片 nil = 未携带（编辑时保留现状）；[] = 显式清空。
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
