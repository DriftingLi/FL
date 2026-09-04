// Package service 实现业务服务层。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复，图文分离发图）。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// 论坛范围常量。
const (
	ForumScopeAll     = "all"     // 全部（综合讨论区 + 章节讨论区）
	ForumScopeGeneral = "general" // 综合讨论区（chapter_id IS NULL）
	ForumScopeChapter = "chapter" // 指定章节讨论区
)

// 论坛帖子类别常量（#364）。
//
// 类别判"帖子意图"，scope/chapter_id 判"内容坐标"，两者正交但有一格非法：
// discussion+NULL=综合讨论区、discussion+N=章节讨论区、question+NULL=全局问答、
// question+N=非法。不预留第三个值——求职信息是常驻实体，不在论坛内。
const (
	ForumCategoryDiscussion = "discussion" // 讨论帖（存量帖子的默认值）
	ForumCategoryQuestion   = "question"   // 问答帖（可被采纳，走积分直记）
)

// normalizeForumCategory 校验并归一帖子类别：空串归一为 discussion（向后兼容，移动端不传）。
// 非空且不在值域内返回错误。归一后再落到模型上，避免依赖数据库 DEFAULT
// （GORM 对带 default tag 的零值字段会跳过 INSERT，内存对象拿不到回填值）。
func normalizeForumCategory(category string) (string, error) {
	switch category = strings.TrimSpace(category); category {
	case "":
		return ForumCategoryDiscussion, nil
	case ForumCategoryDiscussion, ForumCategoryQuestion:
		return category, nil
	default:
		return "", fmt.Errorf("帖子类别无效: %s", category)
	}
}

// 采纳积分常量（#366）：每帖只发一次分，走流水直记（非任务制）。
const (
	AcceptBonusPoints   = 40               // 答主采纳奖励
	AcceptActionPoints  = 5                // 楼主采纳行为奖励
	ReasonAcceptedBonus = "accepted_bonus" // 流水原因：被采纳奖励
	ReasonAcceptAction  = "accept_action"  // 流水原因：采纳行为奖励
	ReasonRollback      = "rollback"       // 流水原因：违规回收（#376）
)

// ErrNotTopicOwner 只有楼主可采纳/取消/更换。
var ErrNotTopicOwner = errors.New("只有楼主可以执行此操作")

// 论坛发图限制。
const (
	ForumTopicMaxImages = 9 // 主题最多图片数
	ForumReplyMaxImages = 3 // 回复最多图片数
)

// ForumAuthor 论坛作者信息（展示名为昵称）。
type ForumAuthor struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

// DisplayName 返回论坛展示名（昵称）。
func (a ForumAuthor) DisplayName() string {
	return a.Username
}

// ForumTopicDTO 论坛主题列表/详情对象。
type ForumTopicDTO struct {
	ID              int64       `json:"id"`
	ChapterID       *int        `json:"chapter_id"`
	Category        string      `json:"category"` // discussion | question（#364）
	ChapterTitle    string      `json:"chapter_title"`
	Title           string      `json:"title"`
	Content         string      `json:"content"`
	Images          []string    `json:"images"`
	ViewCount       int         `json:"view_count"`
	ReplyCount      int         `json:"reply_count"`
	LastReplyAt     *string     `json:"last_reply_at"`
	CreatedAt       string      `json:"created_at"`
	Author          ForumAuthor `json:"author"`
	CanDelete       bool        `json:"can_delete"`
	LikesCount      int64       `json:"likes_count"`
	LikedByMe       bool        `json:"liked_by_me"`
	AcceptedReplyID *int64      `json:"accepted_reply_id,omitempty"`
	SolvedAt        *string     `json:"solved_at,omitempty"`
	RewardIssued    bool        `json:"reward_issued"`
}

// ForumReplyDTO 论坛回复对象。
type ForumReplyDTO struct {
	ID         int64       `json:"id"`
	TopicID    int64       `json:"topic_id"`
	ParentID   *int64      `json:"parent_id,omitempty"`
	ParentName string      `json:"parent_name,omitempty"` // 被回复人的展示名
	Content    string      `json:"content"`
	Images     []string    `json:"images"`
	CreatedAt  string      `json:"created_at"`
	Author     ForumAuthor `json:"author"`
	CanDelete  bool        `json:"can_delete"`
	LikesCount int64       `json:"likes_count"`
	LikedByMe  bool        `json:"liked_by_me"`
	IsAccepted bool        `json:"is_accepted"`
}

// ForumService 论坛服务。
type ForumService struct {
	db              *gorm.DB
	fileSvc         *FileStore
	notificationSvc *NotificationService
	counters        ForumCounter // 计数列唯一写入口（spec #297）
	// points 积分簿记通道（ADR-0023 forum 收编）：采纳奖励与违规回收经其事务内
	// 导出方法落账，forum 内不再直写积分流水/余额；依赖方向 forum→points 单向无环。
	points *PointsService

	logger *zap.Logger
}

// NewForumService 构造论坛服务。
// fileSvc 用于删除帖子/回复时清理图片存储（可 nil，nil 时跳过清理）；
// notificationSvc 用于论坛事件站内信（回复/举报处理/管理端删帖，见各触发点）；
// counters 为 likes_count / reply_count 唯一写入口（与 AuthService 共享同一实例）；
// points 为积分簿记通道（采纳奖励/违规回收经其事务内导出方法落账，ADR-0023）。
func NewForumService(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, counters ForumCounter, points *PointsService, logger *zap.Logger) *ForumService {
	return &ForumService{db: db, fileSvc: fileSvc, notificationSvc: notificationSvc, counters: counters, points: points, logger: logger}
}

// topicRow 列表查询的扫描结构。
type topicRow struct {
	ID              int64
	ChapterID       *int
	Category        string
	ChapterTitle    string
	Title           string
	Content         string
	Images          string
	ViewCount       int
	ReplyCount      int
	LikesCount      int64
	AcceptedReplyID *int64
	SolvedAt        *time.Time
	LastReplyAt     *time.Time
	CreatedAt       time.Time
	UserID          int
	Username        string
	AvatarURL       string
}

func (r topicRow) toDTO(viewerID int) ForumTopicDTO {
	var lastReplyAt *string
	if r.LastReplyAt != nil {
		s := formatISO(*r.LastReplyAt)
		lastReplyAt = &s
	}
	var solvedAt *string
	if r.SolvedAt != nil {
		s := formatISO(*r.SolvedAt)
		solvedAt = &s
	}
	return ForumTopicDTO{
		ID:              r.ID,
		ChapterID:       r.ChapterID,
		Category:        r.Category,
		ChapterTitle:    r.ChapterTitle,
		Title:           r.Title,
		Content:         r.Content,
		Images:          parseImageURLs(r.Images),
		ViewCount:       r.ViewCount,
		ReplyCount:      r.ReplyCount,
		LikesCount:      r.LikesCount,
		AcceptedReplyID: r.AcceptedReplyID,
		SolvedAt:        solvedAt,
		LastReplyAt:     lastReplyAt,
		CreatedAt:       formatISO(r.CreatedAt),
		Author: ForumAuthor{
			UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL,
		},
		CanDelete: r.UserID == viewerID,
	}
}

