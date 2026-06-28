// Package service 实现核心业务逻辑
// 本文件：主评估服务 ValuationService
// 公式：残值 = 基准原价 × Kt × Kh × Kb × Kc × Km
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
//  5. 残值 = 基准价 × Kt × Kh × Kb × Kc × Km
//  6. 置信区间 = 残值 × (1 ± confidence_range)
//  7. 生成维度评分与文本建议
//  8. 持久化到 evaluations 表
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

	// 9. 主公式：残值 = 基准原价 × Kt × Kh × Kb × Kc × Km
	estimated := originalPrice * ktRes.KTime * khRes.KHours * kbRes.KBrand * kcRes.KCondition * kmRes.KMarket

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
		BatteryType:                result.BatteryType,
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
func (s *ValuationService) lookupOriginalPrice(ctx context.Context, req *model.EvaluationRequest) (float64, error) {
	// 1. 精确匹配
	op, err := s.dictRepo.FindOriginalPriceMatch(ctx,
		req.BrandType, req.Brand, req.VehicleType, req.Series,
		req.Tonnage, req.ConfigType, req.MastType, req.MastHeightMM, req.BatteryType)
	if err == nil {
		return op.OriginalPrice, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("精确匹配原价失败: %w", err)
	}
	// 2. 模糊匹配（按 brand_type + brand + vehicle_type + series + tonnage）
	op, err = s.dictRepo.FindOriginalPriceFuzzy(ctx,
		req.BrandType, req.Brand, req.VehicleType, req.Series, req.Tonnage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, model.ErrOriginalPriceNotFound
		}
		return 0, fmt.Errorf("模糊匹配原价失败: %w", err)
	}
	return op.OriginalPrice, nil
}

// buildDimensionScores 把 5 个 K 包装成中文标签的 map
// 维度顺序与雷达图保持一致：时间维度 / 使用强度 / 品牌 / 车况 / 市场
func buildDimensionScores(r *model.EvaluationResult) map[string]float64 {
	return map[string]float64{
		"时间维度": roundTo4(r.KTime),
		"使用强度": roundTo4(r.KHours),
		"品牌":   roundTo4(r.KBrand),
		"车况":   roundTo4(r.KCondition),
		"市场":   roundTo4(r.KMarket),
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

	// 4. 品牌维度
	switch {
	case r.KBrand >= 1.10:
		s = append(s, "品牌力强（进口一线），保值能力优秀")
	case r.KBrand >= 1.00:
		s = append(s, "品牌力较好，残值具备一定支撑")
	case r.KBrand >= 0.85:
		s = append(s, "品牌力中等，残值持平行业平均")
	default:
		s = append(s, "品牌力偏弱，残值相对偏低")
	}

	// 5. 时间维度
	if r.KTime < 0.50 {
		s = append(s, "使用年限较长，残值随时间明显折减")
	}

	// 6. 使用强度维度
	switch {
	case r.KHours >= 1.10:
		s = append(s, "累计使用小时远低于行业平均，机械磨损小")
	case r.KHours <= 0.85:
		s = append(s, "累计使用小时偏高，机械磨损较大")
	}

	// 7. 市场维度
	if r.KMarket < 0.99 {
		s = append(s, "区域市场系数偏低，二手需求较弱")
	} else if r.KMarket > 1.02 {
		s = append(s, "区域市场系数偏高，二手需求旺盛")
	}

	// 8. 残值率
	if r.OriginalPrice > 0 {
		rate := r.EstimatedValue / r.OriginalPrice
		switch {
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
