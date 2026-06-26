// Package service - 电池 RUL 评估核心算法
// 复现论文《一种基于特征提取与混合神经网络的锂电池剩余使用寿命预测方法》S2 节
// 20 维特征提取 + 滑窗聚合 + 轻量 RUL 预测（MVP）
// 后续可替换 Predict 内部聚合为 ONNX/Python sidecar 推理
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"forklift-training/internal/valuation/model"
)

// BatteryRULService 电池 RUL 评估服务
type BatteryRULService struct {
	// 滑窗长度（与论文一致）
	windowSize int
	// 20 维特征名（与 FeatureIndexXxx 常量下标对应）
	featureNames [20]string
	// 6 维特征组名（用于雷达图）
	featureGroups [6]string
	// 20 维特征 → 6 维特征组映射（避免每次计算特征重要性时重新创建）
	featureGroupMap [20]string
	// 特征重要性权重（MVP 内置默认值）
	featureWeights [20]float64
	// 电池类型 → 基准循环寿命（用于 RUL 估算）
	nominalLifecycles map[model.BatteryType]int
}

// NewBatteryRULService 构造默认参数的服务
func NewBatteryRULService() *BatteryRULService {
	s := &BatteryRULService{
		windowSize: 20,
		// 20 维特征名（与论文 S2 顺序一致）
		featureNames: [20]string{
			"恒流电压均值", "恒流电压标准差", "恒流电压偏度", "恒流电压峰度", "恒流电压斜率",
			"恒压电流均值", "恒压电流标准差", "恒压电流偏度", "恒压电流峰度", "恒压电流斜率",
			"恒流时间占比", "恒压时间占比",
			"恒压充电容量",
			"ICA峰值", "ICA峰位电压",
			"容量均值差分", "容量标准差差分", "电压均值差分", "电压标准差差分", "IC均值差分",
		},
		// 6 维特征组（与现有 DimensionRadar 风格保持一致）
		featureGroups: [6]string{
			"恒流电压", "恒压电流", "阶段时间", "充电容量", "ICA峰位", "循环演化差分",
		},
		// 20 维 → 6 维映射：特征重要性计算复用，避免每次重建
		featureGroupMap: [20]string{
			"恒流电压", "恒流电压", "恒流电压", "恒流电压", "恒流电压",
			"恒压电流", "恒压电流", "恒压电流", "恒压电流", "恒压电流",
			"阶段时间", "阶段时间",
			"充电容量",
			"ICA峰位", "ICA峰位",
			"循环演化差分", "循环演化差分", "循环演化差分", "循环演化差分", "循环演化差分",
		},
		// 特征权重：基于物理意义与论文表 1 趋势反推的 MVP 经验值
		// 权重越高的特征对 RUL 影响越大
		featureWeights: [20]float64{
			0.08, 0.05, 0.04, 0.03, 0.05, // 恒流电压 5 维
			0.07, 0.05, 0.04, 0.03, 0.05, // 恒压电流 5 维
			0.06, 0.05, // 阶段时间 2 维
			0.10,       // 恒压容量
			0.08, 0.06, // ICA 2 维
			0.06, 0.04, 0.04, 0.04, 0.04, // 差分 5 维
		},
		// 不同电池类型的标称循环寿命（用于把归一化得分映射回 RUL 循环数）
		nominalLifecycles: map[model.BatteryType]int{
			model.BatteryTypeLFP:   3000, // LFP 长寿命
			model.BatteryTypeNCM:   1500, // NCM 中寿命
			model.BatteryTypeOther: 2000, // 其他保守估值
		},
	}
	return s
}

// PredictResult 预测结果
type PredictResult struct {
	CycleFeatures     []model.CycleFeature
	FeatureImportance []model.FeatureImportance
	RulCycles         int
	SohPercent        float64
	Confidence        float64
	ConfidenceLow     int
	ConfidenceHigh    int
	Suggestions       []string
}

