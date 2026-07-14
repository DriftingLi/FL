// Package service 实现核心业务逻辑
// 本文件：市场系数 Km
// 基于 region_coefficients 表，按 province + city 查询
// 未命中时默认 1.0
package service

import (
	"context"

	"forklift-training/internal/valuation/repository"
)

// 默认市场系数（未匹配到区域时使用）
const defaultKMarket = 1.0

// KmResult 市场系数计算结果
type KmResult struct {
	KMarket  float64 // 市场系数
	Province string  // 省份
	City     string  // 城市
	Matched  bool    // 是否命中 region_coefficients
}

// CalcKMarket 计算市场系数 Km
// province, city: 省份与城市
// dictRepo: 字典仓储
// 未命中时返回 1.0（不阻断流程）
func CalcKMarket(ctx context.Context, province, city string, dictRepo *repository.DictionaryRepository) (KmResult, error) {
	rc, err := dictRepo.GetRegionCoefficient(ctx, province, city)
	if err != nil {
		// 未命中：使用默认值 1.0
		return KmResult{
			KMarket:  defaultKMarket,
			Province: province,
			City:     city,
			Matched:  false,
		}, nil
	}
	// 命中：使用数据库系数
	return KmResult{
		KMarket:  rc.Coefficient,
		Province: rc.Province,
		City:     rc.City,
		Matched:  true,
	}, nil
}
