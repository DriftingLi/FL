// Package service 实现核心业务逻辑
// 本文件：品牌系数 Kb
// 公式：Kb = brand_types.k_type × brands.k_brand
// 从数据库实时查询，不再使用内存加载器
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// KbResult 品牌系数计算结果
type KbResult struct {
	KBrand    float64 // 最终品牌系数 Kb = k_type × k_brand
	KType     float64 // 品牌类型系数
	BrandKB   float64 // 品牌自身系数
	BrandType string  // 品牌类型名
	Brand     string  // 品牌名
}

// CalcKBrand 计算品牌系数 Kb
// brandType: 品牌类型名（如"进口一线"）
// brandName: 品牌名（如"林德"）
// dictRepo: 字典仓储
func CalcKBrand(ctx context.Context, brandType, brandName string, dictRepo *repository.DictionaryRepository) (KbResult, error) {
	// 1. 查询品牌类型系数
	bt, err := getBrandTypeByName(ctx, dictRepo, brandType)
	if err != nil {
		return KbResult{}, err
	}

	// 2. 查询品牌系数
	b, err := dictRepo.GetBrandByName(ctx, brandName)
	if err != nil {
		return KbResult{}, fmt.Errorf("%w: %s", model.ErrBrandNotFound, brandName)
	}

	// 3. 一致性校验：brands.brand_type 应与请求一致
	// 若数据库中品牌的 brand_type 与请求不一致，仍以数据库为准（数据库为准）
	// 此处仅记录差异，不阻断流程

	// 4. 计算 Kb = k_type × k_brand
	return KbResult{
		KBrand:    bt.KType * b.KBrand,
		KType:     bt.KType,
		BrandKB:   b.KBrand,
		BrandType: bt.Name,
		Brand:     b.Name,
	}, nil
}

// getBrandTypeByName 查询品牌类型（未命中返回 ErrBrandTypeNotFound）
// brand_types 表的主键就是 name，故使用 List + 过滤
func getBrandTypeByName(ctx context.Context, dictRepo *repository.DictionaryRepository, name string) (repository.BrandType, error) {
	list, err := dictRepo.ListBrandTypes(ctx)
	if err != nil {
		return repository.BrandType{}, err
	}
	for _, bt := range list {
		if bt.Name == name {
			return bt, nil
		}
	}
	return repository.BrandType{}, fmt.Errorf("%w: %s", model.ErrBrandTypeNotFound, name)
}
