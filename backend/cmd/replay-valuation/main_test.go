package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"forklift-training/internal/valuation"
)

func TestRunBatchJSONLUsesOneFrozenSnapshot(t *testing.T) {
	year := 2021
	series, configType, mastType := "K系列", "标准", "标准门架"
	height, hours := 3000, 9000
	condition, province, city := "B", "安徽省", "合肥市"
	complete := valuation.EvaluationRequest{
		Brand: "林德", VehicleType: "电动叉车", Series: series, Tonnage: 3, ConfigType: configType,
		MastType: mastType, MastHeightMM: height, FactoryYear: year, SaleYear: 2026, UsageHours: hours,
		Province: province, City: city, ConditionRating: condition, HasLicensePlate: true, HasRegistrationCertificate: true,
	}
	input := valuation.ReplayInput{
		Request:            complete,
		Brands:             []valuation.Brand{{ID: 1, Name: "林德", KBrand: 1, IsActive: true}},
		VehicleTypes:       []valuation.VehicleType{{ID: 1, Name: "电动叉车", PowerType: "electric"}},
		ConditionRatings:   []valuation.ConditionRating{{ID: 1, Rating: "B", BaseCoefficient: 0.9}},
		RegionCoefficients: []valuation.RegionCoefficient{{ID: 1, Province: province, City: city, Coefficient: 1}},
		OriginalPrices: []valuation.OriginalPrice{{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: series,
			Tonnage: 3, ConfigType: configType, MastType: mastType, MastHeightMM: height,
			EarliestFactoryYear: 2020, OriginalPrice: 100000}},
		Coefficients: []valuation.CoefficientConfig{
			{Key: valuation.KeyLambdaElectric, Value: 0.12},
			{Key: valuation.KeyLambdaCombustion, Value: 0.1},
			{Key: valuation.KeyAnnualUsageHours, Value: 1750},
			{Key: valuation.KeyKHoursRatioLow, Value: 0.7},
			{Key: valuation.KeyKHoursRatioMid, Value: 1.0},
			{Key: valuation.KeyKHoursRatioHigh, Value: 1.3},
			{Key: valuation.KeyKHoursRatioMax, Value: 1.6},
			{Key: valuation.KeyKcPaintBonus, Value: 0.02},
			{Key: valuation.KeyKcMaintenanceBonus, Value: 0.02},
			{Key: valuation.KeyKcNoLicensePenaltyPct, Value: 0.1},
			{Key: valuation.KeyKcNoRegistrationPenaltyPct, Value: 0.1},
		},
		TrialPolicies: []valuation.TrialPolicy{{Name: "demo", Version: 1, Brand: "林德", VehicleType: "电动叉车", Tonnage: 3,
			MaxWidthRatio: 0.4, UsageHourRatios: []float64{0.85}, ConditionRatings: []string{"B"}, Enabled: true}},
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "snapshot.json")
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, inputJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	request := valuation.TrialRequest{
		Brand: "林德", VehicleType: "电动叉车", Tonnage: 3, FactoryYear: &year, SaleYear: 2026,
		Series: &series, ConfigType: &configType, MastType: &mastType, MastHeightMM: &height,
		UsageHours: &hours, Condition: &condition, Province: &province, City: &city,
		HasLicensePlate: boolPointer(true), HasRegistrationCertificate: boolPointer(true), Policy: "demo",
	}
	caseJSON, err := json.Marshal(valuation.TrialReplayCase{Group: "smoke", Request: request, CompleteRequest: &complete})
	if err != nil {
		t.Fatal(err)
	}
	trialsPath := filepath.Join(dir, "trials.jsonl")
	if err := os.WriteFile(trialsPath, append(caseJSON, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runWithOptions(replayOptions{Input: inputPath, Trials: trialsPath, AppEnv: "development"}, &output); err != nil {
		t.Fatal(err)
	}
	var report valuation.TrialReplayReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("输出不是批量回放报告: %v\n%s", err, output.String())
	}
	metrics := report.Groups["smoke"].Thresholds["40"]
	if metrics.N != 1 || metrics.Valid != 1 || metrics.Excluded != 0 || metrics.Priced != 1 || metrics.Covered != 1 {
		t.Fatalf("JSONL 批量指标错误: %+v", metrics)
	}
	if len(report.SnapshotSHA7) != 7 || report.FormulaHash != valuation.TrialFormulaHash || report.PolicyHash == "" {
		t.Fatalf("缺少快照/策略/公式哈希: %+v", report)
	}
	if len(report.Results) != 0 {
		t.Fatalf("默认不应输出逐条明细: %+v", report.Results)
	}
}

func boolPointer(value bool) *bool { return &value }