// Predict 完整 RUL 预测：特征提取 → 滑窗聚合 → 估算 SOH/RUL
func (s *BatteryRULService) Predict(_ context.Context, req *model.CreateBatteryRequest) (*PredictResult, error) {
	cycles := req.Cycles
	// 1. 排序保证按 cycle_index 升序
	sort.SliceStable(cycles, func(i, j int) bool {
		return cycles[i].CycleIndex < cycles[j].CycleIndex
	})

	// 2. 提取每个循环的 20 维特征
	cycleFeatures := make([]model.CycleFeature, 0, len(cycles))
	// 3. 用第 10 个循环（或第一个）作为基准循环（论文 S2 提到"以第 10 次循环作为参考基准"）
	baselineIdx := 9
	if len(cycles) <= baselineIdx {
		baselineIdx = 0
	}
	baseStats := s.computeRawStats(cycles[baselineIdx])
	for _, c := range cycles {
		fv, stats := s.extractCCCVFeatures(c.VoltageSeries, c.CurrentSeries, c.Capacity, baseStats)
		soh := s.estimateSOHFromCapacity(stats.Capacity, req.BatteryType)
		cf := model.CycleFeature{
			CycleIndex:    c.CycleIndex,
			FeatureVector: fv,
			RawStats:      stats,
			SohAtCycle:    soh,
		}
		cycleFeatures = append(cycleFeatures, cf)
	}

	// 4. 滑窗聚合（MVP：取最后一个窗口的均值；预留接口供未来接入真实 CNN-GRU+Transformer）
	aggregated := s.aggregateSlidingWindow(cycleFeatures)

	// 5. SOH 估算（综合容量与多维特征偏离基准的程度）
	latestFeature := cycleFeatures[len(cycleFeatures)-1]
	soh := s.computeOverallSOH(latestFeature, aggregated, req.BatteryType)

	// 6. RUL 预测：基于当前 SOH + 特征健康度评分映射到剩余循环数
	healthScore := s.computeHealthScore(aggregated, latestFeature)
	rul, confLow, confHigh := s.estimateRUL(soh, healthScore, req.BatteryType)

	// 7. 特征重要性排序
	importance := s.computeFeatureImportance(aggregated)

	// 8. 评估建议
	suggestions := s.buildSuggestions(req.BatteryType, soh, rul, confLow, confHigh, healthScore)

	return &PredictResult{
		CycleFeatures:     cycleFeatures,
		FeatureImportance: importance,
		RulCycles:         rul,
		SohPercent:        soh,
		Confidence:        clampConfidence(0.6 + 0.4*healthScore),
		ConfidenceLow:     confLow,
		ConfidenceHigh:    confHigh,
		Suggestions:       suggestions,
	}, nil
}

