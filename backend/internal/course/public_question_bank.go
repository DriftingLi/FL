package course

import (
	"context"

	"forklift-training/internal/publicscope"
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
// 题库分类独立于公开课程。票 2760 US-12 明确要求
// 「只有题库、暂无公开课的证件也能在题库目录中找到」——题库目录回答的是「有没有题练」，
// 不是「有没有课上」，把有题无课的证件隐藏掉会让最需要练习的人找不到入口。
//
// 排序沿用分类的 sort_order / id（与分类选项同一口径，两个面在页面上并排时顺序一致）。
//
// 无合格题目的分类**不出现在结果里**（而不是出 total=0 的行）：与公开课程分类选项的
// 「空项隐藏」同一呈现决定——把「这个证件没题」渲染成一个可点击的入口，点进去必然是空态。
func (s *Service) ListPublicQuestionBanks(ctx context.Context) (*PublicQuestionBankListDTO, error) {
	// 单条 GROUP BY 同时出「分类 × 题型」与总数：两个维度一次扫，避免逐分类 N+1。
	type row struct {
		CategoryID   int    `gorm:"column:category_id"`
		CategoryName string `gorm:"column:category_name"`
		QuestionType string `gorm:"column:question_type"`
		Count        int64  `gorm:"column:count"`
	}
	var rows []row
	err := s.db.WithContext(ctx).Table(publicscope.QuestionView).
		Where("eligible").
		Select("category_id, category_name, type AS question_type, COUNT(*) AS count").
		Group("category_id, category_name, category_sort_order, type").
		Order("category_sort_order, category_id, type").
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
