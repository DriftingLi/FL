// Package service 实现核心业务逻辑
// 本文件：时间衰减系数 Kt
// 公式：Kt = e^(-λ·age)，age = sale_year - factory_year
// λ 区分电动（默认 0.12）与内燃（默认 0.10），可通过 coefficient_configs 调整
//
// 品牌系数 Kb 与使用强度系数 Kh 不再直接作用于残值，而是对衰减速率 λ 进行修正：
//   λ_adj = λ × (Kh / Kb)
//   Kt_adj = e^(-λ_adj · age) = Kt^(Kh / Kb)
// 这样 age=0 时 Kt_adj 恒等于 1.0，从根本上避免残值率超过 100%
package service

import (
	"context"
	"fmt"
	"math"

	"forklift-training/internal/valuation/model"
)

// KtResult 时间衰减系数计算结果
type KtResult struct {
	KTime float64 // 衰减系数
	Age   int     // 使用年限
}

// CalcKTime 计算时间衰减系数 Kt
// powerType: 动力类型（electric/combustion），决定使用哪个 λ
// factoryYear: 出厂年份
// saleYear: 成交年份
// provider: 系数提供者，提供 λ
func CalcKTime(ctx context.Context, powerType model.PowerType, factoryYear, saleYear int, provider *CoefficientProvider) (KtResult, error) {
	age := saleYear - factoryYear
	if age < 0 {
		return KtResult{}, model.ErrInvalidYear
	}

	// 根据动力类型选取衰减率 λ 的 key
	var key string
	switch powerType {
	case model.PowerTypeElectric:
		key = KeyLambdaElectric
	case model.PowerTypeCombustion:
		key = KeyLambdaCombustion
	default:
		return KtResult{}, fmt.Errorf("未知的动力类型: %s", powerType)
	}

	lambda, err := provider.Get(ctx, key)
	if err != nil {
		return KtResult{}, err
	}

	// Kt = e^(-λ·age)
	kt := math.Exp(-lambda * float64(age))
	return KtResult{KTime: kt, Age: age}, nil
}

// AdjustKTimeByBrandAndIntensity 用品牌系数 Kb 与使用强度系数 Kh 修正时间衰减系数 Kt
// 数学等价：Kt_adj = Kt^(Kh / Kb) = exp(-λ × (Kh/Kb) × age)
//   - Kh 越大（使用强度高）→ 指数越大 → Kt 越小（衰减更快）
//   - Kb 越大（品牌好）→ 指数越小 → Kt 越大（衰减更慢）
//   - age=0 时 Kt=1.0，无论 Kh、Kb 如何变化，Kt_adj 都恒为 1.0
//
// 边界兜底：
//   - kb <= 0：返回 kt 原值（理论上不应出现，保险起见）
//   - kt <= 0：返回 0（极旧车 Kt 可能极小但 > 0，此处兜底负数）
func AdjustKTimeByBrandAndIntensity(kt, kh, kb float64) float64 {
	if kb <= 0 {
		return kt
	}
	if kt <= 0 {
		return 0
	}
	return math.Pow(kt, kh/kb)
}
