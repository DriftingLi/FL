// condition_ratings 车况评级 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
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

// GetConditionRating 按 rating 查询（供 service 计算 Kc 使用）
func (r *DictionaryRepository) GetConditionRating(ctx context.Context, rating string) (ConditionRating, error) {
	return getCached(r, ctx, cacheKey(CachePrefixConditionGet, rating), "查询车况评级",
		`SELECT id, rating, label, base_coefficient FROM condition_ratings WHERE rating = $1`,
		func(row pgx.Row) (ConditionRating, error) {
			var c ConditionRating
			err := row.Scan(&c.ID, &c.Rating, &c.Label, &c.BaseCoefficient)
			return c, err
		}, rating)
}
