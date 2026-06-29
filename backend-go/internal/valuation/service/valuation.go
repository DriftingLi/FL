// Package service 实现核心业务逻辑
// 本文件：主评估服务 ValuationService
// 公式：残值 = 基准原价 × Kt_adj × Kc × Km
//   Kt_adj = Kt^(Kh/Kb) = exp(-λ × (Kh/Kb) × age)
//   品牌系数 Kb 与使用强度系数 Kh 不再直接乘到残值，而是修正时间衰减速率 λ
//   全局兜底：estimated ≤ originalPrice（残值率不超过 100%）
// 集成基准价查询、各 K 系数计算、置信区间、维度评分与建议生成
package service

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// ValuationService 评估服务
// 持有 *pgxpool.Pool 与字典仓储，所有系数从 DB 实时查询
type ValuationService struct {
	pool     *pgxpool.Pool
	dictRepo *repository.DictionaryRepository
	evalRepo *repository.EvaluationRepository
	provider *CoefficientProvider
}

// NewValuationService 构造评估服务
// pool: pgx 连接池
// dictRepo: 字典仓储（brand_types / brands / vehicle_types / condition_ratings / region_coefficients / coefficient_configs / original_prices）
// evalRepo: 评估记录仓储（持久化评估结果）
func NewValuationService(
	pool *pgxpool.Pool,
	dictRepo *repository.DictionaryRepository,
	evalRepo *repository.EvaluationRepository,
) *ValuationService {
	if pool == nil {
		panic("NewValuationService: pool 不能为 nil")
	}
	if dictRepo == nil {
		panic("NewValuationService: dictRepo 不能为 nil")
	}
	if evalRepo == nil {
		panic("NewValuationService: evalRepo 不能为 nil")
	}
	return &ValuationService{
		pool:     pool,
		dictRepo: dictRepo,
		evalRepo: evalRepo,
		provider: NewCoefficientProvider(dictRepo),
	}
}

// Evaluate 执行完整残值评估
// 流程：
//  1. 业务参数校验
//  2. 查询 vehicle_type 派生 power_type（电动/内燃）
//  3. 查询 original_prices 获取基准价（精确匹配 → 模糊匹配 → 错误）
//  4. 计算 Kt / Kh / Kb / Kc / Km
//  5. 用 Kh、Kb 修正时间衰减：Kt_adj = Kt^(Kh/Kb)
//  6. 残值 = 基准价 × Kt_adj × Kc × Km，并钳制 ≤ 基准价
//  7. 置信区间 = 残值 × (1 ± confidence_range)
//  8. 生成维度评分与文本建议
//  9. 持久化到 evaluations 表
func (s *ValuationService) Evaluate(ctx context.Context, req *model.EvaluationRequest) (*model.EvaluationResult, error) {
	// 1. 业务参数校验
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 2. 查询 vehicle_type 派生 power_type
	vt, err := s.dictRepo.GetVehicleTypeByName(ctx, req.VehicleType)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", model.ErrVehicleTypeNotFound, req.VehicleType)
	}
	powerType := model.PowerType(vt.PowerType)

	// 3. 查询基准原价：精确匹配 → 模糊匹配
	originalPrice, err := s.lookupOriginalPrice(ctx, req)
	if err != nil {
		return nil, err
	}

	// 4. 计算 Kt（基于 factory_year 与 sale_year）
	ktRes, err := CalcKTime(ctx, powerType, req.FactoryYear, req.SaleYear, s.provider)
	if err != nil {
		return nil, err
	}

	// 5. 计算 Kh（age 复用 Kt 计算结果）
	khRes, err := CalcKHours(ctx, ktRes.Age, req.UsageHours, s.provider)
	if err != nil {
		return nil, err
	}

	// 6. 计算 Kb（brand_types × brands）
	kbRes, err := CalcKBrand(ctx, req.BrandType, req.Brand, s.dictRepo)
	if err != nil {
		return nil, err
	}

	// 7. 计算 Kc（condition_rating + 修正项）
	kcRes, err := CalcKCondition(ctx, req.ConditionRating,
		req.OriginalPaint, req.HasMaintenanceRecords, req.HasLicensePlate, req.HasRegistrationCertificate, s.dictRepo)
	if err != nil {
		return nil, err
	}

	// 8. 计算 Km（region_coefficients，未命中默认 1.0）
	kmRes, err := CalcKMarket(ctx, req.Province, req.City, s.dictRepo)
	if err != nil {
		return nil, err
	}

	// 9. 主公式：残值 = 基准原价 × Kt_adj × Kc × Km
	//    Kt_adj = Kt^(Kh/Kb)，品牌系数与使用强度系数修正时间衰减速率
	ktAdjusted := AdjustKTimeByBrandAndIntensity(ktRes.KTime, khRes.KHours, kbRes.KBrand)
	estimated := originalPrice * ktAdjusted * kcRes.KCondition * kmRes.KMarket

	// 9.1 全局兜底：残值率不超过 100%
	//     Kt_adj 在 age=0 时为 1.0，但 Kc 最高 1.15、Km 可能 >1.0 仍可能让残值突破原价
	if estimated > originalPrice {
		estimated = originalPrice
	}

	// 10. 置信区间
	confRange, err := s.provider.Get(ctx, KeyConfidenceRange)
	if err != nil || confRange <= 0 {
		confRange = 0.10
	}
	confLow := estimated * (1 - confRange)
	confHigh := estimated * (1 + confRange)

	// 11. 装配结果
	result := &model.EvaluationResult{
		EvaluationRequest: *req,
		OriginalPrice:     originalPrice,
		PowerType:         powerType,
		KTime:             roundTo4(ktRes.KTime),
		KHours:            roundTo4(khRes.KHours),
		KBrand:            roundTo4(kbRes.KBrand),
		KCondition:        roundTo4(kcRes.KCondition),
		KMarket:           roundTo4(kmRes.KMarket),
		KTimeAdjusted:     roundTo4(ktAdjusted),
		EstimatedValue:    roundTo2(estimated),
		ConfidenceLow:     roundTo2(confLow),
		ConfidenceHigh:    roundTo2(confHigh),
	}

	// 12. 派生维度评分 + 文本建议
	result.DimensionScores = buildDimensionScores(result)
	result.Suggestions = buildSuggestions(result)
	return result, nil
}

