// Package service 实现核心业务逻辑
// 本文件：车况系数 Kc（基于车况评级 + 证件/维保/原漆修正）
//
// 公式（000015 重构后，加性 + 乘性混合）：
//
//	Kc_base = base + paint_bonus(若有原厂漆) + maintenance_bonus(有维保记录)
//	cert_factor = (1 - no_license_penalty_pct)(若缺车牌)
//	            × (1 - no_registration_penalty_pct)(若缺登记证)
//	Kc_raw = Kc_base × cert_factor
//	Kc = clamp(Kc_raw, 0.30, 1.15)
//
// 设计意图：
//   - 油漆/保养保持加性微调（影响小，与 base 同量级叠加）
//   - 证件改为乘性扣减（影响大，缺双证时复合放大，匹配"无法正常出售"业务直觉）
//   - 4 个修正值均从 coefficient_configs 实时读取，管理员可在后台调整
//   - 查询失败时回退到旧硬编码默认值（0.03/0.03/0.05/0.05），保证 DB 异常时算法不崩
package service

import (
	"context"

	"forklift-training/internal/valuation/repository"
)

// KcResult 车况系数计算结果
type KcResult struct {
	KCondition float64 // 钳制后的最终 Kc
	Base       float64 // 评级基准系数（condition_rating.base）
	// 加性项
	PaintBonus       float64 // 原厂漆加成（实际值，0 表示无）
	MaintenanceBonus float64 // 维保记录加成（实际值，0 表示无）
	// 乘性项
	LicensePenaltyPct      float64 // 缺车牌扣减比例（实际值，0 表示无扣减）
	RegistrationPenaltyPct float64 // 缺登记证扣减比例（实际值，0 表示无扣减）
	CertFactor             float64 // 证件乘性因子（缺双证时复合累积）
	// 中间与最终
	KcBase float64 // 加性合并后的中间值（base + 加成）
	RawKc  float64 // 钳制前的原始 Kc 值（KcBase × CertFactor）
	Rating string  // 车况评级
	Label  string  // 评级中文名
}

// Kc 修正项默认值（与 000015 迁移前硬编码一致，仅用于 provider 查询失败兜底）
// 注意：DB 中新默认值已调整为 0.02/0.02/0.10/0.10，这里保留旧值仅作兜底
const (
	defaultKcPaintBonus               = 0.03
	defaultKcMaintenanceBonus         = 0.03
	defaultKcNoLicensePenaltyPct      = 0.05
	defaultKcNoRegistrationPenaltyPct = 0.05
	// Kc 钳制区间（保持硬编码，用户未要求暴露）
	kcMinClamp = 0.30
	kcMaxClamp = 1.15
)

// CalcKCondition 计算车况系数 Kc
// rating: 车况评级（A/B/C/D/E）
// originalPaint: 是否原厂漆
// hasMaintenanceRecords: 是否有维保记录
// hasLicensePlate: 是否有车牌
// hasRegistrationCertificate: 是否有登记证
// dictRepo: 字典仓储（用于查询 condition_ratings）
// provider: 系数提供者（用于查询 4 个 Kc 修正项）
func CalcKCondition(
	ctx context.Context,
	rating string,
	originalPaint, hasMaintenanceRecords, hasLicensePlate, hasRegistrationCertificate bool,
	dictRepo *repository.DictionaryRepository,
	provider *CoefficientProvider,
) (KcResult, error) {
	// 1. 查询车况评级基准系数
	//    字典表未命中时用 1.0 兜底（中性车况，不阻断评估流程）
	cr, err := dictRepo.GetConditionRating(ctx, rating)
	if err != nil {
		cr = repository.ConditionRating{
			Rating:          rating,
			Label:           rating,
			BaseCoefficient: 1.0,
		}
	}

	// 2. 从 provider 读取 4 个修正项（失败时回退默认值）
	paintBonus := readWithFallback(ctx, provider, KeyKcPaintBonus, defaultKcPaintBonus)
	maintenanceBonus := readWithFallback(ctx, provider, KeyKcMaintenanceBonus, defaultKcMaintenanceBonus)
	licensePenaltyPct := readWithFallback(ctx, provider, KeyKcNoLicensePenaltyPct, defaultKcNoLicensePenaltyPct)
	registrationPenaltyPct := readWithFallback(ctx, provider, KeyKcNoRegistrationPenaltyPct, defaultKcNoRegistrationPenaltyPct)

	// 3. 装配修正项（条件性生效）
	res := KcResult{
		Base:                   cr.BaseCoefficient,
		Rating:                 cr.Rating,
		Label:                  cr.Label,
		LicensePenaltyPct:      0,
		RegistrationPenaltyPct: 0,
		CertFactor:             1.0,
	}
	if originalPaint {
		res.PaintBonus = paintBonus
	}
	if hasMaintenanceRecords {
		res.MaintenanceBonus = maintenanceBonus
	}
	if !hasLicensePlate {
		res.LicensePenaltyPct = licensePenaltyPct
	}
	if !hasRegistrationCertificate {
		res.RegistrationPenaltyPct = registrationPenaltyPct
	}

	// 4. 加性合并：Kc_base = base + paint_bonus + maintenance_bonus
	res.KcBase = res.Base + res.PaintBonus + res.MaintenanceBonus

	// 5. 乘性合并：cert_factor = (1 - license_pct)(若缺) × (1 - reg_pct)(若缺)
	if res.LicensePenaltyPct > 0 {
		res.CertFactor *= (1.0 - res.LicensePenaltyPct)
	}
	if res.RegistrationPenaltyPct > 0 {
		res.CertFactor *= (1.0 - res.RegistrationPenaltyPct)
	}

	// 6. Kc_raw = Kc_base × cert_factor
	raw := res.KcBase * res.CertFactor
	res.RawKc = raw

	// 7. 钳制到 [0.30, 1.15]
	if raw < kcMinClamp {
		raw = kcMinClamp
	} else if raw > kcMaxClamp {
		raw = kcMaxClamp
	}
	res.KCondition = raw
	return res, nil
}

// readWithFallback 从 provider 读取系数，失败或非正数时返回 fallback 默认值
// 与 khours.go 中 annual/lower/mid/high/maxR 的兜底模式保持一致
// provider 为 nil 时直接返回 fallback（避免 nil 指针 panic，主要供测试与极端兜底使用）
func readWithFallback(ctx context.Context, provider *CoefficientProvider, key string, fallback float64) float64 {
	if provider == nil {
		return fallback
	}
	v, err := provider.Get(ctx, key)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
