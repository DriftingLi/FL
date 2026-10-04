// 估值域 DTO 集合字段的 nonnil 举证（ADR-0065 决策 5 判据 5）。
//
// #1514 波 7 把 internal/valuation/repository 并回域包后，估值域的 DTO **整域**首次进表态锁射程：
// 8 个集合字段按生产者的实测形状改判 nonnil（handler 的 nil→[] 守卫、service 与 repository 的
// make），举证因此落在**组装发生的那一层** —— 走真实端点取信封 data，而不是对着零值 DTO 断言
// （零值切片必然 nil，那不构成出口例）。
package handler

import (
	"fmt"
	"net/http"
	"testing"

	"forklift-training/internal/testutil"
	"forklift-training/internal/valuation"
)

// nonnilOutletsValuation 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsValuation = map[string]func(t *testing.T) any{
	"valuation.EvaluationDetail.dimension_scores":    outletEvalDetailData,
	"valuation.EvaluationDetail.suggestions":         outletEvalDetailData,
	"valuation.EvaluationResponse.dimension_scores":  outletEvalResponseData,
	"valuation.EvaluationResponse.suggestions":       outletEvalResponseData,
	"valuation.BatteryEvaluation.feature_importance": outletBatteryDetailData,
	"valuation.BatteryEvaluation.cycle_features":     outletBatteryDetailData,
	"valuation.BatteryEvaluation.suggestions":        outletBatteryDetailData,
	"valuation.CycleFeature.feature_vector":          outletBatteryFirstCycleFeature,
	"valuation.CreateBatteryResponse.suggestions":    outletBatteryCreateData,
	"valuation.ListBatteryResponse.items":            outletBatteryListData,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsValuation})
}

// outletEvalResponseData POST /api/valuation/evaluations 的 data：dimension_scores / suggestions
// 在 buildEvaluationResponse 里有 nil→[] 守卫。
func outletEvalResponseData(t *testing.T) any {
	t.Helper()
	r, _, _ := newTestValuationEngine(t)
	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("评估提交状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}

// outletBatteryCreateData POST /api/valuation/battery/evaluations 的 data：suggestions 由
// service.BuildBatterySuggestions 的 make 切片经落库回读而来。
func outletBatteryCreateData(t *testing.T) any {
	t.Helper()
	r, _, _, _ := newTestValuationEngineWithStorage(t, newMemStorage())
	w := performRequest(r, http.MethodPost, "/api/valuation/battery/evaluations", batteryCreateRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("电池评估提交状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}

// outletBatteryListData GET /api/valuation/battery/evaluations 的 data：先造一条记录让 items
// 非空（repository 发的是 make([]T, 0, n)，空表也是 []，此处取有数据的那一档证明键在场且非 null）。
func outletBatteryListData(t *testing.T) any {
	t.Helper()
	r, _, _, _ := newTestValuationEngineWithStorage(t, newMemStorage())
	auth := authHeader(t, 1)
	createBatteryForReport(t, r, auth)
	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/battery/evaluations", nil, auth)
	if w.Code != http.StatusOK {
		t.Fatalf("电池列表状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}

// outletBatteryDetailData GET /api/valuation/battery/evaluations/:id 的 data：详情才填
// feature_importance / cycle_features / suggestions（三者带 omitempty，故必须有数据）。
func outletBatteryDetailData(t *testing.T) any {
	t.Helper()
	r, _, _, _ := newTestValuationEngineWithStorage(t, newMemStorage())
	auth := authHeader(t, 1)
	id := createBatteryForReport(t, r, auth)
	w := performRequestWithAuth(r, http.MethodGet, fmt.Sprintf("/api/valuation/battery/evaluations/%d", id), nil, auth)
	if w.Code != http.StatusOK {
		t.Fatalf("电池详情状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}

// outletBatteryFirstCycleFeature 详情里 cycle_features[0]（CycleFeature 自身的举证入口，
// 它的 feature_vector 是定长数组，构造上恒非 null）。
func outletBatteryFirstCycleFeature(t *testing.T) any {
	t.Helper()
	data, ok := outletBatteryDetailData(t).(map[string]any)
	if !ok {
		t.Fatalf("详情 data 不是对象")
	}
	items, _ := data["cycle_features"].([]any)
	if len(items) == 0 {
		t.Fatalf("详情没有 cycle_features（详情端点的 cycle_features 由仓储持久化，测试替身已同步）：%v", data)
	}
	return items[0]
}

// seedEvalDetail 播一条最小评估记录（K 系数为 0）并返回其 ID：详情端点对 NULL suggestions 会走
// service.EnsureSuggestions 回填（读路径上的幂等补齐），维度评分则由 RebuildDerivedFromDetail
// 的切片字面量重建 ⇒ 两格在真实出口上都非 nil。
func seedEvalDetail(t *testing.T, store *memEvalStore, id int64) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	store.records[id] = valuation.EvaluationDetail{ID: id, Brand: "Toyota", VehicleType: "FBT", Series: "S1"}
}

// outletEvalDetailData GET /api/valuation/evaluations/:id 的 data：维度评分由
// service.RebuildDerivedFromDetail 走 BuildDimensionScores 的切片字面量重建，恒非 nil。
func outletEvalDetailData(t *testing.T) any {
	t.Helper()
	r, _, evalStore := newTestValuationEngine(t)
	seedEvalDetail(t, evalStore, 7)
	w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations/7", nil, authHeader(t, 1))
	if w.Code != http.StatusOK {
		t.Fatalf("评估详情状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}
