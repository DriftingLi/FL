// 本文件：课程主类型与其**内容级互斥**不变式（#2760）。
//
// 两件事在这里合流，因为它们说的是同一句话的两半：
//   - 课程有一个持久化单选类型（video / illustrated），公开目录按它分流；
//   - 一门课的章节内容必须**同质**：要么全是视频，要么全非视频。
//
// 为什么要有第二半：主类型只说「这门课挂在哪个目录」，不说「里面真是什么」。允许图文课
// 挂视频章节，访客在「视频课程」里点开的课就可能是文章 —— 目录口径成立的「彻底区分」在
// 内容事实上不成立。用户 2026-11-22 明确要求的是后者（「不要出现一个课程既有图文又有视频」）。
//
// 归属函数的边界：互斥判据只在**章节内容形态**上成立，不涉及专业方向/课程等级/分类
// （那三个是组织维度，与学习形式正交）。
package course

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// 课程类型写面的三条输入事实（ADR-0065 决策 3：各有名有姓，两面共用一张表）。
var (
	// ErrCourseTypeInvalid 类型值不在 video / illustrated 之内（含空串）。
	ErrCourseTypeInvalid = errors.New("课程类型必须是 video 或 illustrated")
	// ErrCourseTypeRequired 创建课程未指定类型。类型是必填：缺省值会让「没选过」与
	// 「选了 illustrated」在库里长得一样，之后无从复核。
	ErrCourseTypeRequired = errors.New("请选择课程类型")
	// ErrCourseContentTypeConflict 该章节内容形态与本课类型冲突（内容级互斥）。
	ErrCourseContentTypeConflict = errors.New("章节内容与本课类型冲突")
)

// ValidCourseTypes 类型的全部合法取值（顺序即公开入口的展示顺序：视频在前）。
//
// 单独一份切片而不是两次 if：写入校验、DTO 生成、前端选项都从这一处取值——
// 两态变三态时只改这里，不会有一个面忘了新值。
var ValidCourseTypes = []string{model.CourseTypeVideo, model.CourseTypeIllustrated}

// IsValidCourseType 类型值是否合法。
func IsValidCourseType(raw string) bool {
	for _, t := range ValidCourseTypes {
		if t == raw {
			return true
		}
	}
	return false
}

// chapterContentNature 把章节内容形态归到课程类型的两侧（#2760 内容级互斥的判据单点）。
//
// 映射表与理由：
//   - video               ⇒ 视频侧
//   - ppt / text / document ⇒ 图文侧（**PPT 归图文**：票 2760 写死「图文课程含 PPT」）
//   - 空串                 ⇒ 图文侧（章节内容类型默认 text，见 model.Chapter 的 gorm default；
//     空串只出现在测试夹具与极早期行，按默认侧归类与库侧 default 同判）
//
// 认不出的新形态（后端日后加一种 content_type）**不猜**：返回 ok=false，调用方按
// 「无法证明同质」处理。猜一侧就会把新形态悄悄塞进某一类课程，而这正是本不变式要防的。
func chapterContentNature(contentType string) (courseType string, ok bool) {
	switch contentType {
	case "video":
		return model.CourseTypeVideo, true
	case "ppt", "text", "document", "":
		return model.CourseTypeIllustrated, true
	default:
		return "", false
	}
}

