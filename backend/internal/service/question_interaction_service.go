package service

import (
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// QuestionCommentDTO 题目评论返回（带作者信息）
type QuestionCommentDTO struct {
	ID         int64  `json:"id"`
	QuestionID int    `json:"question_id"`
	UserID     int    `json:"user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
	Username   string `json:"username"`
	AvatarURL  string `json:"avatar_url"`
}

// QuestionCommentPageResult 题目评论分页结果（spec #962 片四：收口自 handler 内联 gin.H）。
//
// 字段按 JSON key 字母序声明（items / page / page_size / total）：旧形态是 gin.H，
// encoding/json 对 map 按 key 排序输出，声明顺序即字节序（ADR-0009 §2）。
type QuestionCommentPageResult struct {
	Items    []QuestionCommentDTO `json:"items" nullability:"nonnil"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int64                `json:"total"`
}

// QuestionCommentService 题目评论服务
type QuestionCommentService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewQuestionCommentService(db *gorm.DB, logger *zap.Logger) *QuestionCommentService {
	return &QuestionCommentService{db: db, logger: logger}
}

// List 某题的评论列表。scope 必传（ADR-0062 决策 4）：修复前这条查询**连题目存在性都不查**，
// 直调任意 question_id 即可枚举池外题（draft / pending / 源标记真题题 / 非当前证件）的评论，
// 等于给不可见题装了一个只读探针。池外一律按「不存在」上抛，与题目 by-id 读面同口径。
func (s *QuestionCommentService) List(questionID, page, pageSize int, scope QuestionReadScope) ([]QuestionCommentDTO, int64, error) {
	if !scope.VisibleByID(s.db, questionID) {
		return nil, 0, ErrQuestionNotFound
	}
	type row struct {
		model.QuestionComment
		Username  string `gorm:"column:username"`
		AvatarURL string `gorm:"column:avatar_url"`
	}
	// 既有语义保留：本列表无页大小上限、pageSize<=0 不回落默认（Limit(0) 即空页；负值取消 LIMIT），
	// 默认值由 HTTP 层 atoiDefault(page_size,10) 保证。故 default/max 都传 pageSize 自身，
	// 让 paging 的钳制在这些维度上成为空操作；page<=0 → 1 与既有的 offset 下限 0 等价
	//（本方法的返回不含 page，调用方用自己的请求值装配信封）。
	rows, total, _, _, err := paging.QueryWithScan[row](s.db, page, pageSize, pageSize, pageSize,
		"c.created_at DESC", func(q *gorm.DB) *gorm.DB {
			return q.Table("question_comment AS c").
				Select("c.*, u.username, u.avatar_url").
				Joins("LEFT JOIN hrwai_users AS u ON u.id = c.user_id").
				Where("c.question_id = ?", questionID)
		})
	if err != nil {
		return nil, 0, err
	}
	items := make([]QuestionCommentDTO, len(rows))
	for i, r := range rows {
		items[i] = QuestionCommentDTO{
			ID: r.ID, QuestionID: r.QuestionID, UserID: r.UserID,
			Content: r.Content, CreatedAt: formatISO(r.CreatedAt),
			Username: r.Username, AvatarURL: r.AvatarURL,
		}
		if items[i].Username == "" {
			items[i].Username = "已注销用户"
		}
	}
	return items, total, nil
}

// Create 发表评论。scope 必传（ADR-0062 决策 4）：只判「题存在」的旧写法允许把评论挂到
// 学员根本看不见的题上（再由列表/计数收割），写面与读面必须同一口径。
func (s *QuestionCommentService) Create(questionID, userID int, content string, scope QuestionReadScope) (*QuestionCommentDTO, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if len(content) > 500 {
		return nil, errors.New("评论不能超过500字")
	}
	if !scope.VisibleByID(s.db, questionID) {
		return nil, ErrQuestionNotFound
	}
	c := model.QuestionComment{
		QuestionID: questionID,
		UserID:     userID,
		Content:    content,
		CreatedAt:  beijingNow(),
	}
	if err := s.db.Create(&c).Error; err != nil {
		return nil, err
	}
	var u model.HrwaiUser
	_ = s.db.Select("username", "avatar_url").First(&u, userID).Error
	dto := &QuestionCommentDTO{
		ID: c.ID, QuestionID: c.QuestionID, UserID: c.UserID,
		Content: c.Content, CreatedAt: formatISO(c.CreatedAt),
		Username: u.Username, AvatarURL: u.AvatarURL,
	}
	if dto.Username == "" {
		dto.Username = "用户"
	}
	return dto, nil
}

func (s *QuestionCommentService) Delete(commentID int, userID int) error {
	var c model.QuestionComment
	if err := s.db.First(&c, commentID).Error; err != nil {
		return errors.New("评论不存在")
	}
	if c.UserID != userID {
		return errors.New("无权删除")
	}
	return s.db.Delete(&c).Error
}

// 笔记服务已随 ADR-0055 迁到 note_service.go（NoteService）——本文件只管评论与考点。

// QuestionKnowledgeService 考点（题库标签只读）
type QuestionKnowledgeService struct {
	db *gorm.DB
}

func NewQuestionKnowledgeService(db *gorm.DB) *QuestionKnowledgeService {
	return &QuestionKnowledgeService{db: db}
}

func (s *QuestionKnowledgeService) ListForQuestion(questionID int) ([]model.QuestionTag, error) {
	var tags []model.QuestionTag
	err := s.db.Table("question_tag AS t").
		Joins("JOIN question_tag_relation AS r ON r.tag_id = t.id").
		Where("r.question_id = ?", questionID).
		Order("t.sort_order ASC, t.id ASC").
		Find(&tags).Error
	return tags, err
}
