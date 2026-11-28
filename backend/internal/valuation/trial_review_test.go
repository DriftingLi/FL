package valuation

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTrialUnknownBooleanInputsRequireApprovedScenarios(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	input := snapshot.ReplayInput
	input.TrialPolicies[0].OriginalPaintValues = nil
	input.TrialPolicies[0].HasMaintenanceRecordsValues = nil
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Control = TrialControl{Enabled: true, Version: 1}
	req := trialCompleteRequest()
	req.OriginalPaint = nil
	req.HasMaintenanceRecords = nil

	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonNoValidScenario || response.Interval != nil {
		t.Fatalf("未批准的未知布尔项不得静默按 false 给价: %+v", response)
	}
	if !containsTrialField(response.MissingFields, "original_paint") || !containsTrialField(response.MissingFields, "has_maintenance_records") {
		t.Fatalf("应明确追问两个未批准布尔项: %+v", response.MissingFields)
	}
}

func TestTrialApprovedBooleanScenariosWidenIntervalAndAreDisclosed(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	known := trialCompleteRequest()
	knownResponse, err := CalculateTrial(context.Background(), known, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	unknown := known
	unknown.OriginalPaint = nil
	unknown.HasMaintenanceRecords = nil
	response, err := CalculateTrial(context.Background(), unknown, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusPriced || response.Interval == nil || knownResponse.Interval == nil ||
		response.Interval.Max <= knownResponse.Interval.Max || response.Interval.Min != knownResponse.Interval.Min {
		t.Fatalf("批准的布尔情景应枚举有、无并真实扩宽区间: known=%+v unknown=%+v", knownResponse.Interval, response.Interval)
	}
	if !containsTrialString(response.Assumptions, "未知原厂漆按策略批准的有、无情景枚举") ||
		!containsTrialString(response.Assumptions, "未知维保记录按策略批准的有、无情景枚举") {
		t.Fatalf("采用的布尔情景必须进入假设: %+v", response.Assumptions)
	}
}

func containsTrialString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestTrialFollowUpChoosesFieldWithLargestWidthReduction(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	input := snapshot.ReplayInput
	input.TrialPolicies[0].MaxWidthRatio = 0.05
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Control = TrialControl{Enabled: true, Version: 1}
	req := trialCompleteRequest()
	req.Series = nil
	req.UsageHours = nil

	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonTooWide {
		t.Fatalf("夹具应先因工时情景过宽进入追问: %+v", response)
	}
	if len(response.MissingFields) != 1 || response.MissingFields[0] != "usage_hours" {
		t.Fatalf("单候选下补系列不缩窄，应优先询问最能缩窄的工时: %+v", response.MissingFields)
	}
}

func TestTrialFollowUpUsesCandidateDiscriminationAsTieBreaker(t *testing.T) {
	lower := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "保留", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 40000}
	middle := lower
	middle.ID, middle.ConfigType, middle.OriginalPrice = 2, "其它", 50000
	higher := lower
	higher.ID, higher.OriginalPrice = 3, 60000
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{lower, middle, higher})
	input := snapshot.ReplayInput
	input.TrialPolicies[0].MaxWidthRatio = 0.3
	snapshot, err := NewTrialSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Control = TrialControl{Enabled: true, Version: 1}
	req := trialCompleteRequest()
	req.Series = nil
	req.ConfigType = nil
	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonTooWide {
		t.Fatalf("夹具应先过宽: %+v", response)
	}
	if len(response.MissingFields) != 1 || response.MissingFields[0] != "config_type" {
		t.Fatalf("宽度收益相同时应优先选择能区分候选的配置: %+v", response.MissingFields)
	}
}

