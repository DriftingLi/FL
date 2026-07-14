// Package service 实现核心业务逻辑
// 本文件：系数配置实时查询
// 重构后不再使用内存加载器，所有系数从 coefficient_configs 实时查询
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// CoefficientProvider 系数配置提供者（实时查 DB）
// 提供按 key 查询的能力，供各 K 系数计算使用
type CoefficientProvider struct {
	dictRepo *repository.DictionaryRepository
}

// NewCoefficientProvider 构造系数提供者
func NewCoefficientProvider(dictRepo *repository.DictionaryRepository) *CoefficientProvider {
	return &CoefficientProvider{dictRepo: dictRepo}
}

// Get 按 key 读取系数
// 未找到时返回 model.ErrCoefficientNotFound
func (p *CoefficientProvider) Get(ctx context.Context, key string) (float64, error) {
	c, err := p.dictRepo.GetCoefficientByKey(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", model.ErrCoefficientNotFound, key)
	}
	return c.Value, nil
}

// 系数键常量集中定义，避免散落字符串
// 重构后保留 lambda_electric / lambda_combustion / annual_usage_hours / confidence_range
// 以及 Kh 区间阈值 4 个键
// 000015 追加 Kc 修正项 4 个键（油漆/保养加性 + 证件乘性）
const (
	KeyLambdaElectric   = "lambda_electric"    // 电动叉车时间衰减系数 λ
	KeyLambdaCombustion = "lambda_combustion"  // 内燃叉车时间衰减系数 λ
	KeyAnnualUsageHours = "annual_usage_hours" // 年度标准使用小时
	KeyConfidenceRange  = "confidence_range"   // 残值置信区间幅度 ±
	KeyKHoursRatioLow   = "k_hours_ratio_low"  // Kh 区间阈值：低
	KeyKHoursRatioMid   = "k_hours_ratio_mid"  // Kh 区间阈值：中
	KeyKHoursRatioHigh  = "k_hours_ratio_high" // Kh 区间阈值：高
	KeyKHoursRatioMax   = "k_hours_ratio_max"  // Kh 区间阈值：最大

	// Kc 修正项（000015 迁移加入）
	KeyKcPaintBonus               = "kc_paint_bonus"                 // 原厂漆加成（绝对值）
	KeyKcMaintenanceBonus         = "kc_maintenance_bonus"           // 维保记录加成（绝对值）
	KeyKcNoLicensePenaltyPct      = "kc_no_license_penalty_pct"      // 缺车牌扣减比例（乘性）
	KeyKcNoRegistrationPenaltyPct = "kc_no_registration_penalty_pct" // 缺登记证扣减比例（乘性）
)
