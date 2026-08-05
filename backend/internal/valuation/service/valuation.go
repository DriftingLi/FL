// Package service 实现核心业务逻辑
// 本文件：主评估服务 ValuationService
// 公式：残值 = 基准原价 × Kt_adj × Kc × Km
//
//	Kt_adj = Kt^(Kh/Kb) = exp(-λ × (Kh/Kb) × age)
//	品牌系数 Kb 与使用强度系数 Kh 不再直接乘到残值，而是修正时间衰减速率 λ
//	全局兜底：estimated ≤ originalPrice（残值率不超过 100%）
//
// 集成基准价查询、各 K 系数计算、置信区间、维度评分与建议生成
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/cache"
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
//
// 原实现使用 panic 做空值断言，会绕过 error 返回链导致启动流程难以优雅处理。
// 改为返回 error，由调用方在装配阶段决定 fail-fast 策略（main.go 启动时 os.Exit）。
func NewValuationService(
	pool *pgxpool.Pool,
	dictRepo *repository.DictionaryRepository,
	evalRepo *repository.EvaluationRepository,
) (*ValuationService, error) {
	if pool == nil {
		return nil, fmt.Errorf("NewValuationService: pool 不能为 nil")
	}
	if dictRepo == nil {
		return nil, fmt.Errorf("NewValuationService: dictRepo 不能为 nil")
	}
	if evalRepo == nil {
		return nil, fmt.Errorf("NewValuationService: evalRepo 不能为 nil")
	}
	return &ValuationService{
		pool:     pool,
		dictRepo: dictRepo,
		evalRepo: evalRepo,
		provider: NewCoefficientProvider(dictRepo),
	}, nil
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
//
// 缓存：Evaluate 是纯计算函数（Persist 与计算解耦），相同输入产生相同输出，
// 按 req JSON 的 SHA256 缓存 3 分钟（cache.TTLValuation）。
// 所有字典/系数写操作已在 config handler 中统一失效 valuation:result:*。
func (s *ValuationService) Evaluate(ctx context.Context, req *model.EvaluationRequest) (*model.EvaluationResult, error) {
	// 构造缓存 key：对规范化后的 req JSON 做 SHA256
	reqBytes, _ := json.Marshal(req)
	hash := sha256.Sum256(reqBytes)
	cacheKey := cache.SafeKey("valuation", "result", hex.EncodeToString(hash[:]))

	var result model.EvaluationResult
	err := cache.GetOrSetJSON(ctx, cacheKey, cache.TTLValuation, &result, func() (any, error) {
		return s.evaluateInternal(ctx, req)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// evaluateInternal 包含原 Evaluate 的全部计算逻辑（纯函数，无副作用）
func (s *ValuationService) evaluateInternal(ctx context.Context, req *model.EvaluationRequest) (*model.EvaluationResult, error) {
	// 1. 业务参数校验
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 2. 派生 power_type（电动/内燃），决定 Kt 用哪个衰减系数 λ
	//    优先从 vehicle_types 字典表读取；字典表未命中时从车型名推断
	//    （含"内燃"→combustion，其他→electric），原价表车型名不再受字典表约束
	powerType := inferPowerType(req.VehicleType)
	if vt, err := s.dictRepo.GetVehicleTypeByName(ctx, req.VehicleType); err == nil {
		powerType = model.PowerType(vt.PowerType)
	}

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

	// 6. 计算 Kb（直接使用 brands.k_brand）
	kbRes, err := CalcKBrand(ctx, req.Brand, s.dictRepo)
	if err != nil {
		return nil, err
	}

	// 7. 计算 Kc（condition_rating + 修正项，4 个修正项从 coefficient_configs 读取）
	kcRes, err := CalcKCondition(ctx, req.ConditionRating,
		req.OriginalPaint, req.HasMaintenanceRecords, req.HasLicensePlate, req.HasRegistrationCertificate,
		s.dictRepo, s.provider)
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
	result.Suggestions = BuildSuggestions(ctx, FromResult(result), s.provider)
	return result, nil
}

// inferPowerType 从车型名推断动力类型
// 含"内燃"→combustion，其他→electric（电动为仓储车主流派系）
// 仅在 vehicle_types 字典表未命中时作为兜底，避免原价表自由输入的车型名导致评估失败
func inferPowerType(vehicleType string) model.PowerType {
	if strings.Contains(vehicleType, "内燃") {
		return model.PowerTypeCombustion
	}
	return model.PowerTypeElectric
}

// Persist 持久化评估结果到 evaluations 表，返回新 ID
// 由 handler 在拿到 EvaluationResult 后调用
// userID>0 时写入归属（登录用户提交）；userID=0 时落 NULL（匿名提交）
func (s *ValuationService) Persist(ctx context.Context, result *model.EvaluationResult, userID int) (int64, error) {
	if s.evalRepo == nil {
		return 0, fmt.Errorf("evalRepo 未装配")
	}
	params := &repository.CreateEvaluationParams{
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
		UserID:                     userID,
	}
	return s.evalRepo.CreateEvaluation(ctx, params)
}

// lookupOriginalPrice 查询基准原价：先精确匹配，未命中则模糊匹配
// 当字段值为 "无"（字符串）或 0（mast_height_mm）时，模糊匹配会忽略该字段
func (s *ValuationService) lookupOriginalPrice(ctx context.Context, req *model.EvaluationRequest) (float64, error) {
	// 1. 精确匹配
	op, err := s.dictRepo.FindOriginalPriceMatch(ctx,
		req.Brand, req.VehicleType, req.Series,
		req.Tonnage, req.ConfigType, req.MastType, req.MastHeightMM)
	if err == nil {
		return op.OriginalPrice, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("精确匹配原价失败: %w", err)
	}
	// 2. 模糊匹配（按 brand + vehicle_type + series + tonnage）
	//    若 series 为 "其它"，降级为仅按 brand + vehicle_type + tonnage 匹配
	seriesForFuzzy := req.Series
	if seriesForFuzzy == "其它" {
		seriesForFuzzy = ""
	}
	op, err = s.dictRepo.FindOriginalPriceFuzzy(ctx,
		req.Brand, req.VehicleType, seriesForFuzzy, req.Tonnage)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, model.ErrOriginalPriceNotFound
		}
		return 0, fmt.Errorf("模糊匹配原价失败: %w", err)
	}
	return op.OriginalPrice, nil
}

// BuildDimensionScores 由结果字段派生 5 维度评分切片
// 维度顺序与雷达图保持一致：出厂时间 / 使用强度 / 品牌价值 / 市场需求 / 车辆情况
// 每个维度值钳制到 [0, 1]，对应前端雷达图 max=1
// 供 handler.Get 在详情接口实时计算维度评分（dimension_scores 未入库）
func BuildDimensionScores(kTime, kHours, kBrand, kCondition, kMarket float64) []model.DimensionScore {
	return []model.DimensionScore{
		{Label: "出厂时间", Value: roundTo4(clamp01(kTime))},
		{Label: "使用强度", Value: roundTo4(clamp01(kHours))},
		{Label: "品牌价值", Value: roundTo4(clamp01(kBrand))},
		{Label: "市场需求", Value: roundTo4(clamp01(kMarket))},
		{Label: "车辆情况", Value: roundTo4(clamp01(kCondition))},
	}
}

// buildDimensionScores 把结果包装成 5 维中文标签的 map（Evaluate 流程内部使用）
func buildDimensionScores(r *model.EvaluationResult) map[string]float64 {
	scores := BuildDimensionScores(r.KTime, r.KHours, r.KBrand, r.KCondition, r.KMarket)
	m := make(map[string]float64, len(scores))
	for _, s := range scores {
		m[s.Label] = s.Value
	}
	return m
}

// roundTo2 四舍五入到 2 位小数（保留金额精度）
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

// roundTo4 四舍五入到 4 位小数（保留 K 系数精度）
func roundTo4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// clamp01 将值钳制到 [0, 1] 区间（雷达图维度评分归一化）
// NaN 兜底为 0，避免异常系数经雷达图渲染传播为非法值
func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
