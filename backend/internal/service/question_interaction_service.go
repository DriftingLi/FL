// Package service 题目互动（评论 + 笔记）。
package service

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// QuestionCommentDTO 评论输出 DTO。
type QuestionCommentDTO struct {
	ID         int64     `json:"id"`
	QuestionID int       `json:"question_id"`
	UserID     int       `json:"user_id"`
	Username   string    `json:"username"`
	AvatarURL  string    `json:"avatar_url"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	CanDelete  bool      `json:"can_delete"`
}

// QuestionNoteDTO 笔记输出 DTO。
type QuestionNoteDTO struct {
	ID         int64     `json:"id"`
	QuestionID int       `json:"question_id"`
	Content    string    `json:"content"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// KnowledgeTagDTO 考点标签输出 DTO。
type KnowledgeTagDTO struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// QuestionInteractionService 题目互动服务。
type QuestionInteractionService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewQuestionInteractionService 创建题目互动服务。
func NewQuestionInteractionService(db *gorm.DB, logger *zap.Logger) *QuestionInteractionService {
	return &QuestionInteractionService{db: db, logger: logger}
}

// ========== 评论 ==========

// ListComments 获取题目评论列表（按时间升序，仅 status=1）。
func (s *QuestionInteractionService) ListComments(questionID, currentUserID int) ([]QuestionCommentDTO, error) {
	var rows []model.QuestionComment
	err := s.db.Where("question_id = ? AND status = 1", questionID).
		Order("id ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []QuestionCommentDTO{}, nil
	}

	// 批量查用户信息
	userIDs := make([]int, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
	}
	userMap := batchFetchUsers(s.db, userIDs)

	out := make([]QuestionCommentDTO, 0, len(rows))
	for _, r := range rows {
		u := userMap[r.UserID]
		out = append(out, QuestionCommentDTO{
			ID:         r.ID,
			QuestionID: r.QuestionID,
			UserID:     r.UserID,
			Username:   u.Username,
			AvatarURL:  u.AvatarURL,
			Content:    r.Content,
			CreatedAt:  r.CreatedAt,
			CanDelete:  r.UserID == currentUserID,
		})
	}
	return out, nil
}

// CreateComment 发表评论。
func (s *QuestionInteractionService) CreateComment(questionID, userID int, content string) (*QuestionCommentDTO, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("评论内容不能为空")
	}
	c := model.QuestionComment{
		QuestionID: questionID,
		UserID:     userID,
		Content:    strings.TrimSpace(content),
		Status:     1,
	}
	if err := s.db.Create(&c).Error; err != nil {
		return nil, err
	}

	// 查用户信息
	var user model.HrwaiUser
	s.db.Where("id = ?", userID).First(&user)

	return &QuestionCommentDTO{
		ID:         c.ID,
		QuestionID: c.QuestionID,
		UserID:     c.UserID,
		Username:   user.Username,
		AvatarURL:  user.AvatarURL,
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
		CanDelete:  true,
	}, nil
}

// DeleteComment 删除评论（仅本人可删，软删除）。
func (s *QuestionInteractionService) DeleteComment(commentID, userID int) error {
	result := s.db.Model(&model.QuestionComment{}).
		Where("id = ? AND user_id = ?", commentID, userID).
		Update("status", 0)
	if result.RowsAffected == 0 {
		return errors.New("评论不存在或无权删除")
	}
	return result.Error
}

// ========== 笔记 ==========

// GetNote 获取个人笔记。
func (s *QuestionInteractionService) GetNote(questionID, userID int) (*QuestionNoteDTO, error) {
	var note model.QuestionNote
	err := s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&note).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &QuestionNoteDTO{QuestionID: questionID, Content: ""}, nil
	}
	if err != nil {
		return nil, err
	}
	return &QuestionNoteDTO{
		ID:         note.ID,
		QuestionID: note.QuestionID,
		Content:    note.Content,
		UpdatedAt:  note.UpdatedAt,
	}, nil
}

// SaveNote 保存/更新笔记（upsert）。
func (s *QuestionInteractionService) SaveNote(questionID, userID int, content string) (*QuestionNoteDTO, error) {
	var note model.QuestionNote
	err := s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&note).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新笔记
		note = model.QuestionNote{
			QuestionID: questionID,
			UserID:     userID,
			Content:    strings.TrimSpace(content),
		}
		if err := s.db.Create(&note).Error; err != nil {
			return nil, err
		}
	} else if err == nil {
		// 更新已有笔记
		note.Content = strings.TrimSpace(content)
		if err := s.db.Save(&note).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	return &QuestionNoteDTO{
		ID:         note.ID,
		QuestionID: note.QuestionID,
		Content:    note.Content,
		UpdatedAt:  note.UpdatedAt,
	}, nil
}

// DeleteNote 删除笔记。
func (s *QuestionInteractionService) DeleteNote(questionID, userID int) error {
	return s.db.Where("question_id = ? AND user_id = ?", questionID, userID).
		Delete(&model.QuestionNote{}).Error
}

// ========== 考点标签 ==========

// GetKnowledgeTags 获取题目关联的考点标签。
func (s *QuestionInteractionService) GetKnowledgeTags(questionID int) ([]KnowledgeTagDTO, error) {
	var tags []model.QuestionTag
	err := s.db.
		Joins("JOIN question_tag_relation qtr ON question_tag.id = qtr.tag_id").
		Where("qtr.question_id = ?", questionID).
		Order("question_tag.sort_order ASC").
		Find(&tags).Error
	if err != nil {
		return nil, err
	}
	out := make([]KnowledgeTagDTO, 0, len(tags))
	for _, t := range tags {
		out = append(out, KnowledgeTagDTO{
			ID:   t.ID,
			Code: t.Code,
			Name: t.Name,
		})
	}
	return out, nil
}

// batchFetchUsers 批量查询用户信息，返回 userID -> user 映射。
func batchFetchUsers(db *gorm.DB, userIDs []int) map[int]*model.HrwaiUser {
	result := make(map[int]*model.HrwaiUser)
	if len(userIDs) == 0 {
		return result
	}
	var users []model.HrwaiUser
	db.Where("id IN ?", userIDs).Find(&users)
	for i := range users {
		result[users[i].ID] = &users[i]
	}
	return result
}
