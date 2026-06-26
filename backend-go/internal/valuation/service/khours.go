// Package service 实现核心业务逻辑
// 本文件：使用强度系数 Kh
// 公式：Kh = 实际使用小时 / 行业标准小时
// 行业标准：年化 1500~2000 小时，公式中使用 1750 小时/年
// 按比值查表得到 Kh 系数
package service

import (
	"forklift-training/internal/valuation/model"
)

// IndustryStandardHoursPerYear 行业标准年化使用小时数（中间值）
const IndustryStandardHoursPerYear = 1750.0

// hoursRange 强度区间定义
// 上界为开区间，下界为闭区间（除首段外）
type hoursRange struct {
	// Lower 下界（含）
	Lower float64
	// Upper 上界（不含），0 表示无上界
	Upper float64
	// Factor 区间对应的 Kh 系数
	Factor float64
}

// hoursRanges 强度区间与系数映射表（按 Lower 升序）
// 与开发方案一致：
//   < 0.7   → 1.10  远低于行业平均
//   0.7~1.0 → 1.00  正常范围
//   1.0~1.3 → 0.95  高于平均
//   1.3~1.6 → 0.90  接近重型使用
//   > 1.6   → 0.85  超高强度使用
var hoursRanges = []hoursRange{
	{Lower: 0, Upper: 0.7, Factor: 1.10},
	{Lower: 0.7, Upper: 1.0, Factor: 1.00},
	{Lower: 1.0, Upper: 1.3, Factor: 0.95},
	{Lower: 1.3, Upper: 1.6, Factor: 0.90},
	{Lower: 1.6, Upper: 0, Factor: 0.85},
}

// KhResult 使用强度系数计算结果
type KhResult struct {
	KHours     float64 // 强度系数
	Ratio      float64 // 实际/标准比值
	UsageHours int     // 实际使用小时数
	Years      int     // 使用年限
	Standard   float64 // 推算的行业标准小时数
}

// CalcKHours 计算使用强度系数 Kh
// purchaseYear: 购置年份
// saleYear: 成交年份
// usageHours: 累计使用小时数
// usageHours < 0 时返回错误
func CalcKHours(purchaseYear, saleYear, usageHours int) (KhResult, error) {
	if usageHours < 0 {
		return KhResult{}, model.ErrInvalidUsageHours
	}
	if saleYear < purchaseYear {
		return KhResult{}, model.ErrInvalidYear
	}

	years := saleYear - purchaseYear
	// 使用年限为 0 时，比值按 0 处理（视为全新未使用）
	var ratio float64
	var standard float64
	if years == 0 {
		// 同年购置与成交，按 1 个完整使用年计算基准
		standard = IndustryStandardHoursPerYear
		ratio = float64(usageHours) / standard
	} else {
		standard = IndustryStandardHoursPerYear * float64(years)
		ratio = float64(usageHours) / standard
	}

	factor := lookupHoursRange(ratio)
	return KhResult{
		KHours:     factor,
		Ratio:      ratio,
		UsageHours: usageHours,
		Years:      years,
		Standard:   standard,
	}, nil
}

// lookupHoursRange 根据比值查找对应的 Kh 系数
func lookupHoursRange(ratio float64) float64 {
	for _, r := range hoursRanges {
		if r.Upper == 0 {
			// 末段：Lower 及以上
			if ratio >= r.Lower {
				return r.Factor
			}
		} else {
			// 中间段：[Lower, Upper)
			if ratio >= r.Lower && ratio < r.Upper {
				return r.Factor
			}
		}
	}
	// 兜底（理论上不会到达）
	return 0.85
}
