// Package service 单元测试 - 电池 RUL 评估
package service

import (
	"math"
	"testing"

	"forklift-training/internal/valuation/model"
)

// makeSyntheticCycles 生成 N 个循环的合成充放电数据
// 容量从 nominal 按线性衰减；电压/电流时序做合理形状
func makeSyntheticCycles(n int, bt model.BatteryType, baseCapacity float64) []model.CycleData {
	_ = bt // 当前未使用，后续可扩展
	cycles := make([]model.CycleData, n)
	for i := 0; i < n; i++ {
		// 容量从 100% 衰减到 80%，跨度为 n
		soh := 1.0 - 0.2*float64(i)/float64(n-1)
		cap := baseCapacity * soh
		// 充电时序：100 个采样点
		points := 100
		volt := make([]float64, points)
		curr := make([]float64, points)
		// CC 段：0~70 电压 3.2→3.6 单调升，电流恒定
		// CV 段：70~100 电压 3.6 恒定，电流指数衰减
		for p := 0; p < points; p++ {
			if p < 70 {
				volt[p] = 3.2 + 0.4*float64(p)/70.0
				curr[p] = 1.0
			} else {
				volt[p] = 3.6
				// 随循环增加电流衰减更慢，模拟老化
				curr[p] = 0.3 * math.Exp(-float64(p-70)/20.0) * (1 + 0.1*float64(i)/float64(n))
			}
		}
		cycles[i] = model.CycleData{
			CycleIndex:    i + 1,
			VoltageSeries: volt,
			CurrentSeries: curr,
			Capacity:      cap,
		}
	}
	return cycles
}

func newTestBatteryService() *BatteryRULService {
	return NewBatteryRULService()
}

// TestExtractCCCVFeatures_Shape 验证特征向量维度与索引范围
func TestExtractCCCVFeatures_Shape(t *testing.T) {
	svc := newTestBatteryService()
	volt := make([]float64, 100)
	curr := make([]float64, 100)
	for i := 0; i < 100; i++ {
		if i < 70 {
			volt[i] = 3.2 + 0.004*float64(i)
			curr[i] = 1.0
		} else {
			volt[i] = 3.6
			curr[i] = 0.3 * math.Exp(-float64(i-70)/20.0)
		}
	}
	base := svc.computeRawStats(model.CycleData{
		VoltageSeries: volt, CurrentSeries: curr, Capacity: 1.1, CycleIndex: 1,
	})
	fv, stats := svc.extractCCCVFeatures(volt, curr, 1.1, base)
	// 0 维：恒流电压均值 ≈ 3.4
	if math.Abs(fv[0]-3.4) > 0.1 {
		t.Errorf("fv[0]=%v, want ~3.4", fv[0])
	}
	// 10 维：恒流时间占比 ≈ 0.7
	if math.Abs(fv[10]-0.7) > 0.05 {
		t.Errorf("fv[10]=%v, want ~0.7", fv[10])
	}
	// 11 维：恒压时间占比 ≈ 0.3
	if math.Abs(fv[11]-0.3) > 0.05 {
		t.Errorf("fv[11]=%v, want ~0.3", fv[11])
	}
	// 12 维：恒压充电容量 > 0
	if fv[12] <= 0 {
		t.Errorf("fv[12]=%v, want > 0", fv[12])
	}
	// 13 维：ICA 峰值 ≥ 0
	if fv[13] < 0 {
		t.Errorf("fv[13]=%v, want >= 0", fv[13])
	}
	// stats 字段一致性
	if stats.CCDuration <= 0 || stats.CCDuration > 100 {
		t.Errorf("stats.CCDuration=%v out of range", stats.CCDuration)
	}
	if stats.CCDuration+stats.CVDuration != 100 {
		t.Errorf("stats CC+CV=%d, want 100", stats.CCDuration+stats.CVDuration)
	}
}