func containsTrialField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func TestTrialReplaySeparatesOcclusionRateFromWidthLimit(t *testing.T) {
	lower := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K20", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 40000}
	higher := OriginalPrice{ID: 2, Brand: "林德", VehicleType: "电动叉车", Series: "K30", Tonnage: 3,
		ConfigType: "高配", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 60000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{lower, higher})
	complete := snapshot.Request
	complete.Series, complete.ConfigType, complete.MastType, complete.MastHeightMM = lower.Series, lower.ConfigType, lower.MastType, lower.MastHeightMM

	request20 := trialCompleteRequest()
	request20.Series, request20.ConfigType = nil, nil
	request30 := request20
	request30.MastType = nil
	request40 := request30
	request40.MastHeightMM = nil
	report, err := ReplayTrialBatch(context.Background(), snapshot, []TrialReplayCase{
		{Group: "occlusion_20", Request: request20, CompleteRequest: &complete},
		{Group: "occlusion_30", Request: request30, CompleteRequest: &complete},
		{Group: "occlusion_40", Request: request40, CompleteRequest: &complete},
	}, "development")
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Groups["occlusion_20"].Thresholds["20"].TooWide; got != 1 {
		t.Fatalf("20%% 遮挡仍保留两候选，20%% 宽度上限应过宽: %+v", report.Groups["occlusion_20"])
	}
	for _, groupName := range []string{"occlusion_20", "occlusion_30", "occlusion_40"} {
		group := report.Groups[groupName]
		if group.Thresholds["20"].TooWide != 1 || group.Thresholds["30"].TooWide != 1 {
			t.Fatalf("候选宽度为 40%%，20/30%% 上限应过宽: %+v", group)
		}
		if metrics := group.Thresholds["40"]; metrics.Priced != 1 || metrics.Covered != 1 || metrics.CoveredOverPriced != 1 {
			t.Fatalf("40%% 宽度上限应给价并覆盖完整值: %+v", metrics)
		}
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		SnapshotHash   string `json:"snapshot_hash"`
		FormulaHash    string `json:"formula_hash"`
		PolicyHash     string `json:"policy_hash"`
		FormulaVersion string `json:"formula_version"`
		Groups         map[string]struct {
			OcclusionRate float64 `json:"occlusion_rate"`
			Thresholds    map[string]struct {
				WidthLimit float64 `json:"width_limit_ratio"`
			} `json:"thresholds"`
		} `json:"groups"`
		Results []struct {
			MaskedDimensions int     `json:"masked_dimensions"`
			OcclusionRate    float64 `json:"occlusion_rate"`
			Thresholds       map[string]struct {
				Status     string         `json:"status"`
				ReasonCode string         `json:"reason_code"`
				Interval   *TrialInterval `json:"interval"`
				WidthRatio *float64       `json:"width_ratio"`
			} `json:"thresholds"`
		} `json:"results"`
	}
	if err := json.Unmarshal(payload, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.SnapshotHash != snapshot.Hash || evidence.FormulaHash != TrialFormulaHash || evidence.PolicyHash != snapshot.PolicyHash || evidence.FormulaVersion != FormulaVersionV2 {
		t.Fatalf("可复算证据缺少固定快照/策略/公式: %+v", evidence)
	}
	for index, wantRate := range []float64{0.2, 0.3, 0.4} {
		groupName := []string{"occlusion_20", "occlusion_30", "occlusion_40"}[index]
		group := evidence.Groups[groupName]
		if group.OcclusionRate != wantRate || evidence.Results[index].OcclusionRate != wantRate || evidence.Results[index].MaskedDimensions != index+2 {
			t.Fatalf("遮挡率应按 10 个可遮挡维度独立计算: group=%+v result=%+v", group, evidence.Results[index])
		}
		for threshold, wantLimit := range map[string]float64{"20": 0.2, "30": 0.3, "40": 0.4} {
			if got := group.Thresholds[threshold].WidthLimit; got != wantLimit {
				t.Fatalf("阈值键 %s 应明确表示宽度上限 %v: %+v", threshold, wantLimit, group.Thresholds[threshold])
			}
			attempt := evidence.Results[index].Thresholds[threshold]
			if attempt.Status == "" || len(evidence.Results[index].Thresholds) != 3 {
				t.Fatalf("逐条证据应保留三档宽度尝试: %+v", evidence.Results[index])
			}
			if threshold != "40" && index > 0 && (attempt.Status != TrialStatusNeedsInfo || attempt.ReasonCode != TrialReasonTooWide || attempt.Interval != nil) {
				t.Fatalf("低于实际宽度的尝试应留无价证据: %+v", attempt)
			}
		}
	}
	for groupName, group := range report.Groups {
		occluded := 0
		for _, result := range report.Results {
			if result.Group == groupName {
				occluded += result.MaskedDimensions
			}
		}
		if group.OccludedDimensions != occluded || group.OcclusionRate != float64(occluded)/float64(group.N*10) {
			t.Fatalf("分组遮挡率必须可由逐条证据复算: group=%+v", group)
		}
		for threshold, metrics := range group.Thresholds {
			priced, covered, tooWide := 0, 0, 0
			for _, result := range report.Results {
				if result.Group != groupName || !result.CompleteValid {
					continue
				}
				attempt := result.Thresholds[threshold]
				if attempt.Status == TrialStatusPriced && attempt.Interval != nil {
					priced++
					if result.CompleteValue >= attempt.Interval.Min-1e-9 && result.CompleteValue <= attempt.Interval.Max+1e-9 {
						covered++
					}
				} else if attempt.ReasonCode == TrialReasonTooWide {
					tooWide++
				}
			}
			pricedOverValid, coveredOverPriced, tooWideOverValid := 0.0, 0.0, 0.0
			if metrics.Valid > 0 {
				pricedOverValid = float64(priced) / float64(metrics.Valid)
				tooWideOverValid = float64(tooWide) / float64(metrics.Valid)
			}
			if priced > 0 {
				coveredOverPriced = float64(covered) / float64(priced)
			}
			if metrics.Priced != priced || metrics.Covered != covered || metrics.TooWide != tooWide ||
				metrics.PricedOverValid != pricedOverValid || metrics.CoveredOverPriced != coveredOverPriced ||
				metrics.TooWideOverValid != tooWideOverValid {
				t.Fatalf("分组指标必须可由逐档尝试复算: group=%s threshold=%s metrics=%+v", groupName, threshold, metrics)
			}
		}
	}
}
