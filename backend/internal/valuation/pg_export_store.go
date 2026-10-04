// 估值数据 adapter（含导出取数）（原 internal/valuation/repository，#1514 波 7 并回域包）。
// 实现 PgExportStore（seam 定义在消费方域包 internal/valuation，见 spec #75 D4）。
package valuation

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgExportStore 导出取数的估值数据访问 adapter（pgx 实现，契约在域包 internal/valuation）。
type PgExportStore struct {
	pool *pgxpool.Pool
}

// NewPgExportStore 构造导出 adapter。
func NewPgExportStore(pool *pgxpool.Pool) *PgExportStore {
	return &PgExportStore{pool: pool}
}

// ListEvaluationExports 评估记录导出行（与导出契约列一一对应，含主表用户 join）。
// SELECT 列序与 position Scan 均从 EvaluationExportColumns 单点 spec 派生（#229），
// 与 export*.go 的 CSV 表头/取值同源，SQL 返回序与表头序不会彼此漂移。
func (s *PgExportStore) ListEvaluationExports(ctx context.Context) ([]EvaluationExportRow, error) {
	query := "SELECT " + BuildEvalExportSelect() +
		"\n\tFROM evaluations AS e\n\tLEFT JOIN hrwai_users AS u ON u.id = e.user_id\n\tORDER BY e.id DESC"
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]EvaluationExportRow, 0, 16)
	for rows.Next() {
		var r EvaluationExportRow
		dests, commits := ScanEvalExportDestinations(&r)
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
