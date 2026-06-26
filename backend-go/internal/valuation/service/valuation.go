// Package service 实现核心业务逻辑
// 本文件：主评估公式 V = V₀ × Kt × Kh × (w₁·Kw + w₂·Kb + w₃·Kc + w₄·Km)
// 集成全部子系数计算与置信区间
package service

import (
	"context"
	"fmt"
	"math"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// ValuationService 评估服务（聚合各子系数计算）
type ValuationService struct {
	Coefficients *CoefficientLoader
	Brands       *BrandLoader
	Parts        *PartConfigLoader
}

// NewValuationService 构造评估服务
func NewValuationService(c *CoefficientLoader, b *BrandLoader, p *PartConfigLoader) *ValuationService {
	return &ValuationService{
		Coefficients: c,
		Brands:       b,
		Parts:        p,
	}
}

// Evaluate 执行完整残值评估
// 返回包含全部中间系数、最终结果、置信区间的 EvaluationResult
func (s *ValuationService) Evaluate(ctx context.Context, req *model.EvaluationRequest) (*model.EvaluationResult, error) {
	// 1. 业务参数校验
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// 2. 计算 Kt
	ktRes, err := CalcKTime(req.ForkliftType, req.PurchaseYear, req.SaleYear, s.Coefficients)
	if err != nil {
		return nil, err
	}

	// 3. 计算 Kh
	khRes, err := CalcKHours(req.PurchaseYear, req.SaleYear, req.UsageHours)
	if err != nil {
		return nil, err
	}

	// 4. 计算 Kw
	kw, err := CalcKWork(req.WorkCondition)
	if err != nil {
		return nil, err
	}

	// 5. 计算 Kb
	kb, err := s.Brands.CalcKBrand(req.Brand)
	if err != nil {
		return nil, err
	}

	// 6. 计算 Kc
	kcRes, err := CalcKCondition(req.ForkliftType, req.Items, s.Parts, req.CanDrive, req.HydraulicOk)
	if err != nil {
		return nil, err
	}

	// 7. 计算 Km
	km, err := CalcKMarket(s.Coefficients)
	if err != nil {
		return nil, err
	}

	// 8. 读取加权权重
	wWork, err := s.Coefficients.Get(KeyWWorkCondition)
	if err != nil {
		return nil, err
	}
	wBrand, err := s.Coefficients.Get(KeyWBrand)
	if err != nil {
		return nil, err
	}
	wCondition, err := s.Coefficients.Get(KeyWCondition)
	if err != nil {
		return nil, err
	}
	wMarket, err := s.Coefficients.Get(KeyWMarket)
	if err != nil {
		return nil, err
	}

	// 9. 计算 Σ(wᵢ·Kᵢ)
	weightedSum := wWork*kw + wBrand*kb + wCondition*kcRes.KCondition + wMarket*km

	// 10. 主公式 V = V₀ × Kt × Kh × Σ(wᵢ·Kᵢ)
	estimated := req.OriginalPrice * ktRes.KTime * khRes.KHours * weightedSum

	// 11. 置信区间 [V×(1-r), V×(1+r)]
	confRange, err := s.Coefficients.Get(KeyConfidenceRange)
	if err != nil {
		return nil, err
	}
	confLow := estimated * (1 - confRange)
	confHigh := estimated * (1 + confRange)

	// 12. 装配结果
	result := &model.EvaluationResult{
		ForkliftType:   req.ForkliftType,
		Brand:          req.Brand,
		Model:          req.Model,
		OriginalPrice:  req.OriginalPrice,
		PurchaseYear:   req.PurchaseYear,
		SaleYear:       req.SaleYear,
		UsageHours:     req.UsageHours,
		WorkCondition:  req.WorkCondition,
		FuelType:       req.FuelType,
		CanDrive:       req.CanDrive,
		HydraulicOk:    req.HydraulicOk,
		Items:          convertToItemResults(req.Items, s.Parts.GetParts(req.ForkliftType)),
		KTime:          ktRes.KTime,
		KHours:         khRes.KHours,
		KWork:          kw,
		KBrand:         kb,
		KCondition:     kcRes.KCondition,
		KMarket:        km,
		EstimatedValue: roundTo2(estimated),
		ConfidenceLow:  roundTo2(confLow),
		ConfidenceHigh: roundTo2(confHigh),
	}
	// 13. 派生维度评分 + 文本建议
	result.DimensionScores = buildDimensionScores(result)
	result.Suggestions = buildSuggestions(result)
	return result, nil
}

// buildDimensionScores 把 6 个 K 包装成中文标签的 map，方便前端直接展示
func buildDimensionScores(r *model.EvaluationResult) map[string]float64 {
	return map[string]float64{
		"时间维度": roundTo2(r.KTime),
		"使用强度": roundTo2(r.KHours),
		"工况":   roundTo2(r.KWork),
		"品牌":   roundTo2(r.KBrand),
		"车况":   roundTo2(r.KCondition),
		"市场":   roundTo2(r.KMarket),
	}
}

// buildSuggestions 基于评估结果生成文本建议
// 每条建议是一个短句，前端直接用 <li> 列表展示
func buildSuggestions(r *model.EvaluationResult) []string {
	s := make([]string, 0, 8)

	// 1. 车况维度（核心）
	switch {
	case r.KCondition >= 0.95:
		s = append(s, "车况整体保持良好，建议正常保养延续使用")
	case r.KCondition >= 0.80:
		s = append(s, "车况尚可，部分部件存在磨损")
	case r.KCondition >= 0.60:
		s = append(s, "车况一般，多个部件需要维修")
	default:
		s = append(s, "车况较差，建议大修或拆件出售")
	}

	// 2. 品牌维度
	switch {
	case r.KBrand >= 1.0:
		s = append(s, "国际一线品牌保值能力强，残值稳定")
	case r.KBrand >= 0.9:
		s = append(s, "品牌力较好，残值具备一定支撑")
	case r.KBrand < 0.85:
		s = append(s, "二线品牌残值相对偏低")
	}

	// 3. 时间维度
	if r.KTime < 0.70 {
		s = append(s, "使用年限较长，残值随时间明显折减")
	}

	// 4. 使用强度维度
	if r.KHours < 0.85 {
		s = append(s, "累计使用小时偏高，机械磨损较大")
	}

	// 5. 工况维度
	switch r.WorkCondition {
	case model.WorkConditionSite:
		s = append(s, "工地工况强度大，结构件易损")
	case model.WorkConditionCold:
		s = append(s, "冷库环境对电池和液压油寿命有影响")
	case model.WorkConditionPort:
		s = append(s, "港口高强度连续作业，液压与制动系统需重点关注")
	}

	// 6. 行驶 / 液压硬性指标
	if !r.CanDrive {
		s = append(s, "车辆当前无法正常行驶，需先修复再评估")
	}
	if !r.HydraulicOk {
		s = append(s, "液压系统异常，建议维修后再出售")
	}

	// 7. 部件状态统计
	replaceCount, repairCount := 0, 0
	for _, it := range r.Items {
		switch it.Status {
		case model.ItemStatusNeedReplace:
			replaceCount++
		case model.ItemStatusNeedRepair:
			repairCount++
		}
	}
	if replaceCount > 0 {
		s = append(s, fmt.Sprintf("有 %d 个部件需更换", replaceCount))
	}
	if repairCount > 0 {
		s = append(s, fmt.Sprintf("有 %d 个部件需维修", repairCount))
	}

	// 8. 残值率
	if r.OriginalPrice > 0 {
		rate := r.EstimatedValue / r.OriginalPrice
		if rate >= 0.7 {
			s = append(s, "残值率较高，建议按当前车况正常出售")
		} else if rate < 0.3 {
			s = append(s, "残值率较低，建议拆件出售或作为配件使用")
		}
	}

	return s
}

// ReconstructFromRow 从持久化的 Evaluation 行 + 部件状态重建维度评分与建议
// 供 handler.Get 在不回算完整 K 系数的情况下补全 dimension_scores / suggestions
func ReconstructFromRow(eval repository.Evaluation, items []model.ItemResult) (map[string]float64, []string) {
	r := &model.EvaluationResult{
		OriginalPrice:  eval.OriginalPrice,
		WorkCondition:  model.WorkCondition(eval.WorkCondition),
		CanDrive:       eval.CanDrive,
		HydraulicOk:    eval.HydraulicOk,
		KTime:          eval.KTime,
		KHours:         eval.KHours,
		KWork:          eval.KWork,
		KBrand:         eval.KBrand,
		KCondition:     eval.KCondition,
		KMarket:        eval.KMarket,
		EstimatedValue: eval.EstimatedValue,
		Items:          items,
	}
	return buildDimensionScores(r), buildSuggestions(r)
}

// convertToItemResults 将用户提交的 items 与部件配置合并，得到带权重的 ItemResult
func convertToItemResults(items []model.ItemInput, configs []model.PartConfigInfo) []model.ItemResult {
	// 配置索引：O(1) 查找，避免双层循环
	cfgMap := make(map[string]model.PartConfigInfo, len(configs))
	for _, c := range configs {
		cfgMap[c.ItemCode] = c
	}

	out := make([]model.ItemResult, 0, len(items))
	for _, it := range items {
		cfg, ok := cfgMap[it.ItemCode]
		if !ok {
			// 未知条目：跳过或使用默认值
			continue
		}
		out = append(out, model.ItemResult{
			CategoryCode:   cfg.CategoryCode,
			CategoryName:   cfg.CategoryName,
			ItemCode:       cfg.ItemCode,
			ItemName:       cfg.ItemName,
			Status:         it.Status,
			CategoryWeight: cfg.CategoryWeight,
			ItemWeight:     cfg.ItemWeight,
			Score:          it.Status.Score(),
		})
	}
	return out
}

// roundTo2 四舍五入到 2 位小数（保留金额精度）
func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}