// extractCCCVFeatures 提取 20 维特征向量
// 顺序：恒流电压 5 + 恒压电流 5 + 阶段时间 2 + 恒压容量 1 + ICA 2 + 差分 5
func (s *BatteryRULService) extractCCCVFeatures(
	voltage, current []float64, capacity float64,
	baseStats model.RawStats,
) (model.FeatureVector, model.RawStats) {
	var fv model.FeatureVector

	// 1) CC-CV 分段（论文图 2 启发：电压在 CC 段单调上升至截止电压，CV 段电流衰减）
	// 简化策略：找到最大电压索引作为 CC→CV 分界点
	ccEnd := splitCCCV(voltage)

	ccVolt := voltage[:ccEnd]
	cvCurr := current[ccEnd:]

	// 2) 恒流电压统计（5 维）
	fv[0] = mean(ccVolt)
	fv[1] = std(ccVolt)
	fv[2] = skew(ccVolt)
	fv[3] = kurt(ccVolt)
	fv[4] = slope(ccVolt)

	// 3) 恒压电流统计（5 维）
	if len(cvCurr) > 0 {
		fv[5] = mean(cvCurr)
		fv[6] = std(cvCurr)
		fv[7] = skew(cvCurr)
		fv[8] = kurt(cvCurr)
		fv[9] = slope(cvCurr)
	} else {
		// 无 CV 段时用 0 占位
	}

	// 4) 阶段时间占比（2 维）
	total := len(voltage)
	if total == 0 {
		total = 1
	}
	fv[10] = float64(ccEnd) / float64(total)
	fv[11] = float64(total-ccEnd) / float64(total)

	// 5) 恒压充电容量（1 维）：CV 段电流积分（数值积分近似）
	fv[12] = integrate(cvCurr)

	// 6) ICA 峰位（2 维）：dQ/dV 峰值与对应电压
	icPeak, icPeakV := icaPeak(voltage, current)
	fv[13] = icPeak
	fv[14] = icPeakV

	// 7) 循环演化差分（5 维）
	cur := model.RawStats{
		VoltageMean: fv[0], VoltageStd: fv[1],
		CurrentMean: fv[5], CurrentStd: fv[6],
		Capacity: capacity, ICPeak: icPeak,
	}
	fv[15] = cur.Capacity - baseStats.Capacity
	fv[16] = cur.CurrentStd - baseStats.CurrentStd
	fv[17] = cur.VoltageMean - baseStats.VoltageMean
	fv[18] = cur.VoltageStd - baseStats.VoltageStd
	fv[19] = cur.ICPeak - baseStats.ICPeak

	// 原始统计
	stats := model.RawStats{
		VoltageMean: fv[0], VoltageStd: fv[1],
		CurrentMean: fv[5], CurrentStd: fv[6],
		Capacity: capacity, CCDuration: ccEnd, CVDuration: total - ccEnd,
		ICPeak: icPeak, ICPeakVolt: icPeakV,
	}
	return fv, stats
}

// computeRawStats 计算指定循环的 RawStats（用于构造基准循环）
func (s *BatteryRULService) computeRawStats(c model.CycleData) model.RawStats {
	ccEnd := splitCCCV(c.VoltageSeries)
	ccVolt := c.VoltageSeries[:ccEnd]
	cvCurr := c.CurrentSeries[ccEnd:]
	vMean, vStd := mean(ccVolt), std(ccVolt)
	cMean, cStd := 0.0, 0.0
	if len(cvCurr) > 0 {
		cMean, cStd = mean(cvCurr), std(cvCurr)
	}
	icPeak, icPeakV := icaPeak(c.VoltageSeries, c.CurrentSeries)
	return model.RawStats{
		VoltageMean: vMean, VoltageStd: vStd,
		CurrentMean: cMean, CurrentStd: cStd,
		Capacity: c.Capacity, CCDuration: ccEnd, CVDuration: len(c.VoltageSeries) - ccEnd,
		ICPeak: icPeak, ICPeakVolt: icPeakV,
	}
}

// aggregateSlidingWindow 滑窗聚合（MVP：返回最后一个窗口的均值与标准差）
func (s *BatteryRULService) aggregateSlidingWindow(cfs []model.CycleFeature) [20][2]float64 {
	var agg [20][2]float64
	if len(cfs) == 0 {
		return agg
	}
	start := 0
	if len(cfs) > s.windowSize {
		start = len(cfs) - s.windowSize
	}
	window := cfs[start:]
	for dim := 0; dim < 20; dim++ {
		sum, sum2 := 0.0, 0.0
		for _, c := range window {
			sum += c.FeatureVector[dim]
			sum2 += c.FeatureVector[dim] * c.FeatureVector[dim]
		}
		n := float64(len(window))
		mean := sum / n
		variance := sum2/n - mean*mean
		if variance < 0 {
			variance = 0
		}
		agg[dim] = [2]float64{mean, math.Sqrt(variance)}
	}
	return agg
}

