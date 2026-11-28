package valuation

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type formulaVersionWriter struct {
	params *CreateEvaluationParams
}

type capturingReportGenerator struct {
	detail *EvaluationDetail
}

func (g *capturingReportGenerator) GenerateReport(detail *EvaluationDetail, _ []DimensionScore, _ []string) ([]byte, error) {
	copy := *detail
	g.detail = &copy
	return []byte("pdf"), nil
}

func (w *formulaVersionWriter) CreateEvaluation(_ context.Context, params *CreateEvaluationParams) (int64, error) {
	w.params = params
	return 1, nil
}

func TestEvaluateRecordsCurrentFormulaVersion(t *testing.T) {
	writer := &formulaVersionWriter{}
	svc, err := NewService(newFullMemDictReader(), writer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Evaluate(context.Background(), baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.FormulaVersion != "hours-reciprocal-v2" {
		t.Fatalf("FormulaVersion = %q", result.FormulaVersion)
	}
	if _, err := svc.Persist(context.Background(), result, 0); err != nil {
		t.Fatal(err)
	}
	if writer.params.FormulaVersion != "hours-reciprocal-v2" {
		t.Fatalf("持久化 FormulaVersion = %q", writer.params.FormulaVersion)
	}
}

func TestEvaluateSuggestionsUseCurrentFormulaExponent(t *testing.T) {
	svc := newTestValuationService(t, newFullMemDictReader())
	req := baseRequest()
	req.UsageHours = 10000
	result, err := svc.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Suggestions, "使用强度略高于品牌保值能力，时间衰减略快") {
		t.Fatalf("建议未使用新版指数: %v", result.Suggestions)
	}
}

func TestRebuildDerivedLegacyRecordUsesOldFormula(t *testing.T) {
	detail := EvaluationDetail{
		KTime: 0.5488, KHours: 1.10, KBrand: 1,
		FactoryYear: 2019, SaleYear: 2024, LambdaElectric: 0.12,
	}
	RebuildDerivedFromDetail(&detail)
	if detail.FormulaVersion != FormulaVersionLegacy {
		t.Fatalf("FormulaVersion = %q", detail.FormulaVersion)
	}
	if detail.KTimeAdjusted != 0.5168392801977483 {
		t.Fatalf("KTimeAdjusted = %v, want 0.5168392801977483", detail.KTimeAdjusted)
	}
	if detail.DecayAnchor != 0.8763 {
		t.Fatalf("DecayAnchor = %v, want 0.8763", detail.DecayAnchor)
	}
}

func TestLegacyDetailAndReportRebuildMissingVersionAndAnchor(t *testing.T) {
	store := newMemEvalStore()
	store.records[1] = EvaluationDetail{
		ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
		FactoryYear: 2019, SaleYear: 2024, UsageHours: 9000,
		Province: "安徽省", City: "合肥市", ConditionRating: "A",
		OriginalPrice: 100000, KTime: 0.5488, KHours: 0.95, KBrand: 1,
		KCondition: 1, KMarket: 1, EstimatedValue: 50000,
		ConfidenceLow: 45000, ConfidenceHigh: 55000,
		Suggestions: []string{"锁定建议"}, LambdaElectric: 0.12, LambdaCombustion: 0.1,
	}
	svc, err := NewService(newSeedMemDict(), store)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	handler := NewEvaluationHandler(svc, store, zap.NewNop())
	router.GET("/api/valuation/evaluations/:id", handler.Get)

	response := performRequestWithAuth(router, http.MethodGet, "/api/valuation/evaluations/1", nil, authHeader(t, 1))
	if response.Code != http.StatusOK {
		t.Fatalf("详情状态码 = %d: %s", response.Code, response.Body.String())
	}
	detail := mustDecodeData(t, response)
	if detail["formula_version"] != FormulaVersionLegacy || detail["decay_anchor"] != 0.8923 {
		t.Fatalf("详情未按旧版重建: %v", detail)
	}

	generator := &capturingReportGenerator{}
	reportHandler := NewReportHandler(store, generator, zap.NewNop(), newMemStorage(), svc.Resolver())
	reportRouter := gin.New()
	reportRouter.POST("/api/valuation/evaluations/:id/report", reportHandler.Generate)
	response = performRequest(reportRouter, http.MethodPost, "/api/valuation/evaluations/1/report", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("报告状态码 = %d: %s", response.Code, response.Body.String())
	}
	if generator.detail == nil || generator.detail.FormulaVersion != FormulaVersionLegacy || generator.detail.DecayAnchor != 0.8923 {
		t.Fatalf("报告未按旧版重建: %+v", generator.detail)
	}
}

func TestRepeatedEvaluationCreatesIndependentRecords(t *testing.T) {
	router, _, store := newTestValuationEngine(t)
	ids := make([]int64, 0, 2)
	for i := 0; i < 2; i++ {
		response := performRequest(router, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
		if response.Code != http.StatusOK {
			t.Fatalf("提交失败: %d %s", response.Code, response.Body.String())
		}
		data := mustDecodeData(t, response)
		ids = append(ids, int64(data["id"].(float64)))
		if data["formula_version"] != FormulaVersionV2 {
			t.Fatalf("formula_version = %v", data["formula_version"])
		}
	}
	if ids[0] == ids[1] || len(store.records) != 2 {
		t.Fatalf("重复评估应生成独立记录: ids=%v records=%d", ids, len(store.records))
	}
}

func testReplayInput(dict *fullMemDict) ReplayInput {
	coefficients := make([]CoefficientConfig, 0, len(dict.coefficients))
	for key, value := range dict.coefficients {
		coefficients = append(coefficients, CoefficientConfig{Key: key, Value: value})
	}
	return ReplayInput{
		Request:            *baseRequest(),
		Brands:             []Brand{{Name: "林德", KBrand: 1}},
		VehicleTypes:       []VehicleType{{Name: "电动叉车", PowerType: "electric"}},
		ConditionRatings:   []ConditionRating{{Rating: "A", BaseCoefficient: 1}},
		RegionCoefficients: []RegionCoefficient{{Province: "安徽省", City: "合肥市", Coefficient: 1}},
		OriginalPrices:     dict.originalPrices,
		Coefficients:       coefficients,
	}
}

func TestReplayMatchesFormalEvaluationWithoutWriting(t *testing.T) {
	dict := newFullMemDictReader()
	writer := &formulaVersionWriter{}
	svc, err := NewService(dict, writer)
	if err != nil {
		t.Fatal(err)
	}
	input := testReplayInput(dict)
	formal, err := svc.Evaluate(context.Background(), baseRequest())
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := svc.Replay(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, formal) {
		t.Fatalf("离线回放与正式评估不一致:\nreplay=%+v\nformal=%+v", replayed, formal)
	}
	if writer.params != nil {
		t.Fatal("离线回放不应创建正式记录")
	}
}

func TestReplayFuzzyOriginalPriceUsesHighestMatch(t *testing.T) {
	input := testReplayInput(newFullMemDictReader())
	input.Request.ConfigType = "缺失配置"
	lower := input.OriginalPrices[0]
	lower.OriginalPrice = 80000
	higher := lower
	higher.OriginalPrice = 120000
	input.OriginalPrices = []OriginalPrice{lower, higher}

	result, err := Replay(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.OriginalPrice != 120000 {
		t.Fatalf("模糊匹配应取最高原价, got %v", result.OriginalPrice)
	}
}

func TestEvaluateHoursMonotonicAcrossBandsAndThresholds(t *testing.T) {
	for _, vehicleType := range []string{"电动叉车", "内燃叉车"} {
		t.Run(vehicleType, func(t *testing.T) {
			dict := newFullMemDictReader()
			price := dict.originalPrices[0]
			price.VehicleType = vehicleType
			dict.originalPrices = append(dict.originalPrices, price)
			svc := newTestValuationService(t, dict)
			req := baseRequest()
			req.VehicleType = vehicleType
			req.FactoryYear, req.SaleYear = 2019, 2024
			req.HasLicensePlate, req.HasRegistrationCertificate = true, true

			hours := []int{6124, 6125, 8749, 8750, 11374, 11375, 13999, 14000}
			previous := 0.0
			previousFuture := 0.0
			for i, usageHours := range hours {
				req.UsageHours = usageHours
				result, err := svc.Evaluate(context.Background(), req)
				if err != nil {
					t.Fatalf("评估 %d 小时失败: %v", usageHours, err)
				}
				if i > 0 && result.EstimatedValue > previous {
					t.Fatalf("工时从 %d 增至 %d 后残值上升: %.2f -> %.2f", hours[i-1], usageHours, previous, result.EstimatedValue)
				}
				future := result.EstimatedValue * result.DecayAnchor
				if i > 0 && future > previousFuture {
					t.Fatalf("工时从 %d 增至 %d 后未来价值上升: %.2f -> %.2f", hours[i-1], usageHours, previousFuture, future)
				}
				previous = result.EstimatedValue
				previousFuture = future
			}
		})
	}
}

func TestEvaluateZeroAgeCurveAnchorUsesPowerTypeAndCurrentFormula(t *testing.T) {
	cases := []struct {
		vehicleType string
		wantAnchor  float64
	}{
		{"电动叉车", 0.8966},
		{"内燃叉车", 0.9131},
	}
	for _, tc := range cases {
		t.Run(tc.vehicleType, func(t *testing.T) {
			dict := newFullMemDictReader()
			price := dict.originalPrices[0]
			price.VehicleType = tc.vehicleType
			dict.originalPrices = append(dict.originalPrices, price)
			svc := newTestValuationService(t, dict)
			req := baseRequest()
			req.VehicleType = tc.vehicleType
			req.FactoryYear, req.SaleYear = 2024, 2024
			req.UsageHours = 1000
			result, err := svc.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if result.KTimeAdjusted != 1 {
				t.Fatalf("零车龄 KTimeAdjusted = %v", result.KTimeAdjusted)
			}
			if result.DecayAnchor != tc.wantAnchor {
				t.Fatalf("DecayAnchor = %v, want %v", result.DecayAnchor, tc.wantAnchor)
			}
		})
	}
}

func TestRebuildDerivedPreservesLegacyLockedFields(t *testing.T) {
	detail := EvaluationDetail{
		FormulaVersion: FormulaVersionLegacy, KTime: 0.5488, KHours: 1.1, KBrand: 1,
		EstimatedValue: 50000, ConfidenceLow: 45000, ConfidenceHigh: 55000,
		Suggestions: []string{"锁定建议"}, DecayAnchor: 0.91,
	}
	RebuildDerivedFromDetail(&detail)
	if detail.EstimatedValue != 50000 || detail.ConfidenceLow != 45000 || detail.ConfidenceHigh != 55000 ||
		detail.DecayAnchor != 0.91 || len(detail.Suggestions) != 1 || detail.Suggestions[0] != "锁定建议" {
		t.Fatalf("历史锁定字段被改写: %+v", detail)
	}
}

func TestRebuildDerivedNewRecordMissingAnchorUsesCurrentFormula(t *testing.T) {
	detail := EvaluationDetail{
		FormulaVersion: FormulaVersionV2, KTime: 1, KHours: 1.1, KBrand: 1,
		FactoryYear: 2024, SaleYear: 2024, LambdaElectric: 0.12,
	}
	RebuildDerivedFromDetail(&detail)
	if detail.DecayAnchor != 0.8966 {
		t.Fatalf("DecayAnchor = %v, want 0.8966", detail.DecayAnchor)
	}
}
