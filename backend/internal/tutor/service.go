// Package tutor 讲师域：HTTP 出口（handler.go）与导师端课程 / 章节 / 附件实现（service.go）。
package tutor

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/course"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/internal/tutorcap"
)

// Service 导师服务。
type Service struct {
	db            *gorm.DB
	uploadFolder  string
	fileStore     *filestore.FileStore
	slideRenderer *course.SlideRenderer
	chapterWrite  *course.ChapterWriter

	logger *zap.Logger

	// caps 讲师有效能力集解析器（#2599 / 规格 E3）：与 admincap 并列的独立解析器，
	// 实现落在 internal/tutorcap（域包测试要复用它，放在这里会成环）。
	caps *tutorcap.Resolver
}

// NewService 创建导师服务实例。
func NewService(db *gorm.DB, uploadFolder string, fileStore *filestore.FileStore, slideRenderer *course.SlideRenderer, logger *zap.Logger) *Service {
	return &Service{
		db: db, uploadFolder: uploadFolder, fileStore: fileStore, slideRenderer: slideRenderer,
		chapterWrite: course.NewChapterWriter(db, fileStore, slideRenderer, logger),
		logger:       logger, caps: tutorcap.New(db, logger),
	}
}

// ErrChapterFileNotFound 章节文件行不存在（课程章节的附件，与「章节不存在」是两件事）。
// 本文件的「课程/章节不存在」直接用课程域的唯一载体 model.ErrCourseNotFound / course.ErrChapterNotFound。
// （声明必须留在函数文档块之外：它一度夹在下面那条注释与 func 之间，把 godoc 抢走了 ——
// 同形缺陷在 api 层会让整条 swagger 路由消失，见 4c488c3c。）
var ErrChapterFileNotFound = course.ErrChapterFileNotFound

// ClaimCourse 讲师自助认领课程（#2596 B3/B5）：委托课程域，本层不加逻辑——认领是课程域的事实。
func (s *Service) ClaimCourse(tutorID, courseID int) error {
	return course.ClaimCourse(s.db, tutorID, courseID, model.ClaimSourceSelf)
}

// ReleaseCourseClaim 讲师退出认领（#2596 B3/B6）。
func (s *Service) ReleaseCourseClaim(tutorID, courseID int) error {
	return course.ReleaseCourseClaim(s.db, tutorID, courseID)
}

// ListCourseClaimants 课程共认讲师名单（#2596 D1）。课程不存在时返回 model.ErrCourseNotFound
// ——「没有认领人」与「没有这门课」是两件事，前者是空名单 200。
//
// ClaimedByMe 由本层算（tutorID 就是当前讲师）：前端不必自己比对 id，
// 「我在不在名单里」在本层一次查询里就能答完。
func (s *Service) ListCourseClaimants(tutorID, courseID int) (*course.CourseClaimantsResult, error) {
	var c model.Course
	if err := s.db.Select("course_id").First(&c, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err
	}
	claimants, err := course.ListCourseClaimants(s.db, courseID)
	if err != nil {
		return nil, err
	}
	claimedByMe := false
	for i := range claimants {
		if claimants[i].TutorID == tutorID {
			claimants[i].IsMe = true
			claimedByMe = true
		}
	}
	return &course.CourseClaimantsResult{Claimants: claimants, ClaimedByMe: claimedByMe}, nil
}

// GetCourses 导师课程列表（与学员端同口径：已上架 + 已挂载方向/等级/证件，ADR-0012 §2），
// 附学习学员数；实现收敛到课程列表 module（ListCourses）。
//
// 认领两列（#2596 D1）：claimed_by_me 与 claim_count 由一次联表聚合算出（不逐课程 N+1），
// 且**只对讲师面列表填**——学员端/管理端列表不长这两列（CourseDTO 上可空省略）。
func (s *Service) GetCourses(page, pageSize int, credentialID, specialtyID, levelID *int, category scope.CategoryFilter, tutorID int) (course.CoursePageResult, error) {
	result, err := course.ListCourses(s.db, page, pageSize, course.CourseListOptions{
		OnlyMounted: true, CredentialID: credentialID, SpecialtyID: specialtyID, LevelID: levelID,
		CategoryFilter: category, WithPublicState: true, WithStudentCount: true, LegacyIncludesUncategorized: true, DefaultPageSize: 10,
	})
	if err != nil {
		return result, err
	}
	ids := make([]int, 0, len(result.Courses))
	for i := range result.Courses {
		ids = append(ids, result.Courses[i].CourseID)
	}
	counts, mine, err := course.BatchCourseClaimStats(s.db, ids, tutorID)
	if err != nil {
		return result, err
	}
	for i := range result.Courses {
		n := counts[result.Courses[i].CourseID]
		claimed := mine[result.Courses[i].CourseID]
		result.Courses[i].ClaimCount = &n
		result.Courses[i].ClaimedByMe = &claimed
	}
	return result, nil
}

