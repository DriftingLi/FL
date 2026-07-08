// condition_ratings 车况评级 字典 CRUD（从 dictionaries.go 拆分）
// 手写 pgx 仓储，统一使用 *pgxpool.Pool 直接操作
package repository

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// ListConditionRatings 列出全部车况评级
func (r *DictionaryRepository) ListConditionRatings(ctx context.Context) ([]ConditionRating, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, rating, label, base_coefficient FROM condition_ratings ORDER BY base_coefficient DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询车况评级失败: %w", err)
	}
	defer rows.Close()
	out := make([]ConditionRating, 0, 8)
	for rows.Next() {
		var c ConditionRating
		if err := rows.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateConditionRating 新增车况评级
func (r *DictionaryRepository) CreateConditionRating(ctx context.Context, rating, label string, base float64) (ConditionRating, error) {
	var id int
	err := r.pool.QueryRow(ctx,
		`INSERT INTO condition_ratings (rating, label, base_coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (rating) DO UPDATE SET label = EXCLUDED.label, base_coefficient = EXCLUDED.base_coefficient
		 RETURNING id`, rating, label, base).Scan(&id)
	if err != nil {
		return ConditionRating{}, fmt.Errorf("新增车况评级失败: %w", err)
	}
	return ConditionRating{ID: id, Rating: rating, Label: label, BaseCoefficient: base}, nil
}

// UpdateConditionRating 更新车况评级
func (r *DictionaryRepository) UpdateConditionRating(ctx context.Context, id int, label string, base float64) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE condition_ratings SET label = $2, base_coefficient = $3 WHERE id = $1`, id, label, base)
	if err != nil {
		return fmt.Errorf("更新车况评级失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteConditionRating 删除车况评级
func (r *DictionaryRepository) DeleteConditionRating(ctx context.Context, id int) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM condition_ratings WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除车况评级失败: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// GetConditionRating 按 rating 查询（供 service 计算 Kc 使用）
func (r *DictionaryRepository) GetConditionRating(ctx context.Context, rating string) (ConditionRating, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, rating, label, base_coefficient FROM condition_ratings WHERE rating = $1`, rating)
	var c ConditionRating
	if err := row.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient); err != nil {
		return ConditionRating{}, err
	}
	return c, nil
}
