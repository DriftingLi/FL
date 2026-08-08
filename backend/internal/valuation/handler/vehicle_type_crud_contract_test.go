// 车型（vehicle_types）管理员 CRUD HTTP 契约测试。
// 形状：C/U/D 全量 + create/update 双侧 earliest_factory_year 默认 1980 +
// create 应用层必填（"name 与 power_type 必填"）+ update bind 必填。
package handler

import (
	"net/http"
	"testing"
)

func TestVehicleTypeCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/vehicle-types",
		map[string]interface{}{"name": "电动平衡重", "power_type": "electric", "earliest_factory_year": 1995}, adminAuthHeader(t))
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
	if regionVal(t, data, "name") != "电动平衡重" || regionVal(t, data, "power_type") != "electric" || regionVal(t, data, "earliest_factory_year") != float64(1995) {
		t.Errorf("字段回显错误: %v", data)
	}
	rows := rowsOf(dict, "vehicle_types")
	if len(rows) != 1 || rows[0].values["name"] != "电动平衡重" {
		t.Errorf("存储未写入: %+v", rows)
	}
}

func TestVehicleTypeCrud_Create_DefaultYear(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 earliest_factory_year → 默认 1980（create 侧）
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/vehicle-types",
		map[string]interface{}{"name": "电动平衡重", "power_type": "electric"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	data := mustDecodeData(t, w)
	if regionVal(t, data, "earliest_factory_year") != float64(1980) {
		t.Errorf("earliest_factory_year 应默认 1980, got %v", data)
	}
}

func TestVehicleTypeCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 power_type → 400 自定义必填消息
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/vehicle-types",
		map[string]interface{}{"name": "电动平衡重"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "name 与 power_type 必填" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "name 与 power_type 必填")
	}

	// 缺 name 同样拒绝
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/vehicle-types",
		map[string]interface{}{"power_type": "electric"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 name 状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestVehicleTypeCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("vehicle_types").rows = []memRow{{id: 1, values: map[string]interface{}{
		"name": "电动平衡重", "power_type": "electric", "earliest_factory_year": 1995}}}

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/vehicle-types/1",
		map[string]interface{}{"name": "电动叉车", "power_type": "electric", "earliest_factory_year": 2005}, adminAuthHeader(t))
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
	if regionVal(t, data, "name") != "电动叉车" || regionVal(t, data, "earliest_factory_year") != float64(2005) {
		t.Errorf("update 响应错误: %v", data)
	}
}

func TestVehicleTypeCrud_Update_DefaultYear(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("vehicle_types").rows = []memRow{{id: 1, values: map[string]interface{}{
		"name": "电动平衡重", "power_type": "electric", "earliest_factory_year": 1995}}}

	// update 也应用默认 1980（缺 earliest_factory_year → 1980）
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/vehicle-types/1",
		map[string]interface{}{"name": "电动平衡重", "power_type": "electric"}, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	data := mustDecodeData(t, w)
	if regionVal(t, data, "earliest_factory_year") != float64(1980) {
		t.Errorf("update 应默认 1980, got %v", data)
	}
}

func TestVehicleTypeCrud_Update_MissingRequired(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// name/power_type 为 bind 必填：缺失 → 400 请求体格式错误
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/vehicle-types/1",
		map[string]interface{}{"name": "电动平衡重"}, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatalf("业务码 = %d, 期望 400", code)
	}
	if msg != "请求体格式错误: Key: 'PowerType' Error:Field validation for 'PowerType' failed on the 'required' tag" {
		t.Errorf("消息 = %q, 期望 bind required 消息", msg)
	}
}

func TestVehicleTypeCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/vehicle-types/999",
		map[string]interface{}{"name": "电动平衡重", "power_type": "electric"}, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "车型不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "车型不存在")
	}
}

func TestVehicleTypeCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("vehicle_types").rows = []memRow{
		{id: 1, values: map[string]interface{}{"name": "A"}},
		{id: 2, values: map[string]interface{}{"name": "B"}},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/vehicle-types/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	if rows := rowsOf(dict, "vehicle_types"); len(rows) != 1 || rows[0].id != 2 {
		t.Errorf("存储未删除: %+v", rows)
	}
}

func TestVehicleTypeCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/vehicle-types/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "车型不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "车型不存在")
	}
}
