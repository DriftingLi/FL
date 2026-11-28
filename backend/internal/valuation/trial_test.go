package valuation

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"

	"go.uber.org/zap"
)

func TestTrialRejectsMissingReliableInput(t *testing.T) {
	response, err := CalculateTrial(context.Background(), TrialRequest{}, nil, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonInvalidInput {
		t.Fatalf("缺关键输入应 needs_info: %+v", response)
	}
	if len(response.MissingFields) == 0 {
		t.Fatalf("应返回待补项: %+v", response)
	}
}

func TestTrialPolicyDefaultsClosedAndDemoForbiddenInProduction(t *testing.T) {
	snapshot, err := NewTrialSnapshot(ReplayInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		policy string
		appEnv string
		reason string
	}{
		{name: "未显式策略", appEnv: "development", reason: TrialReasonPolicyDisabled},
		{name: "未知策略", policy: "public", appEnv: "development", reason: TrialReasonPolicyDisabled},
		{name: "未配置组合", policy: "pilot", appEnv: "development", reason: TrialReasonPolicyNotConfigured},
		{name: "生产禁用 demo", policy: "demo", appEnv: "production", reason: TrialReasonPolicyForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := trialMinimumRequest()
			req.Policy = tc.policy
			response, err := CalculateTrial(context.Background(), req, snapshot, tc.appEnv)
			if err != nil {
				t.Fatal(err)
			}
			if response.Status != TrialStatusNeedsInfo || response.ReasonCode != tc.reason {
				t.Fatalf("应 fail-closed 返回 %s: %+v", tc.reason, response)
			}
			if response.Interval != nil {
				t.Fatalf("关闭状态不得给价: %+v", response)
			}
		})
	}
}

func trialMinimumRequest() TrialRequest {
	year := 2021
	return TrialRequest{RequestID: "trial-test", Brand: "林德", VehicleType: "电动叉车", Tonnage: 3, FactoryYear: &year, SaleYear: 2026, Policy: "demo"}
}

func TestTrialPreservesNoZeroOtherAndKnownSpecifications(t *testing.T) {
	base := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "无", Tonnage: 3,
		ConfigType: "其它", MastType: "无", MastHeightMM: 0, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	higher := base
	higher.ID, higher.MastHeightMM, higher.OriginalPrice = 2, 3000, 120000
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{base, higher})

	for _, tc := range []struct {
		name       string
		series     string
		mastHeight int
		wantCount  int
		wantMax    float64
	}{
		{name: "无与零是具体配置", series: "无", mastHeight: 0, wantCount: 1, wantMax: 100000},
		{name: "其它不扩展为通配", series: "其它", mastHeight: 0, wantCount: 0},
		{name: "未知门架高度枚举目录", series: "无", wantCount: 2, wantMax: 120000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := trialCompleteRequest()
			req.Series = &tc.series
			req.ConfigType = trialPtr("其它")
			req.MastType = trialPtr("无")
			if tc.name != "未知门架高度枚举目录" {
				req.MastHeightMM = &tc.mastHeight
			} else {
				req.MastHeightMM = nil
			}
			response, err := CalculateTrial(context.Background(), req, snapshot, "development")
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantCount == 0 {
				if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonCatalogConflict {
					t.Fatalf("已知规格冲突应 needs_info: %+v", response)
				}
				return
			}
			if response.Status != TrialStatusPriced || len(response.Candidates) != tc.wantCount {
				t.Fatalf("目录候选数=%d, want=%d: %+v", len(response.Candidates), tc.wantCount, response)
			}
			if response.Interval.Max > tc.wantMax {
				t.Fatalf("不得采用目录外最高价: %+v", response)
			}
		})
	}
}

func TestTrialNeverFallsBackToFuzzyHighestPrice(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	req := trialCompleteRequest()
	req.ConfigType = trialPtr("目录没有的高配置")
	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonCatalogConflict || response.Interval != nil {
		t.Fatalf("精确候选失败时不得模糊取最高价: %+v", response)
	}
}

func TestTrialRequiresCompleteCoefficientSnapshot(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	input := trialReplayInput(t, []OriginalPrice{price})
	input.Coefficients = []CoefficientConfig{{Key: KeyLambdaElectric, Value: 0.12}}
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	response, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonNoValidScenario || response.Interval != nil {
		t.Fatalf("缺系数时不得借硬编码兜底给价: %+v", response)
	}
}

