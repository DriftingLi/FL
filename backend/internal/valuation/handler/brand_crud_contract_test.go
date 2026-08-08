// 品牌（brands）管理员 CRUD HTTP 契约测试（迁移前后同一套断言必须原样通过）。
// 形状：C/U/D 全量 + create 仅 name 必填（"name 必填"）+ update 为 k_brand/is_active 子集
// （响应不含 name）+ k_brand bind 必填。follow region_crud_contract_test.go 的 prior art。
package handler

import (
	"net/http"
	"testing"
)

func TestBrandCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"name": "林德", "k_brand": 1.10, "is_active": true}, adminAuthHeader(t))
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
	if regionVal(t, data, "name") != "林德" || regionVal(t, data, "k_brand") != 1.10 || regionVal(t, data, "is_active") != true {
		t.Errorf("字段回显错误: %v", data)
	}
	rows := rowsOf(dict, "brands")
	if len(rows) != 1 || rows[0].values["name"] != "林德" || rows[0].values["k_brand"] != 1.10 {
		t.Errorf("存储未写入: %+v", rows)
	}
}

func TestBrandCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 name → 400 自定义必填消息
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"k_brand": 1.1}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "name 必填" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "name 必填")
	}

	// 空字符串同样拒绝
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"name": ""}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("空字符串状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestBrandCrud_Create_MissingKBrandAllowed(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 现状：create 不校验 k_brand，缺失时落 0（锁定该行为）
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"name": "合力"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	if regionVal(t, mustDecodeData(t, w), "k_brand") != 0.0 {
		t.Errorf("缺失 k_brand 应落 0")
	}
}

func TestBrandCrud_Create_MalformedJSON(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		"{\"name\":", adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestBrandCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("brands").rows = []memRow{{id: 1, values: map[string]interface{}{
		"name": "林德", "k_brand": 1.10, "is_active": true}}}

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/brands/1",
		map[string]interface{}{"k_brand": 1.12, "is_active": false}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if len(data) != 3 {
		t.Fatalf("update 响应应只含 id+k_brand+is_active（无 name）, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "id") != float64(1) || regionVal(t, data, "k_brand") != 1.12 || regionVal(t, data, "is_active") != false {
		t.Errorf("update 响应错误: %v", data)
	}
	rows := rowsOf(dict, "brands")
	if rows[0].values["k_brand"] != 1.12 || rows[0].values["is_active"] != false {
		t.Errorf("存储未更新: %+v", rows[0].values)
	}
	if rows[0].values["name"] != "林德" {
		t.Errorf("update 不应改动 name: %+v", rows[0].values)
	}
}

func TestBrandCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/brands/999",
		map[string]interface{}{"k_brand": 1.12}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "品牌不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "品牌不存在")
	}
}

func TestBrandCrud_Update_MissingKBrand(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// k_brand 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/brands/1",
		map[string]interface{}{}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'KBrand' Error:Field validation for 'KBrand' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestBrandCrud_Update_BadID(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/brands/abc",
		map[string]interface{}{"k_brand": 1.12}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	_, msg, _ := decodeBody(t, w)
	if msg != "id 必须为整数" {
		t.Errorf("消息 = %q, 期望 %q", msg, "id 必须为整数")
	}
}

func TestBrandCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("brands").rows = []memRow{
		{id: 1, values: map[string]interface{}{"name": "林德"}},
		{id: 2, values: map[string]interface{}{"name": "合力"}},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/brands/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	rows := rowsOf(dict, "brands")
	if len(rows) != 1 || rows[0].id != 2 {
		t.Errorf("存储未删除: %+v", rows)
	}
}

func TestBrandCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/brands/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "品牌不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "品牌不存在")
	}
}
