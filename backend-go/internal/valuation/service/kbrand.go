// Package service 实现核心业务逻辑
// 本文件：品牌系数 Kb
// 公式：Kb = brands.k_brand（直接使用品牌系数，不再乘以品牌类型系数）
// 重构说明：删除 brand_types 表后，Kb 由 brands.k_brand 独立承载
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// KbResult 品牌系数计算结果
type KbResult struct {
	KBrand  float64 // 最终品牌系数 Kb = brands.k_brand
	BrandKB float64 // 品牌自身系数（与 KBrand 相同，保留字段以兼容调用方）
	Brand   string  // 品牌名
}

// CalcKBrand 计算品牌系数 Kb
// brandName: 品牌名（如"林德"）
// dictRepo: 字典仓储
func CalcKBrand(ctx context.Context, brandName string, dictRepo *repository.DictionaryRepository) (KbResult, error) {
	b, err := dictRepo.GetBrandByName(ctx, brandName)
	if err != nil {
		return KbResult{}, fmt.Errorf("%w: %s", model.ErrBrandNotFound, brandName)
	}
	return KbResult{
		KBrand:  b.KBrand,
		BrandKB: b.KBrand,
		Brand:   b.Name,
	}, nil
}
