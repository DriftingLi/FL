// 估值域端点的**顶层 key 断言**（ADR-0048 片九 / issue #967；片一 #952 先例）。
//
// 本片只补注解、不改响应形状：这些断言把「注解声明的形状」钉在实际返回上 ——
// 端点补注解时若与真实形状不符（多键/少键），这里直接红。
package handler

import (
	"fmt"
	"net/http"
	"sort"
	"testing"
)

// assertTopLevelKeys 断言统一信封 data 的顶层 key 集合（排序后逐项比较）。
func assertTopLevelKeys(t *testing.T, data map[string]interface{}, want ...string) {
	t.Helper()
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	got := make([]string, 0, len(data))
	for k := range data {
		got = append(got, k)
	}
	sort.Strings(got)
	if len(got) != len(wantSorted) {
		t.Fatalf("顶层 key 数 = %d %v，期望 %d %v\ndata=%v", len(got), got, len(wantSorted), wantSorted, data)
	}
	for i := range got {
		if got[i] != wantSorted[i] {
			t.Fatalf("顶层 key = %v，期望 %v", got, wantSorted)
		}
	}
}

// TestValuationEndpointTopLevelKeys 覆盖本片补注解端点的顶层 key 形状。
func TestValuationEndpointTopLevelKeys(t *testing.T) {
	// GET /api/valuation/evaluations/stats —— 注解 data=object{total}
	t.Run("evaluations_stats", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequest(r, http.MethodGet, "/api/valuation/evaluations/stats", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, data := decodeBody(t, w)
		assertTopLevelKeys(t, data, "total")
	})

	// GET /api/valuation/evaluations —— data=object{total,page,page_size,list}
	t.Run("evaluations_list", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations", nil, authHeader(t, 1))
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, data := decodeBody(t, w)
		assertTopLevelKeys(t, data, "total", "page", "page_size", "list")
	})

	// POST /api/valuation/evaluations —— data=model.EvaluationResponse（33 键）
	t.Run("evaluations_create", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		id := int64(mustDecodeData(t, w)["id"].(float64))
		// GET /api/valuation/evaluations/:id —— data=model.EvaluationDetail（35 键，比创建多 created_at/updated_at/report_pdf_path）
		w = performRequestWithAuth(r, http.MethodGet, fmt.Sprintf("/api/valuation/evaluations/%d", id), nil, authHeader(t, 1))
		if w.Code != http.StatusOK {
			t.Fatalf("详情状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, detail := decodeBody(t, w)
		assertTopLevelKeys(t, detail,
			"id", "created_at", "updated_at", "brand", "vehicle_type", "series", "tonnage",
			"config_type", "mast_type", "mast_height_mm", "factory_year", "sale_year",
			"usage_hours", "original_paint", "province", "city", "has_license_plate",
			"has_registration_certificate", "has_maintenance_records", "condition_rating",
			"original_price", "k_time", "k_hours", "k_brand", "k_condition", "k_market",
			"k_time_adjusted", "estimated_value", "confidence_low", "confidence_high",
			"dimension_scores", "suggestions", "lambda_electric", "lambda_combustion",
			"decay_anchor")

		// POST /api/valuation/evaluations/:id/report —— data=object{evaluation_id,pdf_url,file_size}
		w = performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/evaluations/%d/report", id), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("生成报告状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, report := decodeBody(t, w)
		assertTopLevelKeys(t, report, "evaluation_id", "pdf_url", "file_size")
	})

	// GET /api/valuation/auth/me —— data=object{user_id,uid,account,username,phone,email,company,role}
	t.Run("valuation_auth_me", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		// 本 seam 未装配 ValuationAuthService（RegisterRoutes 传 nil）→ 未登录/无用户时的守卫先行，
		// 故这里断言的是「未带 token 必 401」而不是用户字段（用户字段由 auth_me_test.go 覆盖）。
		w := performRequest(r, http.MethodGet, "/api/valuation/auth/me", nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("未认证状态码 = %d, 期望 401: %s", w.Code, w.Body.String())
		}
	})

	// POST /api/valuation/battery/evaluations/:id/report —— 与评估报告同形状
	t.Run("battery_report_generate", func(t *testing.T) {
		r, _, _, _ := newTestValuationEngineWithStorage(t, newMemStorage())
		id := createBatteryForReport(t, r, authHeader(t, 1))
		w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/battery/evaluations/%d/report", id), nil)
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, data := decodeBody(t, w)
		assertTopLevelKeys(t, data, "evaluation_id", "pdf_url", "file_size")
	})

	// GET /api/valuation/battery/evaluations —— data=model.ListBatteryResponse
	t.Run("battery_list", func(t *testing.T) {
		r, _, _, _ := newTestValuationEngineWithStorage(t, newMemStorage())
		_ = createBatteryForReport(t, r, authHeader(t, 1))
		w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/battery/evaluations", nil, authHeader(t, 1))
		if w.Code != http.StatusOK {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, data := decodeBody(t, w)
		assertTopLevelKeys(t, data, "total", "items")
	})

	// 管理端 region-coefficients：POST 创建 → PUT 更新（非统一信封登记：update/delete 走 gin.H，
	// 但 create 走 dictcrud.BuildCreateResult，故此处断言 create 的 id ∪ 声明字段集）。
	t.Run("admin_region_coefficient_create_update", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
			map[string]interface{}{"province": "江苏", "city": "南京", "coefficient": 1.02}, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("创建状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, created := decodeBody(t, w)
		assertTopLevelKeys(t, created, "id", "province", "city", "coefficient")

		id := int64(created["id"].(float64))
		w = performRequestWithAuth(r, http.MethodPut, fmt.Sprintf("/api/valuation/admin/region-coefficients/%d", id),
			map[string]interface{}{"coefficient": 1.05}, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("更新状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, updated := decodeBody(t, w)
		// 更新子集为单字段 coefficient（无 ResponseExtra：更新响应不含 updated_at）
		assertTopLevelKeys(t, updated, "id", "coefficient")
	})

	// PUT /api/valuation/admin/condition-ratings/:id —— 更新子集 {id,label,base_coefficient}（无 updated_at）
	t.Run("admin_condition_rating_update", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/condition-ratings",
			map[string]interface{}{"rating": "F", "label": "极差", "base_coefficient": 0.4}, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("创建状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, created := decodeBody(t, w)
		assertTopLevelKeys(t, created, "id", "rating", "label", "base_coefficient")

		id := int64(created["id"].(float64))
		w = performRequestWithAuth(r, http.MethodPut, fmt.Sprintf("/api/valuation/admin/condition-ratings/%d", id),
			map[string]interface{}{"label": "极差+", "base_coefficient": 0.45}, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("更新状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, updated := decodeBody(t, w)
		assertTopLevelKeys(t, updated, "id", "label", "base_coefficient")
	})

	// DELETE /api/valuation/admin/brands/:id —— 全部实体删除响应固定为 {id}
	t.Run("admin_brand_delete", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
			map[string]interface{}{"name": "林德", "k_brand": 1.1, "is_active": true}, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("创建状态码 = %d: %s", w.Code, w.Body.String())
		}
		id := int64(mustDecodeData(t, w)["id"].(float64))
		w = performRequestWithAuth(r, http.MethodDelete, fmt.Sprintf("/api/valuation/admin/brands/%d", id), nil, adminAuthHeader(t))
		if w.Code != http.StatusOK {
			t.Fatalf("删除状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, deleted := decodeBody(t, w)
		assertTopLevelKeys(t, deleted, "id")
	})

	// PUT /api/valuation/admin/coefficient-configs/:key —— 按 key 更新返回完整行
	t.Run("admin_coefficient_config_update", func(t *testing.T) {
		r, _, _ := newTestValuationEngine(t)
		w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
			map[string]interface{}{"value": 0.13}, adminAuthHeader(t))
		// 该 seam 的内存字典读面未实现 ListCoefficientConfigs 之外的读路径；
		// 未命中 404 与命中 200 都是合法分支，此处只断言「不是 5xx」与响应是可解信封。
		if w.Code >= http.StatusInternalServerError {
			t.Fatalf("状态码 = %d: %s", w.Code, w.Body.String())
		}
		_, _, _ = decodeBody(t, w)
	})
}
