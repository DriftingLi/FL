// Package course 管理端课程 CRUD。
package course

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/coerce"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/scope"
	"forklift-training/internal/sortorder"
)

// AdminService 管理端课程服务。
type AdminService struct {
	db      *gorm.DB
	fileSvc *filestore.FileStore
	// notifSvc 站内信服务（#2537 / ADR-0095）：章节审核结果通知讲师，与状态流转同事务。
	// 可 nil——nil 时跳过投递（测试装配与「通知域未接线」的历史路径）。
	notifSvc *notification.Service

	logger *zap.Logger
}

// NewAdminService 创建管理端课程服务实例。fileSvc 用于删除章节时清理幻灯片/图文图片
// （可 nil，nil 时跳过）；notifSvc 用于章节审核结果站内信（可 nil，nil 时跳过投递）。
func NewAdminService(db *gorm.DB, fileSvc *filestore.FileStore, notifSvc *notification.Service, logger *zap.Logger) *AdminService {
	return &AdminService{db: db, fileSvc: fileSvc, notifSvc: notifSvc, logger: logger}
}

// GetCourses 管理端课程列表。filter 支持 hot|featured|all（空串 = all；#1096：显式参数，不再是变参）。
func (s *AdminService) GetCourses(page, pageSize int, keyword string, credentialID, specialtyID, levelID *int, category scope.CategoryFilter, filter string) (CoursePageResult, error) {
	return ListCourses(s.db, page, pageSize, CourseListOptions{
		Keyword: keyword, CredentialID: credentialID, SpecialtyID: specialtyID, LevelID: levelID,
		CategoryFilter: category, Filter: filter, WithPublicState: true, DefaultPageSize: 10,
	})
}

// GetCourseDetail 管理端课程详情。
func (s *AdminService) GetCourseDetail(courseID int) (*AdminCourseDetailDTO, error) {
	course, chapterList, err := loadCourseWithChapters(s.db, courseID)
	if err != nil {
		return nil, err
	}
	detail := CourseToDTO(course)
	fillCourseMeta(s.db, course, &detail)
	items := []CourseDTO{detail}
	if err := FillCoursePublicStates(s.db, items); err != nil {
		return nil, err
	}
	return &AdminCourseDetailDTO{
		CourseDTO: items[0],
		Chapters:  chapterList,
	}, nil
}

// ErrCoursePointsPriceInvalid 积分价非法（#2447）：可写值只有 ≥1 的整数；「免费」不写 0——
// 000002 迁移的列约束是 points_price IS NULL OR points_price > 0，免费的落点是 NULL。
var ErrCoursePointsPriceInvalid = errors.New("积分价必须为大于0 的整数，免费请留空")

// applyCoursePointsPrice 应用积分价（#2447）。三种输入形态（见 CourseInput 注释）：
// 携带值（≥1 校验后落库）；显式 null（清空为免费）；未携带（编辑保留现状 / 新建免费）。
func applyCoursePointsPrice(course *model.Course, in *CourseInput) error {
	if in.PointsPrice != nil {
		if *in.PointsPrice < 1 {
			return ErrCoursePointsPriceInvalid
		}
		course.PointsPrice = coerce.IntPtr(*in.PointsPrice)
		return nil
	}
	if in.pointsPriceCleared {
		course.PointsPrice = nil
	}
	return nil
}

