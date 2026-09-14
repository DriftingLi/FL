// Package service 实现业务服务层。
// 本文件：全局搜索（ADR-0018 引入，ADR-0049 定口径）—— course/chapter/question/content/topic
// 五个分区的 LOWER LIKE 聚合（Postgres/SQLite 双兼容，元字符转义）。
//
// 口径要点（ADR-0049）：
//   - 职责 = 主题检索：命中必须可解释（命中位置 hit_field + 命中片段 snippet）；
//   - 每个分区自带可见性谓词（课程挂载不变式 / 题库池 / 精选已发布 / 帖子物理删除即消失）；
//   - 排序 = 一级「标题命中优先」，二级按内容性质分派（常青内容用编辑信号，时效内容用活跃度）；
//   - 检索事实匿名落库（关键词 + 命中数 + 时间，不指向人）；搜索历史不入服务端。
package service

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// maxSearchKeywordLen 关键词长度上限（按**字符**计，非字节）：超长关键词只会拖慢顺序 LIKE，
// 对学员没有任何可用性收益（#980）。
const maxSearchKeywordLen = 200

// 搜索类型（分区）。
const (
	SearchTypeCourse   = "course"
	SearchTypeChapter  = "chapter"
	SearchTypeQuestion = "question"
	SearchTypeContent  = "content"
	SearchTypeTopic    = "topic"
)

// 命中位置（ADR-0049 决策 6）：结果必须说清「在哪儿命中」。
const (
	SearchHitTitle = "title"
	SearchHitBody  = "body"
	SearchHitReply = "reply"
)

// 命中窗口：以首个命中位置为中心截取（前 snippetLead 个字符 + 共 snippetWidth 个字符），
// 而不是正文开头——长正文里两者几乎无关。
const (
	snippetWidth = 80
	snippetLead  = 30
)

// escapeLike 转义 LIKE 的元字符：用户串必须按**字面**匹配——
// 否则「100%」会退化成「以 100 开头」、「%」直接命中全表（#980）。
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// likePattern 构造 LIKE 模式串；与 escapeLike 成对使用，查询侧一律带 ESCAPE 子句。
func likePattern(keyword string) string {
	return "%" + escapeLike(strings.ToLower(keyword)) + "%"
}

// SearchService 全局搜索服务。
type SearchService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewSearchService 构造全局搜索服务。
func NewSearchService(db *gorm.DB, logger *zap.Logger) *SearchService {
	return &SearchService{db: db, logger: logger}
}

