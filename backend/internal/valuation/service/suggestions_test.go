package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forklift-training/internal/valuation/model"
)

// memResolver 内存版系数配置读取（测试用）。
type memResolver struct {
	values map[string]float64
}

func (r *memResolver) Get(_ context.Context, key string) (float64, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return 0, errors.New("coefficient not found: " + key)
}

func (r *memResolver) ReadFloat(ctx context.Context, key string, fallback float64) float64 {
	return coefReadFloat(ctx, r, key, fallback)
}

func newDefaultResolver() *memResolver {
	return &memResolver{values: map[string]float64{
		KeyKcNoLicensePenaltyPct:      0.10,
		KeyKcNoRegistrationPenaltyPct: 0.10,
		KeyKcPaintBonus:               0.02,
		KeyKcMaintenanceBonus:         0.02,
	}}
}

func suggestionTexts(in SuggestionsInput, resolver CoefficientResolver) []string {
	return BuildSuggestions(context.Background(), in, resolver)
}

func TestBuildSuggestions_ConditionSections(t *testing.T) {
	cases := []struct {
		name  string
		input SuggestionsInput
		want  string
	}{
		{"优秀", SuggestionsInput{KCondition: 1.05}, "车况优秀"},
		{"良好", SuggestionsInput{KCondition: 0.90}, "车况良好"},
		{"一般", SuggestionsInput{KCondition: 0.70}, "整备后出售"},
		{"较差", SuggestionsInput{KCondition: 0.50}, "折价处理"},
		{"很差", SuggestionsInput{KCondition: 0.30}, "拆件出售"},
	}
	for _, c := range cases {
		out := suggestionTexts(c.input, newDefaultResolver())
		if len(out) == 0 || !strings.Contains(out[0], c.want) {
			t.Errorf("%s: 首条建议应为车况文案（含 %q），得到 %v", c.name, c.want, out)
		}
	}
}

func TestBuildSuggestions_LicenseAndCert(t *testing.T) {
	in := SuggestionsInput{KCondition: 0.8, HasLicensePlate: false, HasRegistrationCertificate: false}
	out := suggestionTexts(in, newDefaultResolver())
	joined := strings.Join(out, "|")
	if !strings.Contains(joined, "缺少车牌，残值扣减 10%") {
		t.Errorf("缺车牌建议应含扣减 10%%: %v", out)
	}
	if !strings.Contains(joined, "缺少登记证，残值扣减 10%") {
		t.Errorf("缺登记证建议应含扣减 10%%: %v", out)
	}
	if !strings.Contains(joined, "车牌与登记证均缺失") {
		t.Errorf("双证缺失应给出强警告: %v", out)
	}
}

// TestBuildSuggestions_ConfigDrivenPercentages 管理员调整配置后，建议百分比动态跟随（PDF 重建路径回归）。
func TestBuildSuggestions_ConfigDrivenPercentages(t *testing.T) {
	resolver := newDefaultResolver()
	resolver.values[KeyKcNoLicensePenaltyPct] = 0.05
	resolver.values[KeyKcPaintBonus] = 0.03

	in := SuggestionsInput{KCondition: 0.8, HasLicensePlate: false, OriginalPaint: true}
	out := suggestionTexts(in, resolver)
	joined := strings.Join(out, "|")

	if !strings.Contains(joined, "缺少车牌，残值扣减 5%") {
		t.Errorf("应读取配置 5%%，得到: %v", out)
	}
	if !strings.Contains(joined, "原厂漆完整，加成 3%") {
		t.Errorf("应读取配置 3%%，得到: %v", out)
	}
}

func TestBuildSuggestions_BrandIntensityRatio(t *testing.T) {
	in := SuggestionsInput{KCondition: 0.8, KBrand: 1.0, KHours: 1.2} // 比值 1.2 → 加速
	out := suggestionTexts(in, newDefaultResolver())
	if !strings.Contains(strings.Join(out, "|"), "时间衰减被加速") {
		t.Errorf("Kh/Kb=1.2 应提示衰减加速: %v", out)
	}

	in2 := SuggestionsInput{KCondition: 0.8, KBrand: 1.2, KHours: 1.0} // 比值 0.83 → 减缓
	out2 := suggestionTexts(in2, newDefaultResolver())
	if !strings.Contains(strings.Join(out2, "|"), "时间衰减被明显减缓") {
		t.Errorf("Kh/Kb<0.90 应提示衰减减缓: %v", out2)
	}
}

