// Package service 测试：课程 DTO shape-lock（B5 决策 D6）。
// 断言 JSON key 集合与 B5 前的 map 契约逐字一致——前端契约零改动是最高优先级约束。
package service

import "testing"

func sampleCourseDTO() CourseDTO {
	return CourseDTO{
		CourseID:              1,
		Name:                  "叉车基础",
		Description:           "描述",
		CoverImage:            "/static/uploads/covers/1.png",
		Duration:              120,
		SpecialtyID:           ptrInt(1),
		LevelID:               ptrInt(2),
		TheoryHours:           20,
		PracticeHours:         10,
		CertificateTemplateID: nil,
		SortOrder:             3,
		Status:                1,
		CreatedAt:             "2026-08-01T10:00:00",
	}
}

func TestCourseDTOShapeLock(t *testing.T) {
	c := sampleCourseDTO()
	// 未填充的可选字段（chapter_count 等）省略；created_at 等必填字段恒在
	assertShapeLock(t, c,
		"course_id", "name", "description", "cover_image", "duration",
		"specialty_id", "level_id", "theory_hours", "practice_hours",
		"certificate_template_id", "sort_order", "status", "created_at",
	)

	// 列表/详情填充路径：chapter_count / prerequisite_course_ids 恒存在（含 0 与空数组）
	zero := int64(0)
	c.ChapterCount = &zero
	ids := []int{}
	c.PrerequisiteCourseIDs = &ids
	assertShapeLock(t, c,
		"course_id", "name", "description", "cover_image", "duration",
		"specialty_id", "level_id", "theory_hours", "practice_hours",
		"certificate_template_id", "sort_order", "status", "created_at",
		"chapter_count", "prerequisite_course_ids",
	)
	if b, _ := marshalJSON(t, c); string(b) != `{"certificate_template_id":null,"chapter_count":0,"cover_image":"/static/uploads/covers/1.png","course_id":1,"created_at":"2026-08-01T10:00:00","description":"描述","duration":120,"level_id":2,"name":"叉车基础","practice_hours":10,"prerequisite_course_ids":[],"sort_order":3,"specialty_id":1,"status":1,"theory_hours":20}` {
		t.Fatalf("course 序列化与旧 map 契约不符（含 null/空数组语义）: %s", b)
	}

	// 详情元数据填充路径
	c.CertificateName = "测试证书"
	c.Specialty = &SpecialtyBriefDTO{SpecialtyID: 1, Code: "mt", Name: "维修"}
	c.Level = &LevelBriefDTO{LevelID: 2, Code: "bg", Name: "入门"}
	c.CertificateTemplate = &CertificateTemplateDTO{ID: 1, Code: "C1", Name: "证书", Description: "", ValidityDays: 365, TemplateURL: "https://x/t.pdf"}
	c.Prerequisites = &[]CourseBriefDTO{{CourseID: 9, Name: "前置"}}
	c.StudentCount = &zero
	chs := []ChapterDTO{}
	c.Chapters = &chs
	assertShapeLock(t, c,
		"course_id", "name", "description", "cover_image", "duration",
		"specialty_id", "level_id", "theory_hours", "practice_hours",
		"certificate_template_id", "sort_order", "status", "created_at",
		"chapter_count", "prerequisite_course_ids", "certificate_name",
		"specialty", "level", "certificate_template", "prerequisites",
		"student_count", "chapters",
	)
}

func TestChapterDTOShapeLock(t *testing.T) {
	ch := ChapterDTO{
		ChapterID: 1, CourseID: 1, Title: "第一章", Content: "正文", ContentURL: "",
		ContentType: "text", FileURL: "", Description: "", Duration: 30, OrderNum: 1,
		CreatedAt: "2026-08-01T10:00:00",
	}
	assertShapeLock(t, ch,
		"chapter_id", "course_id", "title", "content", "content_url",
		"content_type", "file_url", "description", "duration", "order_num", "created_at",
	)

	// 章节详情：files 恒存在（含空数组）+ 上下章 ID（null 语义）+ study_status
	detail := ChapterDetailDTO{
		ChapterDTO:        ch,
		Files:             []ChapterFileDTO{},
		PreviousChapterID: nil,
		NextChapterID:     nil,
		StudyStatus:       "not_started",
	}
	assertShapeLock(t, detail,
		"chapter_id", "course_id", "title", "content", "content_url",
		"content_type", "file_url", "description", "duration", "order_num", "created_at",
		"files", "previous_chapter_id", "next_chapter_id", "study_status",
	)
}

func TestChapterFileDTOShapeLock(t *testing.T) {
	// legacy 兼容条目：file_id=0、chapter_id=int（非 null）
	legacy := ChapterFileDTO{
		ChapterID: ptrInt(1), ContentType: "document", CreatedAt: "2026-08-01T10:00:00",
		FileID: 0, FileName: "a.pdf", FileSize: 0, FileURL: "/static/uploads/chapters/a.pdf",
	}
	assertShapeLock(t, legacy,
		"chapter_id", "content_type", "created_at", "file_id", "file_name", "file_size", "file_url",
	)

	// chapter_file 表条目：chapter_id 为 null 时 key 仍在（null 语义）
	noChapter := ChapterFileDTO{
		ContentType: "ppt", CreatedAt: "2026-08-01T10:00:00",
		FileID: 2, FileName: "b.ppt", FileSize: 1024, FileURL: "/static/uploads/chapters/b.ppt",
	}
	assertShapeLock(t, noChapter,
		"chapter_id", "content_type", "created_at", "file_id", "file_name", "file_size", "file_url",
	)
	if b, _ := marshalJSON(t, noChapter); string(b) != `{"chapter_id":null,"content_type":"ppt","created_at":"2026-08-01T10:00:00","file_id":2,"file_name":"b.ppt","file_size":1024,"file_url":"/static/uploads/chapters/b.ppt"}` {
		t.Fatalf("chapter_file 序列化与旧 map 契约不符（chapter_id 为 null）: %s", b)
	}
}

func TestCourseEnvelopeDTOShapeLock(t *testing.T) {
	detail := CourseDetailDTO{CourseInfo: sampleCourseDTO(), Chapters: []ChapterDTO{}, Progress: 0}
	assertShapeLock(t, detail, "course_info", "chapters", "progress")

	admin := AdminCourseDetailDTO{CourseDTO: sampleCourseDTO(), Chapters: []ChapterDTO{}}
	keys := topLevelKeys(t, admin)
	for _, k := range []string{"course_id", "name", "chapters"} {
		if !keys[k] {
			t.Errorf("管理端课程详情缺少 key: %s", k)
		}
	}

	tutor := TutorCourseChaptersDTO{Course: sampleCourseDTO(), Chapters: []ChapterDTO{}}
	assertShapeLock(t, tutor, "course", "chapters")

	slides := ChapterSlidesDTO{ChapterID: 1, Slides: []string{}}
	assertShapeLock(t, slides, "chapter_id", "slides")

	progress := StudyProgressDTO{RecordID: 1, Progress: 50.5, StudyDuration: 30}
	assertShapeLock(t, progress, "record_id", "progress", "study_duration")

	page := CoursePageResult{Courses: []CourseDTO{sampleCourseDTO()}, Page: 1, Pages: 1, Total: 1}
	assertShapeLock(t, page, "courses", "page", "pages", "total")
}
