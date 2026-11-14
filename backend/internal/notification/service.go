// Package notification 站内信通知域：站内信写入、事件构造器与列表/已读接口。
//
// 本包是 internal/<域> 形态的样板之一（ADR-0070）：handler.go 是 HTTP 出口，
// service.go / events.go 是域实现，包内分层靠文件名而非子目录。
// 本文件：站内信通知写入与读取（P0 通知基础设施，当前仅站内信渠道）。
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// NotificationDTO 站内信通知展示对象。
// Payload 为结构化业务标记（JSONB，加性字段，如 review_status），旧契约字段不变。
type NotificationDTO struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Link    string `json:"link"`
	// Payload 站内信结构化标记（JSONB 落库 payload）。ADR-0048 决策 6：非响应面不定型，
	// 故用 swaggertype 钉成不透明 object（生成物渲染 Record<string, unknown>），
	// 具体键的形状（review_status / topic_id / reply_id / points / reason）由前端 UI 收窄类型声明。
	Payload   model.JSONB `json:"payload,omitempty" swaggertype:"object" extensions:"x-optional"`
	IsRead    bool        `json:"is_read"`
	CreatedAt string      `json:"created_at"`
	ReadAt    *string     `json:"read_at,omitempty" extensions:"x-optional"`
}

// GormCreator 通知写入执行器（*gorm.DB 与 *gorm.Tx 均满足，事务内写入用）。
type GormCreator interface {
	Create(value interface{}) *gorm.DB
}

// Service 站内信通知服务。
type Service struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewService 构造通知服务。
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create 创建一条学员收件站内信通知（payload 为可选结构化标记，nil 表示无）。
func (s *Service) Create(userID int, typ, title, content, link string, payload model.JSONB) error {
	return s.CreateWithTx(s.db, userID, typ, title, content, link, payload, time.Now())
}

// CreateWithTx 在指定事务/连接内创建**学员收件**站内信（收件人落 user_id）。
// 业务事件（如资料审核）与业务写同事务提交，避免通知丢失；createdAt 由调用方控制时区语义。
// 讲师收件走 CreateForTutorWithTx——两个 id 空间不可互读（ADR-0095），故不共用一个入口。
func (s *Service) CreateWithTx(tx GormCreator, userID int, typ, title, content, link string, payload model.JSONB, createdAt time.Time) error {
	n := model.Notification{
		UserID:    &userID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Link:      link,
		Payload:   payload,
		CreatedAt: createdAt,
	}
	return tx.Create(&n).Error
}

// CreateForTutorWithTx 在指定事务/连接内创建**讲师收件**站内信（收件人落 tutor_id）。
// 与 CreateWithTx 分族而非共用一个「收件人 id」参数：两个 id 空间不可互读（ADR-0095），
// 签名里区分才不会被误传。
func (s *Service) CreateForTutorWithTx(tx GormCreator, tutorID int, typ, title, content, link string, payload model.JSONB, createdAt time.Time) error {
	n := model.Notification{
		TutorID:   &tutorID,
		Type:      typ,
		Title:     title,
		Content:   content,
		Link:      link,
		Payload:   payload,
		CreatedAt: createdAt,
	}
	return tx.Create(&n).Error
}

// 论坛采纳通知类型（#369）。
const (
	NotifTypeForumAcceptAnswerer = "forum_accept_answerer" // 答主：你的回答被采纳 +40
	NotifTypeForumAcceptOwner    = "forum_accept_owner"    // 楼主：你采纳了答案 +5
)