func TestBuildSuggestions_MarketAndResidualRate(t *testing.T) {
	// 市场偏高
	in := SuggestionsInput{KCondition: 0.8, KMarket: 1.05, OriginalPrice: 100000, EstimatedValue: 90000}
	out := suggestionTexts(in, newDefaultResolver())
	if !strings.Contains(strings.Join(out, "|"), "二手需求旺盛") {
		t.Errorf("KMarket>1.02 应提示需求旺盛: %v", out)
	}
	// 残值率 ≥ 70%
	in2 := SuggestionsInput{KCondition: 0.8, KMarket: 1.0, OriginalPrice: 100000, EstimatedValue: 80000}
	out2 := suggestionTexts(in2, newDefaultResolver())
	if !strings.Contains(strings.Join(out2, "|"), "残值率较高") {
		t.Errorf("残值率 80%% 应提示正常出售: %v", out2)
	}
	// 残值率 < 30%
	in3 := SuggestionsInput{KCondition: 0.2, KMarket: 0.9, OriginalPrice: 100000, EstimatedValue: 20000}
	out3 := suggestionTexts(in3, newDefaultResolver())
	if !strings.Contains(strings.Join(out3, "|"), "拆件出售") {
		t.Errorf("残值率 20%% 应提示拆件: %v", out3)
	}
}

// TestSuggestions_ResultAndDetail_Consistent 同一输入经 FromResult 与 FromDetail 映射后，建议输出一致（PDF 重建与评估流程同源）。
func TestSuggestions_ResultAndDetail_Consistent(t *testing.T) {
	resolver := newDefaultResolver()
	ctx := context.Background()

	r := &model.EvaluationResult{
		EvaluationRequest: model.EvaluationRequest{
			OriginalPaint: true, HasMaintenanceRecords: true,
			HasLicensePlate: false, HasRegistrationCertificate: true,
		},
		KCondition: 0.8, KHours: 1.1, KBrand: 1.0, KTime: 0.7, KMarket: 1.01,
		OriginalPrice: 100000, EstimatedValue: 70000,
	}
	d := &model.EvaluationDetail{
		OriginalPaint: true, HasMaintenanceRecords: true,
		HasLicensePlate: false, HasRegistrationCertificate: true,
		KCondition: 0.8, KHours: 1.1, KBrand: 1.0, KTime: 0.7, KMarket: 1.01,
		OriginalPrice: 100000, EstimatedValue: 70000,
	}

	fromResult := BuildSuggestions(ctx, FromResult(r), resolver)
	fromDetail := BuildSuggestions(ctx, FromDetail(d), resolver)

	if len(fromResult) != len(fromDetail) {
		t.Fatalf("结果与详情建议数量不一致: %d vs %d", len(fromResult), len(fromDetail))
	}
	for i := range fromResult {
		if fromResult[i] != fromDetail[i] {
			t.Errorf("第 %d 条建议不一致: %q vs %q", i, fromResult[i], fromDetail[i])
		}
	}
}

func TestBuildBatterySuggestions(t *testing.T) {
	out := BuildBatterySuggestions(model.BatteryTypeLFP, 90, 500, 450, 550, 1.0)
	joined := strings.Join(out, "|")
	if !strings.Contains(joined, "SOH≥95") && !strings.Contains(joined, "80%≤SOH<95%") {
		t.Errorf("健康度文案缺失: %v", out)
	}
	if !strings.Contains(joined, "预测剩余循环数约 500 次（置信区间 450~550）") {
		t.Errorf("剩余寿命文案缺失: %v", out)
	}
	if !strings.Contains(joined, "LFP") {
		t.Errorf("LFP 类型提示缺失: %v", out)
	}
	// 稳定性提示仅在 health < 0.5 时出现
	if strings.Contains(joined, "特征波动较大") {
		t.Errorf("health=1.0 不应出现稳定性提示: %v", out)
	}
	unstable := BuildBatterySuggestions(model.BatteryTypeNCM, 50, 100, 80, 120, 0.3)
	if !strings.Contains(strings.Join(unstable, "|"), "特征波动较大") {
		t.Errorf("health<0.5 应提示稳定性: %v", unstable)
	}
}
