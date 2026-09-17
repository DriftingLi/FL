// Package service 实现业务服务层。
// 本文件：全局搜索（ADR-0018 引入，ADR-0049 定口径，ADR-0050 决策 2 收敛为分区半描述符）——
// course/chapter/question/content/topic 五个分区的 LOWER LIKE 聚合（Postgres/SQLite 双兼容，
// 元字符转义）。本文件是**引擎骨架 + 共享 helper**；五个分区的六槽位声明在 search_partitions.go。
//
// 口径要点（ADR-0049）：
//   - 职责 = 主题检索：命中必须可解释（命中位置 hit_field + 命中片段 snippet）；
//   - 每个分区自带可见性谓词（课程挂载不变式 / 题库池 / 精选已发布 / 帖子物理删除即消失）；
//   - 排序 = 一级「标题命中优先」，二级按内容性质分派（常青内容用编辑信号，时效内容用活跃度）；
//   - 检索事实匿名落库（关键词 + 命中数 + 时间，不指向人）；搜索历史不入服务端。
//
// 分发与聚合都走分区声明表（searchPartitions）：加分区只写一份声明，不存在
// 「switch / DTO / 方法体 / handler 四处同步」的手拼面。
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
	"forklift-training/pkg/paging"
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

// searchAllSectionLimit 聚合搜索每个分区的 top N（ADR-0049；契约：SearchAllDTO 各分区最多 5 条）。
const searchAllSectionLimit = 5

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

// 搜索分页的钳制口径：默认页大小 20、上限 100，越界（<=0 或 >100）**回退默认值**（不截断到上限）。
// 实现在 paging.ClampMax（装配单点的钳制面），本文件不再自留第二份（ADR-0056 §1）。
const (
	searchDefaultPageSize = 20
	searchMaxPageSize     = 100
)

// searchParams 引擎骨架的入参面：分区声明之外的全部差异（关键词、命中模式串、分页、证件分区）。
// page/pageSize 保存**原始入参**：钳制在 searchPartitionPage 里由 paging 单点完成。
type searchParams struct {
	keyword  string
	like     string
	page     int
	pageSize int
	cred     *int
}

// newSearchParams 构造 LIKE 模式串 + 原始分页参数（钳制见 searchPartitionPage）。
func newSearchParams(keyword string, page, pageSize int, cred *int) searchParams {
	return searchParams{keyword: keyword, like: likePattern(keyword), page: page, pageSize: pageSize, cred: cred}
}

// searchPartitionRunner 分区声明的类型擦除形态：行类型 R 由 bind 实例化时固定，
// 引擎与聚合循环只看得到「键 + 分页入口」，因此分区表可以是同质切片。
type searchPartitionRunner struct {
	key    string
	search func(s *SearchService, p searchParams) ([]SearchItemDTO, int64, error)
}

// bind 把泛型分区声明与引擎骨架实例化为一个类型擦除的分区条目。
// 新增分区 = 写一份 partitionSpec（search_partitions.go）+ 在 searchPartitions 登记一行。
func bind[R any](spec partitionSpec[R]) searchPartitionRunner {
	return searchPartitionRunner{key: spec.key, search: func(s *SearchService, p searchParams) ([]SearchItemDTO, int64, error) {
		return searchPartitionPage(s, spec, p)
	}}
}