func TestTrialRejectsUnknownConditionInsteadOfNeutralFallback(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	req := trialCompleteRequest()
	req.Condition = trialPtr("Z")
	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonConditionUnreliable || response.Interval != nil {
		t.Fatalf("未知车况不得借正式路径中性兜底给价: %+v", response)
	}
}

func TestTrialReplayBatchReportsThreeWidthThresholds(t *testing.T) {
	lower := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 40000}
	higher := lower
	higher.ID, higher.OriginalPrice = 2, 60000
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{lower, higher})
	complete := snapshot.Request
	invalidComplete := complete
	invalidComplete.Series = ""
	conflict := trialCompleteRequest()
	conflict.ConfigType = trialPtr("不存在")
	fuzzyComplete := complete
	fuzzyComplete.ConfigType = "缺失配置"
	report, err := ReplayTrialBatch(context.Background(), snapshot, []TrialReplayCase{
		{Group: "masked", Request: trialCompleteRequest(), CompleteRequest: &complete},
		{Group: "masked", Request: trialCompleteRequest(), CompleteRequest: &invalidComplete},
		{Group: "conflict", Request: conflict, CompleteRequest: &complete},
		{Group: "fuzzy_complete", Request: trialCompleteRequest(), CompleteRequest: &fuzzyComplete},
	}, "development")
	if err != nil {
		t.Fatal(err)
	}
	if report.SnapshotSHA7 != shortHash(snapshot.Hash) || report.SnapshotHash != snapshot.Hash || report.FormulaHash != TrialFormulaHash || report.PolicyHash != snapshot.PolicyHash {
		t.Fatalf("回放缺少固定快照/策略/公式哈希: %+v", report)
	}
	group := report.Groups["masked"]
	if group.N != 2 || group.Valid != 1 || group.Excluded != 1 {
		t.Fatalf("分组分母错误: %+v", group)
	}
	for _, threshold := range []string{"20", "30"} {
		metrics := group.Thresholds[threshold]
		if metrics.Priced != 0 || metrics.TooWide != 1 || metrics.UnpricedReasons[TrialReasonTooWide] != 1 {
			t.Fatalf("%s%% 应全部过宽: %+v", threshold, metrics)
		}
	}
	metrics := group.Thresholds["40"]
	if metrics.Priced != 1 || metrics.Covered != 1 || metrics.TooWide != 0 || metrics.PricedOverValid != 1 || metrics.CoveredOverPriced != 1 {
		t.Fatalf("40%% 应给价并覆盖正式值: %+v", metrics)
	}
	conflictMetrics := report.Groups["conflict"].Thresholds["40"]
	if conflictMetrics.Priced != 0 || conflictMetrics.UnpricedReasons[TrialReasonCatalogConflict] != 1 {
		t.Fatalf("无价原因应单列: %+v", conflictMetrics)
	}
	fuzzyGroup := report.Groups["fuzzy_complete"]
	if fuzzyGroup.N != 1 || fuzzyGroup.Valid != 0 || fuzzyGroup.Excluded != 1 || report.Results[3].CompleteError != "complete_catalog_not_exact" {
		t.Fatalf("正式对照不得借模糊最高价: group=%+v result=%+v", fuzzyGroup, report.Results[3])
	}
	for threshold, fuzzyMetrics := range fuzzyGroup.Thresholds {
		if fuzzyMetrics.N != 1 || fuzzyMetrics.Valid != 0 || fuzzyMetrics.Excluded != 1 ||
			fuzzyMetrics.Priced != 0 || fuzzyMetrics.Covered != 0 || fuzzyMetrics.TooWide != 0 ||
			fuzzyMetrics.PricedOverValid != 0 || fuzzyMetrics.CoveredOverPriced != 0 || fuzzyMetrics.TooWideOverValid != 0 ||
			len(fuzzyMetrics.UnpricedReasons) != 0 {
			t.Fatalf("%s%% 排除项不得进入任何指标分子: %+v", threshold, fuzzyMetrics)
		}
	}
}

func TestTrialHTTPRouteUsesSnapshotAndWritesNothing(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	reader := &stubTrialSnapshotReader{snapshot: newTrialSnapshotForTest(t, []OriginalPrice{price})}
	router, _, evaluations := newTestValuationEngine(t)
	RegisterTrialRoutes(router, reader, zap.NewNop(), "development")

	response := performRequest(router, http.MethodPost, "/api/valuation/trials", trialCompleteRequest())
	if response.Code != http.StatusOK {
		t.Fatalf("试算状态码=%d: %s", response.Code, response.Body.String())
	}
	data := mustDecodeData(t, response)
	if data["status"] != TrialStatusPriced || data["interval"] == nil {
		t.Fatalf("试算响应无效: %v", data)
	}
	if reader.loads != 1 || len(evaluations.records) != 0 {
		t.Fatalf("应只读一次快照且零正式写入: loads=%d evaluations=%d", reader.loads, len(evaluations.records))
	}
}