// SearchItemDTO 搜索结果条目。
type SearchItemDTO struct {
	Type  string `json:"type"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Cover string `json:"cover"`
	// Summary 开头截断的旧口径，**保留**：移动端老客户端仍读它（ADR-0048 契约只增不破）。
	Summary string `json:"summary"`
	// Snippet 命中片段：源串中首个命中位置前后的窗口（ADR-0049 决策 6）。
	Snippet string `json:"snippet"`
	// HitField 命中位置：title | body | reply。
	HitField string `json:"hit_field"`
	// ParentID 章节结果所属课程 ID（其余类型为 0）——章节落点需要课程与章节两个参数。
	ParentID int64 `json:"parent_id"`
}

// SearchSectionDTO 分区结果（全部搜索时每类 top N）。
type SearchSectionDTO struct {
	Items []SearchItemDTO `json:"items"`
	Total int64           `json:"total"`
}

// SearchAllDTO 全部搜索结果（各分区 top 5 + 总数）。
type SearchAllDTO struct {
	Keyword   string           `json:"keyword"`
	Courses   SearchSectionDTO `json:"courses"`
	Chapters  SearchSectionDTO `json:"chapters"`
	Questions SearchSectionDTO `json:"questions"`
	Contents  SearchSectionDTO `json:"contents"`
	Topics    SearchSectionDTO `json:"topics"`
}

// SearchPageDTO 指定类型搜索结果（分页）。
type SearchPageDTO struct {
	Keyword string          `json:"keyword"`
	Type    string          `json:"type"`
	Total   int64           `json:"total"`
	Page    int             `json:"page"`
	Pages   int             `json:"pages"`
	Items   []SearchItemDTO `json:"items"`
}

// ZeroResultKeywordDTO 零结果词（运营面，ADR-0049 决策 7）。
type ZeroResultKeywordDTO struct {
	Keyword    string `json:"keyword"`
	Times      int64  `json:"times"`
	LastSeenAt string `json:"last_seen_at"`
}

// findRuneIndex 在 source 中按**字符**定位 keyword 的首次出现（大小写不敏感）。
// 不用 strings.Index 的原因：Unicode 大小写折叠会改变字节长度，按字节下标切片会切错位置。
func findRuneIndex(source, keyword string) int {
	src := []rune(source)
	kw := []rune(keyword)
	if len(kw) == 0 || len(kw) > len(src) {
		return -1
	}
	for i := 0; i+len(kw) <= len(src); i++ {
		matched := true
		for j := range kw {
			if unicode.ToLower(src[i+j]) != unicode.ToLower(kw[j]) {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

// snippetAround 命中窗口：以首个命中位置为中心取窗口；未命中时 ok=false。
func snippetAround(source, keyword string) (string, bool) {
	if source == "" || keyword == "" {
		return "", false
	}
	idx := findRuneIndex(source, keyword)
	if idx < 0 {
		return "", false
	}
	runes := []rune(source)
	start := idx - snippetLead
	if start < 0 {
		start = 0
	}
	end := start + snippetWidth
	if end > len(runes) {
		end = len(runes)
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out = out + "…"
	}
	return out, true
}

// hitOf 单标题 + 单正文分区的命中位置与片段：标题命中优先，片段优先给**正文**窗口
// （标题本身已经在 title 字段里，重复它没有信息量）。
func hitOf(title, body, keyword string) (string, string) {
	if _, ok := snippetAround(title, keyword); ok {
		if s, bodyHit := snippetAround(body, keyword); bodyHit {
			return SearchHitTitle, s
		}
		return SearchHitTitle, snippetOf(body, snippetWidth)
	}
	if s, ok := snippetAround(body, keyword); ok {
		return SearchHitBody, s
	}
	return SearchHitBody, snippetOf(body, snippetWidth)
}

// hitOfTopic 论坛主题三轴命中：标题 > 正文 > 回复（回复只有在标题与正文都未命中时才算回复命中）。
func hitOfTopic(title, body, reply, keyword string) (string, string) {
	if _, ok := snippetAround(title, keyword); ok {
		if s, bodyHit := snippetAround(body, keyword); bodyHit {
			return SearchHitTitle, s
		}
		if s, replyHit := snippetAround(reply, keyword); replyHit {
			return SearchHitTitle, s
		}
		return SearchHitTitle, snippetOf(body, snippetWidth)
	}
	if s, ok := snippetAround(body, keyword); ok {
		return SearchHitBody, s
	}
	if s, ok := snippetAround(reply, keyword); ok {
		return SearchHitReply, s
	}
	return SearchHitBody, snippetOf(body, snippetWidth)
}

// orderWithHitRank 一级排序「标题命中优先」+ 分区二级键。标题命中判据带占位符（用户串必须参数化）。
func orderWithHitRank(titleHitSQL, secondary, like string) clause.OrderBy {
	if titleHitSQL == "" {
		return clause.OrderBy{Columns: []clause.OrderByColumn{{Column: clause.Column{Name: secondary, Raw: true}}}}
	}
	return clause.OrderBy{Expression: clause.Expr{
		SQL:  "CASE WHEN " + titleHitSQL + " THEN 0 ELSE 1 END, " + secondary,
		Vars: []interface{}{like},
	}}
}

// searchSection 单类型搜索：top limit 条 + 总数。
func (s *SearchService) searchSection(searchType, keyword string, limit int, credentialID ...*int) (SearchSectionDTO, error) {
	items, total, err := s.searchItems(searchType, keyword, 1, limit, credentialID...)
	if err != nil {
		return SearchSectionDTO{}, err
	}
	return SearchSectionDTO{Items: items, Total: total}, nil
}

// searchItems 单类型分页搜索。
func (s *SearchService) searchItems(searchType, keyword string, page, pageSize int, credentialID ...*int) ([]SearchItemDTO, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	like := likePattern(keyword)
	var cred *int
	if len(credentialID) > 0 {
		cred = credentialID[0]
	}

	switch searchType {
	case SearchTypeCourse:
		return s.searchCourses(keyword, like, page, pageSize, cred)
	case SearchTypeChapter:
		return s.searchChapters(keyword, like, page, pageSize, cred)
	case SearchTypeQuestion:
		return s.searchQuestions(keyword, like, page, pageSize, cred)
	case SearchTypeContent:
		return s.searchContents(keyword, like, page, pageSize)
	case SearchTypeTopic:
		return s.searchTopics(keyword, like, page, pageSize)
	default:
		return nil, 0, errors.New("搜索类型仅支持 course/chapter/question/content/topic")
	}
}

// searchCourses 课程分区：匹配课名 + 简介；常青内容 → 编辑信号排序。
func (s *SearchService) searchCourses(keyword, like string, page, pageSize int, cred *int) ([]SearchItemDTO, int64, error) {
	const titleHit = "LOWER(name) LIKE ? ESCAPE '\\'"
	bodyHit := "LOWER(description) LIKE ? ESCAPE '\\'"
	q := s.db.Model(&model.Course{}).
		Where("status = 1 AND specialty_id IS NOT NULL AND level_id IS NOT NULL").
		Where("("+titleHit+" OR "+bodyHit+")", like, like)
	if cred != nil {
		q = q.Where("credential_id = ?", *cred)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.Course
	if err := q.Select("course_id, name, cover_image, description").
		Order(orderWithHitRank(titleHit, "sort_order ASC, is_hot DESC, is_featured DESC, course_id ASC", like)).
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for _, r := range rows {
		hitField, snippet := hitOf(r.Name, r.Description, keyword)
		items = append(items, SearchItemDTO{
			Type: SearchTypeCourse, ID: int64(r.CourseID), Title: r.Name, Cover: r.CoverImage,
			Summary: snippetOf(r.Description, 80), Snippet: snippet, HitField: hitField,
		})
	}
	return items, total, nil
}

// searchChapters 章节分区：匹配章节标题 + 正文 + 描述；可见性跟随所属课程（已发布 + 挂载不变式）。
func (s *SearchService) searchChapters(keyword, like string, page, pageSize int, cred *int) ([]SearchItemDTO, int64, error) {
	const titleHit = "LOWER(chapter.title) LIKE ? ESCAPE '\\'"
	contentHit := "LOWER(chapter.content) LIKE ? ESCAPE '\\'"
	descHit := "LOWER(chapter.description) LIKE ? ESCAPE '\\'"
	mounted := s.db.Model(&model.Course{}).Select("course_id").
		Where("status = 1 AND specialty_id IS NOT NULL AND level_id IS NOT NULL")
	if cred != nil {
		mounted = mounted.Where("credential_id = ?", *cred)
	}
	q := s.db.Model(&model.Chapter{}).
		Where("course_id IN (?)", mounted).
		Where("("+titleHit+" OR "+contentHit+" OR "+descHit+")", like, like, like)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.Chapter
	if err := q.Select("chapter_id, course_id, title, content, description").
		Order(orderWithHitRank(titleHit, "order_num ASC, chapter_id ASC", like)).
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for _, r := range rows {
		body := r.Content
		if body == "" {
			body = r.Description
		}
		hitField, snippet := hitOf(r.Title, body, keyword)
		if hitField == SearchHitBody && !containsFold(body, keyword) {
			// 命中在 description 而 content 非空：片段改用 description，避免片段与命中无关
			if s2, ok := snippetAround(r.Description, keyword); ok {
				snippet = s2
			}
		}
		items = append(items, SearchItemDTO{
			Type: SearchTypeChapter, ID: int64(r.ChapterID), ParentID: int64(r.CourseID),
			Title: r.Title, Summary: snippetOf(body, 80), Snippet: snippet, HitField: hitField,
		})
	}
	return items, total, nil
}

// searchQuestions 题目分区：只匹配题干（解析与答案**不进**匹配面，ADR-0049「明确不做」）；
// 走题库池口径（published + 排源标记真题题 + 当前证件）。
func (s *SearchService) searchQuestions(keyword, like string, page, pageSize int, cred *int) ([]SearchItemDTO, int64, error) {
	q := s.db.Model(&model.Question{}).
		Where("status = ? AND LOWER(content) LIKE ? ESCAPE '\\'", "published", like).
		Where(excludeSourceTagsSQL)
	if cred != nil {
		q = q.Where("credential_id = ?", *cred)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.Question
	if err := q.Select("id, content").
		Order("id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for _, r := range rows {
		hitField, snippet := hitOf("", r.Content, keyword)
		items = append(items, SearchItemDTO{
			Type: SearchTypeQuestion, ID: int64(r.ID),
			Title: snippetOf(r.Content, 50), Summary: snippetOf(r.Content, 50),
			Snippet: snippet, HitField: hitField,
		})
	}
	return items, total, nil
}

// searchContents 内容精选分区：匹配标题 + 摘要 + 正文；按发布时间倒序（内容时效）。
func (s *SearchService) searchContents(keyword, like string, page, pageSize int) ([]SearchItemDTO, int64, error) {
	const titleHit = "LOWER(title) LIKE ? ESCAPE '\\'"
	summaryHit := "LOWER(summary) LIKE ? ESCAPE '\\'"
	bodyHit := "LOWER(content) LIKE ? ESCAPE '\\'"
	q := s.db.Model(&model.FeaturedContent{}).
		Where("status = 1").
		Where("("+titleHit+" OR "+summaryHit+" OR "+bodyHit+")", like, like, like)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []model.FeaturedContent
	if err := q.Select("content_id, title, cover_image, summary, content").
		Order(orderWithHitRank(titleHit, "COALESCE(published_at, created_at) DESC, content_id DESC", like)).
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for _, r := range rows {
		body := r.Content
		if body == "" {
			body = r.Summary
		}
		hitField, snippet := hitOf(r.Title, body, keyword)
		if hitField == SearchHitBody && !containsFold(body, keyword) {
			// 同上：命中落在 summary 而正文更长时，片段跟着命中的字段走
			if s2, ok := snippetAround(r.Summary, keyword); ok {
				snippet = s2
			}
		}
		items = append(items, SearchItemDTO{
			Type: SearchTypeContent, ID: int64(r.ContentID), Title: r.Title, Cover: r.CoverImage,
			Summary: snippetOf(r.Summary, 80), Snippet: snippet, HitField: hitField,
		})
	}
	return items, total, nil
}

// searchTopics 论坛主题分区：匹配标题 + 正文 + **回复正文**；时效内容 → 活跃度排序
// （帖子精选 / 已采纳优先，再按最近回复）。命中在回复时必须标注 reply。
func (s *SearchService) searchTopics(keyword, like string, page, pageSize int) ([]SearchItemDTO, int64, error) {
	const titleHit = "LOWER(title) LIKE ? ESCAPE '\\'"
	bodyHit := "LOWER(content) LIKE ? ESCAPE '\\'"
	replyHit := "EXISTS (SELECT 1 FROM forum_replies fr WHERE fr.topic_id = forum_topics.id AND LOWER(fr.content) LIKE ? ESCAPE '\\')"
	q := s.db.Model(&model.ForumTopic{}).
		Where("("+titleHit+" OR "+bodyHit+" OR "+replyHit+")", like, like, like)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	secondary := "is_featured DESC, CASE WHEN accepted_reply_id IS NOT NULL THEN 0 ELSE 1 END, " +
		"COALESCE(last_reply_at, created_at) DESC, id DESC"
	var rows []model.ForumTopic
	if err := q.Select("id, title, content").
		Order(orderWithHitRank(titleHit, secondary, like)).
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for _, r := range rows {
		reply := ""
		if !containsFold(r.Title, keyword) && !containsFold(r.Content, keyword) {
			var fr model.ForumReply
			if err := s.db.Model(&model.ForumReply{}).
				Where("topic_id = ? AND LOWER(content) LIKE ? ESCAPE '\\'", r.ID, like).
				Order("id ASC").First(&fr).Error; err == nil {
				reply = fr.Content
			}
		}
		hitField, snippet := hitOfTopic(r.Title, r.Content, reply, keyword)
		items = append(items, SearchItemDTO{
			Type: SearchTypeTopic, ID: r.ID, Title: r.Title,
			Summary: snippetOf(r.Content, 80), Snippet: snippet, HitField: hitField,
		})
	}
	return items, total, nil
}

// containsFold 大小写不敏感的包含判定（按字符，不做 Unicode 折叠的长度假设）。
func containsFold(haystack, needle string) bool {
	return needle != "" && findRuneIndex(haystack, needle) >= 0
}

// Search 全局搜索。searchType 为空时返回各分区 top 5；否则该类型分页结果。
func (s *SearchService) Search(keyword, searchType string, page, pageSize int, credentialID ...*int) (any, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("关键词不能为空")
	}
	if utf8.RuneCountInString(keyword) > maxSearchKeywordLen {
		return nil, fmt.Errorf("关键词过长（最多 %d 个字符）", maxSearchKeywordLen)
	}
	var cred *int
	if len(credentialID) > 0 {
		cred = credentialID[0]
	}
	if searchType == "" {
		courses, err := s.searchSection(SearchTypeCourse, keyword, 5, cred)
		if err != nil {
			return nil, err
		}
		chapters, err := s.searchSection(SearchTypeChapter, keyword, 5, cred)
		if err != nil {
			return nil, err
		}
		questions, err := s.searchSection(SearchTypeQuestion, keyword, 5, cred)
		if err != nil {
			return nil, err
		}
		contents, err := s.searchSection(SearchTypeContent, keyword, 5, cred)
		if err != nil {
			return nil, err
		}
		topics, err := s.searchSection(SearchTypeTopic, keyword, 5, cred)
		if err != nil {
			return nil, err
		}
		s.recordSearchFact(keyword, "", searchFactCounts{
			course: courses.Total, chapter: chapters.Total, question: questions.Total,
			content: contents.Total, topic: topics.Total,
		})
		return &SearchAllDTO{
			Keyword: keyword, Courses: courses, Chapters: chapters,
			Questions: questions, Contents: contents, Topics: topics,
		}, nil
	}
	items, total, err := s.searchItems(searchType, keyword, page, pageSize, cred)
	if err != nil {
		return nil, err
	}
	s.recordSearchFact(keyword, searchType, countsForType(searchType, total))
	return &SearchPageDTO{
		Keyword: keyword, Type: searchType, Total: total,
		Page: page, Pages: response.PageCount(total, pageSize), Items: items,
	}, nil
}

// countsForType 指定类型搜索的命中数：只落在该分区，其余分区为 0。
func countsForType(searchType string, total int64) searchFactCounts {
	switch searchType {
	case SearchTypeCourse:
		return searchFactCounts{course: total}
	case SearchTypeChapter:
		return searchFactCounts{chapter: total}
	case SearchTypeQuestion:
		return searchFactCounts{question: total}
	case SearchTypeContent:
		return searchFactCounts{content: total}
	case SearchTypeTopic:
		return searchFactCounts{topic: total}
	default:
		return searchFactCounts{}
	}
}

// searchFactCounts 一次搜索的各分区命中数（聚合搜索逐区落数，指定类型只落该区）。
type searchFactCounts struct {
	course   int64
	chapter  int64
	question int64
	content  int64
	topic    int64
}

// total 各分区命中数之和（= 该次搜索的总命中数）。
func (c searchFactCounts) total() int64 {
	return c.course + c.chapter + c.question + c.content + c.topic
}

// recordSearchFact 检索事实（ADR-0049 决策 7）：匿名、尽力而为——
// 埋点失败绝不影响搜索本身，也绝不记录 user / 证件 / 设备。
func (s *SearchService) recordSearchFact(keyword, searchType string, counts searchFactCounts) {
	fact := model.SearchFact{
		Keyword: keyword, SearchType: searchType,
		CourseHits: counts.course, ChapterHits: counts.chapter, QuestionHits: counts.question,
		ContentHits: counts.content, TopicHits: counts.topic,
		TotalHits: counts.total(), CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&fact).Error; err != nil && s.logger != nil {
		s.logger.Warn("记录检索事实失败", zap.Error(err))
	}
}

// normalizeDBTime 把驱动回传的时间串规范成 ISO；解析不出时**原样回传**（不猜、不丢）。
func normalizeDBTime(raw string) string {
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return formatISO(t)
		}
	}
	return raw
}

// ZeroResultKeywords 零结果词（运营面）：按关键词聚合次数与最近出现时间。
// days <= 0 取 30 天；limit 缺省 50，越界（<=0 或 >200）同样回落到 50。
func (s *SearchService) ZeroResultKeywords(days, limit int) ([]ZeroResultKeywordDTO, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	since := beijingNow().AddDate(0, 0, -days)
	// LastSeenAt 在**字符串面**上收口：SQLite 的 MAX(created_at) 回字符串、Postgres 回 timestamptz，
	// 扫描类型不同；统一取字符串再规范化成 ISO（ADR-0043 时间契约）。
	var rows []struct {
		Keyword    string
		Times      int64
		LastSeenAt string
	}
	if err := s.db.Model(&model.SearchFact{}).
		Select("keyword, COUNT(*) AS times, MAX(created_at) AS last_seen_at").
		// 零结果词 = **聚合搜索**（search_type 空串）且总命中为 0：
		// 指定类型搜索的 0 命中只说明「该分区没有」，不等于「平台没有」。
		Where("search_type = '' AND total_hits = 0 AND created_at >= ?", since).
		Group("keyword").
		Order("times DESC, last_seen_at DESC").
		Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ZeroResultKeywordDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, ZeroResultKeywordDTO{Keyword: r.Keyword, Times: r.Times, LastSeenAt: normalizeDBTime(r.LastSeenAt)})
	}
	return out, nil
}
