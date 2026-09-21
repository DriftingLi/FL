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
	// CourseID 目标所属课程ID：**仅 target_type = chapter 有意义** —— 章节落点
	// `chapter-view` 要 `course_id` + `chapter_id` 两个键（ADR-0014），而收藏表只存 target_id。
	// 其余类型恒为 0（不适用，不是「未知」）；键恒在、非 null（0 哨兵口径见 #1089 Q2）。
	CourseID  int    `json:"course_id"`
	Title     string `json:"title"`
	Cover     string `json:"cover"`
	CreatedAt string `json:"created_at"`
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
	// CourseID 目标所属课程（仅章节有意义，其余类型为 0）；与 FavoriteDTO.CourseID 同口径。
	CourseID int
	Found    bool
}

// validateFavoriteTarget 校验收藏目标类型合法且存在/可见。
// 课程要求已发布且挂载（挂载不变式与学员端列表口径一致）；题目要求**在题库池内**；
// 精选内容要求已发布；**章节的可见性跟随所属课程**（已发布 + 挂载不变式，与搜索的章节分区同一
// 谓词，见 #1132 —— course_mount_scope.go 的自述早已把「收藏目标校验」列为该谓词的消费方）；
// 帖子仅要求存在。
//
// 读面（favoriteTargetsMeta / List）**保持快照口径不变**：写时校验、读到的是当时的快照，
// 目标日后下架不会让收藏行消失（course 支的既有形状即如此）。
//
// qScope（ADR-0062 决策 4）只在题目支生效：修复前这里手拼 `status = 'published'`，
// 既不排源标记真题题也不分当前证件 ⇒ 收藏一道真题题后，经 GET /api/favorites 的题干快照
// 就把「真题题只经真题卷出现」的口径破了。题目支的判据宿主从此在 question_pool_scope.go。
func validateFavoriteTarget(db *gorm.DB, targetType string, targetID int, qScope QuestionReadScope) error {
	switch targetType {
	case FavoriteTargetCourse:
		// 复用学员可见性单点的 by-id 形态（ADR-0058），不在此手拼谓词。
		if !CourseVisibleByID(db, targetID) {
			return errors.New("课程不存在或不可收藏")
		}
	case FavoriteTargetChapter:
		var cnt int64
		// 章节可见性跟随课程：谓词复用挂载不变式单点，不手拼（#1132）。
		mounted := MountedCourseScope(db.Model(&model.Course{}).Select("course_id").Where("status = 1"))
		db.Model(&model.Chapter{}).Where("chapter_id = ? AND course_id IN (?)", targetID, mounted).Count(&cnt)
		if cnt == 0 {
			return errors.New("章节不存在或不可收藏")
		}
	case FavoriteTargetQuestion:
		// 题目支：题库池 by-id 判定（published + 排源标记真题题 + 当前证件），单点复用不手拼。
		if !qScope.VisibleByID(db, targetID) {
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
		// course_id 在**同一次查询**里一并取回（章节落点需要它，不新增往返）。
		db.Select("chapter_id, course_id, title").Where("chapter_id IN ?", ids).Find(&rows)
		for _, r := range rows {
			result[r.ChapterID] = favoriteTargetMeta{Title: r.Title, CourseID: r.CourseID, Found: true}
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
// qScope 由入口装配（ADR-0062 决策 4）：只有题目支消费它，其余目标类型不读该参数。
func (s *FavoriteService) Add(userID int, targetType string, targetID int, qScope QuestionReadScope) (*FavoriteDTO, error) {
	targetType = strings.TrimSpace(targetType)
	if targetID <= 0 {
		return nil, errors.New("收藏目标 ID 无效")
	}
	if err := validateFavoriteTarget(s.db, targetType, targetID, qScope); err != nil {
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
		// Add / List 两条路径共用同一 meta ⇒ course_id 同口径，零额外查询（#1089 Q2）。
		dto.Title, dto.Cover, dto.CourseID = meta.Title, meta.Cover, meta.CourseID
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

// favoriteTargetSubquery 收藏目标的归属分区子查询：course → course_id / question → id。
// 谓词由归属分区具名谓词给出（ADR-0056 §2）；credentialID 为 nil 时返回空串（调用方整支跳过，
// 不生成半截 SQL）。
func favoriteTargetSubquery(targetType string, credentialID *int) (string, []any) {
	clause, args := entityOwnedByClause("credential_id", credentialID)
	if clause == "" {
		return "", nil
	}
	table, column := "question", "id"
	if targetType == FavoriteTargetCourse {
		table, column = "course", "course_id"
	}
	return "SELECT " + column + " FROM " + table + " WHERE " + clause, args
}

// List 我的收藏列表（targetType 可选过滤；目标已删除的条目跳过）。
func (s *FavoriteService) List(userID int, targetType string, page, pageSize int, credentialID *int) (*FavoritePageResult, error) {
	targetType = strings.TrimSpace(targetType)
	rows, total, page, pageSize, err := paging.QueryWithMax[model.Favorite](s.db, page, pageSize, 20, 100,
		"created_at DESC, favorite_id DESC",
		func(q *gorm.DB) *gorm.DB {
			q = q.Where("user_id = ?", userID)
			if targetType != "" {
				q = q.Where("target_type = ?", targetType)
			}
			// 收藏目标的证件分区是**归属分区**（ADR-0056 §2）：读目标自身的证件列，nil = 不分区、看全部。
			// 「混合类型」分支只过滤 course/question 两个分区，其余类型保持（既有语义）。
			if credentialID != nil && (targetType == FavoriteTargetCourse || targetType == FavoriteTargetQuestion) {
				sub, args := favoriteTargetSubquery(targetType, credentialID)
				q = q.Where("target_id IN ("+sub+")", args...)
			} else if credentialID != nil && targetType == "" {
				courseSub, courseArgs := favoriteTargetSubquery(FavoriteTargetCourse, credentialID)
				questionSub, questionArgs := favoriteTargetSubquery(FavoriteTargetQuestion, credentialID)
				q = q.Where("(target_type NOT IN (?, ?) OR (target_type = ? AND target_id IN ("+courseSub+")) OR (target_type = ? AND target_id IN ("+questionSub+")))",
					FavoriteTargetCourse, FavoriteTargetQuestion, FavoriteTargetCourse, courseArgs[0], FavoriteTargetQuestion, questionArgs[0])
			}
			return q
		})
	if err != nil {
		return nil, err
	}
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
				dto.Title, dto.Cover, dto.CourseID = meta.Title, meta.Cover, meta.CourseID
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