// ForumTopicPageResult 论坛主题分页结果。
type ForumTopicPageResult struct {
	Page   int             `json:"page"`
	Pages  int             `json:"pages"`
	Topics []ForumTopicDTO `json:"topics"`
	Total  int64           `json:"total"`
}

// parseForumCategoryArg 解析**列表查询**的 category 参数。
//
// 注意与 normalizeForumCategory（发帖路径）语义相反，不要合并成一函数：
// 列表里空串 = 不过滤（移动端与既有页面向后兼容，两类都看得到）；
// 发帖里空串 = 归一为 discussion。若把两者写成一套，移动端不传参数就会只看到讨论帖。
func parseForumCategoryArg(category string) (string, error) {
	switch category = strings.TrimSpace(category); category {
	case "":
		return "", nil
	case ForumCategoryDiscussion, ForumCategoryQuestion:
		return category, nil
	default:
		return "", fmt.Errorf("帖子类别无效: %s", category)
	}
}

// parseSolvedArg 解析列表查询的 solved 参数（#367）。
// 空或 all = 不过滤；solved = 已解决（accepted_reply_id 非空）；unsolved = 求助（accepted_reply_id 为空）。
func parseSolvedArg(solved string) (string, error) {
	switch v := strings.TrimSpace(strings.ToLower(solved)); v {
	case "", "all":
		return "", nil
	case "solved":
		return "solved", nil
	case "unsolved":
		return "unsolved", nil
	default:
		return "", fmt.Errorf("solved 参数无效: %s", solved)
	}
}

// TopicListInput 主题列表查询条件。
//
// 用 struct 而非位置参数：本方法有 scope/keyword/sort/order/category 五个 string，
// 位置传错（如把 category 落进 keyword）编译通过且语义全错。
type TopicListInput struct {
	Scope     string // all（默认）/ general / chapter
	Category  string // 空或 all = 不过滤；discussion / question = 按类别分流
	Solved    string // 空或 all = 不过滤；solved / unsolved（#367，仅问答帖有意义）
	ChapterID int
	Page      int
	PageSize  int
	Keyword   string
	Sort      string // latest（默认）/ hot
	Order     string // desc（默认）/ asc
}

// ListTopics 分页查询主题。
// scope: all（默认）/ general（综合讨论区）/ chapter（需配合 chapterID）；sort: latest（默认，时间）/ hot（热度：点赞数→回复数→浏览数）；order: desc（默认）/ asc（正序）。
func (s *ForumService) ListTopics(in TopicListInput) (*ForumTopicPageResult, error) {
	scope := in.Scope
	chapterID, page, pageSize := in.ChapterID, in.Page, in.PageSize
	keyword, sort, order := in.Keyword, in.Sort, in.Order
	category, err := parseForumCategoryArg(in.Category)
	if err != nil {
		return nil, err
	}
	solved, err := parseSolvedArg(in.Solved)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		scope = ForumScopeAll
	}
	if scope == ForumScopeChapter && chapterID <= 0 {
		return nil, errors.New("查询章节讨论区需要有效的 chapter_id")
	}
	if sort != "hot" {
		sort = "latest"
	}
	dir := "DESC"
	if order == "asc" {
		dir = "ASC"
	}
	// 统一 sort 别名：time 作为 latest 的别名（兼容详情页旧值）
	if sort == "time" {
		sort = "latest"
	}
	orderClause := "COALESCE(t.last_reply_at, t.created_at) " + dir + ", t.id " + dir
	if sort == "hot" {
		orderClause = "t.likes_count " + dir + ", t.reply_count " + dir + ", t.view_count " + dir + ", t.id " + dir
	}

	rows, total, page, pageSize := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		orderClause,
		func(q *gorm.DB) *gorm.DB {
			q = q.Table("forum_topics AS t").
				Select("t.id, t.chapter_id, t.category, t.title, t.content, t.images, t.view_count, t.reply_count, t.likes_count, t.accepted_reply_id, t.solved_at, t.last_reply_at, t.created_at, " +
					"u.id AS user_id, u.username, u.avatar_url, " +
					"COALESCE(ch.title, '') AS chapter_title").
				Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id")
			switch scope {
			case ForumScopeGeneral:
				q = q.Where("t.chapter_id IS NULL")
			case ForumScopeChapter:
				q = q.Where("t.chapter_id = ?", chapterID)
			}
			// 类别过滤必须与上面的 scope 共存在同一条 WHERE 里：
			// scope=general 的定义就是 chapter_id IS NULL，而问答帖的 chapter_id 恒为 NULL，
			// 漏掉这条会把全部问答帖灌进讨论 Tab（不要在应用层事后过滤）。
			if category != "" {
				q = q.Where("t.category = ?", category)
			}
			if solved == "solved" {
				q = q.Where("t.accepted_reply_id IS NOT NULL")
			} else if solved == "unsolved" {
				q = q.Where("t.accepted_reply_id IS NULL")
			}
			if keyword = strings.TrimSpace(keyword); keyword != "" {
				like := "%" + keyword + "%"
				q = q.Where("(t.title ILIKE ? OR t.content ILIKE ?)", like, like)
			}
			return q
		})

	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(0))
	}
	// 批量回填 reward_issued（#367 二次确认分支所需）：仅对问答帖且已发放过的主题标记。
	s.enrichRewardIssued(items)
	return &ForumTopicPageResult{
		Page:   page,
		Pages:  response.PageCount(total, pageSize),
		Topics: items,
		Total:  total,
	}, nil
}

