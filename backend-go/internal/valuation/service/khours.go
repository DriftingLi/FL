// Package service 实现核心业务逻辑
// 本文件：使用强度系数 Kh
// 公式：ratio = usage_hours / (annual_usage_hours · age)
// 按 ratio 落入的区间查表得到 Kh：
//
//	ratio < 0.7    → 1.10  远低于行业平均
//	0.7 ~ 1.0      → 1.00  正常范围
//	1.0 ~ 1.3      → 0.95  高于平均
//	1.3 ~ 1.6      → 0.90  接近重型使用
//	ratio >= 1.6   → 0.85  超高强度使用
package service

import (
	"context"

	"forklift-training/internal/valuation/model"
)

// 默认年化使用小时数（与 coefficient_configs 中 annual_usage_hours 默认值一致）
// 仅在系数查询失败时作为兜底使用
const defaultAnnualUsageHours = 1750.0

// KhResult 使用强度系数计算结果
type KhResult struct {
	KHours     float64 // 强度系数
	Ratio      float64 // 实际/标准比值
	UsageHours int     // 实际使用小时数
	Age        int     // 使用年限
	Standard   float64 // 推算的行业标准小时数
}

// CalcKHours 计算使用强度系数 Kh
// age 必须来自 Kt 计算（factory_year 与 sale_year 之差），保证一致性
// 区间阈值通过 provider 从 coefficient_configs 读取（key: k_hours_ratio_low/mid/high/max）
func CalcKHours(ctx context.Context, age, usageHours int, provider *CoefficientProvider) (KhResult, error) {
	if usageHours < 0 {
		return KhResult{}, model.ErrInvalidUsageHours
	}
	if age < 0 {
		return KhResult{}, model.ErrInvalidYear
	}

	// 读取年化标准小时数（失败时使用默认值 1750）
	annual, err := provider.Get(ctx, KeyAnnualUsageHours)
	if err != nil || annual <= 0 {
		annual = defaultAnnualUsageHours
	}

	// 计算标准小时数（age=0 时按 1 年计算）
	effectiveAge := age
	if effectiveAge == 0 {
		effectiveAge = 1
	}
	standard := annual * float64(effectiveAge)
	ratio := float64(usageHours) / standard

	// 读取 4 个区间阈值（失败时使用硬编码默认值）
	low, errLow := provider.Get(ctx, KeyKHoursRatioLow)
	if errLow != nil || low <= 0 {
		low = 0.7
	}
	mid, errMid := provider.Get(ctx, KeyKHoursRatioMid)
	if errMid != nil || mid <= 0 {
		mid = 1.0
	}
	high, errHigh := provider.Get(ctx, KeyKHoursRatioHigh)
	if errHigh != nil || high <= 0 {
		high = 1.3
	}
	maxR, errMax := provider.Get(ctx, KeyKHoursRatioMax)
	if errMax != nil || maxR <= 0 {
		maxR = 1.6
	}

	// 区间查表（保持原有逻辑：低段闭区间，高段开区间，末段闭区间）
	var factor float64
	switch {
	case ratio < low:
		factor = 1.10
	case ratio < mid:
		factor = 1.00
	case ratio < high:
		factor = 0.95
	case ratio < maxR:
		factor = 0.90
	default:
		factor = 0.85
	}

	return KhResult{
		KHours:     factor,
		Ratio:      ratio,
		UsageHours: usageHours,
		Age:        age,
		Standard:   standard,
	}, nil
}
