// Package handler 实现 HTTP 处理器
// 本文件：评估历史列表车型过滤的 handler 透传断言（#400）——vehicle_type 与 brand
// 组合透传仓储，无参数时透传空串（行为不变）。SQL 级过滤语义由仓储集成测试覆盖
// （evaluations_integration_test.go，CI 提供 Postgres）。
package handler

import (
	"net/http"
	"net/url"
	"testing"

	"forklift-training/internal/storage"
)

// lastFilterOf 读取 fake 记录的最近一次过滤参数。
func lastFilterOf(t *testing.T, store *memEvalStore) evalListFilter {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.lastFilter
}

// TestEvaluationList_VehicleTypeFilterForwarded 车型筛选参数经 handler 透传仓储：
// 与 brand 组合、单独使用、无参数三种形状（无参数时透传空串，行为不变）。
func TestEvaluationList_VehicleTypeFilterForwarded(t *testing.T) {
	r, _, evalStore, _ := newTestValuationEngineWithStorage(t, storage.NewLocalStorage(t.TempDir()))
	auth := authHeader(t, 7)

	// 1) vehicle_type + brand 组合
	w := performRequestWithAuth(r, http.MethodGet,
		"/api/valuation/evaluations?page=1&page_size=10&vehicle_type="+url.QueryEscape("电动平衡重")+"&brand="+url.QueryEscape("杭叉"),
		nil, auth)
	if w.Code != http.StatusOK {
		t.Fatalf("组合筛选状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	got := lastFilterOf(t, evalStore)
	if got.VehicleType != "电动平衡重" || got.Brand != "杭叉" {
		t.Errorf("组合筛选透传错误: vehicle_type=%q brand=%q", got.VehicleType, got.Brand)
	}
	if got.UserID != 7 {
		t.Errorf("列表应按当前登录用户过滤: got %d", got.UserID)
	}

	// 2) 仅 vehicle_type（brand 透传空串）
	w2 := performRequestWithAuth(r, http.MethodGet,
		"/api/valuation/evaluations?vehicle_type="+url.QueryEscape("内燃叉车"), nil, auth)
	if w2.Code != http.StatusOK {
		t.Fatalf("仅车型筛选状态码 = %d\nbody=%s", w2.Code, w2.Body.String())
	}
	got2 := lastFilterOf(t, evalStore)
	if got2.VehicleType != "内燃叉车" || got2.Brand != "" {
		t.Errorf("仅车型筛选透传错误: vehicle_type=%q brand=%q", got2.VehicleType, got2.Brand)
	}

	// 3) 无参数：两者透传空串（无该参数时行为不变）
	w3 := performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations", nil, auth)
	if w3.Code != http.StatusOK {
		t.Fatalf("无参数状态码 = %d\nbody=%s", w3.Code, w3.Body.String())
	}
	got3 := lastFilterOf(t, evalStore)
	if got3.VehicleType != "" || got3.Brand != "" {
		t.Errorf("无参数时应透传空串（行为不变）: vehicle_type=%q brand=%q", got3.VehicleType, got3.Brand)
	}
}
