// Package service 实现业务服务层。
// 本文件：全局搜索（ADR-0018）—— course/question/content/topic 多表 LIKE 聚合。
// 关键词匹配用 LOWER(col) LIKE LOWER(?)（Postgres/SQLite 双兼容）；
// 课程限已发布+挂载（挂载不变式）、题目限已发布且走题库池口径（排真题题，#386）、
// 精选限已发布、帖子全量（删除即物理删除）。
package service

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/response"
)

// maxSearchKeywordLen 关键词长度上限（按**字符**计，非字节）：超长关键词只会拖慢顺序 LIKE，
// 对学员没有任何可用性收益（#980）。
const maxSearchKeywordLen = 200

// escapeLike 转义 LIKE 的元字符（ % _）：用户串必须按**字面**匹配——
// 否则「100%」会退化成「以 100 开头」、「%」直接命中全表（#980）。
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// likePattern 构造 LIKE 模式串；与 escapeLike 成对使用，查询侧一律带 ESCAPE '\'。
func likePattern(keyword string) string {
	return "%" + escapeLike(strings.ToLower(keyword)) + "%"
}

// 搜索类型。
const (
	SearchTypeCourse   = "course"
	SearchTypeQuestion = "question"
	SearchTypeContent  = "content"
	SearchTypeTopic    = "topic"
)

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
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Cover   string `json:"cover"`
	Summary string `json:"summary"`
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

	var (
		total int64
		items []SearchItemDTO
	)
	switch searchType {
	case SearchTypeCourse:
		var cnt int64
		q := s.db.Model(&model.Course{}).
			Where("status = 1 AND specialty_id IS NOT NULL AND level_id IS NOT NULL AND LOWER(name) LIKE ? ESCAPE '\\'", like)
		if len(credentialID) > 0 && credentialID[0] != nil {
			q = q.Where("credential_id = ?", *credentialID[0])
		}
		q.Count(&cnt)
		total = cnt
		var rows []model.Course
		q.Select("course_id, name, cover_image, description").
			Order("course_id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
		for _, r := range rows {
			items = append(items, SearchItemDTO{
				Type: searchType, ID: int64(r.CourseID), Title: r.Name,
				Cover: r.CoverImage, Summary: snippetOf(r.Description, 80),
			})
		}
	case SearchTypeQuestion:
		var cnt int64
		q := s.db.Model(&model.Question{}).
			Where("status = ? AND LOWER(content) LIKE ? ESCAPE '\\'", "published", like).
			// 题库池口径（#386，同练习/模考抽题）：来源标记标签的真题题退出搜索，
			// 「真题题只经真题卷出现」的按套解锁门禁不因搜索豁免
			Where(excludeSourceTagsSQL)
		if len(credentialID) > 0 && credentialID[0] != nil {
			q = q.Where("credential_id = ?", *credentialID[0])
		}
		q.Count(&cnt)
		total = cnt
		var rows []model.Question
		q.Select("id, content").
			Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
		for _, r := range rows {
			items = append(items, SearchItemDTO{
				Type: searchType, ID: int64(r.ID), Title: snippetOf(r.Content, 50),
			})
		}
	case SearchTypeContent:
		var cnt int64
		q := s.db.Model(&model.FeaturedContent{}).
			Where("status = 1 AND (LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(summary) LIKE ? ESCAPE '\\')", like, like)
		q.Count(&cnt)
		total = cnt
		var rows []model.FeaturedContent
		q.Select("content_id, title, cover_image, summary").
			Order("content_id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
		for _, r := range rows {
			items = append(items, SearchItemDTO{
				Type: searchType, ID: int64(r.ContentID), Title: r.Title,
				Cover: r.CoverImage, Summary: snippetOf(r.Summary, 80),
			})
		}
	case SearchTypeTopic:
		var cnt int64
		q := s.db.Model(&model.ForumTopic{}).
			Where("LOWER(title) LIKE ? ESCAPE '\\' OR LOWER(content) LIKE ? ESCAPE '\\'", like, like)
		q.Count(&cnt)
		total = cnt
		var rows []model.ForumTopic
		q.Select("id, title, content").
			Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows)
		for _, r := range rows {
			items = append(items, SearchItemDTO{
				Type: searchType, ID: r.ID, Title: r.Title, Summary: snippetOf(r.Content, 80),
			})
		}
	default:
		return nil, 0, errors.New("搜索类型仅支持 course/question/content/topic")
	}
	if items == nil {
		items = []SearchItemDTO{}
	}
	return items, total, nil
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
		return &SearchAllDTO{
			Keyword: keyword, Courses: courses, Questions: questions, Contents: contents, Topics: topics,
		}, nil
	}
	items, total, err := s.searchItems(searchType, keyword, page, pageSize, cred)
	if err != nil {
		return nil, err
	}
	return &SearchPageDTO{
		Keyword: keyword, Type: searchType, Total: total,
		Page: page, Pages: response.PageCount(total, pageSize), Items: items,
	}, nil
}