// requireChapterContentMatchesCourse 内容级互斥写面护栏（#2760）：
// 往一门课里写章节之前，先答「这个内容形态与这门课的类型相容吗」。
//
// 为什么要这道护栏而不是只在管理端 UI 上限制：能改**章节生效形态**的写入方有多条
// （管理员建章/改章、讲师上传章节文件、讲师删/换附件后重算），UI 只是其中一条的入口。
// 不变式落在服务层才有意义 —— UI 层的限制是提示，不是判据。
//
// 边界：文件库的挂接/换挂（internal/tutorfile 的 AttachFile / SwapChapterFile）**不在**
// 这道护栏的射程内 —— 它们只改 tutor_file.chapter_id 的挂接关系，不改 chapter.content_type
// 这个形态判据（形态来自 chapter 行与其已发布槽）。把它们算进来会是一句假话。
//
// contentType 传**即将生效的**形态（不是 chapter 行当前值）：讲师上传视频到图文章节时，
// 生效后形态就是 video，冲突要按生效后判。
//
// 查不动章节/课程一律上抛，不压成「不冲突」（ADR-0064 决策 1）。
func requireChapterContentMatchesCourse(db *gorm.DB, courseID int, contentType string) error {
	nature, ok := chapterContentNature(contentType)
	if !ok {
		// 认不出的形态不能证明同质 ⇒ 拒。留一条不猜的口径，新形态接入时必须显式登记。
		return fmt.Errorf("%w：不支持的内容形态 %q", ErrCourseContentTypeConflict, contentType)
	}
	var course model.Course
	if err := db.Select("course_id", "course_type").First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrCourseNotFound
		}
		return err
	}
	if course.CourseType == nature {
		return nil
	}
	return fmt.Errorf("%w：本课是%s课程，不能放入%s章节", ErrCourseContentTypeConflict,
		courseTypeLabel(course.CourseType), contentTypeLabel(contentType))
}

// chapterCourseID 取章节所属课程 id（章节不存在 ⇒ ErrChapterNotFound）。
// 互斥护栏的调用方手里只有章节 id，需要它换算课程类型。
func chapterCourseID(db *gorm.DB, chapterID int) (int, error) {
	var chapter model.Chapter
	if err := db.Select("chapter_id", "course_id").First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrChapterNotFound
		}
		return 0, err
	}
	return chapter.CourseID, nil
}

// RecomputeChapterContentTypeAfterFileChange 讲师删/换附件后重算章节生效形态（#2760 内容级互斥）。
//
// 讲师删掉章节的最后一个视频文件后，旧代码把 ContentType 硬写成 "text"——
// 一门视频课去掉唯一视频附件，当场变成图文章节，目录却还说它是视频课。
//
// 口径：**不跨侧**。按剩余挂接文件的第一条重算形态（与既有 file_url = remaining[0] 同序），
// 但若重算结果会跨到课程类型的另一侧（视频课算出图文 / 图文课算出视频），就保留原形态、
// 只更新 file_url。理由：删文件只是「内容少了」，不该顺手把章节改判成另一类内容；
// 一个空的视频章节仍是视频形态，不破坏「这门课全是视频」的目录承诺。
// 没有剩余文件时回 text，但同样要过不跨侧这一关（视频课会保留 video）。
//
// 跨包导出给 internal/tutor 用：形态归类的唯一出处在本文件（chapterContentNature），
// 不让 tutor 包自己另写一份 text/video 判断——两处一分叉，护栏就形同虚设。
func RecomputeChapterContentTypeAfterFileChange(db *gorm.DB, chapterID int) error {
	var chapter model.Chapter
	if err := db.Select("chapter_id", "course_id", "content_type").First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChapterNotFound
		}
		return err // 查不动上抛（ADR-0064 决策 1）
	}
	var course model.Course
	if err := db.Select("course_id", "course_type").First(&course, chapter.CourseID).Error; err != nil {
		return err
	}

	var remaining []model.ChapterFile
	if err := db.Where("chapter_id = ?", chapterID).Order("created_at").Find(&remaining).Error; err != nil {
		return err
	}
	newContentType, newFileURL := "text", ""
	if len(remaining) > 0 {
		newContentType, newFileURL = remaining[0].ContentType, remaining[0].FileURL
	}
	// 不跨侧：重算结果若与课程类型相反，保留原形态。
	if nature, ok := chapterContentNature(newContentType); ok && nature != course.CourseType {
		newContentType = chapter.ContentType
	}
	return db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
		Updates(map[string]any{"content_type": newContentType, "file_url": newFileURL}).Error
}