// searchPartitionPage 分区搜索引擎骨架（唯一实现）：count → 命中排序 → 分页 scan → DTO 装配。
// 五个分区共用本函数；分区差异全部收敛在 partitionSpec 的槽位里（ADR-0050 决策 2）。
//
// 分页/错误模式收编到 paging.QueryWithScan（ADR-0056 §1 的装配单点；本处是最后一处手写 count/offset，
// 由 #1095 的新错误模式暴露）。逐处核对过的语义等价：
//   - 钳制：paging.ClampMax(page, pageSize, 20, 100)，与收编前 newSearchParams 的口径逐字一致
//     （越界**回退默认值**，不截断到上限）；本函数收到的 page/pageSize 是原始入参，钳制后用于 offset/limit。
//   - count 口径：build 里的 Select/Order 只作用于行查询——GORM 的 Count 用 count(*) 覆盖 SELECT 子句、
//     并在无 GROUP BY 时删掉 ORDER BY，故 count SQL 仍是 `SELECT count(*) FROM … WHERE …`，
//     与收编前「先 Count（只有 where/joins）再 Select+Order+分页」逐字同形。
//   - 命中排序（标题命中优先）：LIKE 模式串**必须参数化**，所以 clause.OrderBy 走 build 而不是
//     QueryWithScan 的 order 形参（后者是字符串，塞不下带 Vars 的 CASE 表达式）。
//   - 响应里的 page/pages 不经过本函数：Search 仍按**原始**入参算 response.PageCount(total, pageSize)
//     （越界页的响应字节零漂移；钳制只影响这一页取哪几行）。
func searchPartitionPage[R any](s *SearchService, spec partitionSpec[R], p searchParams) ([]SearchItemDTO, int64, error) {
	rows, total, _, _, err := paging.QueryWithScan[R](s.db, p.page, p.pageSize, searchDefaultPageSize, searchMaxPageSize, "",
		func(_ *gorm.DB) *gorm.DB {
			q := spec.match(s, p)
			if spec.scope != nil {
				q = spec.scope(s, q, p)
			}
			return q.Select(spec.selects).Order(orderWithHitRank(spec.titleHit, spec.secondary, p.like))
		})
	if err != nil {
		return nil, 0, err
	}
	items := make([]SearchItemDTO, 0, len(rows))
	for i := range rows {
		hitField, snippet := spec.hit(s, &rows[i], p)
		items = append(items, spec.assemble(rows[i], hitField, snippet))
	}
	return items, total, nil
}

// searchItems 单类型分页搜索：分发 = 分区表查表（无 switch——加分区不会漏改分发分支）。
func (s *SearchService) searchItems(searchType, keyword string, page, pageSize int, credentialID *int) ([]SearchItemDTO, int64, error) {
	part, ok := searchPartitionByKey(searchType)
	if !ok {
		return nil, 0, fmt.Errorf("搜索类型仅支持 %s", strings.Join(searchPartitionKeys(), "/"))
	}
	return part.search(s, newSearchParams(keyword, page, pageSize, credentialID))
}

// containsFold 大小写不敏感的包含判定（按字符，不做 Unicode 折叠的长度假设）。
func containsFold(haystack, needle string) bool {
	return needle != "" && findRuneIndex(haystack, needle) >= 0
}

// Search 全局搜索。searchType 为空时返回各分区 top 5；否则该类型分页结果。
func (s *SearchService) Search(keyword, searchType string, page, pageSize int, credentialID *int) (any, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("关键词不能为空")
	}
	if utf8.RuneCountInString(keyword) > maxSearchKeywordLen {
		return nil, fmt.Errorf("关键词过长（最多 %d 个字符）", maxSearchKeywordLen)
	}
	cred := credentialID
	if searchType == "" {
		// 聚合 = 遍历分区声明表逐区装配（top N + 总数）：响应形状零漂移，加分区只改声明表。
		sections := make(map[string]SearchSectionDTO, len(searchPartitions))
		counts := searchFactCounts{}
		for _, part := range searchPartitions {
			items, total, err := part.search(s, newSearchParams(keyword, 1, searchAllSectionLimit, cred))
			if err != nil {
				return nil, err
			}
			sections[part.key] = SearchSectionDTO{Items: items, Total: total}
			counts = counts.set(part.key, total)
		}
		s.recordSearchFact(keyword, "", counts)
		return &SearchAllDTO{
			Keyword: keyword,
			Courses: sections[SearchTypeCourse], Chapters: sections[SearchTypeChapter],
			Questions: sections[SearchTypeQuestion], Contents: sections[SearchTypeContent],
			Topics: sections[SearchTypeTopic],
		}, nil
	}
	items, total, err := s.searchItems(searchType, keyword, page, pageSize, cred)
	if err != nil {
		return nil, err
	}
	s.recordSearchFact(keyword, searchType, searchFactCounts{}.set(searchType, total))
	return &SearchPageDTO{
		Keyword: keyword, Type: searchType, Total: total,
		Page: page, Pages: response.PageCount(total, pageSize), Items: items,
	}, nil
}

// searchFactCounts 一次搜索的各分区命中数（聚合搜索逐区落数，指定类型只落该区）。
type searchFactCounts struct {
	course   int64
	chapter  int64
	question int64
	content  int64
	topic    int64
}

// set 按分区键落一处命中数（分区键 → 事实表列名的映射只此一处）。
func (c searchFactCounts) set(searchType string, total int64) searchFactCounts {
	switch searchType {
	case SearchTypeCourse:
		c.course = total
	case SearchTypeChapter:
		c.chapter = total
	case SearchTypeQuestion:
		c.question = total
	case SearchTypeContent:
		c.content = total
	case SearchTypeTopic:
		c.topic = total
	}
	return c
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
