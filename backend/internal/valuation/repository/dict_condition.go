// condition_ratings 车况评级 字典：只读方法（写面已迁至 dictcrud 描述符驱动，见 ADR-0008）。
package repository

import (
	"context"
)

// ListConditionRatings 列出全部车况评级
func (r *DictionaryRepository) ListConditionRatings(ctx context.Context) ([]ConditionRating, error) {
	return readList[ConditionRating](r, ctx, readSpecConditionList, CacheKeyConditionList, "查询车况评级")
}

// GetConditionRating 按 rating 查询（供 service 计算 Kc 使用）
func (r *DictionaryRepository) GetConditionRating(ctx context.Context, rating string) (ConditionRating, error) {
	return readGet[ConditionRating](r, ctx, readSpecConditionGet, cacheKey(CachePrefixConditionGet, rating), "查询车况评级", rating)
}
