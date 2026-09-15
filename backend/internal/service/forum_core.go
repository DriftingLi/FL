// Package service 实现业务服务层。
// 本文件：论坛域共享内核（ADR-0050 决策 3）——ForumService（学员交互 + 个人集合）与
// ForumModerationService（论坛治理）共享的依赖与私有 helper。
//
// 同包分文件（被否备选：独立 package）：deleteReplyWithImages / fetchTopicDTO /
// enrichRewardIssued 这类 helper 保持包内可见；拆独立 package 会迫使它们提升导出，
// interface 反而变宽。两个 service 各持一个 forumCore 实例（实例分离、依赖共享）。
package service

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// forumCore 论坛域共享依赖与私有 helper（接收者沿用 s，与两个 service 同形）。
type forumCore struct {
	db              *gorm.DB
	fileSvc         *FileStore
	notificationSvc *NotificationService
	counters        ForumCounter // 计数列唯一写入口（spec #297）
	// rewards 奖励政策 module（ADR-0047 §3 / spec #927）：发放、回收与发放事实判定的
	// 唯一实现处——治理侧写（Award / Reclaim）、共享读（AcceptRewardIssued）都经它。
	rewards *forumRewardPolicy
	logger  *zap.Logger
}

// newForumCore 装配共享内核：两个 module 各调用一次（实例分离），依赖实例同源。
func newForumCore(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, counters ForumCounter, points *PointsService, logger *zap.Logger) forumCore {
	return forumCore{db: db, fileSvc: fileSvc, notificationSvc: notificationSvc, counters: counters,
		rewards: newForumRewardPolicy(points, notificationSvc), logger: logger}
}

// collectTopicImages 收集主题 + 全部回复（含子回复）的图片 URL（须在删除前调用）。
// 作者自删（deleteTopicWithImages）与管理端强删（AdminDeleteTopic）共用同一收集实现。
func (s *forumCore) collectTopicImages(topic *model.ForumTopic) ([]string, error) {
	urls := parseImageURLs(string(topic.Images))
	var replyImages []string
	if err := s.db.Model(&model.ForumReply{}).
		Where("topic_id = ?", topic.ID).
		Pluck("images", &replyImages).Error; err != nil {
		return nil, err
	}
	for _, raw := range replyImages {
		urls = append(urls, parseImageURLs(raw)...)
	}
	return urls, nil
}

// deleteTopicWithImages 删除主题前收集主题 + 全部回复（含子回复）的图片并清理存储。
func (s *forumCore) deleteTopicWithImages(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		return err
	}
	urls, err := s.collectTopicImages(&topic)
	if err != nil {
		return err
	}
	if err := s.db.Delete(&model.ForumTopic{}, topicID).Error; err != nil {
		return err
	}
	s.deleteImages(urls)
	return nil
}

// deleteReplyWithImages 删除回复前收集本回复 + 全部下级回复的图片并清理存储。
// 下级回复通过 parent_id 递归收集（单表递归 CTE 或逐层查询）。
func (s *forumCore) deleteReplyWithImages(replyID, topicID int64) error {
	urls, err := s.collectReplyImages(replyID)
	if err != nil {
		return err
	}
	if err := s.deleteReplyByID(replyID, topicID); err != nil {
		return err
	}
	s.deleteImages(urls)
	return nil
}

// collectReplyImages 收集回复及其全部下级回复（parent_id 链条）的图片 URL。
func (s *forumCore) collectReplyImages(replyID int64) ([]string, error) {
	var urls []string

	var self model.ForumReply
	if err := s.db.First(&self, replyID).Error; err != nil {
		return nil, err
	}
	urls = append(urls, parseImageURLs(string(self.Images))...)

	// BFS 收集下级回复
	level := []int64{replyID}
	for len(level) > 0 {
		var children []model.ForumReply
		if err := s.db.Where("parent_id IN ?", level).Find(&children).Error; err != nil {
			return nil, err
		}
		if len(children) == 0 {
			break
		}
		level = level[:0]
		for _, ch := range children {
			urls = append(urls, parseImageURLs(string(ch.Images))...)
			level = append(level, ch.ID)
		}
	}
	return urls, nil
}