// GetTopic 主题详情（含回复，回复带被回复人信息），并累加浏览量。
// replySort: time/latest（默认，时间）/ hot（热度：点赞数→时间）；order: asc/desc（默认 asc 对 time，desc 对 hot；显式传入时统一覆盖）
func (s *ForumService) GetTopic(topicID int64, viewerID int, replySort, order string) (map[string]any, error) {
	if replySort == "latest" {
		replySort = "time"
	}
	var row topicRow
	err := s.db.Table("forum_topics AS t").
		Select("t.id, t.chapter_id, t.category, t.title, t.content, t.images, t.view_count, t.reply_count, t.likes_count, t.accepted_reply_id, t.solved_at, t.last_reply_at, t.created_at, "+
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

	// 浏览量 +1（失败不影响主流程）
	_ = s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	row.ViewCount++

	// 记录去重浏览（用于 daily_browse 积分，排除自帖，同一帖每日一次）
	if viewerID != 0 && viewerID != int(row.UserID) {
		viewDate := time.Now().In(clock.Location()).Format("2006-01-02")
		_ = s.db.Exec("INSERT INTO forum_topic_views (user_id, topic_id, view_date) VALUES (?,?,?) ON CONFLICT (user_id, topic_id, view_date) DO NOTHING", viewerID, topicID, viewDate).Error
	}

	// 回复列表（含被回复人展示名）
	var replies []struct {
		ID         int64
		TopicID    int64
		ParentID   *int64
		Content    string
		Images     string
		LikesCount int64
		CreatedAt  time.Time
		UserID     int
		Username   string
		AvatarURL  string
		ParentName string
	}
	dir := "ASC"
	if order == "desc" {
		dir = "DESC"
	} else if order == "" {
		// 默认：time/latest 按正序（先发先排），hot 按热度倒序
		if replySort == "hot" {
			dir = "DESC"
		} else {
			dir = "ASC"
		}
	}
	replyOrder := "r.created_at " + dir + ", r.id " + dir
	if replySort == "hot" {
		replyOrder = "r.likes_count " + dir + ", r.created_at ASC, r.id ASC"
	}
	if err := s.db.Table("forum_replies AS r").
		Select("r.id, r.topic_id, r.parent_id, r.content, r.images, r.likes_count, r.created_at, "+
			"u.id AS user_id, u.username, u.avatar_url, "+
			"COALESCE(pu.username, '') AS parent_name").
		Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
		Joins("LEFT JOIN forum_replies AS pr ON pr.id = r.parent_id").
		Joins("LEFT JOIN hrwai_users AS pu ON pu.id = pr.user_id").
		Where("r.topic_id = ?", topicID).
		Order(replyOrder).
		Scan(&replies).Error; err != nil {
		return nil, err
	}

	replyDTOs := make([]ForumReplyDTO, 0, len(replies))
	for _, r := range replies {
		isAcc := row.AcceptedReplyID != nil && *row.AcceptedReplyID == r.ID
		replyDTOs = append(replyDTOs, ForumReplyDTO{
			ID: r.ID, TopicID: r.TopicID, ParentID: r.ParentID, ParentName: r.ParentName,
			Content: r.Content, Images: parseImageURLs(r.Images), CreatedAt: formatISO(r.CreatedAt),
			Author: ForumAuthor{
				UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL,
			},
			CanDelete:  r.UserID == viewerID,
			LikesCount: r.LikesCount,
			IsAccepted: isAcc,
		})
	}
	// 批量回填当前用户是否已赞（计数已由 likes_count 列提供，单一 helper 收敛）
	s.enrichReplyLikedByMe(replyDTOs, viewerID)

	// 点赞状态（ADR-0018）：详情返回计数已由列提供，仅需回填是否已赞。
	topicDTO := row.toDTO(viewerID)
	s.enrichTopicLikedByMe([]*ForumTopicDTO{&topicDTO}, viewerID)
	if s.hasRewardIssued(topicID) {
		topicDTO.RewardIssued = true
	}

	return map[string]any{
		"topic":   topicDTO,
		"replies": replyDTOs,
	}, nil
}

// CreateTopicInput 发帖条件。Category 为空归一为 discussion（移动端旧契约不传）。
type CreateTopicInput struct {
	UserID    int
	ChapterID *int
	Category  string
	Title     string
	Content   string
	Images    []string
}

// CreateTopic 发帖。chapterID 为 nil/0 表示发到综合讨论区。
// images 为主题图片 URL 列表（最多 ForumTopicMaxImages 张，仅接受本站 images/forum/ 前缀）。
func (s *ForumService) CreateTopic(in CreateTopicInput) (*ForumTopicDTO, error) {
	category, err := normalizeForumCategory(in.Category)
	if err != nil {
		return nil, err
	}
	userID, chapterID, title, content, images := in.UserID, in.ChapterID, in.Title, in.Content, in.Images
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 100 {
		return nil, errors.New("标题长度需在 1-100 个字符之间")
	}
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 10000 {
		return nil, errors.New("内容长度需在 1-10000 个字符之间")
	}
	if err := validateForumImages(images, ForumTopicMaxImages); err != nil {
		return nil, err
	}

	// 非法组合在进库前拒绝：问答帖一律不属于任何章节。
	// 数据库层有同名 CHECK 作生产兜底（见迁移 000004），此处是能被契约测试守住的行为层。
	// 注意只判 >0：chapter_id 传 0 或不传按既有语义归一为综合区，不得在此收紧。
	if category == ForumCategoryQuestion && chapterID != nil && *chapterID > 0 {
		return nil, errors.New("问答帖不属于任何章节，不能指定 chapter_id")
	}

	var cid *int
	if chapterID != nil && *chapterID > 0 {
		var cnt int64
		if err := s.db.Model(&model.Chapter{}).Where("chapter_id = ?", *chapterID).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt == 0 {
			return nil, errors.New("章节不存在")
		}
		cid = chapterID
	}

	now := beijingNow()
	topic := model.ForumTopic{
		ChapterID: cid,
		// 显式写入归一后的非空类别，不依赖数据库 DEFAULT：
		// GORM 对带 default tag 的零值字段会跳过 INSERT，那样内存对象（下面的 DTO）会拿到空串。
		Category:  category,
		UserID:    userID,
		Title:     title,
		Content:   content,
		Images:    marshalImageURLs(images),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&topic).Error; err != nil {
		return nil, err
	}

	var u model.HrwaiUser
	if err := s.db.First(&u, userID).Error; err != nil {
		return nil, err
	}
	return &ForumTopicDTO{
		ID:        topic.ID,
		ChapterID: topic.ChapterID,
		Category:  topic.Category,
		Title:     topic.Title,
		Content:   topic.Content,
		Images:    images,
		CreatedAt: formatISO(topic.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// ReplyTopic 回复主题或回复某条回复（parentReplyID 非空时）。
// images 为回复图片 URL 列表（最多 ForumReplyMaxImages 张，仅接受本站 images/forum/ 前缀）。
func (s *ForumService) ReplyTopic(userID int, topicID int64, content string, parentReplyID *int64, images []string) (*ForumReplyDTO, error) {
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 5000 {
		return nil, errors.New("回复内容长度需在 1-5000 个字符之间")
	}
	if err := validateForumImages(images, ForumReplyMaxImages); err != nil {
		return nil, err
	}

	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("主题不存在")
		}
		return nil, err
	}

	// 校验被回复的回复存在且属于同一主题
	var parentName string
	var parentAuthorID int // 被回复人（楼中楼通知用）
	if parentReplyID != nil && *parentReplyID > 0 {
		var parent model.ForumReply
		if err := s.db.First(&parent, *parentReplyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("被回复的回复不存在")
			}
			return nil, err
		}
		if parent.TopicID != topicID {
			return nil, errors.New("被回复的回复不属于该主题")
		}
		parentAuthorID = parent.UserID
		var pu model.HrwaiUser
		if err := s.db.First(&pu, parent.UserID).Error; err == nil {
			parentName = ForumAuthor{
				UserID: pu.ID, Username: pu.Username, AvatarURL: pu.AvatarURL,
			}.DisplayName()
		}
	}

	// 回复人展示名（通知文案用；查询失败回退空串，不阻断回复）
	var replier model.HrwaiUser
	_ = s.db.Select("username").First(&replier, userID).Error
	replierName := replier.Username

	now := beijingNow()
	reply := model.ForumReply{
		TopicID:   topicID,
		UserID:    userID,
		ParentID:  parentReplyID,
		Content:   content,
		Images:    marshalImageURLs(images),
		CreatedAt: now,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&reply).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ForumTopic{}).Where("id = ?", topicID).
			Updates(map[string]any{
				"reply_count":   gorm.Expr("reply_count + 1"),
				"last_reply_at": now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		// 站内信通知（与回复同事务提交，避免通知丢失；与资料审核同模式）：
		// 1) 楼主被回复（回复人是楼主本人时不通知）
		// 2) 楼中楼被回复人（非自己、非楼主——楼主已由 1) 覆盖，避免重复通知）
		link := fmt.Sprintf("/training/forum/%d", topicID)
		payload := forumTopicPayload(topicID)
		if topic.UserID != userID {
			if err := s.notificationSvc.CreateWithTx(tx, topic.UserID, "forum_reply",
				"你的帖子有新回复",
				fmt.Sprintf("%s 回复了你的帖子「%s」", replierName, topic.Title),
				link, payload, now); err != nil {
				return err
			}
		}
		if parentAuthorID != 0 && parentAuthorID != userID && parentAuthorID != topic.UserID {
			if err := s.notificationSvc.CreateWithTx(tx, parentAuthorID, "forum_reply",
				"你的回复有新回复",
				fmt.Sprintf("%s 在帖子「%s」中回复了你", replierName, topic.Title),
				link, payload, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var u model.HrwaiUser
	if err := s.db.First(&u, userID).Error; err != nil {
		return nil, err
	}
	return &ForumReplyDTO{
		ID: reply.ID, TopicID: reply.TopicID, ParentID: reply.ParentID,
		ParentName: parentName, Content: reply.Content, Images: images, CreatedAt: formatISO(reply.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// DeleteTopic 删除主题（仅作者本人）。主题与全部回复（含子回复）的图片一并清理。
func (s *ForumService) DeleteTopic(userID int, topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("主题不存在")
		}
		return err
	}
	if topic.UserID != userID {
		return errors.New("只能删除自己发布的主题")
	}
	// 巡检计数：楼主删除自己已解决的帖子时累加（不回滚积分，仅计数）
	if topic.AcceptedReplyID != nil {
		_ = s.incrementDeletedAfterAccepted()
	}
	return s.deleteTopicWithImages(topicID)
}

// AdminDeleteTopic 管理员删除任意主题（不校验作者）。图片一并清理；站内信通知作者。
// 若该帖曾产生采纳分（accepted_bonus），则按 rollback 原因写对冲流水并扣减余额（封底 0，幂等）。
func (s *ForumService) AdminDeleteTopic(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("主题不存在")
		}
		return err
	}
	// 先收集图片（需在删除前读取）
	urls := []string{}
	// 复用 deleteTopicWithImages 的图片收集逻辑，但在此处先做以便事务外清理
	var rawTopic model.ForumTopic
	_ = s.db.First(&rawTopic, topicID).Error
	if rawTopic.ID != 0 {
		urls = append(urls, parseImageURLs(string(rawTopic.Images))...)
		var replyImages []string
		_ = s.db.Model(&model.ForumReply{}).Where("topic_id = ?", topicID).Pluck("images", &replyImages).Error
		for _, raw := range replyImages {
			urls = append(urls, parseImageURLs(raw)...)
		}
	}
	// 事务内：删帖 + 违规回收（复用封底 0 语义）
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.ForumTopic{}, topicID).Error; err != nil {
			return err
		}
		// 违规回收：若曾发放过 accepted_bonus 且未回滚，则扣回
		if topic.AcceptedReplyID != nil {
			if err := s.rollbackAcceptedBonusTx(tx, topicID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 清理文件（事务外，尽力而为）
	s.deleteImages(urls)
	// 通知作者（尽力而为：内容已删，通知失败不回滚，仅记日志）
	if err := s.notificationSvc.Create(topic.UserID, "forum_topic_deleted",
		"你的帖子已被删除",
		"管理员删除了你的帖子「"+topic.Title+"」。",
		"", nil); err != nil {
		s.logger.Warn("删帖通知发送失败", zap.Int64("topic_id", topicID), zap.Error(err))
	}
	return nil
}

// deleteTopicWithImages 删除主题前收集主题 + 全部回复（含子回复）的图片并清理存储。
func (s *ForumService) deleteTopicWithImages(topicID int64) error {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		return err
	}
	urls := parseImageURLs(string(topic.Images))
	var replyImages []string
	if err := s.db.Model(&model.ForumReply{}).
		Where("topic_id = ?", topicID).
		Pluck("images", &replyImages).Error; err != nil {
		return err
	}
	for _, raw := range replyImages {
		urls = append(urls, parseImageURLs(raw)...)
	}
	if err := s.db.Delete(&model.ForumTopic{}, topicID).Error; err != nil {
		return err
	}
	s.deleteImages(urls)
	return nil
}

// incrementDeletedAfterAccepted 楼主删除已解决帖的巡检计数 +1（存于 system_settings）。
func (s *ForumService) incrementDeletedAfterAccepted() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var setting model.SystemSetting
		err := tx.Where("key = ?", "deleted_after_accepted").First(&setting).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			setting = model.SystemSetting{Key: "deleted_after_accepted", Value: "1", Description: "删除已解决帖计数", UpdatedAt: clock.Now()}
			return tx.Create(&setting).Error
		}
		if err != nil {
			return err
		}
		// value 存为字符串整数
		v, _ := strconv.Atoi(setting.Value)
		v++
		return tx.Model(&model.SystemSetting{}).Where("key = ?", "deleted_after_accepted").Updates(map[string]any{"value": strconv.Itoa(v), "updated_at": clock.Now()}).Error
	})
}