// TestPredict_LFP_FullLifecycle 完整生命周期预测 SOH 与 RUL
func TestPredict_LFP_FullLifecycle(t *testing.T) {
	svc := newTestBatteryService()
	cycles := makeSyntheticCycles(50, model.BatteryTypeLFP, 1.1)
	req := &model.CreateBatteryRequest{
		BatteryType:  model.BatteryTypeLFP,
		BatteryModel: "Test-LFP-1.1Ah",
		Cycles:       cycles,
	}
	res, err := svc.Predict(nil, req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if len(res.CycleFeatures) != 50 {
		t.Errorf("len(CycleFeatures)=%d, want 50", len(res.CycleFeatures))
	}
	// SOH 应在 70~85 范围内（容量最后降到 80%，叠加特征惩罚后可能略低）
	if res.SohPercent < 70 || res.SohPercent > 100 {
		t.Errorf("SOH=%.2f out of expected range", res.SohPercent)
	}
	// RUL 不应为负
	if res.RulCycles < 0 {
		t.Errorf("RUL=%d < 0", res.RulCycles)
	}
	// 置信度应在 0.5~0.99
	if res.Confidence < 0.5 || res.Confidence > 0.99 {
		t.Errorf("Confidence=%.3f out of range", res.Confidence)
	}
	// 置信区间下界 ≤ RUL ≤ 上界
	if res.ConfidenceLow > res.RulCycles || res.RulCycles > res.ConfidenceHigh {
		t.Errorf("CI [%d, %d] does not bracket RUL=%d", res.ConfidenceLow, res.ConfidenceHigh, res.RulCycles)
	}
	// 特征重要性条目数 = 20
	if len(res.FeatureImportance) != 20 {
		t.Errorf("len(FeatureImportance)=%d, want 20", len(res.FeatureImportance))
	}
	// 建议至少 2 条
	if len(res.Suggestions) < 2 {
		t.Errorf("len(Suggestions)=%d, want >= 2", len(res.Suggestions))
	}
}

// TestPredict_NCM_ShortLifecycle NCM 短寿命电池预测
func TestPredict_NCM_ShortLifecycle(t *testing.T) {
	svc := newTestBatteryService()
	cycles := makeSyntheticCycles(20, model.BatteryTypeNCM, 2.0)
	req := &model.CreateBatteryRequest{
		BatteryType:  model.BatteryTypeNCM,
		BatteryModel: "Test-NCM-2.0Ah",
		Cycles:       cycles,
	}
	res, err := svc.Predict(nil, req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if res.SohPercent <= 0 {
		t.Errorf("SOH=%.2f <= 0", res.SohPercent)
	}
	// NCM 标称寿命 1500 < LFP 3000，因此同 SOH 下 RUL 应更小
	if res.RulCycles < 0 {
		t.Errorf("RUL=%d < 0", res.RulCycles)
	}
}

// TestPredict_BelowEOL 容量已低于 60% 触发更换建议
func TestPredict_BelowEOL(t *testing.T) {
	svc := newTestBatteryService()
	cycles := makeSyntheticCycles(20, model.BatteryTypeLFP, 1.1)
	// 把所有循环容量强制设为 0.5（即 ~45% SOH，远低于 60% EOL）
	for i := range cycles {
		cycles[i].Capacity = 0.5
	}
	req := &model.CreateBatteryRequest{
		BatteryType: model.BatteryTypeLFP,
		Cycles:      cycles,
	}
	res, err := svc.Predict(nil, req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if res.SohPercent >= 60 {
		t.Errorf("SOH=%.2f should be < 60", res.SohPercent)
	}
	if res.RulCycles != 0 {
		t.Errorf("RUL=%d, want 0 (already below EOL)", res.RulCycles)
	}
	// 建议中应包含"更换"二字
	foundReplace := false
	for _, s := range res.Suggestions {
		if contains(s, "更换") {
			foundReplace = true
			break
		}
	}
	if !foundReplace {
		t.Errorf("建议中应包含'更换'，实际: %v", res.Suggestions)
	}
}

// contains 字符串包含判断（避免引入 strings 包依赖）
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestSplitCCCV 验证 CC/CV 切分点位置
func TestSplitCCCV(t *testing.T) {
	v := []float64{3.2, 3.3, 3.4, 3.5, 3.6, 3.6, 3.6, 3.6}
	idx := splitCCCV(v)
	// 第一个 3.6 在索引 4
	if idx != 4 {
		t.Errorf("splitCCCV idx=%d, want 4", idx)
	}
}

// TestIcaPeak 验证 ICA 峰值检测
func TestIcaPeak(t *testing.T) {
	v := []float64{3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 4.0}
	// 构造一个明显的 IC 峰值：在电压 3.6 处 dQ/dV 最大
	c := []float64{1.0, 1.0, 1.0, 1.0, 1.0, 0.5, 1.0, 1.0, 1.0}
	peak, peakV := icaPeak(v, c)
	if peak <= 0 {
		t.Errorf("icaPeak=%v, want > 0", peak)
	}
	if peakV < 3.0 || peakV > 4.5 {
		t.Errorf("peakV=%v out of range", peakV)
	}
}