func TestTrialHTTPRouteDefaultsClosedBeforeSnapshotRead(t *testing.T) {
	reader := &stubTrialSnapshotReader{}
	router, _, evaluations := newTestValuationEngine(t)
	RegisterTrialRoutes(router, reader, zap.NewNop(), "development")
	req := trialCompleteRequest()
	req.Policy = ""
	response := performRequest(router, http.MethodPost, "/api/valuation/trials", req)
	if response.Code != http.StatusForbidden || reader.loads != 0 || len(evaluations.records) != 0 {
		t.Fatalf("默认关闭应 fail-closed 且不读快照: code=%d loads=%d body=%s", response.Code, reader.loads, response.Body.String())
	}
}

type stubTrialSnapshotReader struct {
	snapshot *TrialSnapshot
	loads    int
}

func (s *stubTrialSnapshotReader) LoadTrialSnapshot(context.Context, TrialRequest) (*TrialSnapshot, error) {
	s.loads++
	if s.snapshot == nil {
		return nil, fmt.Errorf("测试快照未配置")
	}
	return s.snapshot, nil
}

func TestTrialWidthThresholdAndInvalidAggregation(t *testing.T) {
	interval, reason := BuildTrialInterval([]float64{60000, 40000, 0, -1, math.NaN(), math.Inf(1)})
	if reason != "" || interval == nil {
		t.Fatalf("有效正情景应聚合: interval=%+v reason=%s", interval, reason)
	}
	if interval.Min != 40000 || interval.Max != 60000 || interval.Mid != 50000 || interval.Width != 20000 || interval.WidthRatio != 0.4 {
		t.Fatalf("4–6 万应为 40%%: %+v", interval)
	}
	if _, reason := BuildTrialInterval([]float64{math.MaxFloat64, math.MaxFloat64}); reason != TrialReasonMidpointNonPositive {
		t.Fatalf("中点溢出应 fail-closed, reason=%s", reason)
	}

	lower := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 40000}
	higher := lower
	higher.ID, higher.OriginalPrice = 2, 60000
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{lower, higher})
	req := trialCompleteRequest()

	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusPriced || response.Interval == nil || math.Abs(response.Interval.WidthRatio-0.4) > 1e-6 {
		t.Fatalf("40%% 上限应包含边界: interval=%+v response=%+v", response.Interval, response)
	}
	response, err = CalculateTrialAtWidthLimit(context.Background(), req, snapshot, "development", 0.2)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonTooWide || response.Interval != nil || response.WidthRatio == nil {
		t.Fatalf("超宽应追问且不暴露价格: %+v", response)
	}
}

func TestTrialSnapshotFreezesCatalogCoefficientsAndPolicy(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	input := trialReplayInput(t, []OriginalPrice{price})
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	before, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}

	input.OriginalPrices[0].OriginalPrice = 999999
	input.Coefficients[0].Value = 999
	input.TrialPolicies[0].MaxWidthRatio = 0.01
	after, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if before.Snapshot.Hash != after.Snapshot.Hash || before.Snapshot.PolicyVersion != after.Snapshot.PolicyVersion ||
		before.Interval == nil || after.Interval == nil || *before.Interval != *after.Interval {
		t.Fatalf("源数据变更不得漂移已冻结快照: before=%+v after=%+v", before, after)
	}

	changed := trialReplayInput(t, []OriginalPrice{price})
	changed.Coefficients[0].Value++
	changedSnapshot, err := NewTrialSnapshot(changed)
	if err != nil {
		t.Fatal(err)
	}
	if changedSnapshot.Hash == snapshot.Hash {
		t.Fatal("目录/系数/策略应参与快照哈希")
	}
}

func trialReplayInput(t *testing.T, prices []OriginalPrice) ReplayInput {
	t.Helper()
	snapshot := newTrialSnapshotForTest(t, prices)
	return snapshot.ReplayInput
}

