// Package service 实现业务服务层。
// 本文件：学员端论坛（综合讨论区 + 章节讨论区，支持回复别人的回复，图文分离发图）——
// 学员交互 + 个人集合。管理端治理动作（举报处置 / 意图认定 / 强删与违规回收）在
// forum_moderation_service.go；共享依赖与私有 helper 在 forum_core.go（ADR-0050 决策 3）。
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
	"forklift-training/internal/geolocation"
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

// 论坛帖子类别常量（#364；ADR-0040 起收窄回两值，只表达作者意图）。
//
// 类别判"帖子意图"，scope/chapter_id 判"内容坐标"，两者正交但有一格非法：
// discussion+NULL=综合讨论区、discussion+N=章节讨论区、question+NULL=全局问答、
// question+N=非法。experience（#706 备考经验）可挂章节也可不挂，但不可被采纳；
// 求职信息仍是常驻实体，不在论坛内，意图值域到此为止。
//
// 「备考经验」**不是意图**（ADR-0040）：它是管理端认定（forum_topics.is_experience），
// 学员不能自述。判「是不是经验帖」看 is_experience，不要再读 category == 'experience'。
const (
	ForumCategoryDiscussion = "discussion" // 讨论帖（存量帖子的默认值）
	ForumCategoryQuestion   = "question"   // 问答帖（可被采纳，走积分直记）
	// ForumCategoryExperience 是**只读的历史值**（ADR-0040）：写入路径已不再接受（见 normalizeForumCategory），
	// 仅剩读侧消费——列表筛选 parseForumCategoryArg 过滤存量行，与待退役的 growth_first_experience 判定。
	ForumCategoryExperience = "experience"
)

// 论坛正文格式常量（ADR-0044）。
//
// 格式是**作者自述的声明位**，不是系统猜测：同一段文字按 text 与按 markdown 渲染结果不同
// （例如「1. 检查电瓶」在 markdown 下会变成有序列表），所以必须由作者决定，
// 而不是由服务端根据内容「看起来像不像 markdown」来推断。
const (
	ForumContentFormatText     = "text"     // 纯文本：保留换行，不做任何语法解释
	ForumContentFormatMarkdown = "markdown" // 受限 Markdown 子集
)

// normalizeContentFormat 校验并归一**正文格式声明**：空串归一为 text（向后兼容——
// 移动端与存量客户端不带该字段），其余必须在两值域内。
//
// 非法值返回错误而**不是**静默归一：静默回退会让客户端以为自己设置生效了，
// 直到用户发现排版没出来。口径与 normalizeForumCategory 一致（TrimSpace + 大小写敏感）。
func normalizeContentFormat(format string) (string, error) {
	switch format = strings.TrimSpace(format); format {
	case "":
		return ForumContentFormatText, nil
	case ForumContentFormatText, ForumContentFormatMarkdown:
		return format, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrContentFormatInvalid, format)
	}
}

// normalizeForumCategory 校验并归一**意图**：空串归一为 discussion（向后兼容，移动端不传）。
// 非空且不在两值域内返回错误——`experience` 在此被拒（ADR-0040：自称不产生事实，经验由管理端认定）。
// 归一后再落到模型上，避免依赖数据库 DEFAULT
// （GORM 对带 default tag 的零值字段会跳过 INSERT，内存对象拿不到回填值）。
func normalizeForumCategory(category string) (string, error) {
	switch category = strings.TrimSpace(category); category {
	case "":
		return ForumCategoryDiscussion, nil
	case ForumCategoryDiscussion, ForumCategoryQuestion:
		return category, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrCategoryInvalid, category)
	}
}

// 采纳积分常量（#366）：每帖只发一次分，走流水直记（非任务制）。
// ReasonRollback（违规回收流水原因）已随回收实现收编移入积分域（#609，points_service.go）。
const (
	AcceptBonusPoints   = 40               // 答主采纳奖励
	AcceptActionPoints  = 5                // 楼主采纳行为奖励
	ReasonAcceptedBonus = "accepted_bonus" // 流水原因：被采纳奖励
	ReasonAcceptAction  = "accept_action"  // 流水原因：采纳行为奖励
)

// 加精奖励常量（#742）：管理端加精一次性直记给帖主，每帖幂等一次
// （取消重精不重复发分，以流水存在判定，与 accepted_bonus 同模式）。
const (
	FeaturedBonusPoints = 30               // 加精奖励
	ReasonFeaturedBonus = "featured_bonus" // 流水原因：帖子被加精
)

// 论坛错误哨兵族（第十二波票 5，#1168）：同一事实一个哨兵，api 侧 forumErrStatus 域表按档渲染——
// 存在性→404、所有权→403、状态前置/校验→400；未命中域表的 error 一律 DB/未知故障 → 500。
// 沿用积分域哨兵纪律（ADR-0024 / #611 形态）：handler 以 errors.Is 映射，不做 err.Error() 字符串比对。

// —— 存在性一族（→404）——

// ErrTopicNotFound 主题不存在（#811 收敛为哨兵：handler 以 errors.Is 映射 404，
// 不做 err.Error() 字符串比对——沿用积分域哨兵纪律，CONTEXT.md「积分错误哨兵」同精神）。
var ErrTopicNotFound = errors.New("主题不存在")

// ErrReplyNotFound 回复不存在（票 5 前是五个读点各写的裸「回复不存在」×5 + 「被回复的回复不存在」，同一事实）。
var ErrReplyNotFound = errors.New("回复不存在")

// ErrForumReportNotFound 论坛举报记录不存在（名带 Forum 前缀避开求职举报域既有 ErrReportNotFound 的包级撞名）。
var ErrForumReportNotFound = errors.New("举报不存在")

// ErrChapterNotFound 发帖/筛选指向的章节不存在。
var ErrChapterNotFound = errors.New("章节不存在")

// —— 所有权（→403）——

// ErrNotTopicOwner 只有楼主可采纳/取消/更换。
var ErrNotTopicOwner = errors.New("只有楼主可以执行此操作")

// ErrNotTopicAuthor 主题删除仅限作者本人。
var ErrNotTopicAuthor = errors.New("只能删除自己发布的主题")

// ErrNotReplyAuthor 回复删除仅限作者本人。
var ErrNotReplyAuthor = errors.New("只能删除自己发布的回复")

// —— 状态前置（→400）——

// ErrAcceptOwnReply 楼主不能采纳自己的回答（自问自答禁止，ADR-0028）。
var ErrAcceptOwnReply = errors.New("不能采纳自己的回答")

// ErrAcceptNotQuestion 采纳动作只适用于问答帖。
var ErrAcceptNotQuestion = errors.New("只有问答帖可采纳回答")

// ErrCancelAcceptNotQuestion 取消采纳只适用于问答帖。
var ErrCancelAcceptNotQuestion = errors.New("只有问答帖可取消采纳")