// rollbackAcceptedBonusTx 在事务内执行违规回收：若该帖曾发放 accepted_bonus 且未回滚，则按
// rollback 原因对冲扣减答主余额（封底 0，幂等）。簿记经 PointsService 事务内通道（ADR-0023）：
// 占坑键 rollback:{topicID} 即「已处理」标记；余额不足按余额截断、余额为 0 时仅落占坑行
// （不再写 Delta:0 流水——points_ledger CHECK (delta <> 0)，#384 缺陷修复）。
func (s *ForumService) rollbackAcceptedBonusTx(tx *gorm.DB, topicID int64) error {
	// 幂等：已存在 rollback 流水（存量数据标记）则跳过；占坑行接管后续幂等
	var existed int64
	if err := tx.Model(&model.PointsLedger{}).Where("reason = ? AND ref_type = ? AND ref_id = ?", ReasonRollback, "forum_topic", fmt.Sprintf("%d", topicID)).Count(&existed).Error; err != nil {
		return err
	}
	if existed > 0 {
		return nil
	}
	// 查找原发放流水（取最近一条）
	var orig model.PointsLedger
	if err := tx.Where("reason = ? AND ref_type = ? AND ref_id = ?", ReasonAcceptedBonus, "forum_topic", fmt.Sprintf("%d", topicID)).Order("created_at DESC").First(&orig).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if orig.Delta <= 0 {
		return nil
	}
	return s.points.SettleRewardTx(tx, PointsEntry{
		UserID: orig.UserID, Delta: -orig.Delta, Reason: ReasonRollback,
		RefType: "forum_topic", RefID: fmt.Sprintf("%d", topicID),
		IdemKey:   "rollback:" + fmt.Sprintf("%d", topicID),
		FloorZero: true,
	})
}