// Persist 持久化评估结果到 evaluations 表，返回新 ID
// 由 handler 在拿到 EvaluationResult 后调用
func (s *ValuationService) Persist(ctx context.Context, result *model.EvaluationResult) (int64, error) {
	if s.evalRepo == nil {
		return 0, fmt.Errorf("evalRepo 未装配")
	}
	params := &repository.CreateEvaluationParams{
		BrandType:                  result.BrandType,
		Brand:                      result.Brand,
		VehicleType:                result.VehicleType,
		Series:                     result.Series,
		Tonnage:                    result.Tonnage,
		ConfigType:                 result.ConfigType,
		MastType:                   result.MastType,
		MastHeightMM:               result.MastHeightMM,
		FactoryYear:                result.FactoryYear,
		SaleYear:                   result.SaleYear,
		UsageHours:                 result.UsageHours,
		OriginalPaint:              result.OriginalPaint,
		Province:                   result.Province,
		City:                       result.City,
		HasLicensePlate:            result.HasLicensePlate,
		HasRegistrationCertificate: result.HasRegistrationCertificate,
		HasMaintenanceRecords:      result.HasMaintenanceRecords,
		ConditionRating:            result.ConditionRating,
		OriginalPrice:              result.OriginalPrice,
		KTime:                      result.KTime,
		KHours:                     result.KHours,
		KBrand:                     result.KBrand,
		KCondition:                 result.KCondition,
		KMarket:                    result.KMarket,
		EstimatedValue:             result.EstimatedValue,
		ConfidenceLow:              result.ConfidenceLow,
		ConfidenceHigh:             result.ConfidenceHigh,
	}
	return s.evalRepo.CreateEvaluation(ctx, params)
}