// GetCourseChapters 导师章节列表（含文件）。
// 文件列表批量装载（一次 IN 查询）消除逐章节 N+1。
func (s *Service) GetCourseChapters(courseID int) (*course.TutorCourseChaptersDTO, error) {
	// 局部变量不叫 course：包名 course 已被课程域包占用（P2 波 3b-1）。
	var c model.Course
	if err := s.db.First(&c, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrCourseNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	var chapters []model.Chapter
	// 「查不动」如实上抛（ADR-0062 票6）：原先不查 error ⇒ chapter 表读不动时讲师看到的是一个
	// 空章节列表（200 假绿），而不是「这次没读到」。与课程域 loadCourseWithChapters 同一判据。
	if err := s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters).Error; err != nil {
		return nil, err
	}

	filesByChapter := course.LoadChapterFilesBulk(s.db, chapters)
	resultChapters := make([]course.ChapterDTO, 0, len(chapters))
	for i := range chapters {
		ch := &chapters[i]
		fileList := filesByChapter[ch.ChapterID]
		// legacy 兼容：无 chapter_file 条目且 chapter.file_url 非空时折叠 legacy 条目
		if len(fileList) == 0 && ch.FileURL != "" {
			legacy := course.LegacyFileEntry(ch)
			fileList = []course.ChapterFileDTO{legacy}
		}
		if fileList == nil {
			fileList = []course.ChapterFileDTO{}
		}
		chDTO := course.ChapterToDTO(ch)
		chDTO.Files = &fileList
		resultChapters = append(resultChapters, chDTO)
	}
	cd := course.CourseToDTO(&c)
	if err := course.FillCoursePublicState(s.db, &cd); err != nil {
		return nil, err
	}
	return &course.TutorCourseChaptersDTO{
		Course:   cd,
		Chapters: resultChapters,
	}, nil
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表，供导师端编辑页使用）。
// 实现收敛到共享章节详情 module；导师端不回填 study_status（fillStudyStatus=false）。
func (s *Service) GetChapterDetail(chapterID int) (*course.TutorChapterDetailDTO, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, course.ErrChapterNotFound
		}
		return nil, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	detail, err := course.ChapterDetailShared(s.db, &chapter, false, 0, false)
	if err != nil {
		return nil, err
	}
	result := &course.TutorChapterDetailDTO{ChapterDetailDTO: *detail}
	if chapter.ReviewStatus == model.ChapterReviewPending {
		result.ReviewSnapshot, err = course.GetChapterReviewSnapshot(s.db, chapterID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// RequireChapterWrite 在解析上传文件前校验章节写入权。
func (s *Service) RequireChapterWrite(chapterID, tutorID int) error {
	return s.chapterWrite.RequireWritable(chapterID, tutorID)
}

// UploadChapterImage 上传章节内嵌图片，章节写入不变式由 course.ChapterWriter 承载。
func (s *Service) UploadChapterImage(ctx context.Context, chapterID, tutorID int, content []byte, filename string) (string, error) {
	return s.chapterWrite.UploadImage(ctx, chapterID, tutorID, content, filename)
}

// UploadChapterFile 上传章节文件。
//
// 认领谓词前置（#2596 / 规格 B2）：能进讲师面 ≠ 能改这门课的章节。tutorID 是当前登录
// 讲师，未认领该课程即拒（course.ErrCourseClaimRequired → 403）。
func (s *Service) UploadChapterFile(chapterID, tutorID int, filename string, fileContent []byte) (*course.ChapterFileDTO, error) {
	return s.chapterWrite.UploadFile(context.Background(), chapterID, tutorID, filename, fileContent)
}

// SubmitChapterReview 提交审核（#2498 C2 显式动作）：冻结整章快照（正文+文件挂接清单）
// 进在审槽；同章单 pending（C8）。审核期间讲师改的是草稿，不影响在审内容（C4）。
//
// 认领谓词前置（#2596 / 规格 B2）：提交审核是「把这章推进发布流程」，与写正文同属编辑权，
// 未认领即拒。
func (s *Service) SubmitChapterReview(ctx context.Context, chapterID, tutorID int) (*course.ChapterDTO, error) {
	return s.chapterWrite.SubmitForReview(ctx, chapterID, tutorID)
}

// UpdateChapterInfo 更新章节信息（讲师面）。tutorID 是当前登录讲师：章节归属为空时
// 顺手落定（ADR-0095 决策二），非空不动——归属是「作者」而非「最后编辑者」。
//
// 认领谓词前置（#2596 / 规格 B2）：未认领该课程的讲师不能改它的章节。
func (s *Service) UpdateChapterInfo(chapterID, tutorID int, in *course.ChapterInput) (*course.ChapterDTO, error) {
	return s.chapterWrite.Update(context.Background(), chapterID, tutorID, in)
}

// DeleteChapterFileByID 删除章节文件。
//
// 认领谓词前置（#2596 / 规格 B2）：删附件同样是改章节。文件未挂章节（ChapterID 为空）时
// 无处可认领——这类孤儿行只可能是历史遗留，允许删（否则永远清不掉）；挂着的必须有认领。
func (s *Service) DeleteChapterFileByID(fileID, tutorID int) (*DeleteFileResult, error) {
	if err := s.chapterWrite.DeleteFile(context.Background(), fileID, tutorID); err != nil {
		return nil, err
	}
	return &DeleteFileResult{FileID: fileID, Deleted: true}, nil
}

// BatchDeleteChapterFiles 批量删除文件。
//
// 认领谓词前置（#2596 / 规格 B2）：删附件同样是改章节。未认领的文件**记失败**而不是整批拒——
// 批量面的既有语义就是「不能删的进 failed_ids、能删的照删」（见 BatchDeleteFilesResult），
// 这里再立一套「有未授权就一行不动」等于给同一件事两种语义：调用方拿到 403 时既不知道
// 是哪几个文件、也丢掉了已经算好的失败清单。整批原子性只属于单条写入，不属于批量删除。
func (s *Service) BatchDeleteChapterFiles(fileIDs []int, tutorID int) (*BatchDeleteFilesResult, error) {
	successCount, failedIDs, err := s.chapterWrite.BatchDeleteFiles(context.Background(), fileIDs, tutorID)
	if err != nil {
		return nil, err
	}
	return &BatchDeleteFilesResult{
		SuccessCount: successCount,
		FailedCount:  len(failedIDs),
		FailedIDs:    failedIDs,
	}, nil
}
