// Package repository - 估值数据 adapter（含导出取数）
// 实现 export.ExportStore（seam 定义在消费方 internal/valuation/export，见 spec #75 D4）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	vexport "forklift-training/internal/valuation/export"
)

// ExportStore 导出取数的估值数据访问 adapter（pgx 实现，契约在 internal/valuation/export）。
type ExportStore struct {
	pool *pgxpool.Pool
}

// NewExportStore 构造导出 adapter。
func NewExportStore(pool *pgxpool.Pool) *ExportStore {
	return &ExportStore{pool: pool}
}

// ListEvaluationExports 评估记录导出行（与导出契约列一一对应，含主表用户 join）。
// SELECT 列序与 position Scan 均从 export.EvaluationExportColumns 单点 spec 派生（#229），
// 与 export 的 CSV 表头/取值同源，SQL 返回序与表头序不会彼此漂移。
func (s *ExportStore) ListEvaluationExports(ctx context.Context) ([]vexport.EvaluationExportRow, error) {
	query := "SELECT " + vexport.BuildEvalExportSelect() +
		"\n\tFROM evaluations AS e\n\tLEFT JOIN hrwai_users AS u ON u.id = e.user_id\n\tORDER BY e.id DESC"
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]vexport.EvaluationExportRow, 0, 16)
	for rows.Next() {
		var r vexport.EvaluationExportRow
		dests, commits := vexport.ScanEvalExportDestinations(&r)
		if err := rows.Scan(dests...); err != nil {
			return nil, err
		}
		for _, commit := range commits {
			commit()
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