// CreateCourse 创建课程并校验一级分类。
func (s *AdminService) CreateCourse(in *CourseInput) (*CourseDTO, error) {
	if in == nil || in.Name == nil || *in.Name == "" {
		return nil, ErrCourseNameRequired
	}
	if in.CategoryID == nil {
		return nil, ErrCourseCategoryRequired
	}
	status := int16(1)
	if in.Status != nil {
		status = *in.Status
	}
	// 课程类型必填（#2760）：未携带即拒，不落默认值——「没选过」与「选了 illustrated」
	// 若在库里同形，之后就无从复核，而公开目录的分流全靠这一格。
	//
	// 顺序：类型校验排在 applyCourseTrainingFields（分类/方向/等级校验）**之后**——
	// 「分类不存在」比「没选类型」更根本（分类错了整门课都建不成），既有用例也按那个顺序
	// 断言错误。故这里只做**值域**的早校验，必填判定放到分类校验之后。
	courseType, provided := courseTypeFromInput(in)
	if provided && !IsValidCourseType(courseType) {
		return nil, ErrCourseTypeInvalid
	}
	course := model.Course{
		Name:      *in.Name,
		Status:    status,
		CreatedAt: clock.Now(),
	}
	if provided {
		course.CourseType = courseType
	}
	if in.Description != nil {
		course.Description = *in.Description
	}
	if in.CoverImage != nil {
		course.CoverImage = *in.CoverImage
	}
	if in.Duration != nil {
		course.Duration = *in.Duration
	}
	if err := applyCourseTrainingFields(s.db, &course, in); err != nil {
		return nil, err
	}
	// 类型必填（#2760）：放在分类/方向/等级校验之后 —— 分类错了整门课都建不成，
	// 那个错误更根本，先答它。
	if !provided {
		return nil, ErrCourseTypeRequired
	}
	if err := applyCoursePointsPrice(&course, in); err != nil {
		return nil, err
	}
	if in.SortOrder == nil {
		course.SortOrder = sortorder.NextValue(s.db, "course",
			map[string]any{"category_id": *course.CategoryID})
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&course).Error; err != nil {
			return err
		}
		if course.Status != status {
			if err := tx.Model(&course).Update("status", status).Error; err != nil {
				return err
			}
			course.Status = status
		}
		return replaceCoursePrerequisites(tx, course.CourseID, in.PrerequisiteCourseIDs)
	}); err != nil {
		return nil, err
	}
	result := CourseToDTO(&course)
	fillChapterCount(s.db, course.CourseID, &result)
	FillPrereqIDs(s.db, course.CourseID, &result)
	if err := FillCoursePublicState(s.db, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateCourse 更新课程。
func (s *AdminService) UpdateCourse(courseID int, in *CourseInput) (*CourseDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if in == nil {
		in = &CourseInput{}
	}
	// 原类型要在任何赋值之前留底：下面的护栏只在「类型真变化」时触发（见下方注释），
	// 拿它和这里的值比才知道变没变。
	originalCourseType := course.CourseType
	if in.Name != nil && *in.Name != "" {
		course.Name = *in.Name
	}
	if in.Description != nil {
		course.Description = *in.Description
	}
	if in.CoverImage != nil {
		course.CoverImage = *in.CoverImage
	}
	if in.Duration != nil {
		course.Duration = *in.Duration
	}
	if in.Status != nil {
		course.Status = *in.Status
	}
	// 课程类型（#2760）：携带则校验后落值；未携带保留原值——老调用方（只改 is_hot 的开关、
	// 只切上下架）不该因为没带这个字段就 400。
	if courseType, provided := courseTypeFromInput(in); provided {
		if !IsValidCourseType(courseType) {
			return nil, ErrCourseTypeInvalid
		}
		course.CourseType = courseType
	}
	if err := applyCourseTrainingFields(s.db, &course, in); err != nil {
		return nil, err
	}
	// 改类型必须有内容级证据（#2760 内容级互斥）：类型与章节内容同质是**不变式**，不是
	// 只在建课时成立的一次性校验。否则管理员在图文课里挂了 3 个视频章节，再把类型改成
	// video 就悄悄合规了——那正是「一门课既有图文又有视频」要防的形态。
	//
	// 只在类型**真的变化**时判：规格要求「课程内容更新后仍由我决定主类型」，存量混合课
	// （迁移预填时留下、等待复核）在管理员改个名字或切一下上下架时不该被这道护栏挡住——
	// 那会把「复核前维持归类稳定」变成「什么都改不了」。没带 course_type 时 course.CourseType
	// 保留原值，与 originalCourseType 必然相等，自然跳过。
	if course.CourseType != originalCourseType {
		if err := requireCourseTypeMatchesChapters(s.db, courseID, course.CourseType); err != nil {
			return nil, err
		}
	}
	if err := applyCoursePointsPrice(&course, in); err != nil {
		return nil, err
	}
	if err := s.db.Save(&course).Error; err != nil {
		return nil, err
	}
	if in.PrerequisiteCourseIDs != nil {
		if err := replaceCoursePrerequisites(s.db, courseID, in.PrerequisiteCourseIDs); err != nil {
			return nil, err
		}
	}
	result := CourseToDTO(&course)
	fillChapterCount(s.db, courseID, &result)
	FillPrereqIDs(s.db, courseID, &result)
	if err := FillCoursePublicState(s.db, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// 交换排序的三件课程侧输入事实（ADR-0065 决策 3）。此前它们是裸 errors.New ⇒ 落在
// 端点默认面 400，与「查不动」「待交换的项不存在」挤成同一格；现在各自有名，两面共用一张表。
var (
	ErrCourseNotMountedForSort = errors.New("未分类的课程不能参与排序")
	ErrCourseSortGroupMismatch = errors.New("只能交换同一分类内的课程")
	// ErrCourseSwapTargetNotFound 指 body 里 `swap_with` 指向的那门课不存在 —— 与「路径里那门课
	// 不存在」（model.ErrCourseNotFound ⇒ 404）是两件事实。改之前两次 First 都回同一个哨兵，于是
	// 「你要换的东西没有」对外答 404「课程不存在」，与前置课程那一支的 400 也不一致（ADR-0065 决策 3）。
	ErrCourseSwapTargetNotFound = errors.New("待交换的课程不存在")
)

// SwapCourseSort 交换同一分类内两门课程的位置。
func (s *AdminService) SwapCourseSort(a, b int) error {
	var ca, cb model.Course
	if err := s.db.First(&ca, a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrCourseNotFound
		}
		return err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if err := s.db.First(&cb, b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCourseSwapTargetNotFound // 坏引用是输入不合法(400)，不是「路径资源没有」(404)
		}
		return err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if ca.CategoryID == nil || cb.CategoryID == nil {
		return ErrCourseNotMountedForSort
	}
	if *ca.CategoryID != *cb.CategoryID {
		return ErrCourseSortGroupMismatch
	}
	return sortorder.SwapPositions(s.db, &model.Course{}, "course_id", a, b,
		map[string]any{"category_id": *ca.CategoryID})
}

// DeleteCourse 删除课程。
func (s *AdminService) DeleteCourse(courseID int) (*DeleteCourseResult, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if err := s.db.Delete(&course).Error; err != nil {
		return nil, err
	}
	return &DeleteCourseResult{CourseID: courseID}, nil
}

// CreateChapter 创建章节。
func (s *AdminService) CreateChapter(courseID int, in *ChapterInput) (*ChapterDTO, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if in == nil || in.Title == nil || *in.Title == "" {
		return nil, errors.New("章节标题不能为空")
	}
	// 内容级互斥（#2760）：新建章节的默认形态是 text（= 图文侧），若这门课是视频课，
	// 现在建章就已经埋下混合课。第 ③ 步写内容形态时还会再判一次，但这里必须**先**拒：
	// 「建一个注定不合规的空章节」本身就不该发生。
	if err := requireChapterContentMatchesCourse(s.db, courseID, chapterContentTypeOrDefault(in.ContentType)); err != nil {
		return nil, err
	}

	var maxOrder int
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).
		Select("COALESCE(MAX(order_num), 0)").Scan(&maxOrder)

	chapter := model.Chapter{
		CourseID: courseID,
		Title:    *in.Title,
		// 内容形态必须与校验时用的是**同一个值**（#2760 内容级互斥）：
		// 上面按 chapterContentTypeOrDefault(in.ContentType) 判同质，这里就必须原样落库。
		// 若只判不落，传 content_type=video 会「按视频通过、按 text 落库」——
		// 不变式在写入那一刻就破了，而公开归类仍然说这是视频课。
		ContentType: chapterContentTypeOrDefault(in.ContentType),
		OrderNum:    maxOrder + 1,
		CreatedAt:   clock.Now(),
	}
	if in.Content != nil {
		chapter.Content = *in.Content
	}
	if in.Duration != nil {
		chapter.Duration = *in.Duration
	}
	if err := s.db.Create(&chapter).Error; err != nil {
		return nil, err
	}
	d := ChapterToDTO(&chapter)
	return &d, nil
}

// UpdateChapter 更新章节。
func (s *AdminService) UpdateChapter(ctx context.Context, chapterID int, in *ChapterInput) (*ChapterDTO, error) {
	var chapter model.Chapter
	if err := s.db.WithContext(ctx).First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if in == nil {
		in = &ChapterInput{}
	}
	if in.Title != nil && *in.Title != "" {
		chapter.Title = *in.Title
	}
	if in.Content != nil {
		chapter.Content = *in.Content
	}
	if in.Duration != nil {
		chapter.Duration = *in.Duration
	}
	if in.OrderNum != nil {
		chapter.OrderNum = *in.OrderNum
	}
	// 内容形态要与本课类型同质（#2760 内容级互斥）：显式传了 content_type 就按传入值判，
	// 没传则本章形态不变、无需再判（它当初写入时已过同一道护栏）。
	if in.ContentType != nil {
		if err := requireChapterContentMatchesCourse(s.db, chapter.CourseID, *in.ContentType); err != nil {
			return nil, err
		}
		chapter.ContentType = *in.ContentType
	}
	// C7 管理员写面直接生效：编辑即发布——同步已发布槽（正文+文件挂接清单）、状态 published、
	// 清驳回理由、弃可能存在的在审快照（管理员写即终审，自审空转）。
	// #2537：发布与通知作者**同一事务**（ADR-0095 决策五强一致族）——通知写失败则发布回滚，
	// 不留「已生效但作者不知道」的静默洞。
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&chapter).Error; err != nil {
			return err
		}
		tutorID, err := publishWorkingCopyInTx(tx, &chapter)
		if err != nil {
			return err
		}
		return s.notifyChapterAdminEdit(tx, &chapter, tutorID)
	})
	if err != nil {
		return nil, err
	}
	chapter.ReviewStatus = model.ChapterReviewPublished
	chapter.RejectReason = ""
	d := ChapterToDTO(&chapter)
	return &d, nil
}

