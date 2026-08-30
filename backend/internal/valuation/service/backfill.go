// Package service 实现核心业务逻辑
// 本文件：评估建议历史回填（ADR-0004 评估事实性）。
package service

import (
	"context"

	"forklift-training/internal/valuation/model"
)

// EvaluationBackfillStore 回填所需的评估记录读取/更新面（生产为 pgx 仓储，测试为内存替身）。
type EvaluationBackfillStore interface {
	ListEvaluationsForBackfill(ctx context.Context) ([]model.EvaluationDetail, error)
	UpdateEvaluationSuggestions(ctx context.Context, id int64, suggestions []string) error
}

// BackfillEvaluationSuggestions 为缺失建议的历史评估记录用**当前**系数配置回填（幂等：跳过已有建议）。
// 历史记录的当时系数配置不可考，当前配置是最佳近似（ADR-0004 记录为未来路径）。
// 建议输入与 PDF 重建同源走 FromDetail 单一映射，不再维护第三份字段拷贝。
// 返回更新的记录数。
func BackfillEvaluationSuggestions(ctx context.Context, dict DictionaryReader, store EvaluationBackfillStore) (int, error) {
	rows, err := store.ListEvaluationsForBackfill(ctx)
	if err != nil {
		return 0, err
	}
	snap, err := LoadCoefficientSnapshot(ctx, dict)
	if err != nil {
		return 0, err
	}
	updated := 0
	for i := range rows {
		row := &rows[i]
		if len(row.Suggestions) > 0 {
			continue // 幂等：已有锁定建议的记录跳过
		}
		if err := store.UpdateEvaluationSuggestions(ctx, row.ID, BuildSuggestions(ctx, FromDetail(row), snap)); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
