// 规格字典（吨位/门架类型/门架高度/电池类型/传动/发动机）管理员 CRUD HTTP 契约测试。
// 形状一致：单字段唯一列 + C/D（无 PUT）+ create bind 必填 + DO NOTHING。
// 6 个实体表驱动覆盖，断言与迁移前逐项一致。
package handler

import (
	"fmt"
	"net/http"
	"testing"
)

type specCase struct {
	entity      string // 内存表名
	path        string // 路由段
	field       string // body/响应字段名
	value       interface{}
	bindName    string // bind 必填错误消息中的字段名
	notFoundMsg string // 404 消息
	label       string // 500 消息中的实体标签
}

var specCases = []specCase{
	{"tonnages", "tonnages", "value", 3.5, "Value", "吨位不存在", "吨位"},
	{"mast_types", "mast-types", "name", "三级门架", "Name", "门架类型不存在", "门架类型"},
	{"mast_heights", "mast-heights", "value_mm", 3000, "ValueMM", "门架高度不存在", "门架高度"},
	{"battery_types", "battery-types", "name", "磷酸铁锂", "Name", "电池类型不存在", "电池类型"},
	{"transmission_types", "transmission-types", "name", "手波", "Name", "传动系统类型不存在", "传动系统类型"},
	{"engine_types", "engine-types", "name", "国产发动机", "Name", "发动机类型不存在", "发动机类型"},
}

func TestSpecsCrud_Create(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, dict, _ := newTestValuationEngine(t)

			w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/"+tc.path,
				map[string]interface{}{tc.field: tc.value}, adminAuthHeader(t))
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
			if got := regionVal(t, data, tc.field); fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tc.value) {
				t.Errorf("%s = %v, 期望 %v", tc.field, got, tc.value)
			}
			rows := rowsOf(dict, tc.entity)
			if len(rows) != 1 {
				t.Fatalf("存储未写入: %+v", rows)
			}
			if fmt.Sprintf("%v", rows[0].values[tc.field]) != fmt.Sprintf("%v", tc.value) {
				t.Errorf("存储值错误: %+v", rows[0].values)
			}
		})
	}
}

func TestSpecsCrud_Create_MissingRequired(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, _, _ := newTestValuationEngine(t)

			// 字段缺失 → 400 bind required 消息
			w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/"+tc.path,
				map[string]interface{}{}, adminAuthHeader(t))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
			}
			code, msg, _ := decodeBody(t, w)
			want := "请求体格式错误: Key: '" + tc.bindName +
				"' Error:Field validation for '" + tc.bindName + "' failed on the 'required' tag"
			if code != http.StatusBadRequest || msg != want {
				t.Errorf("code=%d msg=%q, 期望 400 %q", code, msg, want)
			}
		})
	}
}

func TestSpecsCrud_Create_MalformedJSON(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, _, _ := newTestValuationEngine(t)

			w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/"+tc.path,
				"{\""+tc.field+":", adminAuthHeader(t))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestSpecsCrud_Delete(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, dict, _ := newTestValuationEngine(t)
			dict.table(tc.entity).rows = []memRow{
				{id: 1, values: map[string]interface{}{tc.field: tc.value}},
				{id: 2, values: map[string]interface{}{tc.field: tc.value}},
			}

			w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/"+tc.path+"/1",
				nil, adminAuthHeader(t))
			if w.Code != http.StatusOK {
				t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
			}
			code, _, data := decodeBody(t, w)
			if code != http.StatusOK || regionVal(t, data, "id") != float64(1) {
				t.Errorf("delete 响应错误: code=%d data=%v", code, data)
			}
			if rows := rowsOf(dict, tc.entity); len(rows) != 1 || rows[0].id != 2 {
				t.Errorf("存储未删除: %+v", rows)
			}
		})
	}
}

func TestSpecsCrud_Delete_NotFound(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, _, _ := newTestValuationEngine(t)

			w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/"+tc.path+"/999",
				nil, adminAuthHeader(t))
			if w.Code != http.StatusNotFound {
				t.Fatalf("状态码 = %d, 期望 404\nbody=%s", w.Code, w.Body.String())
			}
			code, msg, _ := decodeBody(t, w)
			if code != http.StatusNotFound || msg != tc.notFoundMsg {
				t.Errorf("code=%d msg=%q, 期望 404 %q", code, msg, tc.notFoundMsg)
			}
		})
	}
}

func TestSpecsCrud_Delete_BadID(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, _, _ := newTestValuationEngine(t)

			w := performRequestWithAuth(r, http.MethodDelete, "/api/valuation/admin/"+tc.path+"/xyz",
				nil, adminAuthHeader(t))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
			}
			_, msg, _ := decodeBody(t, w)
			if msg != "id 必须为整数" {
				t.Errorf("消息 = %q, 期望 %q", msg, "id 必须为整数")
			}
		})
	}
}

func TestSpecsCrud_NoPutRoute(t *testing.T) {
	for _, tc := range specCases {
		t.Run(tc.path, func(t *testing.T) {
			r, _, _ := newTestValuationEngine(t)

			// 规格实体无 update 路由：PUT 不注册 → 404
			w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/"+tc.path+"/1",
				map[string]interface{}{tc.field: tc.value}, adminAuthHeader(t))
			if w.Code != http.StatusNotFound {
				t.Fatalf("PUT 状态码 = %d, 期望 404（无 update 路由）\nbody=%s", w.Code, w.Body.String())
			}
		})
	}
}

// TestSpecsCrud_RequiresAdmin 规格实体写路由要求 admin 权限。
func TestSpecsCrud_RequiresAdmin(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequest(r, http.MethodPost, "/api/valuation/admin/tonnages",
		map[string]interface{}{"value": 3.5})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 状态码 = %d, 期望 401\nbody=%s", w.Code, w.Body.String())
	}
	w = performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/tonnages",
		map[string]interface{}{"value": 3.5}, authHeader(t, 1))
	if w.Code != http.StatusForbidden {
		t.Fatalf("hrwai_user 状态码 = %d, 期望 403\nbody=%s", w.Code, w.Body.String())
	}
}
