// 车况评级（condition_ratings）管理员 CRUD HTTP 契约测试。
// 形状：C/U/D + create 应用层必填（"rating 与 label 必填"）+ update 无 rating 字段
// （响应只含 id+label+base_coefficient）+ update label/base_coefficient bind 必填。
package handler

import (
	"net/http"
	"testing"
)

func TestConditionCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/condition-ratings",
		map[string]interface{}{"rating": "S", "label": "极优", "base_coefficient": 1.10}, adminAuthHeader(t))
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
	if regionVal(t, data, "rating") != "S" || regionVal(t, data, "label") != "极优" || regionVal(t, data, "base_coefficient") != 1.10 {
		t.Errorf("字段回显错误: %v", data)
	}
	rows := rowsOf(dict, "condition_ratings")
	if len(rows) != 1 || rows[0].values["rating"] != "S" {
		t.Errorf("存储未写入: %+v", rows)
	}
}

func TestConditionCrud_Create_MissingBaseAllowed(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 现状：create 不校验 base_coefficient，缺失时落 0（锁定该行为）
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/condition-ratings",
		map[string]interface{}{"rating": "S", "label": "极优"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	if regionVal(t, mustDecodeData(t, w), "base_coefficient") != 0.0 {
		t.Errorf("缺失 base_coefficient 应落 0")
	}
}

func TestConditionCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 label → 400 自定义必填消息
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/condition-ratings",
		map[string]interface{}{"rating": "S"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "rating 与 label 必填" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "rating 与 label 必填")
	}
}

func TestConditionCrud_Create_MalformedJSON(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/condition-ratings",
		"{\"rating\":", adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestConditionCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("condition_ratings").rows = []memRow{{id: 1, values: map[string]interface{}{
		"rating": "S", "label": "极优", "base_coefficient": 1.10}}}

	// update 无 rating 字段：只发 label + base_coefficient
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/condition-ratings/1",
		map[string]interface{}{"label": "优秀", "base_coefficient": 1.05}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if len(data) != 3 {
		t.Fatalf("update 响应应只含 id+label+base_coefficient（无 rating）, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "id") != float64(1) || regionVal(t, data, "label") != "优秀" || regionVal(t, data, "base_coefficient") != 1.05 {
		t.Errorf("update 响应错误: %v", data)
	}
	rows := rowsOf(dict, "condition_ratings")
	if rows[0].values["label"] != "优秀" {
		t.Errorf("存储未更新: %+v", rows[0].values)
	}
	if rows[0].values["rating"] != "S" {
		t.Errorf("update 不应改动 rating: %+v", rows[0].values)
	}
}

func TestConditionCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/condition-ratings/999",
		map[string]interface{}{"label": "优秀", "base_coefficient": 1.05}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "车况评级不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "车况评级不存在")
	}
}

func TestConditionCrud_Update_MissingRequired(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// label/base_coefficient 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/condition-ratings/1",
		map[string]interface{}{"label": "优秀"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'BaseCoefficient' Error:Field validation for 'BaseCoefficient' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestConditionCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("condition_ratings").rows = []memRow{
		{id: 1, values: map[string]interface{}{"rating": "S"}},
		{id: 2, values: map[string]interface{}{"rating": "A"}},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/condition-ratings/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	if rows := rowsOf(dict, "condition_ratings"); len(rows) != 1 || rows[0].id != 2 {
		t.Errorf("存储未删除: %+v", rows)
	}
}

func TestConditionCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/condition-ratings/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "车况评级不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "车况评级不存在")
	}
}