func TestTrialFiltersYearAndPricesSingleExactCandidateWithoutFuzzy(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})

	t.Run("精确年份不相容", func(t *testing.T) {
		req := trialCompleteRequest()
		year := 2019
		req.FactoryYear = &year
		response, err := CalculateTrial(context.Background(), req, snapshot, "development")
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonYearIncompatible {
			t.Fatalf("早于目录最早年份应 needs_info: %+v", response)
		}
	})

	t.Run("年份范围取相容情景", func(t *testing.T) {
		req := trialCompleteRequest()
		req.FactoryYear = nil
		req.FactoryMin, req.FactoryMax = trialPtr(2019), trialPtr(2021)
		response, err := CalculateTrial(context.Background(), req, snapshot, "development")
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != TrialStatusPriced || len(response.Candidates) != 1 {
			t.Fatalf("范围应保留相容单候选: %+v", response)
		}
		if response.Interval == nil || response.Interval.Min <= 0 || response.Interval.Max < response.Interval.Min {
			t.Fatalf("范围应给出有效区间: %+v", response)
		}
	})

	t.Run("完整单候选与正式回放一致且零写入", func(t *testing.T) {
		req := trialCompleteRequest()
		response, err := CalculateTrial(context.Background(), req, snapshot, "development")
		if err != nil {
			t.Fatal(err)
		}
		if response.Status != TrialStatusPriced || response.Interval == nil {
			t.Fatalf("单候选应给价: %+v", response)
		}
		formal, err := Replay(context.Background(), snapshot.ReplayInput)
		if err != nil {
			t.Fatal(err)
		}
		if formal.FormulaVersion != FormulaVersionV2 || response.Interval.Mid != formal.EstimatedValue {
			t.Fatalf("试算应复用 V2 正式路径: trial=%+v formal=%+v", response.Interval, formal)
		}
		if response.Interval.Min != response.Interval.Max || response.Interval.Width != 0 || response.Interval.WidthRatio != 0 {
			t.Fatalf("完整单候选区间应退化: %+v", response.Interval)
		}
		if response.Snapshot.FormulaHash != TrialFormulaHash || len(response.Snapshot.SHA7) != 7 {
			t.Fatalf("缺少公式/快照哈希: %+v", response.Snapshot)
		}
	})
}

func trialPtr[T any](value T) *T { return &value }

func trialCompleteRequest() TrialRequest {
	req := trialMinimumRequest()
	req.Series = trialPtr("K系列")
	req.ConfigType = trialPtr("标准")
	req.MastType = trialPtr("标准门架")
	req.MastHeightMM = trialPtr(3000)
	req.UsageHours = trialPtr(9000)
	req.Condition = trialPtr("B")
	req.Province = trialPtr("安徽省")
	req.City = trialPtr("合肥市")
	req.OriginalPaint = trialPtr(false)
	req.HasLicensePlate = trialPtr(true)
	req.HasRegistrationCertificate = trialPtr(true)
	req.HasMaintenanceRecords = trialPtr(false)
	return req
}

func newTrialSnapshotForTest(t *testing.T, prices []OriginalPrice) *TrialSnapshot {
	t.Helper()
	configs := make([]CoefficientConfig, 0, len(newDefaultConfigReader().values))
	for key, value := range newDefaultConfigReader().values {
		configs = append(configs, CoefficientConfig{Key: key, Value: value})
	}
	conditions := make([]ConditionRating, 0, 5)
	for _, rating := range []string{"A", "B", "C", "D", "E"} {
		conditions = append(conditions, newDefaultMemDict().conditions[rating])
	}
	snapshot, err := NewTrialSnapshot(ReplayInput{
		Request: EvaluationRequest{Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
			ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, FactoryYear: 2021, SaleYear: 2026,
			UsageHours: 9000, Province: "安徽省", City: "合肥市", ConditionRating: "B",
			HasLicensePlate: true, HasRegistrationCertificate: true},
		Brands:             []Brand{{ID: 1, Name: "林德", KBrand: 1, IsActive: true}},
		VehicleTypes:       []VehicleType{{ID: 1, Name: "电动叉车", PowerType: "electric"}},
		ConditionRatings:   conditions,
		RegionCoefficients: []RegionCoefficient{{ID: 1, Province: "安徽省", City: "合肥市", Coefficient: 1}},
		OriginalPrices:     prices,
		Coefficients:       configs,
		TrialPolicies: []TrialPolicy{{Name: "demo", Version: 1, Brand: "林德", VehicleType: "电动叉车", Tonnage: 3,
			MaxWidthRatio: 0.4, UsageHourRatios: []float64{0, 0.85, 1.15, 1.45, 1.7}, ConditionRatings: []string{"B", "C"},
			OriginalPaintValues: []bool{false, true}, HasLicensePlateValues: []bool{false, true},
			HasRegistrationCertificateValues: []bool{false, true}, HasMaintenanceRecordsValues: []bool{false, true}, Enabled: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
