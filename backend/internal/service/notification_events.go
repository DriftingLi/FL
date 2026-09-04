// Package service 实现业务服务层。
// 本文件：站内信事件构造器全域收编（ADR-0027 C1 / ADR-0024 C3 执行欠账）——
// 论坛互动 / 投稿生命周期 / 联系方式交换申请全部手拼调用点收回站内信域，
// 事件形状统一为 ForumAcceptEvent 已验证形态：事件 struct + New*Event 构造函数 + 域内写入方法。
//
// 错误语义两族：
//   - 强一致（Create*Event(tx, …) 返回 error）：与业务写同事务，失败回滚；
//   - 尽力而为（TryCreate*Event(…)）：内部吞错记日志，调用点一行触发（删帖/删回复/举报/申请通知）。
package service

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"forklift-training/internal/model"
)

// 论坛互动通知类型。
const (
	NotifTypeForumReply    = "forum_reply"         // 楼主/被回复人被新回复
	NotifTypeForumTopicDel = "forum_topic_deleted" // 管理端删帖
	NotifTypeForumReplyDel = "forum_reply_deleted" // 管理端删回复
	NotifTypeForumReport   = "forum_report"        // 举报处理结果
)

// 投稿生命周期通知类型。
const (
	NotifTypeContributionApproved = "contribution_approved" // 投稿过审（作者）
	NotifTypeContributionRejected = "contribution_rejected" // 投稿被驳回（作者）
	NotifTypeContributionArchived = "contribution_archived" // 投稿被下架（作者，含追回）
	NotifTypeContributionTier     = "contribution_tier"     // 投稿达阶（作者）
)

// 联系方式交换申请通知类型。
const NotifTypeContactRequest = "contact_request"

// 资料审核通知类型。
const NotifTypeProfileReview = "profile_review"

// ===== 论坛：收到新回复（强一致族，与回复写入同事务） =====

// ForumReplyEvent 收到新回复事件（ADR-0027 C1）：楼主被回复或楼中楼被回复。
type ForumReplyEvent struct {
	// UserID 收件人（楼主或楼中楼被回复人）。
	UserID int
	// ReplierName 回复人展示名（文案用；查询失败时为空的现状由调用方保证）。
	ReplierName string
	// TopicTitle 帖子标题（文案用）。
	TopicTitle string
	// TopicID 帖子 ID（link 与 payload 用）。
	TopicID int64
	// IsReplyToReply 是否楼中楼回复（true=「你的回复有新回复」；false=「你的帖子有新回复」）。
	IsReplyToReply bool
}

// NewTopicReplierEvent 楼主被回复事件。
func NewTopicReplierEvent(userID int, replierName, topicTitle string, topicID int64) ForumReplyEvent {
	return ForumReplyEvent{UserID: userID, ReplierName: replierName, TopicTitle: topicTitle, TopicID: topicID}
}

// NewReplyReplierEvent 楼中楼被回复事件。
func NewReplyReplierEvent(userID int, replierName, topicTitle string, topicID int64) ForumReplyEvent {
	return ForumReplyEvent{UserID: userID, ReplierName: replierName, TopicTitle: topicTitle, TopicID: topicID, IsReplyToReply: true}
}

// CreateForumReplyEvent 在指定事务/连接内创建一条回复互动站内信（与回复写入同事务）。
func (s *NotificationService) CreateForumReplyEvent(tx GormCreator, ev ForumReplyEvent, createdAt time.Time) error {
	title := "你的帖子有新回复"
	content := fmt.Sprintf("%s 回复了你的帖子「%s」", ev.ReplierName, ev.TopicTitle)
	if ev.IsReplyToReply {
		title = "你的回复有新回复"
		content = fmt.Sprintf("%s 在帖子「%s」中回复了你", ev.ReplierName, ev.TopicTitle)
	}
	link := fmt.Sprintf("/training/forum/%d", ev.TopicID)
	return s.CreateWithTx(tx, ev.UserID, NotifTypeForumReply, title, content, link, forumTopicPayload(ev.TopicID), createdAt)
}

// ===== 论坛：管理端删帖 / 删回复（尽力而为族） =====

// ForumTopicDeletedEvent 管理端删除帖子事件（内容已删，通知失败不回滚）。
type ForumTopicDeletedEvent struct {
	UserID     int
	TopicTitle string
}

// NewForumTopicDeletedEvent 构造管理端删帖通知事件。
func NewForumTopicDeletedEvent(userID int, topicTitle string) ForumTopicDeletedEvent {
	return ForumTopicDeletedEvent{UserID: userID, TopicTitle: topicTitle}
}

