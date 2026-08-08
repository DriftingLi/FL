// 系数配置（coefficient_configs）管理员更新 HTTP 契约测试。
// 形状：U-only（无 create/delete）+ PUT /:key + value bind 必填 + 响应为完整行
// （id/key/value/description/updated_at）+ 404 消息附 key。
package handler

import (
	"net/http"
	"testing"
)

func TestCoefficientCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
		map[string]interface{}{"value": 0.15}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if regionVal(t, data, "key") != "lambda_electric" || regionVal(t, data, "value") != 0.15 {
		t.Errorf("key/value 回显错误: %v", data)
	}
	if regionVal(t, data, "id") == nil {
		t.Errorf("响应应含 id: %v", data)
	}
	if regionVal(t, data, "description") != "" {
		t.Errorf("description 应为空字符串（可空列）: %v", data)
	}
	if v := regionVal(t, data, "updated_at"); v == nil || v == "" {
		t.Errorf("updated_at 应为格式化时间串: %v", data)
	}
	if dict.coefficients["lambda_electric"] != 0.15 {
		t.Errorf("存储未更新: %v", dict.coefficients["lambda_electric"])
	}
}

func TestCoefficientCrud_Update_MissingValue(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// value 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
		map[string]interface{}{}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'Value' Error:Field validation for 'Value' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestCoefficientCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 未知 key → 404，消息附 key
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/no_such_key",
		map[string]interface{}{"value": 0.15}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "系数 key 不存在: no_such_key" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "系数 key 不存在: no_such_key")
	}
}

func TestCoefficientCrud_Update_MalformedJSON(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
		"{\"value\":", adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestCoefficientCrud_NoCreateOrDelete(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 系数配置只有 PUT /:key：POST 与 DELETE 不注册 → 404
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/coefficient-configs",
		map[string]interface{}{"key": "x", "value": 0.1}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST 状态码 = %d, 期望 404（无 create 路由）\nbody=%s", w.Code, w.Body.String())
	}
	w = performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/coefficient-configs/lambda_electric",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("DELETE 状态码 = %d, 期望 404（无 delete 路由）\nbody=%s", w.Code, w.Body.String())
	}
}

func TestCoefficientCrud_RequiresAdmin(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequest(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
		map[string]interface{}{"value": 0.15})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 状态码 = %d, 期望 401\nbody=%s", w.Code, w.Body.String())
	}
	w = performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/coefficient-configs/lambda_electric",
		map[string]interface{}{"value": 0.15}, authHeader(t, 1))
	if w.Code != http.StatusForbidden {
		t.Fatalf("hrwai_user 状态码 = %d, 期望 403\nbody=%s", w.Code, w.Body.String())
	}
}