// lookupOriginalPrice 查询基准原价：先精确匹配，未命中则模糊匹配
// 当字段值为 "无"（字符串）或 0（mast_height_mm）时，模糊匹配会忽略该字段
func (s *ValuationService) lookupOriginalPrice(ctx context.Context, req *model.EvaluationRequest) (float64, error) {
	// 1. 精确匹配
	op, err := s.dictRepo.FindOriginalPriceMatch(ctx,
		req.BrandType, req.Brand, req.VehicleType, req.Series,
		req.Tonnage, req.ConfigType, req.MastType, req.MastHeightMM)
	if err == nil {
		return op.OriginalPrice, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("精确匹配原价失败: %w", err)
	}
	// 2. 模糊匹配（按 brand_type + brand + vehicle_type + series + tonnage）
	//    若 series 为 "其它"，降级为仅按 brand_type + brand + vehicle_type + tonnage 匹配
	seriesForFuzzy := req.Series
	if seriesForFuzzy == "其它" {
		seriesForFuzzy = ""
	}
	op, err = s.dictRepo.FindOriginalPriceFuzzy(ctx,
		req.BrandType, req.Brand, req.VehicleType, seriesForFuzzy, req.Tonnage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, model.ErrOriginalPriceNotFound
		}
		return 0, fmt.Errorf("模糊匹配原价失败: %w", err)
	}
	return op.OriginalPrice, nil
}

// buildDimensionScores 把结果包装成 4 维中文标签的 map
// 维度顺序与雷达图保持一致：时间衰减（含品牌/强度修正） / 车况 / 市场 / 残值率
// 残值率维度 = estimated / originalPrice（已钳制 ≤ 1.0）
func buildDimensionScores(r *model.EvaluationResult) map[string]float64 {
	rate := 0.0
	if r.OriginalPrice > 0 {
		rate = r.EstimatedValue / r.OriginalPrice
		if rate > 1.0 {
			rate = 1.0
		}
	}
	return map[string]float64{
		"时间衰减": roundTo4(r.KTimeAdjusted),
		"车况":   roundTo4(r.KCondition),
		"市场":   roundTo4(r.KMarket),
		"残值率":  roundTo4(rate),
	}
}

// buildSuggestions 基于评估结果生成文本建议
// 每条建议是一个短句，前端直接用 <li> 列表展示
func buildSuggestions(r *model.EvaluationResult) []string {
	s := make([]string, 0, 8)

	// 1. 车况维度（核心）
	switch {
	case r.KCondition >= 1.00:
		s = append(s, "车况优秀，原漆、维保记录、证件齐全，建议正常出售")
	case r.KCondition >= 0.85:
		s = append(s, "车况良好，残值稳定，可作为二手设备出售")
	case r.KCondition >= 0.65:
		s = append(s, "车况一般，建议整备后出售以提升残值")
	case r.KCondition >= 0.45:
		s = append(s, "车况较差，多个维度有折损，建议折价处理")
	default:
		s = append(s, "车况很差，建议拆件出售或作为配件使用")
	}

	// 2. 证件缺失提示
	if !r.HasLicensePlate {
		s = append(s, "缺少车牌，残值扣减 5%，建议补办后再出售")
	}
	if !r.HasRegistrationCertificate {
		s = append(s, "缺少登记证，残值扣减 5%，过户需提供登记证")
	}

	// 3. 原厂漆与维保记录加分项提示
	if r.OriginalPaint && r.HasMaintenanceRecords {
		s = append(s, "原厂漆完整且有维保记录，加成 6%，对保值有利")
	} else if r.OriginalPaint {
		s = append(s, "原厂漆完整，加成 3%")
	} else if r.HasMaintenanceRecords {
		s = append(s, "有维保记录，加成 3%")
	}

	// 4. 品牌/强度对时间衰减的修正方向
	//    Kb 高 → 衰减速率被压低（保值好）；Kh 高 → 衰减速率被抬高（磨损大）
	//    用 Kh/Kb 比值判断：> 1.05 加速衰减；< 0.95 减缓衰减；中间视为持平
	ratioHK := 1.0
	if r.KBrand > 0 {
		ratioHK = r.KHours / r.KBrand
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
	if r.KTime < 0.50 {
		s = append(s, "使用年限较长，原始时间衰减明显")
	}

	// 6. 市场维度
	if r.KMarket < 0.99 {
		s = append(s, "区域市场系数偏低，二手需求较弱")
	} else if r.KMarket > 1.02 {
		s = append(s, "区域市场系数偏高，二手需求旺盛")
	}

	// 7. 残值率（已钳制 ≤ 100%）
	if r.OriginalPrice > 0 {
		rate := r.EstimatedValue / r.OriginalPrice
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

// roundTo2 四舍五入到 2 位小数（保留金额精度）
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

// roundTo4 四舍五入到 4 位小数（保留 K 系数精度）
func roundTo4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
