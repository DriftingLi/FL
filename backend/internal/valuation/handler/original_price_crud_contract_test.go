// 原价（original_prices）管理员 CRUD HTTP 契约测试。
// 形状：10 字段宽行 + C/U/D + create/update 返回整行（含 updated_at:""）+ 默认 2000 +
// brand/vehicle_type/series 必填 + original_price > 0。
// 注：List 分页读侧保持 typed（未迁移，契约不变）。
package handler

import (
	"net/http"
	"testing"
)

// fullOriginalPriceBody 完整原价行（不含 id/updated_at）。
func fullOriginalPriceBody() map[string]interface{} {
	return map[string]interface{}{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列",
		"tonnage": 3.0, "config_type": "标准", "mast_type": "标准门架",
		"mast_height_mm": 3000, "earliest_factory_year": 2015, "original_price": 100000.0,
	}
}

func TestOriginalPriceCrud_Create(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		fullOriginalPriceBody(), adminAuthHeader(t))
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
	// 响应 = 整行：id + 9 字段 + updated_at
	if len(data) != 11 {
		t.Fatalf("create 响应应含 11 项（含 updated_at）, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "brand") != "合力" || regionVal(t, data, "original_price") != 100000.0 {
		t.Errorf("字段回显错误: %v", data)
	}
	if regionVal(t, data, "updated_at") != "" {
		t.Errorf("updated_at 应为空字符串（现状响应）, got %v", data)
	}
	rows := rowsOf(dict, "original_prices")
	if len(rows) != 1 || rows[0].values["brand"] != "合力" {
		t.Errorf("存储未写入: %+v", rows)
	}
}

func TestOriginalPriceCrud_Create_DefaultYear(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 earliest_factory_year → 默认 2000（create 侧）
	body := fullOriginalPriceBody()
	delete(body, "earliest_factory_year")
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		body, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	data := mustDecodeData(t, w)
	if regionVal(t, data, "earliest_factory_year") != float64(2000) {
		t.Errorf("earliest_factory_year 应默认 2000, got %v", data)
	}
}

func TestOriginalPriceCrud_Create_RequiredValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// 缺 series → 400 自定义必填消息
	body := fullOriginalPriceBody()
	delete(body, "series")
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		body, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "brand/vehicle_type/series 必填" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "brand/vehicle_type/series 必填")
	}
}

func TestOriginalPriceCrud_Create_PositiveValidation(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// original_price <= 0 → 400 "original_price 必须大于 0"
	body := fullOriginalPriceBody()
	body["original_price"] = 0
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		body, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusBadRequest || msg != "original_price 必须大于 0" {
		t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, "original_price 必须大于 0")
	}

	body = fullOriginalPriceBody()
	body["original_price"] = -5
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		body, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("负数状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestOriginalPriceCrud_Create_MalformedJSON(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/original-prices",
		"{\"brand\":", adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestOriginalPriceCrud_Update(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("original_prices").rows = []memRow{{id: 1, values: map[string]interface{}{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列",
		"tonnage": 3.0, "config_type": "标准", "mast_type": "标准门架",
		"mast_height_mm": 3000, "earliest_factory_year": 2015, "original_price": 100000.0}}}

	body := fullOriginalPriceBody()
	body["original_price"] = 105000.0
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/original-prices/1",
		body, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if len(data) != 11 {
		t.Fatalf("update 响应应含 11 项（整行）, got %d 项: %v", len(data), data)
	}
	if regionVal(t, data, "id") != float64(1) || regionVal(t, data, "original_price") != 105000.0 {
		t.Errorf("update 响应错误: %v", data)
	}
	if regionVal(t, data, "updated_at") != "" {
		t.Errorf("updated_at 应为空字符串, got %v", data)
	}
	rows := rowsOf(dict, "original_prices")
	if rows[0].values["original_price"] != 105000.0 {
		t.Errorf("存储未更新: %+v", rows[0].values)
	}
}

func TestOriginalPriceCrud_Update_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/original-prices/999",
		fullOriginalPriceBody(), adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "原价记录不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "原价记录不存在")
	}
}

func TestOriginalPriceCrud_Update_Validations(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// update 侧同样校验必填与正数
	body := fullOriginalPriceBody()
	delete(body, "brand")
	w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/original-prices/1",
		body, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺 brand 状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	_, msg, _ := decodeBody(t, w)
	if msg != "brand/vehicle_type/series 必填" {
		t.Errorf("消息 = %q, 期望 %q", msg, "brand/vehicle_type/series 必填")
	}

	body = fullOriginalPriceBody()
	body["original_price"] = 0
	w = performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/original-prices/1",
		body, adminAuthHeader(t))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("original_price=0 状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
}

func TestOriginalPriceCrud_Delete(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.table("original_prices").rows = []memRow{
		{id: 1, values: map[string]interface{}{"brand": "A"}},
		{id: 2, values: map[string]interface{}{"brand": "B"}},
	}

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/original-prices/1",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
		t.Errorf("delete 响应错误: code=%d data=%v", code, data)
	}
	if rows := rowsOf(dict, "original_prices"); len(rows) != 1 || rows[0].id != 2 {
		t.Errorf("存储未删除: %+v", rows)
	}
}

func TestOriginalPriceCrud_Delete_NotFound(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/original-prices/999",
		nil, adminAuthHeader(t))
	if w.Code != http.StatusNotFound {
		t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
	}
	code, msg, _ := decodeBody(t, w)
	if code != http.StatusNotFound || msg != "原价记录不存在" {
		t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, "原价记录不存在")
	}
}
