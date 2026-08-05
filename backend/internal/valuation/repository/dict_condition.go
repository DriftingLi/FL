// condition_ratings 车况评级 字典 CRUD（骨架走 dict_helpers 共享实现）
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"forklift-training/internal/cache"
)

// ListConditionRatings 列出全部车况评级
func (r *DictionaryRepository) ListConditionRatings(ctx context.Context) ([]ConditionRating, error) {
	return listCached(r, ctx, "dict:condition:list", "查询车况评级",
		`SELECT id, rating, label, base_coefficient FROM condition_ratings ORDER BY base_coefficient DESC`,
		func(rows pgx.Rows) (ConditionRating, error) {
			var c ConditionRating
			err := rows.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient)
			return c, err
		})
}

// CreateConditionRating 新增车况评级
func (r *DictionaryRepository) CreateConditionRating(ctx context.Context, rating, label string, base float64) (ConditionRating, error) {
	id, err := insertReturningID(r, ctx, "新增车况评级",
		`INSERT INTO condition_ratings (rating, label, base_coefficient) VALUES ($1, $2, $3)
		 ON CONFLICT (rating) DO UPDATE SET label = EXCLUDED.label, base_coefficient = EXCLUDED.base_coefficient
		 RETURNING id`, rating, label, base)
	if err != nil {
		return ConditionRating{}, err
	}
	return ConditionRating{ID: int(id), Rating: rating, Label: label, BaseCoefficient: base}, nil
}

// UpdateConditionRating 更新车况评级
func (r *DictionaryRepository) UpdateConditionRating(ctx context.Context, id int, label string, base float64) error {
	return execWrite(r, ctx, "更新车况评级",
		`UPDATE condition_ratings SET label = $2, base_coefficient = $3 WHERE id = $1`, id, label, base)
}

// DeleteConditionRating 删除车况评级
func (r *DictionaryRepository) DeleteConditionRating(ctx context.Context, id int) error {
	return execWrite(r, ctx, "删除车况评级", `DELETE FROM condition_ratings WHERE id = $1`, id)
}

// GetConditionRating 按 rating 查询（供 service 计算 Kc 使用）
func (r *DictionaryRepository) GetConditionRating(ctx context.Context, rating string) (ConditionRating, error) {
	return getCached(r, ctx, cache.SafeKey("dict", "condition", "get", rating), "查询车况评级",
		`SELECT id, rating, label, base_coefficient FROM condition_ratings WHERE rating = $1`,
		func(row pgx.Row) (ConditionRating, error) {
			var c ConditionRating
			err := row.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient)
			return c, err
		}, rating)
}
