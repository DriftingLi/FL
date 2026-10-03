// 本文件：题目互动域的考点服务（ADR-0070 的「一域两服务」形态，先例 internal/aiassistant）——
// 题库标签的只读查询，按 sort_order 稳定排序。
package questioninteraction

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// KnowledgeService 考点（题库标签只读）——题库标签按题目关联只读查询。
type KnowledgeService struct {
	db *gorm.DB
}

func NewKnowledgeService(db *gorm.DB) *KnowledgeService {
	return &KnowledgeService{db: db}
}

func (s *KnowledgeService) ListForQuestion(questionID int) ([]model.QuestionTag, error) {
	var tags []model.QuestionTag
	err := s.db.Table("question_tag AS t").
		Joins("JOIN question_tag_relation AS r ON r.tag_id = t.id").
		Where("r.question_id = ?", questionID).
		Order("t.sort_order ASC, t.id ASC").
		Find(&tags).Error
	return tags, err
}
