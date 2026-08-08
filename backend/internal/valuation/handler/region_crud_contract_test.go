// 区域系数（tracer）管理员 CRUD HTTP 契约测试。
// 迁移前后同一套断言必须原样通过：锁定路径、JSON 字段名、错误码（pkg/response 信封）。
// 测试在迁移前先锁定当前行为（characterization），迁移后不改断言直接跑。
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"forklift-training/internal/valuation/repository"
)

// regionRow 便捷断言：data 中按名取值。
func regionVal(t *testing.T, data map[string]interface{}, key string) interface{} {
	t.Helper()
	v, ok := data[key]
	if !ok {
		t.Fatalf("响应缺少字段 %q: %v", key, data)
	}
	return v
}

func TestRegionCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "江苏", "city": "苏州", "coefficient": 1.02}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if id := regionVal(t, data, "id"); id != float64(1) {
		t.Errorf("id = %v, 期望 1", id)
	}
	if regionVal(t, data, "province") != "江苏" || regionVal(t, data, "city") != "苏州" {
		t.Errorf("province/city 回显错误: %v", data)
	}
	if regionVal(t, data, "coefficient") != 1.02 {
		t.Errorf("coefficient = %v, 期望 1.02", regionVal(t, data, "coefficient"))
	}
	if len(dict.regions) != 1 || dict.regions[0].ID != 1 || dict.regions[0].Coefficient != 1.02 {
		t.Errorf("存储未写入: %+v", dict.regions)
	}
}

func TestRegionCrud_Create_MissingCoefficientAllowed(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 现状：create 不校验 coefficient，缺失时落 0（锁定该行为）
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "安徽", "city": "合肥"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	if regionVal(t, mustDecodeData(t, w), "coefficient") != 0.0 {
		t.Errorf("缺失 coefficient 应落 0")
	}
}

func TestRegionCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 city → 400 + 自定义必填消息
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "江苏"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "province 与 city 必填" {
		t.Errorf("消息 = %q, 期望 %q", msg, "province 与 city 必填")
	}

	// 空字符串同样拒绝
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "", "city": "", "coefficient": 1.0}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空字符串状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestRegionCrud_Create_MalformedJSON(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		"{\"province\":", adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, _, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
}

func TestRegionCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.regions = []repository.RegionCoefficient{{ID: 1, Province: "江苏", City: "苏州", Coefficient: 1.02}}

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/region-coefficients/1",
		map[string]interface{}{"coefficient": 1.05}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if len(data) != 2 {
		t.Fatalf("update 响应应只含 id+coefficient, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "id") != float64(1) || regionVal(t, data, "coefficient") != 1.05 {
		t.Errorf("update 响应错误: %v", data)
	}
	if dict.regions[0].Coefficient != 1.05 {
		t.Errorf("存储未更新: %+v", dict.regions)
	}
}

func TestRegionCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/region-coefficients/999",
		map[string]interface{}{"coefficient": 1.05}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "区域系数不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "区域系数不存在")
	}
}

func TestRegionCrud_Update_MissingCoefficient(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// coefficient 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/region-coefficients/1",
		map[string]interface{}{}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'Coefficient' Error:Field validation for 'Coefficient' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestRegionCrud_Update_BadID(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/region-coefficients/abc",
		map[string]interface{}{"coefficient": 1.05}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	_, msg, _ := decodeBody(t, w)
	if msg != "id 必须为整数" {
		t.Errorf("消息 = %q, 期望 %q", msg, "id 必须为整数")
	}
}

func TestRegionCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.regions = []repository.RegionCoefficient{
		{ID: 1, Province: "江苏", City: "苏州", Coefficient: 1.02},
		{ID: 2, Province: "安徽", City: "合肥", Coefficient: 1.00},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/region-coefficients/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	if len(dict.regions) != 1 || dict.regions[0].ID != 2 {
		t.Errorf("存储未删除: %+v", dict.regions)
	}
}

func TestRegionCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/region-coefficients/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "区域系数不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "区域系数不存在")
	}
}

func TestRegionCrud_Delete_BadID(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/region-coefficients/xyz",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	_, msg, _ := decodeBody(t, w)
	if msg != "id 必须为整数" {
		t.Errorf("消息 = %q, 期望 %q", msg, "id 必须为整数")
	}
}

func TestRegionCrud_RequiresAdmin(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 无 token → 401
	w := performRequest(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "江苏", "city": "苏州"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 状态码 = %d, 期望 401\nbody=%s", w.Code, w.Body.String())
	}

	// 非 admin 角色 → 403
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/region-coefficients",
		map[string]interface{}{"province": "江苏", "city": "苏州"}, authHeader(t, 1))
	if w.Code != http.StatusForbidden {
		t.Fatalf("hrwai_user 状态码 = %d, 期望 403\nbody=%s", w.Code, w.Body.String())
	}
}

// mustDecodeData 解包信封并返回 data。
func mustDecodeData(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var env struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是统一信封 JSON: %v", err)
	}
	return env.Data
}
