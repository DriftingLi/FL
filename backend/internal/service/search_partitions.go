// Package service 实现业务服务层。
// 本文件：全局搜索分区声明（ADR-0050 决策 2）——course / chapter / question / content / topic
// 五个分区的六槽位半描述符。引擎骨架在 search_service.go（searchPartitionPage）：
// count → 命中排序（标题命中优先 + 分区二级键）→ 分页 scan → DTO 装配。
//
// **半描述符，不做全声明化**：匹配面与 DTO 装配每分区真不同——章节有 description 命中回退、
// 论坛有 EXISTS 回复子查询且命中要回查回复正文；硬声明化会发明一门小 DSL，描述符本身变成
// 新的学习面（ADR-0050 决策 2 被否备选）。所以「匹配面 / scope / hit / 装配」是函数槽位。
//
// 加一个分区 = 写一份 partitionSpec + 在 searchPartitions 登记一行；分发（查表）与聚合
// （遍历声明表）自动跟上，「switch / DTO / 方法体 / handler 四处同步」的手拼面被关闭。
package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// partitionSpec 分区声明（泛型行类型 R）：六个语义槽位 + 两个机械槽位（scan 列 / 标题命中判据）。
type partitionSpec[R any] struct {
	// 槽位①：类型键（SearchTypeCourse…）——分发键，也是聚合响应字段与检索事实列的映射键。
	key string

	// 槽位②：匹配面（WHERE 形状，含 gorm Model 与 LIKE 匹配列）。
	match func(s *SearchService, p searchParams) *gorm.DB
	// 槽位③：可见性 scope 谓词。ADR-0050 决策 1 的具名 scope 在这里接入
	// （MountedCourseScope / QuestionPoolScope）；nil = 该分区无额外可见性谓词。
	scope func(s *SearchService, q *gorm.DB, p searchParams) *gorm.DB

	// selects：scan 列（只取 DTO 用得到的列）。
	selects string
	// titleHit：一级排序判据「标题命中优先」的 SQL（带 like 占位符）；空串 = 该分区无标题面。
	titleHit string
	// 槽位⑤：二级排序键——按内容性质分派（常青内容用编辑信号，时效内容用活跃度）。
	secondary string

	// 槽位④：hit 函数——命中位置与片段（hitOf / hitOfTopic）。
	hit func(s *SearchService, r *R, p searchParams) (string, string)
	// 槽位⑥：DTO 装配函数。
	assemble func(r R, hitField, snippet string) SearchItemDTO
}

// ===== 课程分区：匹配课名 + 简介；常青内容 → 编辑信号排序 =====

const (
	searchCourseTitleHit = "LOWER(name) LIKE ? ESCAPE '\\'"
	searchCourseBodyHit  = "LOWER(description) LIKE ? ESCAPE '\\'"
)

var coursePartition = partitionSpec[model.Course]{
	key: SearchTypeCourse,
	match: func(s *SearchService, p searchParams) *gorm.DB {
		return s.db.Model(&model.Course{}).
			Where("("+searchCourseTitleHit+" OR "+searchCourseBodyHit+")", p.like, p.like)
	},
	scope: func(s *SearchService, q *gorm.DB, p searchParams) *gorm.DB {
		// 挂载不变式（ADR-0006 / ADR-0050 决策 1）叠加已发布；证件分区由读面给定。
		q = MountedCourseScope(q.Where("status = 1"))
		if p.cred != nil {
			q = q.Where("credential_id = ?", *p.cred)
		}
		return q
	},
	selects:   "course_id, name, cover_image, description",
	titleHit:  searchCourseTitleHit,
	secondary: "sort_order ASC, is_hot DESC, is_featured DESC, course_id ASC",
	hit: func(s *SearchService, r *model.Course, p searchParams) (string, string) {
		return hitOf(r.Name, r.Description, p.keyword)
	},
	assemble: func(r model.Course, hitField, snippet string) SearchItemDTO {
		return SearchItemDTO{
			Type: SearchTypeCourse, ID: int64(r.CourseID), Title: r.Name, Cover: r.CoverImage,
			Summary: snippetOf(r.Description, 80), Snippet: snippet, HitField: hitField,
		}
	},
}

// ===== 章节分区：匹配章节标题 + 正文 + 描述；可见性跟随所属课程（已发布 + 挂载不变式）=====

const (
	searchChapterTitleHit   = "LOWER(chapter.title) LIKE ? ESCAPE '\\'"
	searchChapterContentHit = "LOWER(chapter.content) LIKE ? ESCAPE '\\'"
	searchChapterDescHit    = "LOWER(chapter.description) LIKE ? ESCAPE '\\'"
)

