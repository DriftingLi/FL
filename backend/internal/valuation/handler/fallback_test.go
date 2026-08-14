package handler

import (
	"context"
	"net/http"
	"testing"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// TestEvaluationDetailFillsEmptySuggestions 锁定 ADR-0012 §6：
// 车辆详情对旧记录空建议做 fallback（与报告 Prepare 同源），非空建议不被覆盖（ADR-0004）。
func TestEvaluationDetailFillsEmptySuggestions(t *testing.T) {
	r, _, evalStore := newTestValuationEngine(t)
	createEvalWithSuggestions(t, evalStore, nil)

	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations/1", "", authHeader(t, 1))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	sugs, _ := data["suggestions"].([]interface{})
	if len(sugs) == 0 {
		t.Fatalf("旧记录空建议应被 fallback 填充: %v", data["suggestions"])
	}
}

// TestEvaluationDetailKeepsLockedSuggestions 已有建议的记录保持评估时点锁定值。
func TestEvaluationDetailKeepsLockedSuggestions(t *testing.T) {
	r, _, evalStore := newTestValuationEngine(t)
	createEvalWithSuggestions(t, evalStore, []string{"锁定建议"})

	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations/1", "", authHeader(t, 1))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	sugs, _ := data["suggestions"].([]interface{})
	if len(sugs) != 1 || sugs[0] != "锁定建议" {
		t.Fatalf("锁定建议不应被覆盖: %v", data["suggestions"])
	}
}

// TestBatteryDetailFillsEmptySuggestions 电池详情 fallback：记录置信度反推 health，
// 高置信度记录不触发「特征波动」稳定性提示（与预测口径一致）。
func TestBatteryDetailFillsEmptySuggestions(t *testing.T) {
	r, _, _, batteryStore := newTestValuationEngineWithStorage(t, nil)
	if _, err := batteryStore.CreateEvaluation(context.TODO(), &model.BatteryEvaluation{
		ID: 1, BatteryType: model.BatteryTypeLFP, SohPercent: 90,
		RulCycles: 500, ConfidenceLow: 400, ConfidenceHigh: 600,
		Confidence: 0.92, Suggestions: nil,
	}, nil, 1); err != nil {
		t.Fatalf("创建电池记录失败: %v", err)
	}

	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/battery/evaluations/1", "", authHeader(t, 1))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	sugs, _ := data["suggestions"].([]interface{})
	if len(sugs) == 0 {
		t.Fatalf("旧记录空建议应被 fallback 填充: %v", data["suggestions"])
	}
	for _, s := range sugs {
		if str, ok := s.(string); ok && str == "特征波动较大，建议结合历史多循环数据复核预测结果。" {
			t.Fatalf("高置信度记录不应出现稳定性提示: %v", sugs)
		}
	}
}

// createEvalWithSuggestions 向 memEvalStore 写入一条建议可控的评估记录。
func createEvalWithSuggestions(t *testing.T, store *memEvalStore, suggestions []string) {
	t.Helper()
	_, err := store.CreateEvaluation(context.TODO(), &repository.CreateEvaluationParams{
		Brand: "合力", VehicleType: "内燃叉车", FactoryYear: 2020, SaleYear: 2026,
		OriginalPrice: 100000, EstimatedValue: 40000, Suggestions: suggestions,
	})
	if err != nil {
		t.Fatalf("创建评估记录失败: %v", err)
	}
}