// RequireChapterContentNature 跨包出口（讲师上传章节文件 / 删改附件用，见 internal/tutor）：
// 这份即将生效的内容形态与本课类型相容吗。
//
// 为什么要导出而不是让 tutor 包自己判：互斥判据的**唯一**出处在 internal/course
// （chapterContentNature）。tutor 包从自己那里再写一份条件，就是第二份判据——
// 两处一分叉，「管理端拦住了但讲师没拦住」这种半开状态就回来了。
func RequireChapterContentNature(db *gorm.DB, chapterID int, contentType string) error {
	return requireChapterContentMatchesCourseByChapterID(db, chapterID, contentType)
}

// chapterContentTypeOrDefault 章节创建/写入时「没显式给形态」的落点。
//
// 库侧 model.Chapter.ContentType 的 gorm default 就是 text（见 model.Chapter 的 tag），
// 这里取同一个默认值：两处若分叉，「建章时判的形态」与「库里真存的形态」就会不一致，
// 而互斥不变式是拿后者衡量的。
func chapterContentTypeOrDefault(contentType *string) string {
	if contentType == nil || *contentType == "" {
		return "text"
	}
	return *contentType
}

// requireChapterContentMatchesCourseByChapterID 章节 id 版本的互斥护栏
// （讲师上传章节文件这条写面手里只有章节 id）。
func requireChapterContentMatchesCourseByChapterID(db *gorm.DB, chapterID int, contentType string) error {
	courseID, err := chapterCourseID(db, chapterID)
	if err != nil {
		return err
	}
	return requireChapterContentMatchesCourse(db, courseID, contentType)
}

// courseTypeLabel / contentTypeLabel 是互斥错误文案的中文标签。
// 错误文案要能让人看懂「为什么拒」——只报两个英文枚举值，管理员得去查文档才知道该改哪边。
func courseTypeLabel(courseType string) string {
	if courseType == model.CourseTypeVideo {
		return "视频"
	}
	return "图文"
}

func contentTypeLabel(contentType string) string {
	nature, ok := chapterContentNature(contentType)
	if !ok {
		return contentType
	}
	if nature == model.CourseTypeVideo {
		return "视频"
	}
	return "图文"
}

// requireCourseTypeMatchesChapters 改类型的反向护栏（#2760 内容级互斥）：
// 把课改成某类型之前，确认它**现有**章节内容全部落在该类型一侧。
//
// 与 requireChapterContentMatchesCourse 是一对方向相反的守卫，两条都必须在：
// 前者防「往图文章节里塞视频」，后者防「把含视频章节的课改成图文课」——
// 只做一边的话，绕过另一边就能造出混合课（改类型比改内容省事，是最容易被走的那条路）。
//
// 判据取 working 草稿（chapter.content_type）而不是已发布槽：草稿是将要生效的内容，
// 用它判才能拦住「草稿里刚加的视频、还没提交审核就改类型」。
// 查不动章节表一律上抛（ADR-0064 决策 1）。
func requireCourseTypeMatchesChapters(db *gorm.DB, courseID int, courseType string) error {
	var rows []struct {
		ChapterID   int    `gorm:"column:chapter_id"`
		ContentType string `gorm:"column:content_type"`
	}
	if err := db.Model(&model.Chapter{}).
		Select("chapter_id, content_type").
		Where("course_id = ?", courseID).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		nature, ok := chapterContentNature(row.ContentType)
		if !ok {
			return fmt.Errorf("%w：章节 %d 的内容形态 %q 无法归类", ErrCourseContentTypeConflict,
				row.ChapterID, row.ContentType)
		}
		if nature != courseType {
			return fmt.Errorf("%w：章节「%d」是%s内容，不能把本课改为%s课程",
				ErrCourseContentTypeConflict, row.ChapterID,
				contentTypeLabel(row.ContentType), courseTypeLabel(courseType))
		}
	}
	return nil
}

// courseTypeFromInput 解析写入面传来的 course_type（#2760）。
//
// 两种输入形态（与 points_price 的既有口径同构，见 CourseInput 注释）：
//   - 携带（无论值是否合法）⇒ provided=true，由调用方校验值域
//   - 未携带 ⇒ provided=false，编辑时保留原值、创建时由调用方报必填
func courseTypeFromInput(in *CourseInput) (string, bool) {
	if in == nil || in.CourseType == nil {
		return "", false
	}
	return *in.CourseType, true
}
