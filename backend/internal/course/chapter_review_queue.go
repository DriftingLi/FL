// 本文件：章节审核队列与批量操作（#2499，spec C9）——管理端待审队列、在审快照预览、
// 批量过审/批量驳回（统一理由、逐章落状态、失败明示），以及 C10 审核结果站内信。
//
// 队列口径：**有 pending 槽的章节**即待审（单 pending 由槽表复合主键保证，C8），
// 排序按快照冻结时刻（= 提交时刻，C9「按提交时间排序」），先进先审。
package course

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/timefmt"
	"forklift-training/pkg/paging"
)

// ChapterReviewQueueItemDTO 审核队列行（#2499 C9）：管理员不点进详情也能判断审什么。
type ChapterReviewQueueItemDTO struct {
	ChapterID    int    `json:"chapter_id"`
	ChapterTitle string `json:"chapter_title"`
	CourseID     int    `json:"course_id"`
	CourseName   string `json:"course_name"`
	ContentType  string `json:"content_type"`
	SubmittedBy  int    `json:"submitted_by"`
	FrozenAt     string `json:"frozen_at"`
	FileCount    int    `json:"file_count"`
}

// ChapterReviewQueueResult 审核队列分页结果。
type ChapterReviewQueueResult struct {
	Items    []ChapterReviewQueueItemDTO `json:"items" nullability:"nonnil"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
	Total    int64                       `json:"total"`
}

// ChapterReviewSnapshotDTO 在审快照详情（#2499 C9「详情页可预览在审快照完整内容」）。
// Content/ContentType/FileURL 取自 pending 槽（C4 冻结值，不是 working 草稿）；
// Files 是冻结的文件挂接清单（C3 整章一个审核单元）。
type ChapterReviewSnapshotDTO struct {
	ChapterID    int              `json:"chapter_id"`
	ChapterTitle string           `json:"chapter_title"`
	CourseID     int              `json:"course_id"`
	CourseName   string           `json:"course_name"`
	Content      string           `json:"content"`
	ContentType  string           `json:"content_type"`
	FileURL      string           `json:"file_url"`
	SubmittedBy  int              `json:"submitted_by"`
	FrozenAt     string           `json:"frozen_at"`
	Files        []ChapterFileDTO `json:"files" nullability:"nonnil"`
}

// BatchReviewFailureDTO 批量操作的逐章失败明细（#2499 C9「失败明示」）。
type BatchReviewFailureDTO struct {
	ChapterID int    `json:"chapter_id"`
	Reason    string `json:"reason"`
}

// BatchApproveResult 批量过审结果：成功数 + 逐章失败明细（失败不影响其余章节）。
type BatchApproveResult struct {
	ApprovedCount int                     `json:"approved_count"`
	Failures      []BatchReviewFailureDTO `json:"failures" nullability:"nonnil"`
}

// BatchRejectResult 批量驳回结果：成功数 + 逐章失败明细。
type BatchRejectResult struct {
	RejectedCount int                     `json:"rejected_count"`
	Failures      []BatchReviewFailureDTO `json:"failures" nullability:"nonnil"`
}

// ListPendingChapterReviews 待审队列（C9）：按提交时间升序分页，附带课程名与文件数。
func (s *AdminService) ListPendingChapterReviews(ctx context.Context, page, pageSize int) (*ChapterReviewQueueResult, error) {
	slots, total, page, pageSize, err := paging.QueryWithMax[model.ChapterContentSlot](s.db.WithContext(ctx), page, pageSize, 20, 50, "frozen_at ASC, chapter_id ASC",
		func(q *gorm.DB) *gorm.DB {
			return q.Where("slot = ?", model.ChapterReviewPending)
		},
	)
	if err != nil {
		return nil, err
	}
	items := make([]ChapterReviewQueueItemDTO, 0, len(slots))
	if len(slots) == 0 {
		return &ChapterReviewQueueResult{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
	}
	ids := make([]int, 0, len(slots))
	for i := range slots {
		ids = append(ids, slots[i].ChapterID)
	}
	// 章节与课程一次 IN 查齐（N+1 守卫：批量查询数不随队列长度增长）
	var chapters []model.Chapter
	if err := s.db.Where("chapter_id IN ?", ids).Find(&chapters).Error; err != nil {
		return nil, err
	}
	chByID := make(map[int]model.Chapter, len(chapters))
	courseIDs := make([]int, 0, len(chapters))
	for i := range chapters {
		chByID[chapters[i].ChapterID] = chapters[i]
		courseIDs = append(courseIDs, chapters[i].CourseID)
	}
	var courses []model.Course
	if err := s.db.Where("course_id IN ?", courseIDs).Find(&courses).Error; err != nil {
		return nil, err
	}
	courseName := make(map[int]string, len(courses))
	for i := range courses {
		courseName[courses[i].CourseID] = courses[i].Name
	}
	// 文件数：一次 GROUP BY（不是每行查一次）
	type fileCountRow struct {
		ChapterID int
		N         int
	}
	var counts []fileCountRow
	if err := s.db.Model(&model.ChapterFileSlot{}).
		Select("chapter_id, count(*) AS n").
		Where("slot = ? AND chapter_id IN ?", model.ChapterReviewPending, ids).
		Group("chapter_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	fileCount := make(map[int]int, len(counts))
	for _, c := range counts {
		fileCount[c.ChapterID] = c.N
	}

	for i := range slots {
		slot := slots[i]
		ch := chByID[slot.ChapterID]
		items = append(items, ChapterReviewQueueItemDTO{
			ChapterID:    slot.ChapterID,
			ChapterTitle: ch.Title,
			CourseID:     ch.CourseID,
			CourseName:   courseName[ch.CourseID],
			ContentType:  slot.ContentType,
			SubmittedBy:  slot.SubmittedBy,
			FrozenAt:     timefmt.FormatISO(slot.FrozenAt),
			FileCount:    fileCount[slot.ChapterID],
		})
	}
	return &ChapterReviewQueueResult{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

// GetChapterReviewSnapshot 在审快照详情（C9 预览）：读 pending 槽 + 冻结文件清单，
// **不读 working 草稿**——审中讲师改了草稿也不影响管理员看到的内容（C4 消除 TOCTOU）。
func (s *AdminService) GetChapterReviewSnapshot(ctx context.Context, chapterID int) (*ChapterReviewSnapshotDTO, error) {
	var slot model.ChapterContentSlot
	err := s.db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
		First(&slot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrChapterReviewNoPending
	}
	if err != nil {
		return nil, err
	}
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, err
	}
	var course model.Course
	if err := s.db.Select("course_id", "name").First(&course, chapter.CourseID).Error; err != nil {
		return nil, err
	}
	courseName := course.Name
	// 冻结清单：source/file_id 主键序，与提交时一致
	var fileSlots []model.ChapterFileSlot
	if err := s.db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
		Order("source, file_id").Find(&fileSlots).Error; err != nil {
		return nil, err
	}
	files := make([]ChapterFileDTO, 0, len(fileSlots))
	for i := range fileSlots {
		f := fileSlots[i]
		cid := f.ChapterID
		files = append(files, ChapterFileDTO{
			ChapterID:   &cid,
			ContentType: f.ContentType,
			CreatedAt:   timefmt.FormatISO(f.CreatedAt),
			FileID:      f.FileID,
			FileName:    f.FileName,
			FileSize:    f.FileSize,
			FileURL:     f.FileURL,
			Source:      f.Source,
		})
	}
	return &ChapterReviewSnapshotDTO{
		ChapterID:    slot.ChapterID,
		ChapterTitle: chapter.Title,
		CourseID:     chapter.CourseID,
		CourseName:   courseName,
		Content:      slot.Content,
		ContentType:  slot.ContentType,
		FileURL:      slot.FileURL,
		SubmittedBy:  slot.SubmittedBy,
		FrozenAt:     timefmt.FormatISO(slot.FrozenAt),
		Files:        files,
	}, nil
}

// BatchApproveChapterReviews 批量过审（C9）：**逐章独立事务**——一章失败不连坐其余，
// 失败明细逐条回给前端（不整体回滚、不静默跳过），文案走 reviewFailureReason 单点（不外发内部错误原文）。
func (s *AdminService) BatchApproveChapterReviews(ctx context.Context, chapterIDs []int) (*BatchApproveResult, error) {
	result := &BatchApproveResult{Failures: make([]BatchReviewFailureDTO, 0)}
	for _, id := range chapterIDs {
		if err := s.approveOneWithNotice(ctx, id); err != nil {
			result.Failures = append(result.Failures, BatchReviewFailureDTO{ChapterID: id, Reason: reviewFailureReason(err)})
			continue
		}
		result.ApprovedCount++
	}
	return result, nil
}

// BatchRejectChapterReviews 批量驳回（C9）：统一理由，逐章独立事务。
// 理由必填（沿单章驳回口径），空理由直接拒整批（不给「批量改成无理由驳回」的后门）。
func (s *AdminService) BatchRejectChapterReviews(ctx context.Context, chapterIDs []int, reason string) (*BatchRejectResult, error) {
	if len(strings.TrimSpace(reason)) == 0 {
		return nil, fmt.Errorf("%w: 驳回理由不能为空", ErrInvalidReviewInput)
	}
	result := &BatchRejectResult{Failures: make([]BatchReviewFailureDTO, 0)}
	for _, id := range chapterIDs {
		if err := s.rejectOneWithNotice(ctx, id, reason); err != nil {
			result.Failures = append(result.Failures, BatchReviewFailureDTO{ChapterID: id, Reason: reviewFailureReason(err)})
			continue
		}
		result.RejectedCount++
	}
	return result, nil
}

// approveOneWithNotice 单章过审（同一事务）。
//
// #2537 已接通 C10 站内信（ADR-0095）：收件人是**章节作者讲师**（chapter.tutor_id），
// 通知落 notifications.tutor_id——讲师在 hrwai_users 无行，落 user_id 会外键违约或投给
// 撞号的无关学员。强一致族（ADR-0027）：通知写失败整笔审核回滚，不给「已过审但讲师
// 永远不知道」的静默洞。收件人为空（存量章节作者未知）时跳过投递——那是如实的「不知道
// 发给谁」，不是投给错的人。
func (s *AdminService) approveOneWithNotice(ctx context.Context, chapterID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		chapter, err := loadChapterForReview(tx, chapterID)
		if err != nil {
			return err
		}
		// 收件人必须在状态流转**前**读出：过审会把 pending 槽改名为 published，
		// 之后 submitted_by 就查不到了。chapter.tutor_id 是稳定归属，优先用它；
		// 归属为空时回落到本次在审快照的提交人（同一讲师，且比归属更精确到本次）。
		tutorID, err := reviewRecipient(tx, chapter)
		if err != nil {
			return err
		}
		if err := approveChapterReviewInTx(tx, chapterID); err != nil {
			return err
		}
		// 结果状态是**流转后**的事实，不能读流转前快照的 chapter.ReviewStatus（那时还是 pending）。
		return s.notifyChapterReview(tx, chapter, tutorID, model.ChapterReviewPublished, "")
	})
}

// rejectOneWithNotice 单章驳回（同一事务）。C10 投递口径同 approveOneWithNotice。
func (s *AdminService) rejectOneWithNotice(ctx context.Context, chapterID int, reason string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		chapter, err := loadChapterForReview(tx, chapterID)
		if err != nil {
			return err
		}
		tutorID, err := reviewRecipient(tx, chapter)
		if err != nil {
			return err
		}
		if err := rejectChapterReviewInTx(tx, chapterID, reason); err != nil {
			return err
		}
		return s.notifyChapterReview(tx, chapter, tutorID, model.ChapterReviewDraft, reason)
	})
}

// notifyChapterReview 在事务内投递章节审核结果站内信（#2537 / ADR-0095）。
// reviewStatus / reason 是**本次流转的结果**，由调用方显式给出——不读 chapter 行，
// 因为那一行是流转前的快照（approve 前它的 ReviewStatus 还是 pending）。
// notifSvc 为 nil 或收件人为空（存量章节作者未知）时跳过——前者是测试装配，后者是
// 如实的「不知道发给谁」，两者都不该静默投给错的人。
func (s *AdminService) notifyChapterReview(
	tx *gorm.DB, chapter *model.Chapter, tutorID int, reviewStatus, reason string,
) error {
	if s.notifSvc == nil || tutorID <= 0 {
		return nil
	}
	ev := notification.NewChapterReviewEvent(tutorID, chapter.Title,
		chapter.ChapterID, chapter.CourseID, reviewStatus, reason)
	return s.notifSvc.CreateChapterReviewEvent(tx, ev, clock.Now())
}

// reviewRecipient 取本次审核结果的收件人讲师（0 = 不知道发给谁，跳过投递）。
//
// 优先 chapter.tutor_id（稳定归属，ADR-0095）；为空时回落到 pending 快照的
// submitted_by（本次提交人）。两者都为空说明既无归属也无在审快照——跳过。
// 必须在状态流转前调用：过审会把 pending 槽改名 published，之后 submitted_by 查不到。
//
// 「查不到」与「查不动」分开（ADR-0064 决策 1）：快照查询的 DB 故障上抛，不静默读成
// 「没人可发」——那会让投递在故障期悄悄失效，而审核看起来成功。
func reviewRecipient(tx *gorm.DB, chapter *model.Chapter) (int, error) {
	if chapter.TutorID != nil && *chapter.TutorID > 0 {
		return *chapter.TutorID, nil
	}
	var slot model.ChapterContentSlot
	err := tx.Select("submitted_by").
		Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapter.ChapterID).
		First(&slot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if slot.SubmittedBy <= 0 {
		return 0, nil
	}
	return slot.SubmittedBy, nil
}

// loadChapterForReview 取章节并把「不存在」映射成 ErrChapterNotFound 哨兵（handler 的 404 靠它）。
// 裸 tx.First 返回的是 gorm.ErrRecordNotFound，落到 handler 就成了 500（ADR-0064 决策 1 的另一面：
// 查不动与不存在要分开，存在性判断要给出业务哨兵）。
func loadChapterForReview(tx *gorm.DB, chapterID int) (*model.Chapter, error) {
	var chapter model.Chapter
	if err := fetchRow(tx.Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return nil, err
	}
	return &chapter, nil
}

// reviewFailureReason 批量操作逐章失败的对外文案：只回业务哨兵的中文语义，
// 不外发 err.Error()（那会把 gorm/驱动的英文内部错误印给管理员）。
func reviewFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrChapterNotFound):
		return "章节不存在"
	case errors.Is(err, ErrChapterReviewNoPending):
		return "章节没有在审版本"
	case errors.Is(err, ErrChapterReviewPending):
		return "章节已有在审版本"
	case errors.Is(err, ErrInvalidReviewInput):
		return "审核入参不合法"
	default:
		return "审核失败"
	}
}