// DeleteReply 删除回复（仅作者本人；其下级回复随外键级联删除）。
// 本回复与全部下级回复（parent_id 链条）的图片一并清理。
func (s *ForumService) DeleteReply(userID int, replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("回复不存在")
		}
		return err
	}
	if reply.UserID != userID {
		return errors.New("只能删除自己发布的回复")
	}
	return s.deleteReplyWithImages(replyID, reply.TopicID)
}

// AdminDeleteReply 管理员删除任意回复（不校验作者；其下级回复随外键级联删除）。图片一并清理；站内信通知回复作者。
func (s *ForumService) AdminDeleteReply(replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("回复不存在")
		}
		return err
	}
	var topic model.ForumTopic
	if err := s.db.First(&topic, reply.TopicID).Error; err != nil {
		topic.Title = ""
	}
	topicTitle := topic.Title
	// 若该回复是被采纳的回答，则违规回收（幂等，复用同一 rollback 键）
	needsRollback := topic.AcceptedReplyID != nil && *topic.AcceptedReplyID == replyID
	if needsRollback {
		// 在事务外先尝试回收（幂等），失败仅记日志，不阻断删回复
		_ = s.db.Transaction(func(tx *gorm.DB) error {
			return s.rollbackAcceptedBonusTx(tx, topic.ID)
		})
	}
	if err := s.deleteReplyWithImages(replyID, reply.TopicID); err != nil {
		return err
	}
	// 通知回复作者（尽力而为：内容已删，通知失败不回滚，仅记日志）
	if err := s.notificationSvc.Create(reply.UserID, "forum_reply_deleted",
		"你的回复已被删除",
		"管理员删除了你在帖子「"+topicTitle+"」中的回复。",
		fmt.Sprintf("/training/forum/%d", reply.TopicID),
		forumTopicPayload(reply.TopicID)); err != nil {
		s.logger.Warn("删回复通知发送失败", zap.Int64("reply_id", replyID), zap.Error(err))
	}
	return nil
}

// deleteReplyWithImages 删除回复前收集本回复 + 全部下级回复的图片并清理存储。
// 下级回复通过 parent_id 递归收集（单表递归 CTE 或逐层查询）。
func (s *ForumService) deleteReplyWithImages(replyID, topicID int64) error {
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
func (s *ForumService) collectReplyImages(replyID int64) ([]string, error) {
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
func (s *ForumService) deleteImages(urls []string) {
	if s.fileSvc == nil || len(urls) == 0 {
		return
	}
	s.fileSvc.DeleteFiles(urls)
}

// deleteReplyByID 删除回复并回扣主题回复数、刷新最后回复时间。
// 回扣量取回复子树大小 N（parent_id 链，含自身）：外键 ON DELETE CASCADE 会连带删除全部下级回复，
// 固定 -1 会让楼中楼场景计数虚高（spec #297 级联少减修复）。
func (s *ForumService) deleteReplyByID(replyID, topicID int64) error {
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

// countReplySubtree 统计回复子树大小（parent_id 链，含自身），递归 CTE 双方言兼容（PG/SQLite）。
func countReplySubtree(exec *gorm.DB, replyID int64) (int64, error) {
	var n int64
	err := exec.Raw(`WITH RECURSIVE subtree AS (
    SELECT id FROM forum_replies WHERE id = ?
    UNION ALL
    SELECT r.id FROM forum_replies r JOIN subtree st ON r.parent_id = st.id
)
SELECT COUNT(*) FROM subtree`, replyID).Scan(&n).Error
	return n, err
}

// ===== 图片工具 =====

// validateForumImages 校验图片 URL 列表：数量上限 + 来源（仅接受本站 images/forum/ 前缀）。
// 允许 local（/static/uploads/images/forum/...）与 R2（https://.../images/forum/...）两种形式。
func validateForumImages(images []string, max int) error {
	if len(images) == 0 {
		return nil
	}
	if len(images) > max {
		return errors.New("图片数量超出限制（最多 " + strconv.Itoa(max) + " 张）")
	}
	for _, u := range images {
		if !isForumImageURL(u) {
			return errors.New("图片地址无效（仅支持本站上传的论坛图片）")
		}
	}
	return nil
}

// parseImageURLs 将 JSONB 图片数组字符串解析为 URL 列表（无效 JSON 返回空列表）。
func parseImageURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var urls []string
	if err := json.Unmarshal([]byte(raw), &urls); err != nil {
		return nil
	}
	return urls
}

// marshalImageURLs 将 URL 列表序列化为 JSONB 字节。
func marshalImageURLs(urls []string) model.JSONB {
	if len(urls) == 0 {
		return model.JSONB([]byte("[]"))
	}
	b, _ := json.Marshal(urls)
	return model.JSONB(b)
}

// ===== 论坛互动（ADR-0018：点赞 / 举报 / 我的帖子 / 我的回复）=====

// LikeTopic 点赞主题（幂等：重复点赞不报错、不重复计数；事务内同步维护 likes_count）。
func (s *ForumService) LikeTopic(userID int, topicID int64) (int64, error) {
	var cnt int64
	if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, errors.New("主题不存在")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.ForumTopicLike
		if err := tx.Where("topic_id = ? AND user_id = ?", topicID, userID).Limit(1).Find(&existing).Error; err != nil {
			return err
		}
		if existing.ID != 0 {
			return nil
		}
		if err := tx.Create(&model.ForumTopicLike{TopicID: topicID, UserID: userID, CreatedAt: beijingNow()}).Error; err != nil {
			if isDuplicateError(err) {
				return nil
			}
			return err
		}
		return s.counters.AdjustLikes(tx, topicID, 1)
	})
	if err != nil {
		return 0, err
	}
	return s.topicLikesCount(topicID), nil
}

