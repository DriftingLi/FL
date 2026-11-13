// 本文件：章节先审后发状态机（#2498，spec C1–C8）——**独立命名**，不与题库/投稿/资料混称。
// 三态 draft/pending/published + reject_reason（驳回回 draft 记理由，无独立 rejected 态）；
// working 草稿在 chapter 行，在审快照（slot=pending）与已发布槽（slot=published）在
// chapter_content_slot / chapter_file_slot（C4）：提交即冻结、过审原子改槽（C5 旧版切换前保留）、
// 驳回弃快照；单 pending 由槽表复合主键天然保证（C8）。
package course

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
)

var (
	// ErrChapterReviewPending 该章节已有在审版本（C8 单 pending 约束：重复提交拒绝）。
	ErrChapterReviewPending = errors.New("该章节已有在审版本")
	// ErrChapterReviewNoPending 没有在审版本可审（过审/驳回的状态前置哨兵）。
	ErrChapterReviewNoPending = errors.New("该章节没有在审版本")
	// ErrInvalidReviewInput 审核动作入参不合法（驳回理由为空等）。
	ErrInvalidReviewInput = errors.New("审核入参不合法")
)

// SubmitChapterForReview 提交审核（C2 显式动作，讲师面）：冻结整章快照（正文 + 文件挂接
// 清单，C3/C4）进 pending 槽；审中讲师改的是 working 草稿、不影响在审内容。
// 状态转移：→ pending；清 reject_reason（题库 SubmitQuestion 先例）。
func SubmitChapterForReview(ctx context.Context, db *gorm.DB, chapterID, actorID int) error {
	var chapter model.Chapter
	if err := fetchRow(db.WithContext(ctx).Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// C8：单 pending——已有在审快照即拒（复合主键兜底，这里给业务哨兵与 400 映射）
		var n int64
		if err := tx.Model(&model.ChapterContentSlot{}).
			Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return ErrChapterReviewPending
		}
		// 冻结内容快照（working 行当前值）
		snapshot := model.ChapterContentSlot{
			Slot:        model.ChapterReviewPending,
			ChapterID:   chapterID,
			Content:     chapter.Content,
			ContentType: chapter.ContentType,
			FileURL:     chapter.FileURL,
			SubmittedBy: actorID,
		}
		if err := tx.Create(&snapshot).Error; err != nil {
			return err
		}
		// 冻结文件挂接清单（live 读面三源，C3 整章一个审核单元）
		if err := freezeFileManifest(tx, &chapter); err != nil {
			return err
		}
		return tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
			Updates(map[string]any{
				"review_status": model.ChapterReviewPending,
				"reject_reason": "",
			}).Error
	})
}

// ApproveChapterReview 过审（C7/C8 仅管理员）：pending 快照**原子切换**到已发布槽
// （C4：改槽前旧版照常可见，C5），工作副本与已发布一致 ⇒ 状态 published、清驳回理由。
func ApproveChapterReview(ctx context.Context, db *gorm.DB, chapterID int) error {
	var chapter model.Chapter
	if err := fetchRow(db.WithContext(ctx).Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pending model.ChapterContentSlot
		err := tx.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			First(&pending).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrChapterReviewNoPending
		}
		if err != nil {
			return err
		}
		// 原子切换：删旧 published 槽（内容+清单）→ pending 改槽 published
		if err := tx.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPublished, chapterID).
			Delete(&model.ChapterContentSlot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPublished, chapterID).
			Delete(&model.ChapterFileSlot{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ChapterContentSlot{}).
			Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			Update("slot", model.ChapterReviewPublished).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ChapterFileSlot{}).
			Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			Update("slot", model.ChapterReviewPublished).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
			Updates(map[string]any{
				"review_status": model.ChapterReviewPublished,
				"reject_reason": "",
			}).Error
	})
}

