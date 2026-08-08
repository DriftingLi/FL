// 系列（series）管理员 CRUD HTTP 契约测试。
// 形状：C/U/D 全量 + (brand, name) 复合唯一 DO NOTHING（重复 → 500）+ 默认 2000 +
// create 应用层必填（"brand 与 name 必填"）+ update bind 必填。
package handler

import (
	"net/http"
	"testing"
)

func TestSeriesCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/series",
		map[string]interface{}{"brand": "林德", "name": "E系列", "earliest_factory_year": 2015}, adminAuthHeader(t))
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
	if regionVal(t, data, "brand") != "林德" || regionVal(t, data, "name") != "E系列" || regionVal(t, data, "earliest_factory_year") != float64(2015) {
		t.Errorf("字段回显错误: %v", data)
	}
	rows := rowsOf(dict, "series")
	if len(rows) != 1 || rows[0].values["name"] != "E系列" {
		t.Errorf("存储未写入: %+v", rows)
	}
}

func TestSeriesCrud_Create_DefaultYear(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 earliest_factory_year → 默认 2000（create 侧）
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/series",
		map[string]interface{}{"brand": "林德", "name": "E系列"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	data := mustDecodeData(t, w)
	if regionVal(t, data, "earliest_factory_year") != float64(2000) {
		t.Errorf("earliest_factory_year 应默认 2000, got %v", data)
	}
}

func TestSeriesCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 brand → 400 自定义必填消息
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/series",
		map[string]interface{}{"name": "E系列"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "brand 与 name 必填" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "brand 与 name 必填")
	}
}

func TestSeriesCrud_Create_Duplicate(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// (brand, name) 唯一 DO NOTHING：重复 create 无行返回 → 500（锁定现状行为）
	body := map[string]interface{}{"brand": "林德", "name": "E系列"}
	for i := 0; i < 2; i++ {
		w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/series",
			body, adminAuthHeader(t))
		if i == 0 {
			if w.Code != http.StatusOK {
				t.Fatalf("首次创建状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
			}
			continue
		}
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("重复创建状态码 = %d, 期望 500\nbody=%s", w.Code, w.Body.String())
		}
		code, msg, _ := decodeBody(t, w)
		if code != http.StatusInternalServerError || msg != "新增系列失败" {
			t.Errorf("code=%d msg=%q, 期望 500 %q", code, msg, "新增系列失败")
		}
	}
}

func TestSeriesCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("series").rows = []memRow{{id: 1, values: map[string]interface{}{
		"brand": "林德", "name": "E系列", "earliest_factory_year": 2015}}}

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/series/1",
		map[string]interface{}{"brand": "林德", "name": "K系列", "earliest_factory_year": 2016}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if len(data) != 4 {
		t.Fatalf("update 响应应含 id+3 字段, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "name") != "K系列" || regionVal(t, data, "earliest_factory_year") != float64(2016) {
		t.Errorf("update 响应错误: %v", data)
	}
	rows := rowsOf(dict, "series")
	if rows[0].values["name"] != "K系列" {
		t.Errorf("存储未更新: %+v", rows)
	}
}

func TestSeriesCrud_Update_DefaultYear(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("series").rows = []memRow{{id: 1, values: map[string]interface{}{
		"brand": "林德", "name": "E系列", "earliest_factory_year": 2015}}}

	// update 也应用默认 2000
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/series/1",
		map[string]interface{}{"brand": "林德", "name": "K系列"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	data := mustDecodeData(t, w)
	if regionVal(t, data, "earliest_factory_year") != float64(2000) {
		t.Errorf("update 应默认 2000, got %v", data)
	}
}

func TestSeriesCrud_Update_MissingRequired(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// brand/name 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/series/1",
		map[string]interface{}{"brand": "林德"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'Name' Error:Field validation for 'Name' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestSeriesCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/series/999",
		map[string]interface{}{"brand": "林德", "name": "K系列"}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "系列不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "系列不存在")
	}
}

func TestSeriesCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("series").rows = []memRow{
		{id: 1, values: map[string]interface{}{"brand": "林德", "name": "A"}},
		{id: 2, values: map[string]interface{}{"brand": "林德", "name": "B"}},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/series/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	if rows := rowsOf(dict, "series"); len(rows) != 1 || rows[0].id != 2 {
		t.Errorf("存储未删除: %+v", rows)
	}
}

func TestSeriesCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/series/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "系列不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "系列不存在")
	}
}