// notifyChapterAdminEdit C7 管理员直接生效的通知（#2537 / ADR-0095 决策七），在调用方事务内投递。
// 复用 chapter_review 类型，靠 AdminEdit 区分来源文案——两者都是「章节状态变了，
// 去编辑页看」，不需要第二种类型。收件人为空（存量章节作者未知）时跳过。
func (s *AdminService) notifyChapterAdminEdit(tx *gorm.DB, chapter *model.Chapter, tutorID int) error {
	if s.notifSvc == nil || tutorID <= 0 {
		return nil
	}
	ev := notification.NewChapterAdminEditEvent(tutorID, chapter.Title, chapter.ChapterID, chapter.CourseID)
	return s.notifSvc.CreateChapterReviewEvent(tx, ev, clock.Now())
}

// ApproveChapterReview 过审（#2498 C7/C8 仅管理员）：在审快照原子切换到已发布槽；
// #2499 C10：给提交人发审核结果站内信（与状态流转同事务）。
func (s *AdminService) ApproveChapterReview(ctx context.Context, chapterID int) (*ChapterDTO, error) {
	if err := s.approveOneWithNotice(ctx, chapterID); err != nil {
		return nil, err
	}

	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, err
	}
	d := ChapterToDTO(&chapter)
	return &d, nil
}