// UnlikeTopic 取消点赞（幂等：未点赞时直接返回当前计数；事务内同步维护 likes_count）。
func (s *ForumService) UnlikeTopic(userID int, topicID int64) (int64, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("topic_id = ? AND user_id = ?", topicID, userID).Delete(&model.ForumTopicLike{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return s.counters.AdjustLikes(tx, topicID, -1)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return s.topicLikesCount(topicID), nil
}

// topicLikesCount 主题点赞数（读侧只认 likes_count 列为事实源，ADR-0018）。
func (s *ForumService) topicLikesCount(topicID int64) int64 {
	var n int64
	_ = s.db.Model(&model.ForumTopic{}).Select("likes_count").Where("id = ?", topicID).Scan(&n).Error
	return n
}

// enrichTopicLikedByMe 批量回填主题是否已赞（计数已由 likes_count 列提供，LikedByMe 单一 helper 收敛）。
func (s *ForumService) enrichTopicLikedByMe(topics []*ForumTopicDTO, viewerID int) {
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

// toDTORefs 将 ForumTopicDTO 值切片转为指针切片，供 enrich helpers 修改原切片元素。
func toDTORefs(items []ForumTopicDTO) []*ForumTopicDTO {
	refs := make([]*ForumTopicDTO, len(items))
	for i := range items {
		refs[i] = &items[i]
	}
	return refs
}

// enrichReplyLikedByMe 批量回填回复是否已赞（计数已由 likes_count 列提供）。
func (s *ForumService) enrichReplyLikedByMe(replies []ForumReplyDTO, viewerID int) {
	if len(replies) == 0 || viewerID <= 0 {
		return
	}
	ids := make([]int64, 0, len(replies))
	for _, r := range replies {
		ids = append(ids, r.ID)
	}
	var liked []int64
	if err := s.db.Model(&model.ForumReplyLike{}).Where("user_id = ? AND reply_id IN ?", viewerID, ids).Pluck("reply_id", &liked).Error; err != nil {
		return
	}
	lm := make(map[int64]bool, len(liked))
	for _, id := range liked {
		lm[id] = true
	}
	for i := range replies {
		replies[i].LikedByMe = lm[replies[i].ID]
	}
}

// CreateReport 举报主题或回复（topicID/replyID 二选一，由调用方保证）。
func (s *ForumService) CreateReport(userID int, topicID, replyID *int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return errors.New("举报理由长度需在 1-500 个字符之间")
	}
	if (topicID == nil) == (replyID == nil) {
		return errors.New("举报对象必须为主题或回复之一")
	}
	if topicID != nil {
		var cnt int64
		s.db.Model(&model.ForumTopic{}).Where("id = ?", *topicID).Count(&cnt)
		if cnt == 0 {
			return errors.New("主题不存在")
		}
	}
	if replyID != nil {
		var cnt int64
		s.db.Model(&model.ForumReply{}).Where("id = ?", *replyID).Count(&cnt)
		if cnt == 0 {
			return errors.New("回复不存在")
		}
	}
	return s.db.Create(&model.ForumReport{
		ReporterID: userID, TopicID: topicID, ReplyID: replyID,
		Reason: reason, Status: 0, CreatedAt: beijingNow(),
	}).Error
}

// ForumReportDTO 管理端举报条目。
type ForumReportDTO struct {
	ID         int64  `json:"id"`
	ReporterID int    `json:"reporter_id"`
	Reporter   string `json:"reporter"`
	TopicID    *int64 `json:"topic_id,omitempty"`
	TopicTitle string `json:"topic_title"`
	ReplyID    *int64 `json:"reply_id,omitempty"`
	Reason     string `json:"reason"`
	Status     int16  `json:"status"`
	CreatedAt  string `json:"created_at"`
}

// ForumReportPageResult 举报分页结果。
type ForumReportPageResult struct {
	Page    int              `json:"page"`
	Pages   int              `json:"pages"`
	Total   int64            `json:"total"`
	Reports []ForumReportDTO `json:"reports"`
}

// ListReports 管理端举报列表（status: nil 全部 / 0 待处理 / 1 已处理）。
func (s *ForumService) ListReports(page, pageSize int, status *int16) (*ForumReportPageResult, error) {
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
	rows, total, page, pageSize := paging.QueryWithScan[reportRow](s.db, page, pageSize, 20, 100,
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
func (s *ForumService) HandleReport(reportID int64, status int16) error {
	if status != 0 && status != 1 {
		return errors.New("状态仅支持 0（待处理）/ 1（已处理）")
	}
	var report model.ForumReport
	if err := s.db.First(&report, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("举报不存在")
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
// 主题已删时降级文案（不带标题与链接）。
func (s *ForumService) notifyReportHandled(report *model.ForumReport) {
	target := "帖子"
	if report.ReplyID != nil {
		target = "回复"
	}
	content := "你举报的" + target + "已处理完毕。"
	link := ""
	var payload model.JSONB
	if report.TopicID != nil {
		var topic model.ForumTopic
		if err := s.db.Select("title").First(&topic, *report.TopicID).Error; err == nil {
			content = fmt.Sprintf("你举报的%s「%s」已处理完毕。", target, topic.Title)
			link = fmt.Sprintf("/training/forum/%d", *report.TopicID)
			payload = forumTopicPayload(*report.TopicID)
		}
	}
	if err := s.notificationSvc.Create(report.ReporterID, "forum_report", "举报已处理", content, link, payload); err != nil {
		s.logger.Warn("举报处理通知发送失败", zap.Int64("report_id", report.ID), zap.Error(err))
	}
}

// MyTopics 我的帖子（复用主题列表行装配，按最后活跃倒序）。
func (s *ForumService) MyTopics(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	rows, total, page, pageSize := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"COALESCE(t.last_reply_at, t.created_at) DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_topics AS t").
				Select("t.id, t.chapter_id, t.category, t.title, t.content, t.images, t.view_count, t.reply_count, t.likes_count, t.accepted_reply_id, t.solved_at, t.last_reply_at, t.created_at, "+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(ch.title, '') AS chapter_title").
				Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
				Where("t.user_id = ?", userID)
		})
	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(userID))
	}
	// 点赞计数已由 likes_count 列提供，仅需回填是否已赞（单一 helper 收敛）
	s.enrichTopicLikedByMe(toDTORefs(items), userID)
	s.enrichRewardIssued(items)
	return &ForumTopicPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Topics: items, Total: total,
	}, nil
}

// MyReplyDTO 我的回复条目（带主题标题回填）。
type MyReplyDTO struct {
	ID         int64       `json:"id"`
	TopicID    int64       `json:"topic_id"`
	TopicTitle string      `json:"topic_title"`
	ParentID   *int64      `json:"parent_id,omitempty"`
	Content    string      `json:"content"`
	Images     []string    `json:"images"`
	CreatedAt  string      `json:"created_at"`
	Author     ForumAuthor `json:"author"`
}

// MyReplyPageResult 我的回复分页结果。
type MyReplyPageResult struct {
	Page    int          `json:"page"`
	Pages   int          `json:"pages"`
	Total   int64        `json:"total"`
	Replies []MyReplyDTO `json:"replies"`
}

// MyReplies 我的回复（主题被删时标题为空串，条目保留）。
func (s *ForumService) MyReplies(userID, page, pageSize int) (*MyReplyPageResult, error) {
	type myReplyRow struct {
		ID         int64
		TopicID    int64
		TopicTitle string
		ParentID   *int64
		Content    string
		Images     string
		CreatedAt  time.Time
		UserID     int
		Username   string
		AvatarURL  string
	}
	rows, total, page, pageSize := paging.QueryWithScan[myReplyRow](s.db, page, pageSize, 10, 100,
		"r.created_at DESC, r.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_replies AS r").
				Select("r.id, r.topic_id, r.parent_id, r.content, r.images, r.created_at, "+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(t.title, '') AS topic_title").
				Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
				Joins("LEFT JOIN forum_topics AS t ON t.id = r.topic_id").
				Where("r.user_id = ?", userID)
		})
	items := make([]MyReplyDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, MyReplyDTO{
			ID: r.ID, TopicID: r.TopicID, TopicTitle: r.TopicTitle, ParentID: r.ParentID,
			Content: r.Content, Images: parseImageURLs(r.Images), CreatedAt: formatISO(r.CreatedAt),
			Author: ForumAuthor{UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL},
		})
	}
	return &MyReplyPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Replies: items,
	}, nil
}

