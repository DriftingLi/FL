// 本文件：公开网校的**证件题库目录**读面（#2760）。
//
// 与 public_school.go 的关系：那边是「按学习形式找课」，这边是「按证件看题库覆盖」——
// 两者共用同一套证件资格判据（publicSchoolCategoryScope），但读的是不同的表，
// 且题库目录**与有没有公开课无关**（票 2760 US-12：只有题库、暂无公开课的证件也要能找到）。
//
// 口径三件（票 2760 Implementation Decisions 最后一条 / US-15）：
//  1. 只统计**题库池**题目：已发布（published）+ 排除来源标记标签题（is_source_tag，如真题）
//     —— 与学员端练习抽题同一口径，直接复用 internal/questionpool 的三枚 SQL 片段，
//     不在这里重写条件（ADR-0050 决策 1：raw 重写与常量脱钩正是那道要关的漂移窗口）。
//  2. 按题目的 training_category 聚合（不是课程分类、不是证件表）：题目本来就带 category_id，
//     它是「这道题属于哪个证件的题库」的既成事实。
//  3. 只出**汇总**：分类 ID、显示名、总题数、各题型数量。**不出任何题干/选项/答案/解析**——
//     匿名题目详情在票 2760 Out of Scope 里被明确排除。
package course

import (
	"context"

	"forklift-training/internal/questionpool"
)

// PublicQuestionBankCategoryDTO 题库目录的一行：某证件分类的题库汇总。
//
// ByType 只含**实际存在**的题型（零填充会让面变宽：不同证件支持的题型不同，
// 把 5 种题型都列出来会让访客以为那些类型「应该有但是 0 道」）。
type PublicQuestionBankCategoryDTO struct {
	CategoryID   int              `json:"category_id"`
	CategoryName string           `json:"category_name"`
	Total        int64            `json:"total"`
	ByType       map[string]int64 `json:"by_type" swaggertype:"object,integer" nullability:"nonnil"`
}

// PublicQuestionBankListDTO 题库目录响应 {"question_banks": [...]}。
type PublicQuestionBankListDTO struct {
	QuestionBanks []PublicQuestionBankCategoryDTO `json:"question_banks" nullability:"nonnil"`
}

// ListPublicQuestionBanks 公开题库目录：按证件分类聚合题库池的题量与题型分布。
//
// 资格判据（与公开课程同一套，见 publicSchoolCategoryScope）：
//   - training_category.status = 1
//   - 映射到 legacy_credential_id 且该 credential.status = 1
//
// 与公开课程口径的唯一差别：**不要求该分类下有公开课**。票 2760 US-12 明确要求
// 「只有题库、暂无公开课的证件也能在题库目录中找到」——题库目录回答的是「有没有题练」，
// 不是「有没有课上」，把有题无课的证件隐藏掉会让最需要练习的人找不到入口。
//
// 排序沿用分类的 sort_order / id（与分类选项同一口径，两个面在页面上并排时顺序一致）。
//
// 无合格题目的分类**不出现在结果里**（而不是出 total=0 的行）：与公开课程分类选项的
// 「空项隐藏」同一呈现决定——把「这个证件没题」渲染成一个可点击的入口，点进去必然是空态。
func (s *Service) ListPublicQuestionBanks(ctx context.Context) (*PublicQuestionBankListDTO, error) {
	// 单条 GROUP BY 同时出「分类 × 题型」与总数：两个维度一次扫，避免逐分类 N+1。
	// 别名固定 question，与 internal/questionpool 三枚片段内的别名逐字一致。
	type row struct {
		CategoryID   int    `gorm:"column:category_id"`
		CategoryName string `gorm:"column:category_name"`
		QuestionType string `gorm:"column:question_type"`
		Count        int64  `gorm:"column:count"`
	}
	var rows []row
	err := s.db.WithContext(ctx).Table("question").
		// 池口径：已发布 + 排除来源标记（真题）——三枚片段是唯一出处，不就地重写。
		Where(questionpool.PublishedSQL).
		Where(questionpool.ExcludeSourceTagsSQL).
		Joins("JOIN training_category ON training_category.id = question.category_id").
		Joins("JOIN credential ON credential.id = training_category.legacy_credential_id").
		Where("training_category.status = ?", 1).
		Where("credential.status = ?", 1).
		Select("training_category.id AS category_id, training_category.name AS category_name, " +
			"question.type AS question_type, COUNT(*) AS count").
		Group("training_category.id, training_category.name, training_category.sort_order, question.type").
		Order("training_category.sort_order, training_category.id, question.type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// 按分类折叠，顺序由上面的 ORDER BY 决定（首次出现即定序）。
	byCategory := make(map[int]*PublicQuestionBankCategoryDTO, len(rows))
	order := make([]int, 0, len(rows))
	for _, r := range rows {
		item, ok := byCategory[r.CategoryID]
		if !ok {
			item = &PublicQuestionBankCategoryDTO{
				CategoryID:   r.CategoryID,
				CategoryName: r.CategoryName,
				ByType:       make(map[string]int64, 8),
			}
			byCategory[r.CategoryID] = item
			order = append(order, r.CategoryID)
		}
		item.ByType[r.QuestionType] = r.Count
		item.Total += r.Count
	}
	items := make([]PublicQuestionBankCategoryDTO, 0, len(order))
	for _, id := range order {
		items = append(items, *byCategory[id])
	}
	return &PublicQuestionBankListDTO{QuestionBanks: items}, nil
}
