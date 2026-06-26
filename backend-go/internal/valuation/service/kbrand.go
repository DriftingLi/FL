// Package service 实现核心业务逻辑
// 本文件：品牌系数 Kb
// 根据品牌名查表得到对应的品牌系数
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// BrandLoader 品牌加载器（与系数加载器类似，缓存品牌表）
type BrandLoader struct {
	queries *repository.Queries

	// byName 品牌名 → 品牌信息的索引
	byName map[string]model.BrandInfo
}

// NewBrandLoader 构造品牌加载器
func NewBrandLoader(q *repository.Queries) *BrandLoader {
	return &BrandLoader{
		queries: q,
		byName:  make(map[string]model.BrandInfo),
	}
}

// parseModels 解析 brands.models JSONB 字段为字符串切片
// 兼容空数组、null、JSON 异常等场景，统一降级为 []string{}
func parseModels(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

// LoadAll 加载所有激活品牌到内存
func (b *BrandLoader) LoadAll(ctx context.Context) error {
	rows, err := b.queries.ListBrands(ctx)
	if err != nil {
		return fmt.Errorf("加载品牌失败: %w", err)
	}
	b.byName = make(map[string]model.BrandInfo, len(rows))
	for _, r := range rows {
		b.byName[r.Name] = model.BrandInfo{
			ID:     r.ID,
			Name:   r.Name,
			Tier:   r.Tier,
			KBrand: r.KBrand,
			Models: parseModels(r.Models),
		}
	}
	return nil
}

// CalcKBrand 计算品牌系数 Kb
// brandName: 品牌名
// 品牌未找到时返回 model.ErrBrandNotFound
func (b *BrandLoader) CalcKBrand(brandName string) (float64, error) {
	info, ok := b.byName[brandName]
	if !ok {
		return 0, fmt.Errorf("%w: %s", model.ErrBrandNotFound, brandName)
	}
	return info.KBrand, nil
}

// ListBrands 暴露品牌列表（供 API 层使用）
func (b *BrandLoader) ListBrands() []model.BrandInfo {
	out := make([]model.BrandInfo, 0, len(b.byName))
	for _, v := range b.byName {
		out = append(out, v)
	}
	return out
}