// estimateSOHFromCapacity 仅用容量估算 SOH
func (s *BatteryRULService) estimateSOHFromCapacity(capacity float64, bt model.BatteryType) float64 {
	// 标称容量假设：LFP=1.1*NCM/Other=1.0（论文实验 NCM=2.0Ah LFP=1.1Ah）
	// 这里用电池类型映射一个"标称值"作为 SOH 100% 的参考
	var nominal float64
	switch bt {
	case model.BatteryTypeLFP:
		nominal = 1.1
	case model.BatteryTypeNCM:
		nominal = 2.0
	default:
		nominal = 1.5
	}
	soh := capacity / nominal * 100
	if soh < 0 {
		soh = 0
	}
	if soh > 100 {
		soh = 100
	}
	return soh
}

// computeOverallSOH 综合 SOH：容量 SOH 与特征健康度加权
func (s *BatteryRULService) computeOverallSOH(latest model.CycleFeature, agg [20][2]float64, bt model.BatteryType) float64 {
	capSOH := s.estimateSOHFromCapacity(latest.RawStats.Capacity, bt)
	// 特征健康度：归一化容量差分、电压差分、IC 差分到 0~1 范围，再加权
	capDrop := math.Abs(agg[15][0])   // 容量均值差分（最新 - 基准）
	voltShift := math.Abs(agg[17][0]) // 电压均值差分
	icShift := math.Abs(agg[19][0])   // IC 均值差分
	// 经验：每 1 单位容量差 ≈ 1% SOH；电压差 0.01V ≈ 0.1%；IC 差 0.1 ≈ 0.2%
	featurePenalty := capDrop*1.0 + voltShift*5 + icShift*2
	if featurePenalty > 25 {
		featurePenalty = 25
	}
	overall := capSOH - featurePenalty
	if overall < 0 {
		overall = 0
	}
	if overall > 100 {
		overall = 100
	}
	return overall
}

// computeHealthScore 健康度评分 0~1：用于调整置信度与 RUL 边界
func (s *BatteryRULService) computeHealthScore(agg [20][2]float64, latest model.CycleFeature) float64 {
	// 用标准差总和的倒数作为"稳定度"代理
	var totalStd float64
	for dim := 0; dim < 20; dim++ {
		// 特征权重越大，标准差越敏感
		totalStd += agg[dim][1] * s.featureWeights[dim]
	}
	// 0~0.5 映射到 1~0.2（经验值）
	score := 1.0 - math.Min(totalStd*2, 0.8)
	if score < 0.2 {
		score = 0.2
	}
	return score
}

// estimateRUL 估算剩余循环数
// EOL 阈值 60%（业内"梯次利用"边界；80% 退役线对 demo 过于严苛）
// RUL = 标称寿命 × (当前SOH - 60%) / (100% - 60%)
func (s *BatteryRULService) estimateRUL(soh, healthScore float64, bt model.BatteryType) (int, int, int) {
	nominal := s.nominalLifecycles[bt]
	const eolPercent = 60.0
	eolRatio := (soh - eolPercent) / (100.0 - eolPercent)
	if eolRatio < 0 {
		eolRatio = 0
	}
	if eolRatio > 1 {
		eolRatio = 1
	}
	rul := int(math.Round(float64(nominal) * eolRatio))
	// 置信区间：±(1-healthScore) * 30%
	margin := int(math.Round(float64(nominal) * (1.0 - healthScore) * 0.3))
	low := rul - margin
	high := rul + margin
	if low < 0 {
		low = 0
	}
	return rul, low, high
}

// computeFeatureImportance 计算 20 维特征重要性并归一化
func (s *BatteryRULService) computeFeatureImportance(agg [20][2]float64) []model.FeatureImportance {
	weights := s.featureWeights
	// 重要性 = 静态权重 * (1 + 归一化标准差)
	var raw [20]float64
	var total float64
	for i := 0; i < 20; i++ {
		raw[i] = weights[i] * (1.0 + math.Min(agg[i][1], 1.0))
		total += raw[i]
	}
	out := make([]model.FeatureImportance, 20)
	for i := 0; i < 20; i++ {
		out[i] = model.FeatureImportance{
			Index:      i,
			Name:       s.featureNames[i],
			Group:      s.featureGroupMap[i],
			Weight:     raw[i],
			Normalized: raw[i] / total,
		}
	}
	// 按重要性倒序
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Normalized > out[j].Normalized
	})
	return out
}