// LikeReply 点赞评论（幂等；事务内同步维护 likes_count）。
func (s *ForumService) LikeReply(userID int, replyID int64) (int64, error) {
	var cnt int64
	if err := s.db.Model(&model.ForumReply{}).Where("id = ?", replyID).Count(&cnt).Error; err != nil {
		return 0, err
	}
	if cnt == 0 {
		return 0, errors.New("回复不存在")
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.ForumReplyLike
		if err := tx.Where("reply_id = ? AND user_id = ?", replyID, userID).Limit(1).Find(&existing).Error; err != nil {
			return err
		}
		if existing.ID != 0 {
			return nil
		}
		if err := tx.Create(&model.ForumReplyLike{ReplyID: replyID, UserID: userID, CreatedAt: beijingNow()}).Error; err != nil {
			if isDuplicateError(err) {
				return nil
			}
			return err
		}
		return s.counters.AdjustReplyLikes(tx, replyID, 1)
	})
	if err != nil {
		return 0, err
	}
	return s.replyLikesCount(replyID), nil
}

// UnlikeReply 取消点赞评论（幂等；事务内同步维护 likes_count）。
func (s *ForumService) UnlikeReply(userID int, replyID int64) (int64, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("reply_id = ? AND user_id = ?", replyID, userID).Delete(&model.ForumReplyLike{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return s.counters.AdjustReplyLikes(tx, replyID, -1)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return s.replyLikesCount(replyID), nil
}

// replyLikesCount 回复点赞数（读侧只认 likes_count 列为事实源）。
func (s *ForumService) replyLikesCount(replyID int64) int64 {
	var n int64
	_ = s.db.Model(&model.ForumReply{}).Select("likes_count").Where("id = ?", replyID).Scan(&n).Error
	return n
}

// ===== 采纳状态机与积分直记（#366）=====

// AcceptResult 采纳操作结果（复用主题 DTO，便于直接回显状态）。
type AcceptResult struct {
	Topic *ForumTopicDTO `json:"topic"`
}

// AcceptReply 楼主采纳一条回复。
//
// 幂等：重复提交同一 replyID 不再发分；并发采纳靠 CAS 保证只发一次分；
// 更换采纳对象时只改状态不新增流水；取消后重采同样只发一次（以流水是否存在判定）。
func (s *ForumService) AcceptReply(userID int, topicID, replyID int64) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("主题不存在")
		}
		return nil, err
	}
	if topic.UserID != userID {
		return nil, ErrNotTopicOwner
	}
	if topic.Category != ForumCategoryQuestion {
		return nil, errors.New("只有问答帖可采纳回答")
	}
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("回复不存在")
		}
		return nil, err
	}
	if reply.TopicID != topicID {
		return nil, errors.New("回复不属于该主题")
	}
	// 同一回复重复采纳：幂等直接返回当前状态
	if topic.AcceptedReplyID != nil && *topic.AcceptedReplyID == replyID {
		return s.fetchTopicDTO(topicID, userID)
	}
	// 已有采纳，视为更换：只改状态不发分
	if topic.AcceptedReplyID != nil {
		now := beijingNow()
		if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).Updates(map[string]any{
			"accepted_reply_id": replyID,
			"solved_at":         now,
			"updated_at":        now,
		}).Error; err != nil {
			return nil, err
		}
		return s.fetchTopicDTO(topicID, userID)
	}
	// 首次采纳：CAS + 积分直记（同一事务）
	isSelf := reply.UserID == userID
	now := beijingNow()
	todayStart := clock.DayStart(now)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// CAS：仅当仍未采纳时才写入状态
		res := tx.Model(&model.ForumTopic{}).Where("id = ? AND accepted_reply_id IS NULL", topicID).Updates(map[string]any{
			"accepted_reply_id": replyID,
			"solved_at":         now,
			"updated_at":        now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// 并发抢采：已由先胜者写入，放弃发分
			return nil
		}
		// 已采纳过是否已发过分（取消后重采场景）：以流水是否存在判定，每帖只发一次
		var cnt int64
		if err := tx.Model(&model.PointsLedger{}).Where("ref_type = ? AND ref_id = ? AND reason = ?", "forum_topic", fmt.Sprintf("%d", topicID), ReasonAcceptedBonus).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			// 已发过分（取消后重采），只保留状态迁移
			return nil
		}
		if isSelf {
			// 楼主采纳自己：状态已迁移，发分为 0
			return nil
		}
		// ===== 乙档防刷：日封顶 + 配对衰减（零新表，事务内算完） =====
		var dailyAnsCnt int64
		if err := tx.Model(&model.PointsLedger{}).Where("user_id = ? AND reason = ? AND created_at >= ?", reply.UserID, ReasonAcceptedBonus, todayStart).Count(&dailyAnsCnt).Error; err != nil {
			return err
		}
		var dailyAskerCnt int64
		if err := tx.Model(&model.PointsLedger{}).Where("user_id = ? AND reason = ? AND created_at >= ?", userID, ReasonAcceptAction, todayStart).Count(&dailyAskerCnt).Error; err != nil {
			return err
		}
		var pairCnt int64
		if err := tx.Raw("SELECT COUNT(*) FROM forum_topics WHERE user_id = ? AND accepted_reply_id IN (SELECT id FROM forum_replies WHERE user_id = ?)", userID, reply.UserID).Scan(&pairCnt).Error; err != nil {
			return err
		}
		bonusDelta := AcceptBonusPoints
		actionDelta := AcceptActionPoints
		if pairCnt >= 6 {
			bonusDelta = 0
		} else if pairCnt >= 4 {
			bonusDelta = bonusDelta / 2
		}
		if dailyAnsCnt >= 3 {
			bonusDelta = 0
		}
		if dailyAskerCnt >= 5 {
			actionDelta = 0
		}
		if bonusDelta == 0 && actionDelta == 0 {
			return nil
		}
		if bonusDelta > 0 {
			// 簿记经 PointsService 事务内通道（ADR-0023）：占坑键 accepted_bonus:{topicID}
			// 与状态 CAS 双保险「每帖只发一次」
			if err := s.points.SettleRewardTx(tx, PointsEntry{
				UserID: reply.UserID, Delta: bonusDelta, Reason: ReasonAcceptedBonus,
				RefType: "forum_topic", RefID: fmt.Sprintf("%d", topicID),
				IdemKey: "accepted_bonus:" + fmt.Sprintf("%d", topicID),
			}); err != nil {
				return err
			}
		}
		if actionDelta > 0 {
			if err := s.points.SettleRewardTx(tx, PointsEntry{
				UserID: userID, Delta: actionDelta, Reason: ReasonAcceptAction,
				RefType: "forum_topic", RefID: fmt.Sprintf("%d", topicID),
				IdemKey: "accept_action:" + fmt.Sprintf("%d", topicID),
			}); err != nil {
				return err
			}
		}
		// 站内信：答主/楼主（#369），payload 带实际分值，link 锚到回答（ADR-0024 C3 事件构造器单点构造）
		if bonusDelta > 0 || actionDelta > 0 {
			if bonusDelta > 0 {
				if err := s.notificationSvc.CreateForumAcceptEvent(tx, NewAnswererAcceptEvent(reply.UserID, topic.Title, topicID, replyID, bonusDelta), now); err != nil {
					return err
				}
			}
			if actionDelta > 0 {
				if err := s.notificationSvc.CreateForumAcceptEvent(tx, NewOwnerAcceptEvent(userID, topic.Title, topicID, replyID, actionDelta), now); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.fetchTopicDTO(topicID, userID)
}

// CancelAccept 楼主取消采纳（状态回到未解决，已发分不回滚）。
func (s *ForumService) CancelAccept(userID int, topicID int64) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("主题不存在")
		}
		return nil, err
	}
	if topic.UserID != userID {
		return nil, ErrNotTopicOwner
	}
	if topic.Category != ForumCategoryQuestion {
		return nil, errors.New("只有问答帖可取消采纳")
	}
	if topic.AcceptedReplyID == nil {
		return s.fetchTopicDTO(topicID, userID)
	}
	now := beijingNow()
	if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).Updates(map[string]any{
		"accepted_reply_id": nil,
		"solved_at":         nil,
		"updated_at":        now,
	}).Error; err != nil {
		return nil, err
	}
	return s.fetchTopicDTO(topicID, userID)
}

