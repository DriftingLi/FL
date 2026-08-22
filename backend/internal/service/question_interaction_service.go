package service

import (
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// QuestionCommentService 题目评论服务
type QuestionCommentService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewQuestionCommentService(db *gorm.DB, logger *zap.Logger) *QuestionCommentService {
	return &QuestionCommentService{db: db, logger: logger}
}

func (s *QuestionCommentService) List(questionID, page, pageSize int) ([]model.QuestionComment, int64, error) {
	var total int64
	s.db.Model(&model.QuestionComment{}).Where("question_id = ?", questionID).Count(&total)
	var items []model.QuestionComment
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if err := s.db.Where("question_id = ?", questionID).Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *QuestionCommentService) Create(questionID, userID int, content string) (*model.QuestionComment, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("评论内容不能为空")
	}
	if len(content) > 500 {
		return nil, errors.New("评论不能超过500字")
	}
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
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
	return &c, nil
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

// QuestionNoteService 题目笔记服务（每人每题一条）
type QuestionNoteService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewQuestionNoteService(db *gorm.DB, logger *zap.Logger) *QuestionNoteService {
	return &QuestionNoteService{db: db, logger: logger}
}

func (s *QuestionNoteService) Get(questionID, userID int) (*model.QuestionNote, error) {
	var n model.QuestionNote
	if err := s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

func (s *QuestionNoteService) Upsert(questionID, userID int, content string) (*model.QuestionNote, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("笔记内容不能为空")
	}
	if len(content) > 2000 {
		return nil, errors.New("笔记不能超过2000字")
	}
	var q model.Question
	if err := s.db.First(&q, questionID).Error; err != nil {
		return nil, errors.New("题目不存在")
	}
	var n model.QuestionNote
	err := s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&n).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if n.ID != 0 {
		n.Content = content
		n.UpdatedAt = beijingNow()
		if err := s.db.Save(&n).Error; err != nil {
			return nil, err
		}
		return &n, nil
	}
	n = model.QuestionNote{
		QuestionID: questionID,
		UserID:     userID,
		Content:    content,
		UpdatedAt:  beijingNow(),
	}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *QuestionNoteService) Delete(questionID, userID int) error {
	return s.db.Where("question_id = ? AND user_id = ?", questionID, userID).Delete(&model.QuestionNote{}).Error
}

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