var chapterPartition = partitionSpec[model.Chapter]{
	key: SearchTypeChapter,
	match: func(s *SearchService, p searchParams) *gorm.DB {
		return s.db.Model(&model.Chapter{}).
			Where("("+searchChapterTitleHit+" OR "+searchChapterContentHit+" OR "+searchChapterDescHit+")", p.like, p.like, p.like)
	},
	scope: func(s *SearchService, q *gorm.DB, p searchParams) *gorm.DB {
		// 章节可见性跟随课程：同一挂载不变式 scope（不是手拼谓词）。
		mounted := MountedCourseScope(s.db.Model(&model.Course{}).Select("course_id").Where("status = 1"))
		if p.cred != nil {
			mounted = mounted.Where("credential_id = ?", *p.cred)
		}
		return q.Where("course_id IN (?)", mounted)
	},
	selects:   "chapter_id, course_id, title, content, description",
	titleHit:  searchChapterTitleHit,
	secondary: "order_num ASC, chapter_id ASC",
	hit: func(s *SearchService, r *model.Chapter, p searchParams) (string, string) {
		body := r.Content
		if body == "" {
			body = r.Description
		}
		hitField, snippet := hitOf(r.Title, body, p.keyword)
		if hitField == SearchHitBody && !containsFold(body, p.keyword) {
			// 命中在 description 而 content 非空：片段改用 description，避免片段与命中无关
			if s2, ok := snippetAround(r.Description, p.keyword); ok {
				snippet = s2
			}
		}
		return hitField, snippet
	},
	assemble: func(r model.Chapter, hitField, snippet string) SearchItemDTO {
		body := r.Content
		if body == "" {
			body = r.Description
		}
		return SearchItemDTO{
			Type: SearchTypeChapter, ID: int64(r.ChapterID), ParentID: int64(r.CourseID),
			Title: r.Title, Summary: snippetOf(body, 80), Snippet: snippet, HitField: hitField,
		}
	},
}

// ===== 题目分区：只匹配题干（解析与答案**不进**匹配面，ADR-0049「明确不做」）=====

const searchQuestionContentHit = "LOWER(content) LIKE ? ESCAPE '\\'"

var questionPartition = partitionSpec[model.Question]{
	key: SearchTypeQuestion,
	match: func(s *SearchService, p searchParams) *gorm.DB {
		return s.db.Model(&model.Question{}).Where(searchQuestionContentHit, p.like)
	},
	scope: func(s *SearchService, q *gorm.DB, p searchParams) *gorm.DB {
		// 题库池口径单点（published + 排源标记真题题 + 当前证件），ADR-0050 决策 1。
		return QuestionPoolScope(q, p.cred)
	},
	selects: "id, content",
	// 无标题面：题目以题干为标题，命中判据就是题干本身 → 一级排序恒同，直接按 id 倒序。
	titleHit:  "",
	secondary: "id DESC",
	hit: func(s *SearchService, r *model.Question, p searchParams) (string, string) {
		return hitOf("", r.Content, p.keyword)
	},
	assemble: func(r model.Question, hitField, snippet string) SearchItemDTO {
		summary := snippetOf(r.Content, 50)
		return SearchItemDTO{
			Type: SearchTypeQuestion, ID: int64(r.ID),
			Title: summary, Summary: summary, Snippet: snippet, HitField: hitField,
		}
	},
}

// ===== 内容精选分区：匹配标题 + 摘要 + 正文；按发布时间倒序（内容时效）=====

const (
	searchContentTitleHit   = "LOWER(title) LIKE ? ESCAPE '\\'"
	searchContentSummaryHit = "LOWER(summary) LIKE ? ESCAPE '\\'"
	searchContentBodyHit    = "LOWER(content) LIKE ? ESCAPE '\\'"
)