// TryCreateForumTopicDeletedEvent 尽力而为创建删帖通知（失败仅记日志）。
func (s *NotificationService) TryCreateForumTopicDeletedEvent(ev ForumTopicDeletedEvent) {
	content := "管理员删除了你的帖子「" + ev.TopicTitle + "」。"
	s.tryCreateEvent(ev.UserID, NotifTypeForumTopicDel, "你的帖子已被删除", content, "", nil)
}

// ForumReplyDeletedEvent 管理端删除回复事件（内容已删，通知失败不回滚）。
type ForumReplyDeletedEvent struct {
	UserID     int
	TopicTitle string
	TopicID    int64
}

// NewForumReplyDeletedEvent 构造管理端删回复通知事件。
func NewForumReplyDeletedEvent(userID int, topicTitle string, topicID int64) ForumReplyDeletedEvent {
	return ForumReplyDeletedEvent{UserID: userID, TopicTitle: topicTitle, TopicID: topicID}
}

// TryCreateForumReplyDeletedEvent 尽力而为创建删回复通知（失败仅记日志）。
func (s *NotificationService) TryCreateForumReplyDeletedEvent(ev ForumReplyDeletedEvent) {
	content := "管理员删除了你在帖子「" + ev.TopicTitle + "」中的回复。"
	s.tryCreateEvent(ev.UserID, NotifTypeForumReplyDel, "你的回复已被删除", content,
		fmt.Sprintf("/training/forum/%d", ev.TopicID), forumTopicPayload(ev.TopicID))
}

// ===== 论坛：举报处理结果（尽力而为族） =====

// ForumReportHandledEvent 举报处理完成事件。举报对象可能已被删除：
// TopicTitle 为空（主题已删或查询失败）时降级为不带标题与链接的文案。
type ForumReportHandledEvent struct {
	// UserID 收件人（举报人）。
	UserID int
	// ForReply 举报对象是否为回复（false=帖子）。
	ForReply bool
	// TopicID 被举报主题 ID；nil 或无标题时降级（无 link/payload）。
	TopicID *int64
	// TopicTitle 被举报对象标题；空串表示主题已删/查不到，走降级文案。
	TopicTitle string
}

// NewForumReportHandledEvent 构造举报处理完成通知事件。
// topicTitle 由调用方先查主题标题（查不到传空串，走降级文案）。
func NewForumReportHandledEvent(userID int, forReply bool, topicID *int64, topicTitle string) ForumReportHandledEvent {
	return ForumReportHandledEvent{UserID: userID, ForReply: forReply, TopicID: topicID, TopicTitle: topicTitle}
}

// TryCreateForumReportHandledEvent 尽力而为创建举报处理通知（失败仅记日志）。
func (s *NotificationService) TryCreateForumReportHandledEvent(ev ForumReportHandledEvent) {
	target := "帖子"
	if ev.ForReply {
		target = "回复"
	}
	content := "你举报的" + target + "已处理完毕。"
	link := ""
	var payload model.JSONB
	if ev.TopicID != nil && ev.TopicTitle != "" {
		content = fmt.Sprintf("你举报的%s「%s」已处理完毕。", target, ev.TopicTitle)
		link = fmt.Sprintf("/training/forum/%d", *ev.TopicID)
		payload = forumTopicPayload(*ev.TopicID)
	}
	s.tryCreateEvent(ev.UserID, NotifTypeForumReport, "举报已处理", content, link, payload)
}

// ===== 投稿：全生命周期（强一致族，与状态流转/积分入账同事务） =====