// ErrAcceptExperienceTopic 经验帖不可被采纳（ADR-0040，与认定侧守卫互为镜像）。
var ErrAcceptExperienceTopic = errors.New("备考经验帖不可被采纳，请先取消经验认定")

// ErrDesignateAcceptedTopic 已采纳帖不可认定为经验（ADR-0040）。
var ErrDesignateAcceptedTopic = errors.New("已采纳的帖子不可认定为备考经验，请先取消采纳")

// ErrUnfeatureExperienceTopic 撤精须先取消经验认定（经验蕴含精选，ADR-0040）。
var ErrUnfeatureExperienceTopic = errors.New("备考经验帖蕴含精选位，请先取消经验认定")

// ErrCategoryLockedByAccept 已采纳问答帖改类别前须先取消采纳（#811）。
var ErrCategoryLockedByAccept = errors.New("已采纳的问答帖不能改类别，请先取消采纳")

// ErrQuestionChapterConflict 问答帖不得挂章节。
var ErrQuestionChapterConflict = errors.New("问答帖不属于任何章节，不能指定 chapter_id")

// ErrParentReplyMismatch 被回复的回复挂在别的主题下。
var ErrParentReplyMismatch = errors.New("被回复的回复不属于该主题")

// ErrReplyTopicMismatch 被采纳的回复挂在别的主题下。
var ErrReplyTopicMismatch = errors.New("回复不属于该主题")

// —— 参数/校验（→400，动态详情以 %w 包装哨兵，文案逐字保持）——

var (
	ErrContentFormatInvalid = errors.New("正文格式无效")
	ErrCategoryInvalid      = errors.New("帖子类别无效")
	ErrSolvedArgInvalid     = errors.New("solved 参数无效")
	ErrFeaturedArgInvalid   = errors.New("featured 参数无效")
	ErrExperienceArgInvalid = errors.New("is_experience 参数无效")
	ErrSolvedFilterScope    = errors.New("solved 筛选仅对问答帖有意义，请同时指定 category=question")
	ErrChapterIDRequired    = errors.New("查询章节讨论区需要有效的 chapter_id")
	ErrTitleLength          = errors.New("标题长度需在 1-100 个字符之间")
	ErrContentLength        = errors.New("内容长度需在 1-10000 个字符之间")
	ErrReplyContentLength   = errors.New("回复内容长度需在 1-5000 个字符之间")
	ErrImagesTooMany        = errors.New("图片数量超出限制")
	ErrImageURLInvalid      = errors.New("图片地址无效（仅支持本站上传的论坛图片）")
	ErrReportReasonLength   = errors.New("举报理由长度需在 1-500 个字符之间")
	ErrReportTarget         = errors.New("举报对象必须为主题或回复之一")
	ErrReportStatusValue    = errors.New("状态仅支持 0（待处理）/ 1（已处理）")
)

// 论坛发图限制。
const (
	ForumTopicMaxImages = 9 // 主题最多图片数
	ForumReplyMaxImages = 3 // 回复最多图片数
)

