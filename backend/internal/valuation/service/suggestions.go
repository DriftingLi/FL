// Package service 实现核心业务逻辑
// 本文件：建议（suggestions）生成器——评估流程与 PDF 重建共用同一实现。
// 百分比从 coefficient_configs 动态读取（ConfigResolver），不再硬编码，避免
// 新评估建议与重生成 PDF 建议分叉。
package service

import (
	"context"
	"fmt"

	"forklift-training/internal/valuation/model"
)

// ConfigResolver 系数配置读取接口（评估流程用 CoefficientProvider，测试用内存实现）。
type ConfigResolver interface {
	ReadFloat(ctx context.Context, key string, fallback float64) float64
}

// ReadFloat 从 provider 读取系数，失败或非正数时返回 fallback 默认值。
func (p *CoefficientProvider) ReadFloat(ctx context.Context, key string, fallback float64) float64 {
	return readWithFallback(ctx, p, key, fallback)
}

// SuggestionsInput 建议生成的输入（评估结果与持久化记录都可映射，保持 builder 中立）。
type SuggestionsInput struct {
	KCondition                 float64
	HasLicensePlate            bool
	HasRegistrationCertificate bool
	OriginalPaint              bool
	HasMaintenanceRecords      bool
	KHours                     float64
	KBrand                     float64
	KTime                      float64
	KMarket                    float64
	OriginalPrice              float64
	EstimatedValue             float64
}

// FromResult 从评估结果映射建议输入。
func FromResult(r *model.EvaluationResult) SuggestionsInput {
	return SuggestionsInput{
		KCondition:                 r.KCondition,
		HasLicensePlate:            r.HasLicensePlate,
		HasRegistrationCertificate: r.HasRegistrationCertificate,
		OriginalPaint:              r.OriginalPaint,
		HasMaintenanceRecords:      r.HasMaintenanceRecords,
		KHours:                     r.KHours,
		KBrand:                     r.KBrand,
		KTime:                      r.KTime,
		KMarket:                    r.KMarket,
		OriginalPrice:              r.OriginalPrice,
		EstimatedValue:             r.EstimatedValue,
	}
}

// FromDetail 从持久化评估详情映射建议输入（PDF 重建路径）。
func FromDetail(d *model.EvaluationDetail) SuggestionsInput {
	return SuggestionsInput{
		KCondition:                 d.KCondition,
		HasLicensePlate:            d.HasLicensePlate,
		HasRegistrationCertificate: d.HasRegistrationCertificate,
		OriginalPaint:              d.OriginalPaint,
		HasMaintenanceRecords:      d.HasMaintenanceRecords,
		KHours:                     d.KHours,
		KBrand:                     d.KBrand,
		KTime:                      d.KTime,
		KMarket:                    d.KMarket,
		OriginalPrice:              d.OriginalPrice,
		EstimatedValue:             d.EstimatedValue,
	}
}