// deleteImages 清理图片存储文件（fileSvc 为 nil 时跳过，尽力而为）。
func (s *forumCore) deleteImages(urls []string) {
	if s.fileSvc == nil || len(urls) == 0 {
		return
	}
	s.fileSvc.DeleteFiles(urls)
}

// deleteReplyByID 删除回复并回扣主题回复数、刷新最后回复时间。
// 回扣量取回复子树大小 N（parent_id 链，含自身）：外键 ON DELETE CASCADE 会连带删除全部下级回复，
// 固定 -1 会让楼中楼场景计数虚高（spec #297 级联少减修复）。
func (s *forumCore) deleteReplyByID(replyID, topicID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		n, err := countReplySubtree(tx, replyID)
		if err != nil {
			return err
		}
		if err := tx.Delete(&model.ForumReply{}, replyID).Error; err != nil {
			return err
		}
		if err := s.counters.AdjustReplyCounts(tx, topicID, -int(n)); err != nil {
			return err
		}
		var last model.ForumReply
		if err := tx.Where("topic_id = ?", topicID).Order("created_at DESC, id DESC").
			Limit(1).Find(&last).Error; err != nil {
			return err
		}
		var lastAt *time.Time
		if last.ID > 0 {
			lastAt = &last.CreatedAt
		}
		return tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			Update("last_reply_at", lastAt).Error
	})
}

// enrichTopicLikedByMe 批量回填主题是否已赞（计数已由 likes_count 列提供，LikedByMe 单一 helper 收敛）。
func (s *forumCore) enrichTopicLikedByMe(topics []*ForumTopicDTO, viewerID int) {
	if len(topics) == 0 || viewerID <= 0 {
		return
	}
	ids := make([]int64, 0, len(topics))
	for _, t := range topics {
		if t != nil {
			ids = append(ids, t.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	var liked []int64
	if err := s.db.Model(&model.ForumTopicLike{}).Where("user_id = ? AND topic_id IN ?", viewerID, ids).Pluck("topic_id", &liked).Error; err != nil {
		return
	}
	lm := make(map[int64]bool, len(liked))
	for _, id := range liked {
		lm[id] = true
	}
	for _, t := range topics {
		if t != nil {
			t.LikedByMe = lm[t.ID]
		}
	}
}

// fetchTopicDTO 查询主题 DTO（用于采纳后回显，复用 topicRow 装配，不累浏览量）。
func (s *forumCore) fetchTopicDTO(topicID int64, viewerID int) (*ForumTopicDTO, error) {
	var row topicRow
	err := s.db.Table("forum_topics AS t").
		Select(topicRowSelect+
			"u.id AS user_id, u.username, u.avatar_url, "+
			"COALESCE(ch.title, '') AS chapter_title").
		Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
		Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
		Where("t.id = ?", topicID).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	dto := row.toDTO(viewerID)
	// 点赞回填保持与详情一致（尽力而为）
	s.enrichTopicLikedByMe([]*ForumTopicDTO{&dto}, viewerID)
	if s.hasRewardIssued(topicID) {
		dto.RewardIssued = true
	}
	return &dto, nil
}

// enrichRewardIssued 批量回填 reward_issued（#367）。
//
// 判据（该帖的**采纳奖励**是否已发放）与查询实现都在奖励政策 module 里，与写入侧
// 共用同一份 reason 集合——两处实现漂移过一次就是 bug，故收成单点。
func (s *forumCore) enrichRewardIssued(items []ForumTopicDTO) {
	if len(items) == 0 {
		return
	}
	ids := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, t := range items {
		if _, ok := seen[t.ID]; !ok {
			seen[t.ID] = struct{}{}
			ids = append(ids, t.ID)
		}
	}
	issued := s.rewards.AcceptRewardIssued(s.db, ids)
	for i := range items {
		if issued[items[i].ID] {
			items[i].RewardIssued = true
		}
	}
}

// hasRewardIssued 单条查询：该帖的采纳奖励是否已发放（与 enrichRewardIssued 同口径）。
func (s *forumCore) hasRewardIssued(topicID int64) bool {
	return s.rewards.AcceptRewardIssued(s.db, []int64{topicID})[topicID]
}