// RejectChapterReview 驳回（#2498 C7/C8 仅管理员）：弃在审快照，回草稿并记理由；
// #2499 C10：给提交人发驳回站内信（含理由，与状态流转同事务）。
func (s *AdminService) RejectChapterReview(ctx context.Context, chapterID int, reason string) (*ChapterDTO, error) {
	if err := s.rejectOneWithNotice(ctx, chapterID, reason); err != nil {
		return nil, err
	}
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, err
	}
	d := ChapterToDTO(&chapter)
	return &d, nil
}

// DeleteChapter 删除章节，并清理章节关联的存储文件：
// PPT 幻灯片（slides/<chapterID>/ 前缀）与图文 Markdown 图片（images/chapters/<chapterID>/ 前缀）。
func (s *AdminService) DeleteChapter(chapterID int) (*DeleteChapterResult, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0062 票6 / ADR-0064 决策 1）
	}
	if err := s.db.Delete(&chapter).Error; err != nil {
		return nil, err
	}
	if s.fileSvc != nil {
		s.fileSvc.DeleteFiles(s.fileSvc.List(fmt.Sprintf("%s/%d", slideRenderDir, chapterID)))
		s.fileSvc.DeleteFiles(s.fileSvc.List(filestore.ChapterImageDirPrefix + "/" + strconv.Itoa(chapterID)))
	}
	return &DeleteChapterResult{ChapterID: chapterID}, nil
}
