package service

import (
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
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

// QuestionCommentService 题目评论服务
type QuestionCommentService struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewQuestionCommentService(db *gorm.DB, logger *zap.Logger) *QuestionCommentService {
	return &QuestionCommentService{db: db, logger: logger}
}

func (s *QuestionCommentService) List(questionID, page, pageSize int) ([]QuestionCommentDTO, int64, error) {
	var total int64
	s.db.Model(&model.QuestionComment{}).Where("question_id = ?", questionID).Count(&total)
	type row struct {
		model.QuestionComment
		Username  string `gorm:"column:username"`
		AvatarURL string `gorm:"column:avatar_url"`
	}
	var rows []row
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	if err := s.db.Table("question_comment AS c").
		Select("c.*, u.username, u.avatar_url").
		Joins("LEFT JOIN hrwai_users AS u ON u.id = c.user_id").
		Where("c.question_id = ?", questionID).
		Order("c.created_at DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
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

func (s *QuestionCommentService) Create(questionID, userID int, content string) (*QuestionCommentDTO, error) {
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