// BuildSuggestions 基于评估结果生成文本建议
// 每条建议是一个短句，前端直接用 <li> 列表展示
// 000015：证件扣减/油漆保养加成百分比动态读取，并补充可售性提示
// 评估流程与 PDF 重建都走本函数，百分比统一从 resolver 动态读取。
func BuildSuggestions(ctx context.Context, in SuggestionsInput, resolver ConfigResolver) []string {
	s := make([]string, 0, 10)

	// 1. 车况维度（核心）
	switch {
	case in.KCondition >= 1.00:
		s = append(s, "车况优秀，原漆、维保记录、证件齐全，建议正常出售")
	case in.KCondition >= 0.85:
		s = append(s, "车况良好，残值稳定，可作为二手设备出售")
	case in.KCondition >= 0.65:
		s = append(s, "车况一般，建议整备后出售以提升残值")
	case in.KCondition >= 0.45:
		s = append(s, "车况较差，多个维度有折损，建议折价处理")
	default:
		s = append(s, "车况很差，建议拆件出售或作为配件使用")
	}

	// 2. 证件缺失提示 + 可售性警告
	//    缺车牌 → 无法上路；缺登记证 → 无法过户；缺双证 → 无法正常出售
	licensePct := resolver.ReadFloat(ctx, KeyKcNoLicensePenaltyPct, defaultKcNoLicensePenaltyPct)
	regPct := resolver.ReadFloat(ctx, KeyKcNoRegistrationPenaltyPct, defaultKcNoRegistrationPenaltyPct)
	licensePctShown := licensePct * 100
	regPctShown := regPct * 100
	missingBoth := !in.HasLicensePlate && !in.HasRegistrationCertificate

	if !in.HasLicensePlate {
		s = append(s, fmt.Sprintf("缺少车牌，残值扣减 %.0f%%，无法正常上路行驶，建议补办后再出售", licensePctShown))
	}
	if !in.HasRegistrationCertificate {
		s = append(s, fmt.Sprintf("缺少登记证，残值扣减 %.0f%%，无法正常过户，建议补办后交易", regPctShown))
	}
	if missingBoth {
		s = append(s, "车牌与登记证均缺失，无法正常出售与过户，强烈建议补齐证件后再交易")
	}

	// 3. 原厂漆与维保记录加分项提示（百分比动态读取）
	paintBonus := resolver.ReadFloat(ctx, KeyKcPaintBonus, defaultKcPaintBonus)
	maintenanceBonus := resolver.ReadFloat(ctx, KeyKcMaintenanceBonus, defaultKcMaintenanceBonus)
	switch {
	case in.OriginalPaint && in.HasMaintenanceRecords:
		totalPct := (paintBonus + maintenanceBonus) * 100
		s = append(s, fmt.Sprintf("原厂漆完整且有维保记录，加成 %.0f%%，对保值有利", totalPct))
	case in.OriginalPaint:
		s = append(s, fmt.Sprintf("原厂漆完整，加成 %.0f%%", paintBonus*100))
	case in.HasMaintenanceRecords:
		s = append(s, fmt.Sprintf("有维保记录，加成 %.0f%%", maintenanceBonus*100))
	}

	// 4. 品牌/强度对时间衰减的修正方向
	//    Kb 高 → 衰减速率被压低（保值好）；Kh 高 → 衰减速率被抬高（磨损大）
	//    用 Kh/Kb 比值判断：> 1.05 加速衰减；< 0.95 减缓衰减；中间视为持平
	ratioHK := 1.0
	if in.KBrand > 0 {
		ratioHK = in.KHours / in.KBrand
	}
	switch {
	case ratioHK >= 1.10:
		s = append(s, "使用强度显著高于品牌保值能力，时间衰减被加速")
	case ratioHK >= 1.05:
		s = append(s, "使用强度略高于品牌保值能力，时间衰减略快")
	case ratioHK <= 0.90:
		s = append(s, "品牌保值能力强于使用强度折损，时间衰减被明显减缓")
	case ratioHK <= 0.95:
		s = append(s, "品牌保值能力略占优，时间衰减略缓")
	}

	// 5. 原始时间衰减水平（不含品牌/强度修正）
	if in.KTime < 0.50 {
		s = append(s, "使用年限较长，原始时间衰减明显")
	}

	// 6. 市场维度
	if in.KMarket < 0.99 {
		s = append(s, "区域市场系数偏低，二手需求较弱")
	} else if in.KMarket > 1.02 {
		s = append(s, "区域市场系数偏高，二手需求旺盛")
	}

	// 7. 残值率（已钳制 ≤ 100%）
	if in.OriginalPrice > 0 {
		rate := in.EstimatedValue / in.OriginalPrice
		switch {
		case rate >= 1.0:
			s = append(s, "残值率达 100% 上限（综合车况、市场极优），按原价出售")
		case rate >= 0.7:
			s = append(s, "残值率较高，建议按当前车况正常出售")
		case rate < 0.3:
			s = append(s, "残值率较低，建议拆件出售或作为配件使用")
		}
	}

	return s
}

// BuildBatterySuggestions 电池健康度建议（纯函数，EOL 阈值 60%）。
// 预测流程（含特征稳定性提示）与详情记录 fallback 共用同一实现；
// 记录中无特征稳定性分数时传入 health=1.0（不触发稳定性提示）。
func BuildBatterySuggestions(bt model.BatteryType, soh float64, rul, low, high int, health float64) []string {
	out := []string{}
	// 1) 健康度评估（EOL 阈值 60%）
	switch {
	case soh >= 95:
		out = append(out, "电池健康度优秀（SOH≥95%），处于生命初期，建议常规巡检。")
	case soh >= 80:
		out = append(out, "电池健康度良好（80%≤SOH<95%），状态稳定，可继续投入使用。")
	case soh >= 60:
		out = append(out, "电池健康度临近梯次利用边界（60%≤SOH<80%），建议评估应用场景与监测频率。")
	default:
		out = append(out, fmt.Sprintf("电池健康度偏低（SOH=%.1f%%<60%%），已低于 EOL 标准，建议尽快更换。", soh))
	}
	// 2) 剩余寿命
	out = append(out, fmt.Sprintf("预测剩余循环数约 %d 次（置信区间 %d~%d）。", rul, low, high))
	// 3) 类型相关
	switch bt {
	case model.BatteryTypeLFP:
		out = append(out, "LFP 电池循环寿命长，安全性好；如 SOH 仍高，可考虑梯次利用。")
	case model.BatteryTypeNCM:
		out = append(out, "NCM 电池能量密度高但循环寿命较短，注意高温环境与过充风险。")
	}
	// 4) 健康度稳定性
	if health < 0.5 {
		out = append(out, "特征波动较大，建议结合历史多循环数据复核预测结果。")
	}
	return out
}
