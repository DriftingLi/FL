// Package service 实现业务服务层。
// 本文件：带列选择的批量题目加载器（QuestionLoader），消除列表循环内逐题 db.First 的 N+1。
// ADR-0013 候选 8：题干/答案分离，供「答案是否暴露」场景按需 Select 列。
package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// loadQuestionsByIDs 批量加载题目：ids → IN 查询 → id→*Question map（缺失 id 跳过）。
// columns 为可选 Select 列（题干/答案分离，供「答案是否暴露」场景按需取列）；为空时加载全字段。
// 返回 map 由调用方按 id 回填，缺失题目自然跳过（缺省语义与逐题 db.First 的 err==nil 判定一致）。
func loadQuestionsByIDs(db *gorm.DB, ids []int, columns ...string) map[int]*model.Question {
	qMap := map[int]*model.Question{}
	if len(ids) == 0 {
		return qMap
	}
	q := db.Where("id IN ?", ids)
	if len(columns) > 0 {
		q = q.Select(columns)
	}
	var questions []model.Question
	q.Find(&questions)
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}
	return qMap
}
