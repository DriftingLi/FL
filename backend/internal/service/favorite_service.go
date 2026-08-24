// Package service 实现业务服务层。
// 本文件：通用收藏（ADR-0018）—— target_type + target_id 多态收藏，
// 覆盖 course/chapter/question/featured/topic；user+type+id 唯一约束保证幂等。
package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// 收藏对象类型（多态 target_type）。
const (
	FavoriteTargetCourse   = "course"
	FavoriteTargetChapter  = "chapter"
	FavoriteTargetQuestion = "question"
	FavoriteTargetFeatured = "featured"
	FavoriteTargetTopic    = "topic"
)

// FavoriteService 通用收藏服务。
type FavoriteService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewFavoriteService 构造通用收藏服务。
func NewFavoriteService(db *gorm.DB, logger *zap.Logger) *FavoriteService {
	return &FavoriteService{db: db, logger: logger}
}

// FavoriteDTO 收藏条目（带目标快照：标题/封面，目标已删除时该行不出现）。
type FavoriteDTO struct {
	FavoriteID int64  `json:"favorite_id"`
	TargetType string `json:"target_type"`
	TargetID   int    `json:"target_id"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	CreatedAt  string `json:"created_at"`
}

// FavoritePageResult 收藏分页结果。
type FavoritePageResult struct {
	Page      int           `json:"page"`
	Pages     int           `json:"pages"`
	Total     int64         `json:"total"`
	Favorites []FavoriteDTO `json:"favorites"`
}

// favoriteTargetMeta 目标快照。
type favoriteTargetMeta struct {
	Title string
	Cover string
	Found bool
}

// validateFavoriteTarget 校验收藏目标类型合法且存在/可见。
// 课程要求已发布且挂载（挂载不变式与学员端列表口径一致）；题目要求已发布；
// 精选内容要求已发布；章节与帖子仅要求存在。
func validateFavoriteTarget(db *gorm.DB, targetType string, targetID int) error {
	switch targetType {
	case FavoriteTargetCourse:
		var cnt int64
		db.Model(&model.Course{}).
			Where("course_id = ? AND status = 1 AND specialty_id IS NOT NULL AND level_id IS NOT NULL", targetID).
			Count(&cnt)
		if cnt == 0 {
			return errors.New("课程不存在或不可收藏")
		}
	case FavoriteTargetChapter:
		var cnt int64
		db.Model(&model.Chapter{}).Where("chapter_id = ?", targetID).Count(&cnt)
		if cnt == 0 {
			return errors.New("章节不存在")
		}
	case FavoriteTargetQuestion:
		var cnt int64
		db.Model(&model.Question{}).Where("id = ? AND status = ?", targetID, "published").Count(&cnt)
		if cnt == 0 {
			return errors.New("题目不存在或不可收藏")
		}
	case FavoriteTargetFeatured:
		var cnt int64
		db.Model(&model.FeaturedContent{}).Where("content_id = ? AND status = 1", targetID).Count(&cnt)
		if cnt == 0 {
			return errors.New("内容不存在或不可收藏")
		}
	case FavoriteTargetTopic:
		var cnt int64
		db.Model(&model.ForumTopic{}).Where("id = ?", targetID).Count(&cnt)
		if cnt == 0 {
			return errors.New("帖子不存在")
		}
	default:
		return errors.New("收藏类型仅支持 course/chapter/question/featured/topic")
	}
	return nil
}

// favoriteTargetsMeta 批量回填目标快照（按类型分组查询，消除 N+1）。
func favoriteTargetsMeta(db *gorm.DB, targetType string, ids []int) map[int]favoriteTargetMeta {
	result := make(map[int]favoriteTargetMeta, len(ids))
	if len(ids) == 0 {
		return result
	}
	switch targetType {
	case FavoriteTargetCourse:
		var rows []model.Course
		db.Select("course_id, name, cover_image").Where("course_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[r.CourseID] = favoriteTargetMeta{Title: r.Name, Cover: r.CoverImage, Found: true}
		}
	case FavoriteTargetChapter:
		var rows []model.Chapter
		db.Select("chapter_id, title").Where("chapter_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[r.ChapterID] = favoriteTargetMeta{Title: r.Title, Found: true}
		}
	case FavoriteTargetQuestion:
		var rows []model.Question
		db.Select("id, content, image_url").Where("id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[r.ID] = favoriteTargetMeta{Title: snippetOf(r.Content, 50), Cover: r.ImageURL, Found: true}
		}
	case FavoriteTargetFeatured:
		var rows []model.FeaturedContent
		db.Select("content_id, title, cover_image").Where("content_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[r.ContentID] = favoriteTargetMeta{Title: r.Title, Cover: r.CoverImage, Found: true}
		}
	case FavoriteTargetTopic:
		var rows []model.ForumTopic
		db.Select("id, title").Where("id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[int(r.ID)] = favoriteTargetMeta{Title: r.Title, Found: true}
		}
	}
	return result
}

// Add 收藏（幂等：已收藏直接返回既有条目）。
func (s *FavoriteService) Add(userID int, targetType string, targetID int) (*FavoriteDTO, error) {
	targetType = strings.TrimSpace(targetType)
	if targetID <= 0 {
		return nil, errors.New("收藏目标 ID 无效")
	}
	if err := validateFavoriteTarget(s.db, targetType, targetID); err != nil {
		return nil, err
	}
	var existing model.Favorite
	if err := s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Limit(1).Find(&existing).Error; err != nil {
		return nil, err
	}
	if existing.FavoriteID == 0 {
		existing = model.Favorite{
			UserID: userID, TargetType: targetType, TargetID: targetID, CreatedAt: beijingNow(),
		}
		if err := s.db.Create(&existing).Error; err != nil {
			return nil, err
		}
	}
	dto := favoriteToDTO(&existing)
	if meta, ok := favoriteTargetsMeta(s.db, targetType, []int{targetID})[targetID]; ok {
		dto.Title, dto.Cover = meta.Title, meta.Cover
	}
	return &dto, nil
}

// Remove 取消收藏（仅本人；条目不存在报错）。
func (s *FavoriteService) Remove(userID int, favoriteID int64) error {
	res := s.db.Where("favorite_id = ? AND user_id = ?", favoriteID, userID).
		Delete(&model.Favorite{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("收藏不存在")
	}
	return nil
}

// List 我的收藏列表（targetType 可选过滤；目标已删除的条目跳过）。
func (s *FavoriteService) List(userID int, targetType string, page, pageSize int, credentialID ...*int) (*FavoritePageResult, error) {
	targetType = strings.TrimSpace(targetType)
	rows, total, page, pageSize := paging.QueryWithMax[model.Favorite](s.db, page, pageSize, 20, 100,
		"created_at DESC, favorite_id DESC",
		func(q *gorm.DB) *gorm.DB {
			q = q.Where("user_id = ?", userID)
			if targetType != "" {
				q = q.Where("target_type = ?", targetType)
			}
			if len(credentialID) > 0 && credentialID[0] != nil && (targetType == FavoriteTargetCourse || targetType == FavoriteTargetQuestion) {
				if targetType == FavoriteTargetCourse {
					q = q.Where("target_id IN (SELECT course_id FROM course WHERE credential_id = ?)", *credentialID[0])
				} else {
					q = q.Where("target_id IN (SELECT id FROM question WHERE credential_id = ?)", *credentialID[0])
				}
			} else if len(credentialID) > 0 && credentialID[0] != nil && targetType == "" {
				// 混合类型时，仅过滤 course/question 分区，其余类型保持
				q = q.Where("(target_type NOT IN (?, ?) OR (target_type = ? AND target_id IN (SELECT course_id FROM course WHERE credential_id = ?)) OR (target_type = ? AND target_id IN (SELECT id FROM question WHERE credential_id = ?)))", FavoriteTargetCourse, FavoriteTargetQuestion, FavoriteTargetCourse, *credentialID[0], FavoriteTargetQuestion, *credentialID[0])
			}
			return q
		})
	items := make([]FavoriteDTO, 0, len(rows))
	if len(rows) > 0 {
		byType := make(map[string][]int)
		for _, r := range rows {
			byType[r.TargetType] = append(byType[r.TargetType], r.TargetID)
		}
		metas := make(map[string]map[int]favoriteTargetMeta, len(byType))
		for t, ids := range byType {
			metas[t] = favoriteTargetsMeta(s.db, t, ids)
		}
		for _, r := range rows {
			dto := favoriteToDTO(&r)
			if meta, ok := metas[r.TargetType][r.TargetID]; ok && meta.Found {
				dto.Title, dto.Cover = meta.Title, meta.Cover
				items = append(items, dto)
			}
		}
	}
	return &FavoritePageResult{
		Page: page, Pages: response.PageCount(total, pageSize),
		Total: total, Favorites: items,
	}, nil
}

// FavoriteCheckDTO 收藏状态查询结果。
type FavoriteCheckDTO struct {
	Favorited  bool  `json:"favorited"`
	FavoriteID int64 `json:"favorite_id"`
}

// Check 查询目标是否已收藏。
func (s *FavoriteService) Check(userID int, targetType string, targetID int) (*FavoriteCheckDTO, error) {
	targetType = strings.TrimSpace(targetType)
	if targetID <= 0 {
		return nil, errors.New("收藏目标 ID 无效")
	}
	var row model.Favorite
	if err := s.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Limit(1).Find(&row).Error; err != nil {
		return nil, err
	}
	return &FavoriteCheckDTO{Favorited: row.FavoriteID > 0, FavoriteID: row.FavoriteID}, nil
}

// favoriteToDTO 基础 DTO（不含快照）。
func favoriteToDTO(f *model.Favorite) FavoriteDTO {
	return FavoriteDTO{
		FavoriteID: f.FavoriteID,
		TargetType: f.TargetType,
		TargetID:   f.TargetID,
		CreatedAt:  formatISO(f.CreatedAt),
	}
}

// snippetOf 截取前 n 个 rune 作为摘要（超出加省略号）。
func snippetOf(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}

var _ = time.Now