// RejectChapterReview 驳回（C7/C8 仅管理员）：弃在审快照（C4），状态回 draft 并记理由（C1）。
func RejectChapterReview(ctx context.Context, db *gorm.DB, chapterID int, reason string) error {
	var chapter model.Chapter
	if err := fetchRow(db.WithContext(ctx).Where("chapter_id = ?", chapterID), &chapter, ErrChapterNotFound); err != nil {
		return err
	}
	if reason == "" {
		return fmt.Errorf("%w: 驳回理由不能为空", ErrInvalidReviewInput)
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			Delete(&model.ChapterContentSlot{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrChapterReviewNoPending
		}
		if err := tx.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
			Delete(&model.ChapterFileSlot{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
			Updates(map[string]any{
				"review_status": model.ChapterReviewDraft,
				"reject_reason": reason,
			}).Error
	})
}

// publishWorkingCopy 管理员写面直接生效（C7）：把 working 草稿同步进已发布槽
// （正文+文件挂接清单）、状态 published、清理由、弃在审快照。事务内调用或自带事务。
func publishWorkingCopy(db *gorm.DB, chapter *model.Chapter) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("chapter_id = ?", chapter.ChapterID).
			Delete(&model.ChapterContentSlot{}).Error; err != nil {
			return err
		}
		if err := tx.Where("chapter_id = ?", chapter.ChapterID).
			Delete(&model.ChapterFileSlot{}).Error; err != nil {
			return err
		}
		slot := model.ChapterContentSlot{
			Slot:        model.ChapterReviewPublished,
			ChapterID:   chapter.ChapterID,
			Content:     chapter.Content,
			ContentType: chapter.ContentType,
			FileURL:     chapter.FileURL,
		}
		if err := tx.Create(&slot).Error; err != nil {
			return err
		}
		if err := freezeFileManifestInto(tx, chapter, model.ChapterReviewPublished); err != nil {
			return err
		}
		return tx.Model(&model.Chapter{}).Where("chapter_id = ?", chapter.ChapterID).
			Updates(map[string]any{
				"review_status": model.ChapterReviewPublished,
				"reject_reason": "",
			}).Error
	})
}

// MarkChapterDraft 讲师改动审核单元（正文/文件挂接清单）后把工作副本标回草稿（C2）：
// 学生仍见已发布旧版（C5），过审/管理员写面才切换。
//
// 有在审版本时**不改状态**：pending 快照仍在队列里，状态若被打成 draft，讲师页会显示
// 「草稿」却又被 C8 挡住再次提交（自相矛盾）。此时草稿改动只等本次审核结果——
// 过审后 working 即「下一版草稿」，驳回则随驳回回到 draft。
func MarkChapterDraft(db *gorm.DB, chapterID int) error {
	var pending int64
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).
		Count(&pending).Error; err != nil {
		return err
	}
	if pending > 0 {
		return nil
	}
	return db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
		Updates(map[string]any{"review_status": model.ChapterReviewDraft}).Error
}

// freezeFileManifest 从 live 文件读面冻结挂接清单进 pending 槽（source 判别沿 #2497；
// legacy 折叠语义靠真实 chapter 行，勿传零值行）。
func freezeFileManifest(tx *gorm.DB, chapter *model.Chapter) error {
	return freezeFileManifestInto(tx, chapter, model.ChapterReviewPending)
}