var contentPartition = partitionSpec[model.FeaturedContent]{
	key: SearchTypeContent,
	match: func(s *SearchService, p searchParams) *gorm.DB {
		return s.db.Model(&model.FeaturedContent{}).
			Where("("+searchContentTitleHit+" OR "+searchContentSummaryHit+" OR "+searchContentBodyHit+")", p.like, p.like, p.like)
	},
	scope: func(s *SearchService, q *gorm.DB, p searchParams) *gorm.DB {
		return q.Where("status = 1")
	},
	selects:   "content_id, title, cover_image, summary, content",
	titleHit:  searchContentTitleHit,
	secondary: "COALESCE(published_at, created_at) DESC, content_id DESC",
	hit: func(s *SearchService, r *model.FeaturedContent, p searchParams) (string, string) {
		body := r.Content
		if body == "" {
			body = r.Summary
		}
		hitField, snippet := hitOf(r.Title, body, p.keyword)
		if hitField == SearchHitBody && !containsFold(body, p.keyword) {
			// 同上：命中落在 summary 而正文更长时，片段跟着命中的字段走
			if s2, ok := snippetAround(r.Summary, p.keyword); ok {
				snippet = s2
			}
		}
		return hitField, snippet
	},
	assemble: func(r model.FeaturedContent, hitField, snippet string) SearchItemDTO {
		body := r.Content
		if body == "" {
			body = r.Summary
		}
		return SearchItemDTO{
			Type: SearchTypeContent, ID: int64(r.ContentID), Title: r.Title, Cover: r.CoverImage,
			Summary: snippetOf(r.Summary, 80), Snippet: snippet, HitField: hitField,
		}
	},
}

// ===== 论坛主题分区：匹配标题 + 正文 + **回复正文**；时效内容 → 活跃度排序 =====

const (
	searchTopicTitleHit = "LOWER(title) LIKE ? ESCAPE '\\'"
	searchTopicBodyHit  = "LOWER(content) LIKE ? ESCAPE '\\'"
	// 回复命中用 EXISTS 子查询进匹配面（ADR-0049：回复正文属可检索面）。
	searchTopicReplyHit = "EXISTS (SELECT 1 FROM forum_replies fr WHERE fr.topic_id = forum_topics.id AND LOWER(fr.content) LIKE ? ESCAPE '\\')"
)

var topicPartition = partitionSpec[model.ForumTopic]{
	key: SearchTypeTopic,
	match: func(s *SearchService, p searchParams) *gorm.DB {
		return s.db.Model(&model.ForumTopic{}).
			Where("("+searchTopicTitleHit+" OR "+searchTopicBodyHit+" OR "+searchTopicReplyHit+")", p.like, p.like, p.like)
	},
	// 帖子物理删除即消失：无额外 scope 谓词。
	selects:  "id, title, content",
	titleHit: searchTopicTitleHit,
	secondary: "is_featured DESC, CASE WHEN accepted_reply_id IS NOT NULL THEN 0 ELSE 1 END, " +
		"COALESCE(last_reply_at, created_at) DESC, id DESC",
	hit: func(s *SearchService, r *model.ForumTopic, p searchParams) (string, string) {
		// 命中在回复时必须能标注 reply 并给出回复片段：标题与正文都未命中时回查首条命中回复。
		reply := ""
		if !containsFold(r.Title, p.keyword) && !containsFold(r.Content, p.keyword) {
			var fr model.ForumReply
			if err := s.db.Model(&model.ForumReply{}).
				Where("topic_id = ? AND LOWER(content) LIKE ? ESCAPE '\\'", r.ID, p.like).
				Order("id ASC").First(&fr).Error; err == nil {
				reply = fr.Content
			}
		}
		return hitOfTopic(r.Title, r.Content, reply, p.keyword)
	},
	assemble: func(r model.ForumTopic, hitField, snippet string) SearchItemDTO {
		return SearchItemDTO{
			Type: SearchTypeTopic, ID: r.ID, Title: r.Title,
			Summary: snippetOf(r.Content, 80), Snippet: snippet, HitField: hitField,
		}
	},
}

// searchPartitions 分区表（声明顺序 = 聚合响应字段顺序）。加分区只写一份 partitionSpec + 这里一行。
// 引擎与聚合循环都遍历本表，不存在「加了分区忘了登记 switch」的第四处。
var searchPartitions = []searchPartitionRunner{
	bind(coursePartition),
	bind(chapterPartition),
	bind(questionPartition),
	bind(contentPartition),
	bind(topicPartition),
}

// searchPartitionByKey 按类型键取分区（分发 = 查表）。
func searchPartitionByKey(key string) (searchPartitionRunner, bool) {
	for _, part := range searchPartitions {
		if part.key == key {
			return part, true
		}
	}
	return searchPartitionRunner{}, false
}

// searchPartitionKeys 分区键（声明顺序）：未知类型的错误文案与一致性断言同源。
func searchPartitionKeys() []string {
	keys := make([]string, 0, len(searchPartitions))
	for _, part := range searchPartitions {
		keys = append(keys, part.key)
	}
	return keys
}
