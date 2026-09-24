// Package service 实现业务服务层。
// 本文件：论坛治理（ADR-0050 决策 3；词表见 CONTEXT.md「论坛治理」）——管理端对论坛内容的
// 处置动作族：举报处置、意图认定（帖子精选 / 备考经验）、管理端强制删除与违规回收。
//
// seam 判据是 **caller**（入口路由），不是概念：CreateReport（学员发起）留在学员交互侧
// （forum_service.go），HandleReport / ListReports（处置与队列）归本 module。
// 命名 Moderation 对齐能力表既有 forum.moderate（ADR-0047 §3），词汇零新增。
//
// 同包分文件：与 ForumService 共享 forumCore（依赖 + 私有 helper），实例分离；
// api 层管理端路由依赖本 module，学员端 handler 零改动。
package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// ForumModerationService 论坛治理服务。
type ForumModerationService struct {
	forumCore
}

// NewForumModerationService 构造论坛治理服务（依赖与 ForumService 同源，实例分离）。
func NewForumModerationService(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, counters ForumCounter, points *PointsService, logger *zap.Logger) *ForumModerationService {
	return &ForumModerationService{forumCore: newForumCore(db, fileSvc, notificationSvc, counters, points, logger)}
}

// ForumReportDTO 管理端举报条目。
type ForumReportDTO struct {
	ID         int64  `json:"id"`
	ReporterID int    `json:"reporter_id"`
	Reporter   string `json:"reporter"`
	TopicID    *int64 `json:"topic_id,omitempty" extensions:"x-optional"`
	TopicTitle string `json:"topic_title"`
	ReplyID    *int64 `json:"reply_id,omitempty" extensions:"x-optional"`
	Reason     string `json:"reason"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}

// ForumReportPageResult 举报分页结果。
type ForumReportPageResult struct {
	Page    int              `json:"page"`
	Pages   int              `json:"pages"`
	Total   int64            `json:"total"`
	Reports []ForumReportDTO `json:"reports" nullability:"nullable"`
}

// AdminDeleteTopic 管理员删除任意主题（不校验作者）。图片一并清理；站内信通知作者。
// 若该帖产生过任一直记奖励（被采纳 / 采纳动作 / 认定），则按 rollback 原因写对冲流水并扣减余额
// （封底 0，幂等，按 user_id 分组各自追回）。
func (s *ForumModerationService) AdminDeleteTopic(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTopicNotFound
		}
		return err
	}
	// 先收集图片（需在删除前读取，事务外清理）：与作者自删同源的收集实现。
	urls, err := s.collectTopicImages(&topic)
	if err != nil {
		return err
	}
	// 事务内：删帖 + 违规回收（复用封底 0 语义）
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.ForumTopic{}, topicID).Error; err != nil {
			return err
		}
		// 违规回收（ADR-0041）：触发条件是「该帖存在任一正向直记奖励」，而不是「曾被采纳」——
		// 否则「加精但未采纳」的帖子（正是备考经验帖的形状）会被整片漏掉。
		// 范围含答主/楼主/帖主三方，RollbackByRef 内部按 user_id 分组各自追回、封底 0。
		// 违规回收交给奖励政策 module：触发条件（该帖存在任一正向直记奖励）与回收范围
		// （全部直记奖励）都在它的 implementation 里判定，无奖励可回收时 no-op。
		if _, err := s.rewards.Reclaim(tx, topicID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 清理文件（事务外，尽力而为）
	s.deleteImages(urls)
	// 通知作者（尽力而为：内容已删，通知失败不回滚，仅记日志；ADR-0027 C1 收编）
	s.notificationSvc.TryCreateForumTopicDeletedEvent(NewForumTopicDeletedEvent(topic.UserID, topic.Title))
	return nil
}

// AdminDeleteReply 管理员删除任意回复（不校验作者；其下级回复随外键级联删除）。图片一并清理；站内信通知回复作者。
// 若删的是被采纳的回答，只把主题打回未解决（清 accepted_reply_id/solved_at），**不回收积分**——
// 奖励处置的唯一出口是 AdminDeleteTopic（见奖励政策 module Reclaim 的 ref 级一次性说明）。
func (s *ForumModerationService) AdminDeleteReply(replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReplyNotFound
		}
		return err
	}
	var topic model.ForumTopic
	if err := s.db.First(&topic, reply.TopicID).Error; err != nil {
		topic.Title = ""
	}
	topicTitle := topic.Title
	// 若该回复是被采纳的回答，只把主题打回未解决——**不回收奖励**（ADR-0041）。
	// 理由：RollbackByRef 是 ref 级一次性护栏，这里回收会永久占掉该帖的回收机会，
	// 之后管理员删整帖时帖主的 featured_bonus 再也追不回。**删帖才是奖励处置的唯一出口**，
	// 届时答主/楼主/帖主三笔一次全部追回（回收能力最大化）。
	// solved_at 必须显式清：accepted_reply_id 有 ON DELETE SET NULL 外键兜底，solved_at 没有。
	if topic.AcceptedReplyID != nil && *topic.AcceptedReplyID == replyID {
		if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", topic.ID).Updates(map[string]any{
			"accepted_reply_id": nil,
			"solved_at":         nil,
			"updated_at":        beijingNow(),
		}).Error; err != nil {
			return err
		}
	}
	if err := s.deleteReplyWithImages(replyID, reply.TopicID); err != nil {
		return err
	}
	// 通知回复作者（尽力而为：内容已删，通知失败不回滚，仅记日志；ADR-0027 C1 收编）
	s.notificationSvc.TryCreateForumReplyDeletedEvent(NewForumReplyDeletedEvent(reply.UserID, topicTitle, reply.TopicID))
	return nil
}

// ListReports 管理端举报列表（status: nil 全部 / 0 待处理 / 1 已处理）。
func (s *ForumModerationService) ListReports(page, pageSize int, status *int16) (*ForumReportPageResult, error) {
	type reportRow struct {
		ID         int64
		ReporterID int
		Reporter   string
		TopicID    *int64
		TopicTitle string
		ReplyID    *int64
		Reason     string
		Status     int16
		CreatedAt  time.Time
	}
	rows, total, page, pageSize, err := paging.QueryWithScan[reportRow](s.db, page, pageSize, 20, 100,
		"r.created_at DESC, r.id DESC",
		func(q *gorm.DB) *gorm.DB {
			q = q.Table("forum_report AS r").
				Select("r.id, r.reporter_id, r.topic_id, r.reply_id, r.reason, r.status, r.created_at, " +
					"COALESCE(u.username, '') AS reporter, COALESCE(t.title, '') AS topic_title").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = r.reporter_id").
				Joins("LEFT JOIN forum_topics AS t ON t.id = r.topic_id")
			if status != nil {
				q = q.Where("r.status = ?", *status)
			}
			return q
		})
	if err != nil {
		return nil, err
	}
	items := make([]ForumReportDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, ForumReportDTO{
			ID: r.ID, ReporterID: r.ReporterID, Reporter: r.Reporter,
			TopicID: r.TopicID, TopicTitle: r.TopicTitle, ReplyID: r.ReplyID,
			Reason: r.Reason, Status: r.Status, CreatedAt: formatISO(r.CreatedAt),
		})
	}
	return &ForumReportPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Reports: items,
	}, nil
}

// HandleReport 管理端处理举报（status: 0 待处理 / 1 已处理）；标记已处理时站内信通知举报人。
func (s *ForumModerationService) HandleReport(reportID int64, status int16) error {
	if status != 0 && status != 1 {
		return ErrReportStatusValue
	}
	var report model.ForumReport
	if err := s.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrForumReportNotFound
		}
		return err
	}
	if err := s.db.Model(&model.ForumReport{}).Where("id = ?", reportID).
		Update("status", status).Error; err != nil {
		return err
	}
	// 待处理 → 已处理时通知举报人（重复标记不重复通知；尽力而为，失败仅记日志）
	if status == 1 && report.Status != 1 {
		s.notifyReportHandled(&report)
	}
	return nil
}

// notifyReportHandled 举报处理完成站内信。举报对象可能已被删除：
// 主题已删时降级文案（不带标题与链接）；文案/链接/payload 由事件构造器单点（ADR-0027 C1）。
func (s *ForumModerationService) notifyReportHandled(report *model.ForumReport) {
	topicID := report.TopicID
	topicTitle := ""
	if report.TopicID != nil {
		var topic model.ForumTopic
		if err := s.db.Select("title").First(&topic, *report.TopicID).Error; err == nil {
			topicTitle = topic.Title
		}
	}
	s.notificationSvc.TryCreateForumReportHandledEvent(NewForumReportHandledEvent(report.ReporterID, report.ReplyID != nil, topicID, topicTitle))
}

// DesignateExperience 管理端认定「备考经验」（ADR-0040）。
//
// 一个认定动作同时置 is_experience 与 is_featured（经验蕴含精选，库层 CHECK 兜底），
// 并按「认定奖励每帖一次」发 +30 —— 与加精共用同一条流水，故先加精后认定不会重复发分
// （奖励政策 module 的发放事实判定短路），先认定后加精亦然。
// 状态已一致时幂等短路（重复认定不发分不改状态）。
func (s *ForumModerationService) DesignateExperience(topicID int64) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if topic.IsExperience {
		return s.fetchTopicDTO(topicID, 0)
	}
	// 已采纳的帖不可被认定为经验（与 AcceptReply 的守卫互为镜像，二者缺一即有漏洞）：
	// 逃生口是先取消采纳。库层 CHECK 兜底见迁移 000028。
	// 只判「是否有采纳指针」而非意图——同一条规则也兜住历史遗留的悬挂行。
	if topic.AcceptedReplyID != nil {
		return nil, ErrDesignateAcceptedTopic
	}
	now := beijingNow()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// CAS：认定与精选一并置位（两者必须同进，否则撞蕴含 CHECK）。
		// WHERE is_experience = false 保证并发下只有先胜者发分。
		res := tx.Model(&model.ForumTopic{}).
			Where("id = ? AND is_experience = ?", topicID, false).
			Updates(map[string]any{
				"is_experience": true,
				"is_featured":   true,
				"updated_at":    now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 并发抢认定：由先胜者完成副作用
		}
		return s.rewards.Award(tx, forumRewardFact{
			Kind: forumRewardDesignation, TopicID: topic.ID, TopicTitle: topic.Title,
			TopicOwner: topic.UserID, Designation: DesignationExperience, At: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.fetchTopicDTO(topicID, 0)
}

// RevokeExperience 管理端取消经验认定（ADR-0040）：只撤 is_experience，
// **保留精选位**（撤的是归类不是质量认可，管理员可继续让它挂着精选）；已发分不回滚
// （与撤精同政策：认定动作不是违规，回滚会让管理员不敢认定）。
// 状态已一致时幂等短路。
func (s *ForumModerationService) RevokeExperience(topicID int64) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if !topic.IsExperience {
		return s.fetchTopicDTO(topicID, 0)
	}
	// CAS：只改 is_experience，is_featured 原样保留（不写它，避免覆盖并发下的精选变更）
	if err := s.db.Model(&model.ForumTopic{}).
		Where("id = ? AND is_experience = ?", topicID, true).
		Updates(map[string]any{"is_experience": false, "updated_at": beijingNow()}).Error; err != nil {
		return nil, err
	}
	return s.fetchTopicDTO(topicID, 0)
}

// SetFeatured 管理端设置精选位（#742，全类别可用）。
//
// featured=true 且发生状态迁移时，同事务给帖主一次性直记 featured_bonus +30
// （幂等键 featured_bonus:{topicID} + 流水存在判定双保险，取消重精不重复发分，
// 沿用 accepted_bonus 同模式）；featured=false 只改状态，已发分不回滚。
// 状态已一致时幂等短路，不触发任何副作用。
func (s *ForumModerationService) SetFeatured(topicID int64, featured bool) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	// 经验帖蕴含精选（库层 CHECK 兜底）：直接撤精会撞 CHECK，或留下「经验但非精选」的悬挂态。
	// 逃生口是「先取消经验认定」——文案与 #811「已采纳的问答帖不能改类别，请先取消采纳」同构。
	// 判定按认定事实（IsExperience），与意图 Category 无关。
	if !featured && topic.IsExperience {
		return nil, ErrUnfeatureExperienceTopic
	}
	if topic.IsFeatured == featured {
		// 幂等：状态已一致（重复加精/重复取消），不发分不改状态
		return s.fetchTopicDTO(topicID, 0)
	}
	now := beijingNow()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// CAS：仅当状态仍为旧值时写入，并发下先胜者负责发分
		res := tx.Model(&model.ForumTopic{}).
			Where("id = ? AND is_featured = ?", topicID, !featured).
			Update("is_featured", featured)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // 并发抢改：由先胜者完成副作用
		}
		if !featured {
			return nil // 取消精选只改状态，已发分不回滚
		}
		// 认定奖励与「认定备考经验」共用同一实现：每帖只发一次，先认定后加精不重复发分。
		return s.rewards.Award(tx, forumRewardFact{
			Kind: forumRewardDesignation, TopicID: topic.ID, TopicTitle: topic.Title,
			TopicOwner: topic.UserID, Designation: DesignationFeatured, At: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.fetchTopicDTO(topicID, 0)
}