func freezeFileManifestInto(tx *gorm.DB, chapter *model.Chapter, slot string) error {
	for _, f := range loadChapterFiles(tx, chapter) {
		src := f.Source
		if src == "" {
			src = FileSourceChapter
		}
		// created_at 保留来源条目的原时刻（DTO 是 ISO 字符串，解析回原瞬间）——
		// 学员读面与 live 读面必须逐字一致，拿冻结时刻会让两端 shape 对比漂移。
		createdAt := clock.Now()
		if ts, err := time.Parse(time.RFC3339Nano, f.CreatedAt); err == nil {
			createdAt = ts
		}
		row := model.ChapterFileSlot{
			Slot:        slot,
			ChapterID:   chapter.ChapterID,
			Source:      src,
			FileID:      f.FileID,
			FileURL:     f.FileURL,
			FileName:    f.FileName,
			ContentType: f.ContentType,
			FileSize:    f.FileSize,
			CreatedAt:   createdAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// applyPublishedOverlay 学员章节列表的已发布覆盖（C6/C11）：只保留有已发布槽的章节，
// 正文/格式/legacy 文件位换成槽值（C5 重审期间学员见旧版，working 再改不影响）。
//
// 查询故障**上抛**而不是收成空列表：把「读不动槽表」演成「整课没有章节」，会让 DB 抖动
// 被学员读成内容消失（ADR-0064 决策 1：查不动不得被读成不存在）。
func applyPublishedOverlay(db *gorm.DB, chapters []ChapterDTO) ([]ChapterDTO, error) {
	if len(chapters) == 0 {
		return chapters, nil
	}
	ids := make([]int, 0, len(chapters))
	for i := range chapters {
		ids = append(ids, chapters[i].ChapterID)
	}
	var slots []model.ChapterContentSlot
	if err := db.Where("slot = ? AND chapter_id IN ?", model.ChapterReviewPublished, ids).
		Find(&slots).Error; err != nil {
		return nil, err
	}
	byID := make(map[int]model.ChapterContentSlot, len(slots))
	for _, s := range slots {
		byID[s.ChapterID] = s
	}
	result := make([]ChapterDTO, 0, len(chapters))
	for i := range chapters {
		s, ok := byID[chapters[i].ChapterID]
		if !ok {
			continue // 无已发布槽 = 学员侧完全不出现（C6）
		}
		c := chapters[i]
		c.Content = s.Content
		c.ContentType = s.ContentType
		c.FileURL = s.FileURL
		c.ReviewStatus = "" // 审核态是管理面/导师面信息，不进学员读面
		c.RejectReason = ""
		result = append(result, c)
	}
	return result, nil
}

// publishedChapterIDs 课程内有已发布槽的章节 id 集（学员 prev/next 跳转限已发布，
// 否则上下章指针会把未过审章节当入口泄漏出去）。
func publishedChapterIDs(db *gorm.DB, courseID int) (map[int]bool, error) {
	var ids []int
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("slot = ? AND chapter_id IN (?)", model.ChapterReviewPublished, publishedChapterSubquery(db, courseID)).
		Pluck("chapter_id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[int]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

// chapterHasPublished 该章节是否有已发布槽（C11 学员可见谓词半边：课程可见 ∧ 本谓词）。
func chapterHasPublished(db *gorm.DB, chapterID int) (bool, error) {
	var n int64
	if err := db.Model(&model.ChapterContentSlot{}).
		Where("slot = ? AND chapter_id = ?", model.ChapterReviewPublished, chapterID).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// loadPublishedContent 读已发布槽正文（学员读面单点）。无槽返回 nil。
func loadPublishedContent(db *gorm.DB, chapterID int) (*model.ChapterContentSlot, error) {
	var slot model.ChapterContentSlot
	err := db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPublished, chapterID).
		First(&slot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &slot, nil
}

// loadPublishedFiles 读已发布槽文件挂接清单（学员附件投递读面单点）。
func loadPublishedFiles(db *gorm.DB, chapterID int) ([]ChapterFileDTO, error) {
	var rows []model.ChapterFileSlot
	if err := db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPublished, chapterID).
		Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	files := make([]ChapterFileDTO, 0, len(rows))
	for i := range rows {
		r := rows[i]
		cid := r.ChapterID
		files = append(files, ChapterFileDTO{
			ChapterID:   &cid,
			ContentType: r.ContentType,
			CreatedAt:   timefmt.FormatISO(r.CreatedAt),
			FileID:      r.FileID,
			FileName:    r.FileName,
			FileSize:    r.FileSize,
			FileURL:     r.FileURL,
			Source:      r.Source,
		})
	}
	return files, nil
}