// fetchTopicDTO 查询主题 DTO（用于采纳后回显，复用 topicRow 装配，不累浏览量）。
func (s *ForumService) fetchTopicDTO(topicID int64, viewerID int) (*ForumTopicDTO, error) {
	var row topicRow
	err := s.db.Table("forum_topics AS t").
		Select("t.id, t.chapter_id, t.category, t.title, t.content, t.images, t.view_count, t.reply_count, t.likes_count, t.accepted_reply_id, t.solved_at, t.last_reply_at, t.created_at, "+
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

// enrichRewardIssued 批量回填 reward_issued（#367 二次确认分支）。
// 仅查询 question 类帖且 reason=accepted_bonus 的流水，避免无谓扫描。
func (s *ForumService) enrichRewardIssued(items []ForumTopicDTO) {
	if len(items) == 0 {
		return
	}
	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, t := range items {
		if t.Category != ForumCategoryQuestion {
			continue
		}
		sid := fmt.Sprintf("%d", t.ID)
		if _, ok := seen[sid]; !ok {
			seen[sid] = struct{}{}
			ids = append(ids, sid)
		}
	}
	if len(ids) == 0 {
		return
	}
	var issued []string
	if err := s.db.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND reason = ? AND ref_id IN ?", "forum_topic", ReasonAcceptedBonus, ids).
		Distinct("ref_id").Pluck("ref_id", &issued).Error; err != nil {
		return
	}
	m := make(map[string]bool, len(issued))
	for _, id := range issued {
		m[id] = true
	}
	for i := range items {
		if m[fmt.Sprintf("%d", items[i].ID)] {
			items[i].RewardIssued = true
		}
	}
}

// hasRewardIssued 单条查询：该帖是否已产生过 accepted_bonus 流水。
func (s *ForumService) hasRewardIssued(topicID int64) bool {
	var cnt int64
	if err := s.db.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND reason = ? AND ref_id = ?", "forum_topic", ReasonAcceptedBonus, fmt.Sprintf("%d", topicID)).
		Count(&cnt).Error; err != nil {
		return false
	}
	return cnt > 0
}
