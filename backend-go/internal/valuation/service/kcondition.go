// Package service 实现核心业务逻辑
// 本文件：车况系数 Kc（基于车况评级 + 证件/维保/原漆修正）
// 公式：Kc = condition_rating.base
//            + (original_paint ? +0.03 : 0)
//            + (has_maintenance_records ? +0.03 : 0)
//            - (has_license_plate ? 0 : 0.05)
//            - (has_registration_certificate ? 0 : 0.05)
// 最终钳制到 [0.30, 1.15]
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// KcResult 车况系数计算结果
type KcResult struct {
	KCondition    float64 // 钳制后的最终 Kc
	Base           float64 // 评级基准系数（condition_rating.base）
	OriginalPaintBonus     float64 // 原厂漆加成（+0.03 或 0）
	MaintenanceBonus       float64 // 维保记录加成（+0.03 或 0）
	LicensePlatePenalty    float64 // 缺车牌扣减（-0.05 或 0）
	RegistrationPenalty    float64 // 缺登记证扣减（-0.05 或 0）
	RawKc           float64 // 钳制前的原始 Kc 值
	Rating          string  // 车况评级
	Label           string  // 评级中文名
}

// 修正项常量（与需求文档一致）
const (
	bonusOriginalPaint       = 0.03 // 原厂漆加成
	bonusMaintenanceRecords  = 0.03 // 维保记录加成
	penaltyNoLicensePlate    = 0.05 // 缺车牌扣减
	penaltyNoRegistrationCert = 0.05 // 缺登记证扣减
	// Kc 钳制区间
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
func CalcKCondition(
	ctx context.Context,
	rating string,
	originalPaint, hasMaintenanceRecords, hasLicensePlate, hasRegistrationCertificate bool,
	dictRepo *repository.DictionaryRepository,
) (KcResult, error) {
	// 1. 查询车况评级基准系数
	cr, err := dictRepo.GetConditionRating(ctx, rating)
	if err != nil {
		return KcResult{}, fmt.Errorf("%w: %s", model.ErrConditionRatingNotFound, rating)
	}

	// 2. 计算各项修正
	res := KcResult{
		Base:  cr.BaseCoefficient,
		Rating: cr.Rating,
		Label:  cr.Label,
	}
	if originalPaint {
		res.OriginalPaintBonus = bonusOriginalPaint
	}
	if hasMaintenanceRecords {
		res.MaintenanceBonus = bonusMaintenanceRecords
	}
	if !hasLicensePlate {
		res.LicensePlatePenalty = -penaltyNoLicensePlate
	}
	if !hasRegistrationCertificate {
		res.RegistrationPenalty = -penaltyNoRegistrationCert
	}

	// 3. Kc = base + 修正项
	raw := res.Base + res.OriginalPaintBonus + res.MaintenanceBonus + res.LicensePlatePenalty + res.RegistrationPenalty
	res.RawKc = raw

	// 4. 钳制到 [0.30, 1.15]
	if raw < kcMinClamp {
		raw = kcMinClamp
	} else if raw > kcMaxClamp {
		raw = kcMaxClamp
	}
	res.KCondition = raw
	return res, nil
}