// contributionPayload 构造投稿事件结构化标记（contribution_id + title + points + reason），
// 加性扩展，不依赖标题文案判定（照 forumAcceptPayload 形状，ADR-0027 C1 payload 随事件归位站内信域）。
func contributionPayload(contributionID int64, title string, points int, reason string) model.JSONB {
	b, err := json.Marshal(struct {
		ContributionID int64  `json:"contribution_id"`
		Title          string `json:"title"`
		Points         int    `json:"points"`
		Reason         string `json:"reason"`
	}{ContributionID: contributionID, Title: title, Points: points, Reason: reason})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// ContributionApprovedEvent 投稿过审事件。
type ContributionApprovedEvent struct {
	UserID         int
	Title          string
	ContributionID int64
	// Points 到账分值（与积分入账一致）。
	Points int
	// Reason 流水原因（ReasonContributionApproved）。
	Reason string
}

// NewContributionApprovedEvent 构造投稿过审通知事件。
func NewContributionApprovedEvent(userID int, title string, contributionID int64, points int) ContributionApprovedEvent {
	return ContributionApprovedEvent{UserID: userID, Title: title, ContributionID: contributionID, Points: points, Reason: ReasonContributionApproved}
}

// CreateContributionApprovedEvent 在指定事务内创建过审站内信（与入账同事务）。
func (s *NotificationService) CreateContributionApprovedEvent(tx GormCreator, ev ContributionApprovedEvent, createdAt time.Time) error {
	content := fmt.Sprintf("你的投稿「%s」已通过审核，+%d 分已到账", ev.Title, ev.Points)
	return s.CreateWithTx(tx, ev.UserID, NotifTypeContributionApproved, "资料投稿通过审核", content,
		"/training/materials?tab=contribution",
		contributionPayload(ev.ContributionID, ev.Title, ev.Points, ev.Reason), createdAt)
}

// ContributionRejectedEvent 投稿驳回事件（强一致：通知失败返回 error，不落则业务已驳回）。
type ContributionRejectedEvent struct {
	UserID         int
	Title          string
	ContributionID int64
	// RejectReason 驳回原因（文案用）。
	RejectReason string
}

// NewContributionRejectedEvent 构造投稿驳回通知事件。
func NewContributionRejectedEvent(userID int, title string, contributionID int64, rejectReason string) ContributionRejectedEvent {
	return ContributionRejectedEvent{UserID: userID, Title: title, ContributionID: contributionID, RejectReason: rejectReason}
}

// CreateContributionRejectedEvent 创建驳回站内信（调用方传 db；维持既有「驳回通知失败返回 error」语义）。
func (s *NotificationService) CreateContributionRejectedEvent(tx GormCreator, ev ContributionRejectedEvent, createdAt time.Time) error {
	content := fmt.Sprintf("你的投稿「%s」未通过审核：%s", ev.Title, ev.RejectReason)
	// 与原 contributionPayload(..., 0, "") 口径一致：驳回不score、reason 空。
	return s.CreateWithTx(tx, ev.UserID, NotifTypeContributionRejected, "资料投稿被驳回", content,
		"/training/materials?tab=contribution&view=mine",
		contributionPayload(ev.ContributionID, ev.Title, 0, ""), createdAt)
}

// ContributionTierEvent 投稿达阶事件。
type ContributionTierEvent struct {
	UserID         int
	Title          string
	ContributionID int64
	// Threshold 触发档位下载量（文案用）。
	Threshold int
	// Points 本档追加奖励。
	Points int
	// Reason 流水原因（ReasonContributionTier）。
	Reason string
}

// NewContributionTierEvent 构造投稿达阶通知事件。
func NewContributionTierEvent(userID int, title string, contributionID int64, threshold, points int) ContributionTierEvent {
	return ContributionTierEvent{UserID: userID, Title: title, ContributionID: contributionID, Threshold: threshold, Points: points, Reason: ReasonContributionTier}
}

// CreateContributionTierEvent 在指定事务内创建达阶站内信（与入账同事务）。
func (s *NotificationService) CreateContributionTierEvent(tx GormCreator, ev ContributionTierEvent, createdAt time.Time) error {
	content := fmt.Sprintf("你的投稿「%s」下载量达 %d 次，+%d 分已到账", ev.Title, ev.Threshold, ev.Points)
	return s.CreateWithTx(tx, ev.UserID, NotifTypeContributionTier, "资料投稿下载量达阶", content,
		"/training/materials?tab=contribution",
		contributionPayload(ev.ContributionID, ev.Title, ev.Points, ev.Reason), createdAt)
}

// ContributionArchivedEvent 投稿下架事件（含追回：ClawedBack>0 文案带扣减）。
type ContributionArchivedEvent struct {
	UserID         int
	Title          string
	ContributionID int64
	// ArchiveReason 下架原因（文案用）。
	ArchiveReason string
	// ClawedBack 追回分（0=未追回；文案与 payload points 用）。
	ClawedBack int
	// Reason 流水原因（ReasonRollback）。
	Reason string
}

// NewContributionArchivedEvent 构造投稿下架通知事件。
func NewContributionArchivedEvent(userID int, title string, contributionID int64, archiveReason string, clawedBack int) ContributionArchivedEvent {
	return ContributionArchivedEvent{UserID: userID, Title: title, ContributionID: contributionID, ArchiveReason: archiveReason, ClawedBack: clawedBack, Reason: ReasonRollback}
}

// CreateContributionArchivedEvent 在指定事务内创建下架站内信（与追回同事务）。
func (s *NotificationService) CreateContributionArchivedEvent(tx GormCreator, ev ContributionArchivedEvent, createdAt time.Time) error {
	msg := fmt.Sprintf("你的投稿「%s」已下架：%s", ev.Title, ev.ArchiveReason)
	if ev.ClawedBack > 0 {
		msg = fmt.Sprintf("你的投稿「%s」已下架：%s（已回收该稿奖励 %d 分）", ev.Title, ev.ArchiveReason, ev.ClawedBack)
	}
	return s.CreateWithTx(tx, ev.UserID, NotifTypeContributionArchived, "资料投稿已下架", msg,
		"/training/materials?tab=contribution&view=mine",
		contributionPayload(ev.ContributionID, ev.Title, -ev.ClawedBack, ev.Reason), createdAt)
}

// ===== 联系方式交换申请（尽力而为族） =====

// ContactRequestEvent 招聘方发起联系方式交换申请事件（通知学员，不含企业电话）。
type ContactRequestEvent struct {
	// UserID 收件人（学员）。
	UserID int
	// CompanyName 企业名（文案用）。
	CompanyName string
	// ContactName 招聘方联系人名（文案用）。
	ContactName string
	// Message 附言（文案用）。
	Message string
	// ContactRequestID 申请 ID（payload 用）。
	ContactRequestID int64
	// RecruiterID 招聘方 ID（payload 用）。
	RecruiterID int
}

// NewContactRequestEvent 构造联系方式交换申请通知事件。
func NewContactRequestEvent(userID int, companyName, contactName, message string, contactRequestID int64, recruiterID int) ContactRequestEvent {
	return ContactRequestEvent{UserID: userID, CompanyName: companyName, ContactName: contactName, Message: message, ContactRequestID: contactRequestID, RecruiterID: recruiterID}
}

// TryCreateContactRequestEvent 尽力而为创建申请通知（失败仅记日志）。
func (s *NotificationService) TryCreateContactRequestEvent(ev ContactRequestEvent) {
	content := fmt.Sprintf("企业「%s」联系人 %s 申请查看你的联系方式，附言：%s", ev.CompanyName, ev.ContactName, ev.Message)
	b, err := json.Marshal(struct {
		ContactRequestID int64 `json:"contact_request_id"`
		RecruiterID      int   `json:"recruiter_id"`
	}{ContactRequestID: ev.ContactRequestID, RecruiterID: ev.RecruiterID})
	if err != nil {
		b = nil
	}
	s.tryCreateEvent(ev.UserID, NotifTypeContactRequest, "收到联系方式交换申请", content, "/training/resume", model.JSONB(b))
}

// ===== 资料审核结果（对齐事件形状，ADR-0027 C1：四元组旧形态收编） =====

// ProfileReviewEvent 资料审核结果事件（type=profile_review，link 恒空）。
// payload 为结构化判定标记（{"review_status":"approved"|"rejected"}），供前端确定性消费。
type ProfileReviewEvent struct {
	// UserID 收件人（学员）。
	UserID int
	// FieldType 审核字段：ProfileFieldNickname / ProfileFieldAvatar（文案用）。
	FieldType string
	// Status 审核结果：ProfileStatusApproved / ProfileStatusRejected。
	Status string
	// Reason 驳回原因（approved 时为空）。
	Reason string
}

// NewProfileReviewEvent 构造资料审核结果通知事件。
func NewProfileReviewEvent(req *model.ProfileChangeRequest, status, reason string) ProfileReviewEvent {
	fieldType := ""
	if req != nil {
		fieldType = req.FieldType
	}
	return ProfileReviewEvent{UserID: req.UserID, FieldType: fieldType, Status: status, Reason: reason}
}

// CreateProfileReviewEvent 在指定事务/连接内创建资料审核结果站内信（与审核状态流转同事务）。
func (s *NotificationService) CreateProfileReviewEvent(tx GormCreator, ev ProfileReviewEvent, createdAt time.Time) error {
	fieldLabel := "昵称"
	if ev.FieldType == ProfileFieldAvatar {
		fieldLabel = "头像"
	}
	title := "资料审核通过"
	content := "您的" + fieldLabel + "修改已通过审核，修改已生效。"
	payload := reviewStatusPayload(ProfileStatusApproved)
	if ev.Status == ProfileStatusRejected {
		title = "资料审核被驳回"
		content = "您的" + fieldLabel + "修改申请未通过审核"
		if ev.Reason != "" {
			content += "，原因：" + ev.Reason
		}
		content += "。"
		payload = reviewStatusPayload(ProfileStatusRejected)
	}
	return s.CreateWithTx(tx, ev.UserID, NotifTypeProfileReview, title, content, "", payload, createdAt)
}

// ===== 尽力而为写入基座 =====

// tryCreateEvent 尽力而为写入站内信：任何失败（含 receiver 为 nil）仅记日志，不向调用方传播。
// 用于删帖/删回复/举报处理/联系方式交换申请等「内容已删或属辅助通知，失败不回滚」的调用族。
func (s *NotificationService) tryCreateEvent(userID int, typ, title, content, link string, payload model.JSONB) {
	if s == nil {
		return
	}
	if err := s.CreateWithTx(s.db, userID, typ, title, content, link, payload, time.Now()); err != nil {
		s.logger.Warn("站内信尽力而为写入失败", zap.String("type", typ), zap.Int("user_id", userID), zap.Error(err))
	}
}