// forumTopicPayload 构造论坛事件通知结构化标记（{"topic_id": N}），
// 供前端/移动端确定性定位帖子（不依赖 link 文案解析）。
func forumTopicPayload(topicID int64) model.JSONB {
	b, err := json.Marshal(struct {
		TopicID int64 `json:"topic_id"`
	}{TopicID: topicID})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// forumAcceptPayload 构造采纳事件结构化标记（topic_id + reply_id + points + reason），
// 加性扩展 forumTopicPayload，不依赖标题文案判定（#369）。
func forumAcceptPayload(topicID, replyID int64, points int, reason string) model.JSONB {
	b, err := json.Marshal(struct {
		TopicID int64  `json:"topic_id"`
		ReplyID int64  `json:"reply_id"`
		Points  int    `json:"points"`
		Reason  string `json:"reason"`
	}{TopicID: topicID, ReplyID: replyID, Points: points, Reason: reason})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// ForumAcceptEvent 问答采纳事件通知参数（ADR-0024 C3）：站内信域单点构造
// title/content/link/payload 口径，业务侧一行触发。
type ForumAcceptEvent struct {
	// UserID 收件人（答主或楼主）。
	UserID int
	// Type 通知类型：NotifTypeForumAcceptAnswerer / NotifTypeForumAcceptOwner。
	Type string
	// TopicTitle 问答标题（用于文案）。
	TopicTitle string
	// TopicID 主题 ID。
	TopicID int64
	// ReplyID 被采纳回复 ID。
	ReplyID int64
	// Points 到账分值（与实际入账一致）。
	Points int
	// Reason 流水原因（由调用方传入：它是积分域的流水事实，站内信只把它记进 payload）。
	Reason string
}

// answererAcceptTitle 答主被采纳标题。
const answererAcceptTitle = "你的回答被采纳"

// ownerAcceptTitle 楼主采纳动作标题。
const ownerAcceptTitle = "你采纳了答案"

// NewAnswererAcceptEvent 构造答主被采纳通知事件（+40 分到账，link 锚到回答）。
// reason 由调用方传入（与同一事务入账的流水原因同源；流水原因归积分/论坛域所有，站内信只记录它）。
func NewAnswererAcceptEvent(userID int, topicTitle string, topicID, replyID int64, points int, reason string) ForumAcceptEvent {
	return ForumAcceptEvent{
		UserID:     userID,
		Type:       NotifTypeForumAcceptAnswerer,
		TopicTitle: topicTitle,
		TopicID:    topicID,
		ReplyID:    replyID,
		Points:     points,
		Reason:     reason,
	}
}

// NewOwnerAcceptEvent 构造楼主采纳动作通知事件（+5 分到账，link 锚到回答）。
// reason 由调用方传入（同 NewAnswererAcceptEvent）。
func NewOwnerAcceptEvent(userID int, topicTitle string, topicID, replyID int64, points int, reason string) ForumAcceptEvent {
	return ForumAcceptEvent{
		UserID:     userID,
		Type:       NotifTypeForumAcceptOwner,
		TopicTitle: topicTitle,
		TopicID:    topicID,
		ReplyID:    replyID,
		Points:     points,
		Reason:     reason,
	}
}

// CreateForumAcceptEvent 在指定事务/连接内创建一条问答采纳事件站内信。
// 与积分入账同事务提交/回滚（ADR-0023）：通知与到账积分一致。
func (s *Service) CreateForumAcceptEvent(tx GormCreator, ev ForumAcceptEvent, createdAt time.Time) error {
	link := fmt.Sprintf("/training/forum/%d#reply-%d", ev.TopicID, ev.ReplyID)
	title := answererAcceptTitle
	if ev.Type == NotifTypeForumAcceptOwner {
		title = ownerAcceptTitle
	}
	content := fmt.Sprintf("你在问答「%s」中的回答被采纳，+%d 分已到账", ev.TopicTitle, ev.Points)
	if ev.Type == NotifTypeForumAcceptOwner {
		content = fmt.Sprintf("你在问答「%s」中采纳了回答，+%d 分已到账", ev.TopicTitle, ev.Points)
	}
	return s.CreateWithTx(tx, ev.UserID, ev.Type, title, content, link, forumAcceptPayload(ev.TopicID, ev.ReplyID, ev.Points, ev.Reason), createdAt)
}

// 论坛加精通知类型（#742）。
const (
	NotifTypeForumFeatured = "forum_featured" // 帖主：帖子被加精 / 被认定为备考经验，+30（ADR-0040 两种认定共用同一类型与流水）
)

// forumFeaturedPayload 构造加精事件结构化标记（topic_id + points + reason）。
func forumFeaturedPayload(topicID int64, points int, reason string) model.JSONB {
	b, err := json.Marshal(struct {
		TopicID int64  `json:"topic_id"`
		Points  int    `json:"points"`
		Reason  string `json:"reason"`
	}{TopicID: topicID, Points: points, Reason: reason})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// 认定类型（ADR-0040）：加精与「认定备考经验」共用同一笔 featured_bonus 流水，
// 但**文案必须区分**——把「被认定为备考经验」写成「被加精」会让帖主看不懂发生了什么。
const (
	DesignationFeatured   = "featured"   // 精选位认定
	DesignationExperience = "experience" // 备考经验认定（蕴含精选）
)

// ForumFeaturedEvent 帖子认定事件通知参数（ADR-0024 C3，#742 / ADR-0040）。
type ForumFeaturedEvent struct {
	// UserID 收件人（帖主）。
	UserID int
	// TopicTitle 帖子标题（用于文案）。
	TopicTitle string
	// TopicID 主题 ID。
	TopicID int64
	// Points 到账分值（与实际入账一致）。
	Points int
	// Reason 流水原因（由调用方传入：它是积分域的流水事实，站内信只把它记进 payload）。
	Reason string
	// Designation 认定类型（DesignationFeatured | DesignationExperience），决定文案。
	Designation string
}

// NewTopicFeaturedEvent 构造**加精**通知事件（+30 分到账，link 锚到帖子）。
// reason 由调用方传入（与同一事务入账的流水原因同源）。
func NewTopicFeaturedEvent(userID int, topicTitle string, topicID int64, points int, reason string) ForumFeaturedEvent {
	return ForumFeaturedEvent{
		UserID:      userID,
		TopicTitle:  topicTitle,
		TopicID:     topicID,
		Points:      points,
		Reason:      reason,
		Designation: DesignationFeatured,
	}
}

// NewTopicExperienceEvent 构造**认定备考经验**通知事件（同一笔 +30，文案不同）。
// 与 NewTopicFeaturedEvent 拆成两个构造函数，遵 ADR-0027 C1「每个业务事件一个构造函数」：
// 流水 reason 相同不代表业务事件相同，文案口径内聚在站内信域，业务方一行触发。
func NewTopicExperienceEvent(userID int, topicTitle string, topicID int64, points int, reason string) ForumFeaturedEvent {
	return ForumFeaturedEvent{
		UserID:      userID,
		TopicTitle:  topicTitle,
		TopicID:     topicID,
		Points:      points,
		Reason:      reason,
		Designation: DesignationExperience,
	}
}

// CreateTopicFeaturedEvent 在指定事务/连接内创建一条帖子认定事件站内信。
// 与积分入账同事务提交/回滚（ADR-0023）：通知与到账积分一致。
func (s *Service) CreateTopicFeaturedEvent(tx GormCreator, ev ForumFeaturedEvent, createdAt time.Time) error {
	link := fmt.Sprintf("/training/forum/%d", ev.TopicID)
	var title, content string
	switch ev.Designation {
	case DesignationExperience:
		title = "你的帖子被认定为备考经验"
		content = fmt.Sprintf("你的帖子「%s」已被认定为备考经验并进入经验区，+%d 分已到账", ev.TopicTitle, ev.Points)
	default:
		title = "你的帖子被加精"
		content = fmt.Sprintf("你的帖子「%s」被加精精选，+%d 分已到账", ev.TopicTitle, ev.Points)
	}
	return s.CreateWithTx(tx, ev.UserID, NotifTypeForumFeatured, title, content, link, forumFeaturedPayload(ev.TopicID, ev.Points, ev.Reason), createdAt)
}

// reviewStatusPayload 构造审核状态结构化标记，如 {"review_status":"approved"}。
func reviewStatusPayload(reviewStatus string) model.JSONB {
	b, err := json.Marshal(struct {
		ReviewStatus string `json:"review_status"`
	}{ReviewStatus: reviewStatus})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// chapterReviewPayload 章节审核结果的 payload：review_status + 来源标记（#2537 / ADR-0095 决策七）。
// source ∈ {review, admin_edit}——「管理员直接改的」与「审核通过的」在 payload 上可判，
// 消费方不必读 title 文案判事实。
func chapterReviewPayload(reviewStatus, source string) model.JSONB {
	b, err := json.Marshal(struct {
		ReviewStatus string `json:"review_status"`
		Source       string `json:"source"`
	}{ReviewStatus: reviewStatus, Source: source})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// NotificationListPageResult 站内信分页结果（含未读数）。
type NotificationListPageResult struct {
	Items       []NotificationDTO `json:"items" nullability:"nonnil"`
	Page        int               `json:"page"`
	Pages       int               `json:"pages"`
	Total       int64             `json:"total"`
	UnreadCount int64             `json:"unread_count"`
}

// NotificationUnreadCountDTO 未读通知数（spec #962 片四：收口自 handler 内联 gin.H）。
type NotificationUnreadCountDTO struct {
	Count int64 `json:"count"`
}

// ErrNoMailbox 「该角色没有站内信箱」这一事实的唯一载体（ADR-0095）。
// 与「信箱是空的」严格区分：admin / recruiter 当前没有通知投递面，调通知端点得 403
// 而不是空列表——否则前端会看到一个功能没接通的「0 条未读」，那是 #2499 前端假报成功
// 的同一类缺陷。handler 据此映射 403。
var ErrNoMailbox = errors.New("该角色没有站内信箱")

// RecipientRole 收件人身份（ADR-0095 的两个 id 空间）。
type RecipientRole string

const (
	// RecipientStudent 学员收件人（落 notifications.user_id，指向 hrwai_users.id）。
	RecipientStudent RecipientRole = "student"
	// RecipientTutor 讲师收件人（落 notifications.tutor_id，指向 tutor.tutor_id）。
	RecipientTutor RecipientRole = "tutor"
)

// Recipient 站内信收件人：身份 + 该身份表内的 id。
// **两个 id 空间不可互读**——user_id=5 与 tutor_id=5 是两个不同的人（四张身份表各自
// 自增，撞号几乎必然）。读面只查本角色对应的那一列，不跨列兜底。
type Recipient struct {
	Role RecipientRole
	ID   int
}

// RecipientForRole 由登录角色（JWT role）构造收件人——**role→列的映射唯一入口**。
// 收敛在单点而非散在四个读面各写一遍，否则就是第二事实源（ADR-0095）。
// 非学员/讲师角色返回 ErrNoMailbox。
//
// 角色名用字面量而非 core.HrwaiRole：core 已经间接依赖本包（core → aiassistant →
// points → notification），反向 import 会成环。取值与 core 的两条常量一致，由
// TestRecipientForRoleMatchesCoreVocab 钉住，不靠人记。
func RecipientForRole(role string, id int) (Recipient, error) {
	switch role {
	case "hrwai_user":
		return Recipient{Role: RecipientStudent, ID: id}, nil
	case "tutor":
		return Recipient{Role: RecipientTutor, ID: id}, nil
	}
	return Recipient{}, ErrNoMailbox
}

// column 收件人所在的列名（本类型内部使用；不导出以免调用方绕过 RecipientForRole 拼列名）。
//
// 未知角色返回空串而**不是**兜底到 user_id：ADR-0095 明写「不跨列兜底」，兜到学员列正是
// 那类「看起来能跑、实则投错人」的缺陷。Recipient 字段全导出，字面量构造是可能的，
// 所以这里必须 fail-closed，由调用面（List/UnreadCount/...）把空列当成错误。
func (r Recipient) column() string {
	switch r.Role {
	case RecipientTutor:
		return "tutor_id"
	case RecipientStudent:
		return "user_id"
	}
	return ""
}

// errInvalidRecipient 未知收件人角色（含零值 Recipient{}）——fail-closed 的唯一载体。
var errInvalidRecipient = errors.New("收件人身份不合法")

// resolveColumn 取列名并拒绝未知角色（四个读面的统一前置）。
func (r Recipient) resolveColumn() (string, error) {
	col := r.column()
	if col == "" {
		return "", errInvalidRecipient
	}
	return col, nil
}

// List 分页查询当前收件人通知，并附带未读数（一次请求同时支撑列表与角标）。
func (s *Service) List(ctx context.Context, r Recipient, page, pageSize int) (*NotificationListPageResult, error) {
	col, err := r.resolveColumn()
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	var unread int64
	if err := db.Model(&model.Notification{}).
		Where(col+" = ? AND is_read = ?", r.ID, false).Count(&unread).Error; err != nil {
		return nil, err
	}

	rows, total, page, pageSize, err := paging.QueryWithScan[model.Notification](db, page, pageSize, 10, 50,
		"id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Model(&model.Notification{}).Where(col+" = ?", r.ID)
		})
	if err != nil {
		return nil, err
	}

	items := make([]NotificationDTO, 0, len(rows))
	for i := range rows {
		items = append(items, toNotificationDTO(&rows[i]))
	}
	return &NotificationListPageResult{
		Items:       items,
		Page:        page,
		Pages:       response.PageCount(total, pageSize),
		Total:       total,
		UnreadCount: unread,
	}, nil
}

// UnreadCount 查询当前收件人未读通知数。
func (s *Service) UnreadCount(ctx context.Context, r Recipient) (int64, error) {
	col, err := r.resolveColumn()
	if err != nil {
		return 0, err
	}
	var count int64
	err = s.db.WithContext(ctx).Model(&model.Notification{}).
		Where(col+" = ? AND is_read = ?", r.ID, false).
		Count(&count).Error
	return count, err
}

// MarkRead 将单条通知标记为已读（仅限本人；已读或不存在均幂等成功，非本人报错）。
func (s *Service) MarkRead(ctx context.Context, r Recipient, id int64) error {
	col, err := r.resolveColumn()
	if err != nil {
		return err
	}
	db := s.db.WithContext(ctx)
	res := db.Model(&model.Notification{}).
		Where("id = ? AND "+col+" = ? AND is_read = ?", id, r.ID, false).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	// 未更新：校验是否属于本人（不存在或已读都返回成功，保持幂等）
	var count int64
	if err := db.Model(&model.Notification{}).
		Where("id = ? AND "+col+" = ?", id, r.ID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("通知不存在")
	}
	return nil
}

// MarkAllRead 将当前收件人全部未读通知标记为已读。
func (s *Service) MarkAllRead(ctx context.Context, r Recipient) error {
	col, err := r.resolveColumn()
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&model.Notification{}).
		Where(col+" = ? AND is_read = ?", r.ID, false).
		Updates(map[string]any{"is_read": true, "read_at": time.Now()}).Error
}

// toNotificationDTO 组装展示对象。
func toNotificationDTO(n *model.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Content:   n.Content,
		Link:      n.Link,
		Payload:   n.Payload,
		IsRead:    n.IsRead,
		CreatedAt: timefmt.FormatISO(n.CreatedAt),
		ReadAt:    timefmt.FormatTimePtr(n.ReadAt),
	}
}