// 详情页回复分页（ADR-0042）：回复列表的唯一读取形态是分页，旧的「一次性全量返回」已退役。
const (
	ForumReplyDefaultPageSize = 20  // 默认每页回复数
	ForumReplyMaxPageSize     = 100 // 页大小上限（超上限回退默认值，与全仓 ClampMax 同口径）
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
	ID           int64  `json:"id"`
	ChapterID    *int   `json:"chapter_id" extensions:"x-nullable"`
	Category     string `json:"category"` // 意图：discussion | question（ADR-0040）
	ChapterTitle string `json:"chapter_title"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	// ContentFormat 正文格式声明（ADR-0044）：text | markdown。前端据此选渲染方式。
	ContentFormat string   `json:"content_format"`
	Images        []string `json:"images"`
	// IPProvince / IPCity 发布那一刻的属地快照（ADR-0045）。空串 = 无属地，
	// 展示侧据此**整段不渲染**（不显示「未知」、不留占位）。
	// 位置在作者行：它是「这条帖子的作者当时在哪」，不是用户资料。
	IPProvince      string      `json:"ip_province"`
	IPCity          string      `json:"ip_city"`
	ViewCount       int         `json:"view_count"`
	ReplyCount      int         `json:"reply_count"`
	LastReplyAt     *string     `json:"last_reply_at" extensions:"x-nullable"`
	CreatedAt       string      `json:"created_at"`
	Author          ForumAuthor `json:"author"`
	CanDelete       bool        `json:"can_delete"`
	LikesCount      int64       `json:"likes_count"`
	LikedByMe       bool        `json:"liked_by_me"`
	AcceptedReplyID *int64      `json:"accepted_reply_id,omitempty" extensions:"x-optional"`
	SolvedAt        *string     `json:"solved_at,omitempty" extensions:"x-optional"`
	IsFeatured      bool        `json:"is_featured"`   // 认定：精选位
	IsExperience    bool        `json:"is_experience"` // 认定：备考经验（蕴含 is_featured）
	RewardIssued    bool        `json:"reward_issued"`
}

// ForumReplyDTO 论坛回复对象。
type ForumReplyDTO struct {
	ID         int64  `json:"id"`
	TopicID    int64  `json:"topic_id"`
	ParentID   *int64 `json:"parent_id,omitempty" extensions:"x-optional"`
	ParentName string `json:"parent_name,omitempty" extensions:"x-optional"` // 被回复人的展示名
	// ParentAvatarURL 被回复人的头像（ADR-0042「昵称 › 被回复人」行内形态所需）。
	// 与 ParentName 同口径 omitempty：顶层回复（无被回复人）两个字段都不出现。
	ParentAvatarURL string `json:"parent_avatar_url,omitempty" extensions:"x-optional"`
	Content         string `json:"content"`
	// ContentFormat 正文格式声明（ADR-0044）：text | markdown。与主题同口径。
	ContentFormat string      `json:"content_format"`
	Images        []string    `json:"images"`
	CreatedAt     string      `json:"created_at"`
	Author        ForumAuthor `json:"author"`
	CanDelete     bool        `json:"can_delete"`
	LikesCount    int64       `json:"likes_count"`
	LikedByMe     bool        `json:"liked_by_me"`
	IsAccepted    bool        `json:"is_accepted"`
	// IPProvince / IPCity 发布那一刻的属地快照（ADR-0045），与主题同口径：
	// 空串 = 无属地，展示侧接在相对时间之后（「18 小时前 · 上海」），为空则整段不渲染。
	IPProvince string `json:"ip_province"`
	IPCity     string `json:"ip_city"`
}

// ForumImageUploadResultDTO 论坛图片上传结果（spec #962 片四：收口自 handler 内联 gin.H）。
type ForumImageUploadResultDTO struct {
	URL string `json:"url"`
}

// ForumLikeResultDTO 点赞 / 取消点赞结果（主题与回复的 like/unlike 四个端点共用同一形状）。
//
// 字段按 JSON key 字母序声明（liked < likes_count）：旧形态是 gin.H，encoding/json 对 map
// 按 key 排序输出，换成 struct 后序列化字节序不变（ADR-0009 §2 / 片二字节锁口径）。
type ForumLikeResultDTO struct {
	Liked      bool  `json:"liked"`
	LikesCount int64 `json:"likes_count"`
}

// ForumService 论坛服务（学员交互 + 个人集合，ADR-0050 决策 3）。
//
// 治理动作（举报处置 / 意图认定 / 管理端强制删除与违规回收）在 ForumModerationService
// （forum_moderation_service.go）；两者共享 forumCore 的依赖与私有 helper，实例分离。
type ForumService struct {
	forumCore
}

// NewForumService 构造论坛服务。
// fileSvc 用于删除帖子/回复时清理图片存储（可 nil，nil 时跳过清理）；
// notificationSvc 用于论坛事件站内信（回复/举报处理/管理端删帖，见各触发点）；
// counters 为 likes_count / reply_count 唯一写入口（与 AuthService 共享同一实例）；
// points 为积分簿记通道（采纳奖励/违规回收经其事务内导出方法落账，ADR-0023）。
func NewForumService(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, counters ForumCounter, points *PointsService, logger *zap.Logger) *ForumService {
	return &ForumService{forumCore: newForumCore(db, fileSvc, notificationSvc, counters, points, logger)}
}

// topicRow 列表查询的扫描结构。
type topicRow struct {
	ID              int64
	ChapterID       *int
	Category        string
	ChapterTitle    string
	Title           string
	Content         string
	ContentFormat   string
	IPProvince      string
	IPCity          string
	Images          string
	ViewCount       int
	ReplyCount      int
	LikesCount      int64
	AcceptedReplyID *int64
	SolvedAt        *time.Time
	LastReplyAt     *time.Time
	IsFeatured      bool
	IsExperience    bool
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
		ContentFormat:   r.ContentFormat,
		IPProvince:      r.IPProvince,
		IPCity:          r.IPCity,
		Images:          imageURLsForWire(r.Images),
		ViewCount:       r.ViewCount,
		ReplyCount:      r.ReplyCount,
		LikesCount:      r.LikesCount,
		AcceptedReplyID: r.AcceptedReplyID,
		SolvedAt:        solvedAt,
		LastReplyAt:     lastReplyAt,
		IsFeatured:      r.IsFeatured,
		IsExperience:    r.IsExperience,
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

// ForumTopicDetailDTO 主题详情（ADR-0047 §3 读面 module / ADR-0009 §2 typed DTO）。
//
// 字段声明**按 JSON key 字母序**（page / pages / replies / topic / total）：旧形态是
// map[string]any，而 encoding/json 对 map 按 key 排序输出 —— 字母序声明保证换成 struct 后
// 序列化字节序不变（ADR-0009 §2 把这条列为最高优先级约束）。
type ForumTopicDetailDTO struct {
	Page    int             `json:"page"`
	Pages   int             `json:"pages"`
	Replies []ForumReplyDTO `json:"replies"`
	Topic   ForumTopicDTO   `json:"topic"`
	Total   int64           `json:"total"`
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
	case ForumCategoryDiscussion, ForumCategoryQuestion, ForumCategoryExperience:
		return category, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrCategoryInvalid, category)
	}
}

// parseSolvedArg 解析列表查询的 solved 参数（#367）。
// 空或 all = 不过滤；solved = 已解决（accepted_reply_id 非空）；unsolved = 求助（accepted_reply_id 为空）。
//
// **类别校验在调用侧**（ListTopics）：solved 只对问答帖有意义，故非空时必须同时指定
// category=question，否则 400。旧实现在任何类别下都无条件拼 accepted_reply_id 条件——
// 对 discussion/experience 而言该列恒为 NULL，用户拿到的是**静默空列表**而非明确拒绝。
func parseSolvedArg(solved string) (string, error) {
	switch v := strings.TrimSpace(strings.ToLower(solved)); v {
	case "", "all":
		return "", nil
	case "solved":
		return "solved", nil
	case "unsolved":
		return "unsolved", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrSolvedArgInvalid, solved)
	}
}

// parseForumFeaturedArg 解析列表查询的 featured 参数（#742）。
// 空 = 不过滤；true = 仅精选；false = 仅非精选（管理端找待精候选）。
// 语义与 solved 同构：均为布尔派生列的等值过滤，随 WHERE 共存于主查询。
func parseForumFeaturedArg(featured string) (string, error) {
	switch v := strings.TrimSpace(strings.ToLower(featured)); v {
	case "":
		return "", nil
	case "true":
		return "true", nil
	case "false":
		return "false", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrFeaturedArgInvalid, featured)
	}
}

// parseForumExperienceArg 解析列表查询的 is_experience 参数（ADR-0040）。
// 空 = 不过滤；true = 仅经验认定（经验 Tab）；false = 仅非经验。
// 与 solved/featured 同构：布尔派生列的等值过滤，随 WHERE 共存于主查询。
//
// 注意与 category=experience **不是**同一件事：那是遗留意图值（存量行已降级、写入已收窄），
// 保留接受该值只为不让旧客户端拿到 400；它过滤不出任何行，经验 Tab 一律走本参数。
func parseForumExperienceArg(experience string) (string, error) {
	switch v := strings.TrimSpace(strings.ToLower(experience)); v {
	case "":
		return "", nil
	case "true":
		return "true", nil
	case "false":
		return "false", nil
	default:
		return "", fmt.Errorf("%w: %s", ErrExperienceArgInvalid, experience)
	}
}

// TopicListInput 主题列表查询条件。
//
// 用 struct 而非位置参数：本方法有 scope/keyword/sort/order/category 五个 string，
// 位置传错（如把 category 落进 keyword）编译通过且语义全错。
type TopicListInput struct {
	Scope    string // all（默认）/ general / chapter
	Category string // 空或 all = 不过滤；discussion / question / experience = 按类别分流
	Solved   string // 空或 all = 不过滤；solved / unsolved（#367，仅问答帖有意义）
	Featured string // 空 = 不过滤；true = 仅精选；false = 仅非精选（#742）
	// IsExperience 空 = 不过滤；true = 仅备考经验认定；false = 仅非经验（ADR-0040）。
	// 经验 Tab 的唯一判据——**不要再改回 category=experience**（存量行已降级，过滤不出行）。
	IsExperience string
	ChapterID    int
	Page         int
	PageSize     int
	Keyword      string
	Sort         string // latest（默认）/ hot / created（按发帖时间，#722）
	Order        string // desc（默认）/ asc
}

// ListTopics 分页查询主题。
// scope: all（默认）/ general（综合讨论区）/ chapter（需配合 chapterID）；sort: latest（默认，活跃度）/ hot（热度：点赞数→回复数→浏览数）/ created（发帖时间，#722）；order: desc（默认）/ asc（正序）。
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
	featured, err := parseForumFeaturedArg(in.Featured)
	if err != nil {
		return nil, err
	}
	isExperience, err := parseForumExperienceArg(in.IsExperience)
	if err != nil {
		return nil, err
	}
	// solved 只对问答帖有意义（accepted_reply_id 只在 question 帖上非空）。
	// 缺 category=question 时报 400 而非静默返回空列表——与 solved 非法值同口径。
	if solved != "" && category != ForumCategoryQuestion {
		return nil, ErrSolvedFilterScope
	}
	if scope == "" {
		scope = ForumScopeAll
	}
	if scope == ForumScopeChapter && chapterID <= 0 {
		return nil, ErrChapterIDRequired
	}
	if sort != "hot" && sort != "created" {
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
	// created（#722）：按发帖时间排，区别于 latest 的活跃度口径。
	// experience 列表必带 category 等值过滤，idx_forum_topics_category_created 天然命中，无需新索引。
	if sort == "created" {
		orderClause = "t.created_at " + dir + ", t.id " + dir
	}

	rows, total, page, pageSize, err := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		orderClause,
		func(q *gorm.DB) *gorm.DB {
			q = q.Table("forum_topics AS t").
				Select(topicRowSelect +
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
			// 精选过滤（#742）：与 scope/category 同一条 WHERE，不在应用层事后过滤。
			if featured == "true" {
				q = q.Where("t.is_featured = TRUE")
			} else if featured == "false" {
				q = q.Where("t.is_featured = FALSE")
			}
			// 经验认定过滤（ADR-0040）：同上，与 scope/category/solved/featured 共存在一条 WHERE。
			// 经验 Tab 的唯一判据——**不要再改回 category=experience**（存量行已降级，过滤不出行）。
			if isExperience == "true" {
				q = q.Where("t.is_experience = TRUE")
			} else if isExperience == "false" {
				q = q.Where("t.is_experience = FALSE")
			}
			if keyword = strings.TrimSpace(keyword); keyword != "" {
				like := "%" + keyword + "%"
				q = q.Where("(t.title ILIKE ? OR t.content ILIKE ?)", like, like)
			}
			return q
		})
	if err != nil {
		return nil, err
	}

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

// TopicDetailInput 主题详情查询条件（ADR-0042）。
// 用 struct 而非位置参数，理由同 TopicListInput：本方法有 sort/order/page/page_size 多个标量，
// 位置传错编译通过而语义全错。
type TopicDetailInput struct {
	TopicID   int64
	ViewerID  int
	ReplySort string // latest（别名 time）/ hot
	Order     string // asc / desc；空 = 按 sort 的默认方向
	Page      int    // <=0 回退 1
	PageSize  int    // <=0 或超上限回退 ForumReplyDefaultPageSize
}

// GetTopic 主题详情（回复**分页**返回，带被回复人信息），并累加浏览量。
// replySort: time/latest（默认，时间）/ hot（热度：点赞数→时间）；order: asc/desc（默认 asc 对 time，desc 对 hot；显式传入时统一覆盖）。
//
// 置顶（ADR-0042）：被采纳回复固定占首页第一条并从排序结果中剔除，故第 k 页（k>=2）的
// 其余回复从 (k-1)*pageSize-1 起算 —— 不是朴素的 (k-1)*pageSize。
//
// total 为**实时 COUNT**（分页必须与实际行数一致，否则会出现空页）；topic.reply_count 是
// 列表页消费的反范式计数列，两者由计数单写入口保持同值。
func (s *ForumService) GetTopic(in TopicDetailInput) (*ForumTopicDetailDTO, error) {
	topicID, viewerID := in.TopicID, in.ViewerID
	replySort, order := in.ReplySort, in.Order
	if replySort == "latest" {
		replySort = "time"
	}
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

	// 浏览量只在**真实浏览**时 +1（ADR-0041）：以既有浏览去重行的插入成败为事实源，
	// 排除自帖、每人每日每帖一次；复用去重表，不新增列，存量数值不回填。
	// 旧实现是「详情请求即 +1」——不去重、不排作者，而 hot 排序第三键正是 view_count，
	// 自己反复刷新就能把帖子推上热门。
	if viewerID != 0 && viewerID != int(row.UserID) {
		viewDate := time.Now().In(clock.Location()).Format("2006-01-02")
		// 同一语句同时服务两个目的：daily_browse 的去重事实源 + 浏览量的计数闸门。
		res := s.db.Exec("INSERT INTO forum_topic_views (user_id, topic_id, view_date) VALUES (?,?,?) ON CONFLICT (user_id, topic_id, view_date) DO NOTHING", viewerID, topicID, viewDate)
		if res.Error == nil && res.RowsAffected > 0 {
			_ = s.db.Model(&model.ForumTopic{}).Where("id = ?", topicID).
				UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
			row.ViewCount++
		}
	}

	// 回复游标方向：time/latest 默认正序（先发先排），hot 默认倒序。
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
	// ===== 回复分页 + 置顶（ADR-0042）=====
	// 回复流 = [置顶条] + [其余按 sort/order 排序]，按 page_size 切块：
	// 置顶条占首页第一格，并从排序结果里剔除（否则同一条会既在首位、又在自然位置重复出现）。
	page, pageSize := paging.ClampMax(in.Page, in.PageSize, ForumReplyDefaultPageSize, ForumReplyMaxPageSize)

	// count 与 scan 同一 WHERE 作用域（同一 baseQuery），故 total 与列表页的 reply_count 同源。
	var total int64
	if err := s.replyBaseQuery(topicID, nil).Count(&total).Error; err != nil {
		return nil, err
	}

	acceptedID := row.AcceptedReplyID
	// 置顶条只在首页取。它占掉首页一格，故其余回复的 offset 要按此折算：
	// 第 k 页（k>=2）的其余回复从 (k-1)*pageSize - 1 开始——不是朴素的 (k-1)*pageSize。
	offset := (page - 1) * pageSize
	limit := pageSize
	if acceptedID != nil {
		offset = (page-1)*pageSize - 1
		if page == 1 {
			offset = 0
			limit = pageSize - 1 // 首页给置顶条留一格
		}
	}
	if offset < 0 {
		offset = 0
	}

	replyDTOs := make([]ForumReplyDTO, 0, pageSize)
	if acceptedID != nil && page == 1 {
		var pinned replyRow
		if err := s.replyBaseQuery(topicID, nil).Where("r.id = ?", *acceptedID).Scan(&pinned).Error; err != nil {
			return nil, err
		}
		if pinned.ID != 0 {
			replyDTOs = append(replyDTOs, pinned.toDTO(viewerID, acceptedID))
		}
	}

	// limit<=0 只在 pageSize=1 且首页有置顶条时出现——此时首页就是置顶条本身，不再查其余。
	if limit > 0 {
		var replies []replyRow
		if err := s.replyBaseQuery(topicID, acceptedID).
			Order(replyOrder).
			Offset(offset).Limit(limit).
			Scan(&replies).Error; err != nil {
			return nil, err
		}
		for _, r := range replies {
			replyDTOs = append(replyDTOs, r.toDTO(viewerID, acceptedID))
		}
	}
	// 批量回填当前用户是否已赞（计数已由 likes_count 列提供，单一 helper 收敛）
	s.enrichReplyLikedByMe(replyDTOs, viewerID)

	// 点赞状态（ADR-0018）：详情返回计数已由列提供，仅需回填是否已赞。
	topicDTO := row.toDTO(viewerID)
	s.enrichTopicLikedByMe([]*ForumTopicDTO{&topicDTO}, viewerID)
	if s.hasRewardIssued(topicID) {
		topicDTO.RewardIssued = true
	}

	return &ForumTopicDetailDTO{
		Page:    page,
		Pages:   response.PageCount(total, pageSize),
		Replies: replyDTOs,
		Topic:   topicDTO,
		Total:   total,
	}, nil
}

// replyRow 详情页回复行的扫描结构（置顶查询与分页查询共用同一投影，避免两处漂移）。
type replyRow struct {
	ID            int64
	TopicID       int64
	ParentID      *int64
	Content       string
	ContentFormat string
	IPProvince    string
	IPCity        string
	Images        string
	LikesCount    int64
	CreatedAt     time.Time
	UserID        int
	Username      string
	AvatarURL     string
	ParentName    string
	// ParentAvatarURL 被回复人的头像（join pr→pu 回填）。
	ParentAvatarURL string
}

// toDTO 行 → DTO。acceptedReplyID 为该帖当前采纳的回复 id（nil = 未采纳），据此打 is_accepted。
func (r replyRow) toDTO(viewerID int, acceptedReplyID *int64) ForumReplyDTO {
	return ForumReplyDTO{
		ID: r.ID, TopicID: r.TopicID, ParentID: r.ParentID,
		ParentName: r.ParentName, ParentAvatarURL: r.ParentAvatarURL,
		Content: r.Content, ContentFormat: r.ContentFormat,
		IPProvince: r.IPProvince, IPCity: r.IPCity,
		Images: imageURLsForWire(r.Images), CreatedAt: formatISO(r.CreatedAt),
		Author: ForumAuthor{
			UserID: r.UserID, Username: r.Username, AvatarURL: r.AvatarURL,
		},
		CanDelete:  r.UserID == viewerID,
		LikesCount: r.LikesCount,
		IsAccepted: acceptedReplyID != nil && *acceptedReplyID == r.ID,
	}
}

// replyRowSelect 详情页回复行的共享投影（置顶查询与分页查询共用同一份，新增字段只改这一处）。
const replyRowSelect = "r.id, r.topic_id, r.parent_id, r.content, r.content_format, r.ip_province, r.ip_city, r.images, r.likes_count, r.created_at, " +
	"u.id AS user_id, u.username, u.avatar_url, " +
	"COALESCE(pu.username, '') AS parent_name, COALESCE(pu.avatar_url, '') AS parent_avatar_url"

// replyBaseQuery 详情页回复查询的共享装配：同一 WHERE 同时服务 count 与 scan。
// excludeReplyID 非 nil 时剔除该条——置顶条已单独取得，不应再出现在排序结果里。
func (s *ForumService) replyBaseQuery(topicID int64, excludeReplyID *int64) *gorm.DB {
	q := s.db.Table("forum_replies AS r").
		Select(replyRowSelect).
		Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
		Joins("LEFT JOIN forum_replies AS pr ON pr.id = r.parent_id").
		Joins("LEFT JOIN hrwai_users AS pu ON pu.id = pr.user_id").
		Where("r.topic_id = ?", topicID)
	if excludeReplyID != nil {
		q = q.Where("r.id <> ?", *excludeReplyID)
	}
	return q
}

// CreateTopicInput 发帖条件。Category 为空归一为 discussion（移动端旧契约不传）。
type CreateTopicInput struct {
	UserID    int
	ChapterID *int
	Category  string
	Title     string
	Content   string
	// ContentFormat 正文格式声明（ADR-0044）。空串归一为 text——移动端旧契约不传该字段。
	ContentFormat string
	Images        []string
	// ClientIP 发布请求的客户端 IP（handler 传 middleware.ClientIP(c)，即可信取 IP 单点）。
	// 属地是**发布那一刻**的快照：只在这里解析一次并落库（ADR-0045）。
	ClientIP string
}

// CreateTopic 发帖。chapterID 为 nil/0 表示发到综合讨论区。
// images 为主题图片 URL 列表（最多 ForumTopicMaxImages 张，仅接受本站 images/forum/ 前缀）。
func (s *ForumService) CreateTopic(in CreateTopicInput) (*ForumTopicDTO, error) {
	category, err := normalizeForumCategory(in.Category)
	if err != nil {
		return nil, err
	}
	contentFormat, err := normalizeContentFormat(in.ContentFormat)
	if err != nil {
		return nil, err
	}
	userID, chapterID, title, content, images := in.UserID, in.ChapterID, in.Title, in.Content, in.Images
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 100 {
		return nil, ErrTitleLength
	}
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 10000 {
		return nil, ErrContentLength
	}
	if err := validateForumImages(images, ForumTopicMaxImages); err != nil {
		return nil, err
	}

	// 非法组合在进库前拒绝：问答帖一律不属于任何章节。
	// 数据库层有同名 CHECK 作生产兜底（见迁移 000005），此处是能被契约测试守住的行为层。
	// 注意只判 >0：chapter_id 传 0 或不传按既有语义归一为综合区，不得在此收紧。
	if category == ForumCategoryQuestion && chapterID != nil && *chapterID > 0 {
		return nil, ErrQuestionChapterConflict
	}

	var cid *int
	if chapterID != nil && *chapterID > 0 {
		var cnt int64
		if err := s.db.Model(&model.Chapter{}).Where("chapter_id = ?", *chapterID).Count(&cnt).Error; err != nil {
			return nil, err
		}
		if cnt == 0 {
			return nil, ErrChapterNotFound
		}
		cid = chapterID
	}

	// 属地快照（ADR-0045）：发布那一刻取一次。解析不出来就是空串（内网 / 保留地址 / 库无该段），
	// 不报错也不阻断发帖——展示侧「为空即整段不渲染」。
	region := geolocation.Resolve(in.ClientIP)

	now := beijingNow()
	topic := model.ForumTopic{
		ChapterID: cid,
		// 显式写入归一后的非空类别，不依赖数据库 DEFAULT：
		// GORM 对带 default tag 的零值字段会跳过 INSERT，那样内存对象（下面的 DTO）会拿到空串。
		Category: category,
		UserID:   userID,
		Title:    title,
		Content:  content,
		// 与 category 同理显式写入：model 上带 default tag，GORM 会跳过零值字段的 INSERT，
		// 那样内存对象（下面的 DTO）会拿到空串而不是归一后的 text。
		ContentFormat: contentFormat,
		// 与 category / content_format 同理显式写入：model 上带 default tag，GORM 会跳过零值字段，
		// 那样内存对象（下面的 DTO）会与库里不一致。
		IPProvince: region.Province,
		IPCity:     region.City,
		Images:     marshalImageURLs(images),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.db.Create(&topic).Error; err != nil {
		return nil, err
	}

	var u model.HrwaiUser
	if err := s.db.First(&u, userID).Error; err != nil {
		return nil, err
	}
	return &ForumTopicDTO{
		ID:            topic.ID,
		ChapterID:     topic.ChapterID,
		Category:      topic.Category,
		Title:         topic.Title,
		Content:       topic.Content,
		ContentFormat: topic.ContentFormat,
		IPProvince:    topic.IPProvince,
		IPCity:        topic.IPCity,
		Images:        imagesForWire(images),
		CreatedAt:     formatISO(topic.CreatedAt),
		Author: ForumAuthor{
			UserID: u.ID, Username: u.Username, AvatarURL: u.AvatarURL,
		},
		CanDelete: true,
	}, nil
}

// UpdateTopicInput 编辑帖子条件（#811）。
//
// 可改字段仅 title / content / images / category；**chapter_id 不在契约内**——
// 移动端 PUT 载荷不传（编辑不迁移章节归属），故更新路径按既有行的 chapter_id
// 判定发帖同构的不变量（question 不得挂章节）。
type UpdateTopicInput struct {
	UserID   int
	TopicID  int64
	Category string
	Title    string
	Content  string
	Images   []string
}

// UpdateTopic 作者本人编辑帖子（#811）。
//
// 校验顺序与语义与发帖同构：
//   - 主题不存在 → ErrTopicNotFound（handler 映射 404）
//   - 非作者本人 → ErrNotTopicOwner（handler 映射 403，与采纳/取消采纳同 owner 语义）
//   - 类别归一走 normalizeForumCategory（空串归一 discussion 向后兼容；
//     **不要**误用 parseForumCategoryArg——那是列表查询语义，空串 = 不过滤）
//   - 标题 1-100 / 正文 1-10000 / 图片走 validateForumImages（≤9 张 + 仅本站 images/forum/ 前缀）
//   - question 不得带 chapter_id：编辑不改章节归属，按既有行判定
//
// 明确不做（#811 范围）：编辑历史/版本留痕、管理员代为编辑、is_edited 列、
// 改 GET 详情响应形态（DTO 已含 category）。
func (s *ForumService) UpdateTopic(in UpdateTopicInput) (*ForumTopicDTO, error) {
	var topic model.ForumTopic
	if err := s.db.First(&topic, in.TopicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if topic.UserID != in.UserID {
		return nil, ErrNotTopicOwner
	}

	category, err := normalizeForumCategory(in.Category)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(in.Title)
	content := strings.TrimSpace(in.Content)
	if utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 100 {
		return nil, ErrTitleLength
	}
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 10000 {
		return nil, ErrContentLength
	}
	if err := validateForumImages(in.Images, ForumTopicMaxImages); err != nil {
		return nil, err
	}
	// 与发帖同构的不变量（对照 CreateTopic）：问答帖不属于任何章节。
	// 编辑不迁移章节，故按既有行的 chapter_id 判定；数据库 CHECK 只在迁移 000005、
	// 测试库 AutoMigrate 覆盖不到，行为层必须自己守住。
	if category == ForumCategoryQuestion && topic.ChapterID != nil && *topic.ChapterID > 0 {
		return nil, ErrQuestionChapterConflict
	}
	// 已采纳的帖子禁止改类别（2026-09-11 维护者裁定）：采纳状态只在问答帖有意义，
	// 迁移类别会把 accepted_reply_id/solved_at 留在非问答帖上（答主已发的分按既有政策
	// 「取消采纳不回滚」保留），产生「非问答帖带采纳」的悬挂态。
	// 判定按采纳事实而非当前类别——同一条规则也兜住历史遗留的悬挂行。
	// 逃生口：先取消采纳（CancelAccept 清空 accepted_reply_id）再改类别。
	if topic.AcceptedReplyID != nil && category != ForumCategoryQuestion {
		return nil, ErrCategoryLockedByAccept
	}

	// 显式写全四字段（map 更新：category 归一后的非空值不受 GORM 零值跳过影响）
	if err := s.db.Model(&model.ForumTopic{}).Where("id = ?", in.TopicID).Updates(map[string]any{
		"category":   category,
		"title":      title,
		"content":    content,
		"images":     marshalImageURLs(in.Images),
		"updated_at": beijingNow(),
	}).Error; err != nil {
		return nil, err
	}
	// 详情响应形态不变（DTO 已含 category），重新装配以回显图片/点赞等派生字段
	return s.fetchTopicDTO(in.TopicID, in.UserID)
}

// ReplyTopicInput 发回复条件。
//
// 用 struct 而非位置参数，理由同 CreateTopicInput / TopicDetailInput：本方法有 content 与
// content_format 两个极易互串的 string，位置传错能编译通过而语义全错。
type ReplyTopicInput struct {
	UserID        int
	TopicID       int64
	Content       string
	ParentReplyID *int64   // 非空即楼中楼
	Images        []string // 最多 ForumReplyMaxImages 张，仅接受本站 images/forum/ 前缀
	// ContentFormat 正文格式声明（ADR-0044）。空串归一为 text——移动端旧契约不传该字段。
	ContentFormat string
	// ClientIP 发布请求的客户端 IP（与发帖同口径）。
	ClientIP string
}

// ReplyTopic 回复主题或回复某条回复（ParentReplyID 非空时）。
func (s *ForumService) ReplyTopic(in ReplyTopicInput) (*ForumReplyDTO, error) {
	userID, topicID, parentReplyID, images := in.UserID, in.TopicID, in.ParentReplyID, in.Images
	content := strings.TrimSpace(in.Content)
	if utf8.RuneCountInString(content) < 1 || utf8.RuneCountInString(content) > 5000 {
		return nil, ErrReplyContentLength
	}
	// 回复与主题同口径：空串归一为 text（移动端旧契约不传该字段），非法值 400。
	contentFormat, err := normalizeContentFormat(in.ContentFormat)
	if err != nil {
		return nil, err
	}
	if err := validateForumImages(images, ForumReplyMaxImages); err != nil {
		return nil, err
	}

	var topic model.ForumTopic
	if err := s.db.First(&topic, topicID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTopicNotFound
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
				return nil, ErrReplyNotFound
			}
			return nil, err
		}
		if parent.TopicID != topicID {
			return nil, ErrParentReplyMismatch
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

	// 属地快照（ADR-0045），与发帖同口径：发布那一刻取一次，解析不出来即空串。
	region := geolocation.Resolve(in.ClientIP)

	now := beijingNow()
	reply := model.ForumReply{
		TopicID:  topicID,
		UserID:   userID,
		ParentID: parentReplyID,
		Content:  content,
		// 显式写入归一后的格式，不依赖数据库 DEFAULT（与发帖同理：GORM 跳过带 default tag 的零值）。
		ContentFormat: contentFormat,
		IPProvince:    region.Province,
		IPCity:        region.City,
		Images:        marshalImageURLs(images),
		CreatedAt:     now,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
		// 站内信通知（与回复同事务提交，避免通知丢失；事件构造器单点，ADR-0027 C1）：
		// 1) 楼主被回复（回复人是楼主本人时不通知）
		// 2) 楼中楼被回复人（非自己、非楼主——楼主已由 1) 覆盖，避免重复通知）
		if topic.UserID != userID {
			if err := s.notificationSvc.CreateForumReplyEvent(tx,
				NewTopicReplierEvent(topic.UserID, replierName, topic.Title, topicID), now); err != nil {
				return err
			}
		}
		if parentAuthorID != 0 && parentAuthorID != userID && parentAuthorID != topic.UserID {
			if err := s.notificationSvc.CreateForumReplyEvent(tx,
				NewReplyReplierEvent(parentAuthorID, replierName, topic.Title, topicID), now); err != nil {
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
		ParentName: parentName, Content: reply.Content, ContentFormat: reply.ContentFormat,
		IPProvince: reply.IPProvince, IPCity: reply.IPCity,
		Images: imagesForWire(images), CreatedAt: formatISO(reply.CreatedAt),
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
			return ErrTopicNotFound
		}
		return err
	}
	if topic.UserID != userID {
		return ErrNotTopicAuthor
	}
	// 巡检计数：楼主删除自己已解决的帖子时累加（不回滚积分，仅计数）
	if topic.AcceptedReplyID != nil {
		_ = s.incrementDeletedAfterAccepted()
	}
	return s.deleteTopicWithImages(topicID)
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

// DeleteReply 删除回复（仅作者本人；其下级回复随外键级联删除）。
// 本回复与全部下级回复（parent_id 链条）的图片一并清理。
func (s *ForumService) DeleteReply(userID int, replyID int64) error {
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrReplyNotFound
		}
		return err
	}
	if reply.UserID != userID {
		return ErrNotReplyAuthor
	}
	return s.deleteReplyWithImages(replyID, reply.TopicID)
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
		return fmt.Errorf("%w（最多 %d 张）", ErrImagesTooMany, max)
	}
	for _, u := range images {
		if !IsSiteAttachmentURL(u, ForumImageDirPrefix) {
			return ErrImageURLInvalid
		}
	}
	return nil
}

// parseImageURLs 将 JSONB 图片数组字符串解析为 URL 列表（无效 JSON 返回空列表）。
// 只给「计数/差集/内部聚合」用；写进响应 DTO 的一律走 imageURLsForWire（票12 出口归一）。
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

// imageURLsForWire 读面出口归一：无图即空数组，不是 null（ADR-0062 决策 12）。
// 生成的契约（frontend/src/api/generated/forum.ts 的 `images: string[]`）承诺数组，
// 而旧写法把 nil 序列化成 `"images": null` —— 今天只靠各页面手写 `|| []` / `&&` 存活，
// 新消费者写 `topic.images.length` 类型全绿、运行时 TypeError。
// 「无图」与「图片数据缺失」在本域是同一件事，所以不把它升成第二个值（不选 x-nullable）。
func imageURLsForWire(raw string) []string {
	return imagesForWire(parseImageURLs(raw))
}

// imagesForWire 同一归一判据的「已是切片」形态：写面回显请求体的图片数组时也走它
// （客户端不发 `images` 键时 in.Images 是 nil，同样会序列化成 null）。
func imagesForWire(urls []string) []string {
	if urls == nil {
		return []string{}
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
		return 0, ErrTopicNotFound
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
		return ErrReportReasonLength
	}
	if (topicID == nil) == (replyID == nil) {
		return ErrReportTarget
	}
	if topicID != nil {
		var cnt int64
		s.db.Model(&model.ForumTopic{}).Where("id = ?", *topicID).Count(&cnt)
		if cnt == 0 {
			return ErrTopicNotFound
		}
	}
	if replyID != nil {
		var cnt int64
		s.db.Model(&model.ForumReply{}).Where("id = ?", *replyID).Count(&cnt)
		if cnt == 0 {
			return ErrReplyNotFound
		}
	}
	return s.db.Create(&model.ForumReport{
		ReporterID: userID, TopicID: topicID, ReplyID: replyID,
		Reason: reason, Status: 0, CreatedAt: beijingNow(),
	}).Error
}

// MyTopics 我的帖子（复用主题列表行装配，按最后活跃倒序）。
func (s *ForumService) MyTopics(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	rows, total, page, pageSize, err := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"COALESCE(t.last_reply_at, t.created_at) DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_topics AS t").
				Select(topicRowSelect+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(ch.title, '') AS chapter_title").
				Joins("JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
				Where("t.user_id = ?", userID)
		})
	if err != nil {
		return nil, err
	}
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

// topicRowSelect topicRow 的共享投影（#742 审查收敛）：新增 topicRow 字段时只改这一处，
// 全部列表/详情/个人视图查询共用，避免散落 5 处的投影字符串漂移。
const topicRowSelect = "t.id, t.chapter_id, t.category, t.title, t.content, t.content_format, t.ip_province, t.ip_city, t.images, t.view_count, t.reply_count, t.likes_count, t.accepted_reply_id, t.solved_at, t.last_reply_at, t.is_featured, t.is_experience, t.created_at, "

// personalTopicSelect 个人动态三列表的行装配投影（与 MyTopics 逐字一致，被删主题字段 NULL 由 Scan 零值承载）。
const personalTopicSelect = topicRowSelect +
	"u.id AS user_id, u.username, u.avatar_url, " +
	"COALESCE(ch.title, '') AS chapter_title"

// finishPersonalTopics 行装配收尾单点：DTO 转换 + 点赞回填 + 发分回填 + 分页信封。
func (s *ForumService) finishPersonalTopics(rows []topicRow, total int64, page, pageSize int, userID int) *ForumTopicPageResult {
	items := make([]ForumTopicDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.toDTO(userID))
	}
	s.enrichTopicLikedByMe(toDTORefs(items), userID)
	s.enrichRewardIssued(items)
	return &ForumTopicPageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Topics: items, Total: total,
	}
}

// MyLikedTopics 赞过（#701）：点赞行驱动 + 主题/作者/章节 LEFT JOIN，按点赞时间倒序。
// 主题被删时条目保留、标题回空串（与 MyReplies 口径一致）。
func (s *ForumService) MyLikedTopics(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	rows, total, page, pageSize, err := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"l.created_at DESC, l.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_topic_like AS l").
				Select(personalTopicSelect).
				Joins("LEFT JOIN forum_topics AS t ON t.id = l.topic_id").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
				Where("l.user_id = ?", userID)
		})
	if err != nil {
		return nil, err
	}
	return s.finishPersonalTopics(rows, total, page, pageSize, userID), nil
}

// MyViewHistory 浏览记录（#701）：浏览行驱动 + 主题/作者/章节 LEFT JOIN，按主题去重取最近一次浏览倒序。
// 去重面是「同一主题多日多行取最近一行」：先按主题聚合出每主题最近浏览（派生表），
// 再 LEFT JOIN 主题取行装配——count 与 scan 同走派生表，去重语义在计数侧同样成立。
// 自帖在写入侧已排除（GetTopic 不记录自帖浏览），此处不再过滤。主题被删时条目保留。
func (s *ForumService) MyViewHistory(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	latestViews := s.db.Table("forum_topic_views AS v").
		Select("v.topic_id, MAX(v.viewed_at) AS last_viewed").
		Where("v.user_id = ?", userID).
		Group("v.topic_id")
	rows, total, page, pageSize, err := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"lv.last_viewed DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("(?) AS lv", latestViews).
				Select(personalTopicSelect).
				Joins("LEFT JOIN forum_topics AS t ON t.id = lv.topic_id").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id")
		})
	if err != nil {
		return nil, err
	}
	return s.finishPersonalTopics(rows, total, page, pageSize, userID), nil
}

// MyObservedTopics 我的围观（#701）：浏览行驱动 + 排除四项直接互动（本人发帖、本人回复、
// 本人主题点赞、本人对该主题的收藏；回复点赞不计入），按最近浏览倒序。主题被删时条目保留。
// 同浏览记录：先按主题聚合最近浏览，再做互动排除——排除谓词落在聚合后的主题维度上。
func (s *ForumService) MyObservedTopics(userID, page, pageSize int) (*ForumTopicPageResult, error) {
	latestViews := s.db.Table("forum_topic_views AS v").
		Select("v.topic_id, MAX(v.viewed_at) AS last_viewed").
		Where("v.user_id = ?", userID).
		Group("v.topic_id")
	rows, total, page, pageSize, err := paging.QueryWithScan[topicRow](s.db, page, pageSize, 10, 100,
		"lv.last_viewed DESC, t.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("(?) AS lv", latestViews).
				Select(personalTopicSelect).
				Joins("LEFT JOIN forum_topics AS t ON t.id = lv.topic_id").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = t.user_id").
				Joins("LEFT JOIN chapter AS ch ON ch.chapter_id = t.chapter_id").
				Where("t.user_id IS NULL OR t.user_id <> ?", userID).
				Where("NOT EXISTS (SELECT 1 FROM forum_replies r WHERE r.topic_id = t.id AND r.user_id = ?)", userID).
				Where("NOT EXISTS (SELECT 1 FROM forum_topic_like l WHERE l.topic_id = t.id AND l.user_id = ?)", userID).
				Where("NOT EXISTS (SELECT 1 FROM favorite f WHERE f.target_type = 'topic' AND f.target_id = t.id AND f.user_id = ?)", userID)
		})
	if err != nil {
		return nil, err
	}
	return s.finishPersonalTopics(rows, total, page, pageSize, userID), nil
}

// MyReplyDTO 我的回复条目（带主题标题回填）。
type MyReplyDTO struct {
	ID         int64  `json:"id"`
	TopicID    int64  `json:"topic_id"`
	TopicTitle string `json:"topic_title"`
	ParentID   *int64 `json:"parent_id,omitempty" extensions:"x-optional"`
	Content    string `json:"content"`
	// ContentFormat 正文格式声明（ADR-0044）：列表摘要据此决定是否剥成纯文本。
	ContentFormat string      `json:"content_format"`
	Images        []string    `json:"images"`
	CreatedAt     string      `json:"created_at"`
	Author        ForumAuthor `json:"author"`
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
		ID            int64
		TopicID       int64
		TopicTitle    string
		ParentID      *int64
		Content       string
		ContentFormat string
		Images        string
		CreatedAt     time.Time
		UserID        int
		Username      string
		AvatarURL     string
	}
	rows, total, page, pageSize, err := paging.QueryWithScan[myReplyRow](s.db, page, pageSize, 10, 100,
		"r.created_at DESC, r.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Table("forum_replies AS r").
				Select("r.id, r.topic_id, r.parent_id, r.content, r.content_format, r.images, r.created_at, "+
					"u.id AS user_id, u.username, u.avatar_url, COALESCE(t.title, '') AS topic_title").
				Joins("JOIN hrwai_users AS u ON u.id = r.user_id").
				Joins("LEFT JOIN forum_topics AS t ON t.id = r.topic_id").
				Where("r.user_id = ?", userID)
		})
	if err != nil {
		return nil, err
	}
	items := make([]MyReplyDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, MyReplyDTO{
			ID: r.ID, TopicID: r.TopicID, TopicTitle: r.TopicTitle, ParentID: r.ParentID,
			Content: r.Content, ContentFormat: r.ContentFormat,
			Images: imageURLsForWire(r.Images), CreatedAt: formatISO(r.CreatedAt),
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
		return 0, ErrReplyNotFound
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
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if topic.UserID != userID {
		return nil, ErrNotTopicOwner
	}
	if topic.Category != ForumCategoryQuestion {
		return nil, ErrAcceptNotQuestion
	}
	// 经验帖不可被采纳（ADR-0040）：认定不限制意图，管理员可以认定一篇 question 帖，
	// 若不拦就会出现「经验 + 已采纳」的组合——它与领域边界冲突（一次性提问归问答、
	// 可复用经验输出归经验），也让经验区里混进带采纳状态的帖子。
	// 逃生口：管理员先取消经验认定（与「经验帖撤精须先取消认定」互为镜像）。
	// 库层另有 CHECK chk_forum_topics_experience_not_accepted 兜底（迁移 000028）。
	if topic.IsExperience {
		return nil, ErrAcceptExperienceTopic
	}
	var reply model.ForumReply
	if err := s.db.First(&reply, replyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReplyNotFound
		}
		return nil, err
	}
	if reply.TopicID != topicID {
		return nil, ErrReplyTopicMismatch
	}
	// 禁止采纳自己（ADR-0028）：自问自答在交互层直接拒绝（替代旧「静默零分发」），
	// 消除「采纳成功却 0 分」的误导；界面层对楼主自己的回答不呈现采纳入口。
	if reply.UserID == userID {
		return nil, ErrAcceptOwnReply
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
	// 首次采纳：CAS + 积分直记（同一事务）。采纳他人回复（自采纳已在上层拒绝）。
	now := beijingNow()
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
		// 奖励发放交给奖励政策 module（ADR-0047 §3）：发放事实判定、防刷求值、幂等占坑、
		// 写流水与站内信构造都在它的 implementation 里，这里只声明「采纳发生了」。
		return s.rewards.Award(tx, forumRewardFact{
			Kind: forumRewardAccept, TopicID: topicID, TopicTitle: topic.Title,
			TopicOwner: userID, AnswererID: reply.UserID, ReplyID: replyID, At: now,
		})
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
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	if topic.UserID != userID {
		return nil, ErrNotTopicOwner
	}
	if topic.Category != ForumCategoryQuestion {
		return nil, ErrCancelAcceptNotQuestion
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