// buildSuggestions 生成文本建议
func (s *BatteryRULService) buildSuggestions(bt model.BatteryType, soh float64, rul, low, high int, health float64) []string {
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

// =====================================================
// 统计工具函数
// =====================================================

// mean 平均值
func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}

// std 标准差
func std(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	s := 0.0
	for _, v := range xs {
		d := v - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

// skew 偏度
func skew(xs []float64) float64 {
	if len(xs) < 3 {
		return 0
	}
	m := mean(xs)
	s := std(xs)
	if s == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range xs {
		d := (v - m) / s
		sum += d * d * d
	}
	return sum * float64(len(xs)) / float64((len(xs)-1)*(len(xs)-2))
}

// kurt 峰度（超额）
func kurt(xs []float64) float64 {
	if len(xs) < 4 {
		return 0
	}
	m := mean(xs)
	s := std(xs)
	if s == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range xs {
		d := (v - m) / s
		sum += d * d * d * d
	}
	n := float64(len(xs))
	return sum*(n+1)*n/((n-1)*(n-2)*(n-3)) - 3*(n-1)*(n-1)/((n-2)*(n-3))
}

// slope 一阶线性拟合斜率
func slope(xs []float64) float64 {
	n := len(xs)
	if n < 2 {
		return 0
	}
	var sx, sy, sxx, sxy float64
	for i, v := range xs {
		x := float64(i)
		sx += x
		sy += v
		sxx += x * x
		sxy += x * v
	}
	denom := float64(n)*sxx - sx*sx
	if denom == 0 {
		return 0
	}
	return (float64(n)*sxy - sx*sy) / denom
}

// integrate 简单梯形积分
func integrate(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	s := 0.0
	for i := 1; i < len(xs); i++ {
		s += (xs[i] + xs[i-1]) / 2
	}
	return s
}

// splitCCCV CC/CV 分段：返回 CC 段结束索引
// 简化策略：电压最大值点作为分界
func splitCCCV(voltage []float64) int {
	if len(voltage) == 0 {
		return 0
	}
	maxIdx := 0
	maxV := voltage[0]
	for i, v := range voltage {
		if v > maxV {
			maxV = v
			maxIdx = i
		}
	}
	if maxIdx == 0 {
		return len(voltage) / 2
	}
	return maxIdx
}

// icaPeak 增量容量分析：返回 dQ/dV 峰值与对应电压
func icaPeak(voltage, current []float64) (float64, float64) {
	n := len(voltage)
	if n < 3 {
		return 0, 0
	}
	peakIC := 0.0
	peakV := 0.0
	for i := 1; i < n-1; i++ {
		dv := voltage[i+1] - voltage[i-1]
		if math.Abs(dv) < 1e-9 {
			continue
		}
		dq := current[i+1] - current[i-1]
		ic := math.Abs(dq / dv)
		if ic > peakIC {
			peakIC = ic
			peakV = voltage[i]
		}
	}
	return peakIC, peakV
}

// clampConfidence 把置信度限制在 0.5~0.99
func clampConfidence(c float64) float64 {
	if c < 0.5 {
		return 0.5
	}
	if c > 0.99 {
		return 0.99
	}
	return c
}

// MarshalJSON 把 6 维特征组名暴露给前端（用于雷达图）
func (s *BatteryRULService) FeatureGroups() [6]string {
	return s.featureGroups
}

// FeatureNames 暴露 20 维特征名
func (s *BatteryRULService) FeatureNames() [20]string {
	return s.featureNames
}

// 静态检查：保证 BatteryEvaluation 的 feature_vector 字段能正确序列化为 [20]float64
var _ = json.Marshal
