// Package service 诊断只读代理测试（计划 批次2）：httptest fake 助手覆盖
// brands/models/fault-codes 解析、分页参数透传、手册资源代理与 SSRF/路径防御、
// 未配置与失败降级。
package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func newProxyForTest(server *httptest.Server) *DiagnosisProxyService {
	return NewDiagnosisProxyService(server.URL, zap.NewNop())
}

// TestDiagnosisProxy_BrandsAndModels brands 解析为 {value,label}；models 透传 brand 并解析为数组。
func TestDiagnosisProxy_BrandsAndModels(t *testing.T) {
	var lastQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/assistant/api/brands":
			_, _ = w.Write([]byte(`{"code":200,"data":[{"value":"all","label":"全部品牌"},{"value":"hangcha","label":"杭叉叉车"}]}`))
		case "/assistant/api/models":
			_, _ = w.Write([]byte(`{"code":200,"data":["全部车型","CBD15","CBD20"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	proxy := newProxyForTest(server)

	brands, err := proxy.ListBrands(context.Background())
	if err != nil || len(brands) != 2 || brands[1].Value != "hangcha" || brands[1].Label != "杭叉叉车" {
		t.Fatalf("brands 解析不符: %+v err=%v", brands, err)
	}
	models, err := proxy.ListModels(context.Background(), "hangcha")
	if err != nil || len(models) != 3 || models[2] != "CBD20" {
		t.Fatalf("models 解析不符: %+v err=%v", models, err)
	}
	if !strings.Contains(lastQuery, "brand=hangcha") {
		t.Fatalf("models 应透传 brand 查询: %q", lastQuery)
	}
	// brand=all 不附加查询
	_, _ = proxy.ListModels(context.Background(), "all")
	if lastQuery != "" {
		t.Fatalf("brand=all 不应携带查询参数: %q", lastQuery)
	}
}

// TestDiagnosisProxy_FaultCodes 分页/筛选参数透传与 items/total 解析。
func TestDiagnosisProxy_FaultCodes(t *testing.T) {
	var lastQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"total":33,"items":[{"id":1,"brand":"hangcha","brand_cn":"杭叉叉车","model_series":"CBD系列","fault_code":"05","fault_name":"提升接触器开路","symptom":"起升无反应","causes":"线圈脱落","sop_steps":"步骤1","safety_warning":"断电","part_numbers":"","source_file":"a.pdf","page_num":12}]}}`))
	}))
	defer server.Close()
	proxy := newProxyForTest(server)

	page, err := proxy.ListFaultCodes(context.Background(), "hangcha", "05", 2, 30)
	if err != nil || page.Total != 33 || len(page.Items) != 1 {
		t.Fatalf("fault-codes 解析不符: %+v err=%v", page, err)
	}
	it := page.Items[0]
	if it.FaultCode != "05" || it.FaultName != "提升接触器开路" || it.SOPSteps != "步骤1" {
		t.Fatalf("条目字段不符: %+v", it)
	}
	for _, want := range []string{"brand=hangcha", "keyword=05", "page=2", "page_size=30"} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("fault-codes 查询应含 %q: %q", want, lastQuery)
		}
	}
}

// TestDiagnosisProxy_Manual 手册资源代理：合法路径返回内容与 Content-Type；非法路径拒绝。
func TestDiagnosisProxy_Manual(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/assistant/static/manual/ep_test/page_1.png" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	defer server.Close()
	proxy := newProxyForTest(server)

	body, ct, err := proxy.OpenManual(context.Background(), "ep_test/page_1.png")
	if err != nil {
		t.Fatalf("打开手册失败: %v", err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "PNGDATA" || ct != "image/png" {
		t.Fatalf("手册内容/类型不符: %q %q", data, ct)
	}
	// SSRF/穿越与无扩展名拒绝
	for _, bad := range []string{"../etc/passwd", "a/b", "/etc/passwd", "a;rm%20x.png", "?x=1/y.png"} {
		if _, _, err := proxy.OpenManual(context.Background(), bad); err == nil {
			t.Fatalf("非法手册路径应拒绝: %q", bad)
		}
	}
}

// TestDiagnosisProxy_Errors 未配置 baseURL、助手非 200、服务不可达的降级。
func TestDiagnosisProxy_Errors(t *testing.T) {
	// 未配置
	unconf := NewDiagnosisProxyService("", zap.NewNop())
	if _, err := unconf.ListBrands(context.Background()); err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置应返回友好错误: %v", err)
	}
	// 助手 503（detail 透出）
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"detail":"knowledge offline"}`))
	}))
	defer server.Close()
	if _, err := newProxyForTest(server).ListFaultCodes(context.Background(), "", "", 1, 20); err == nil || !strings.Contains(err.Error(), "knowledge offline") {
		t.Fatalf("非 200 应透出 detail: %v", err)
	}
	// 信封 code 非 0/200
	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":500,"data":null}`))
	}))
	defer server2.Close()
	if _, err := newProxyForTest(server2).ListBrands(context.Background()); err == nil || !strings.Contains(err.Error(), "code 500") {
		t.Fatalf("code 非 0/200 应报错: %v", err)
	}
}
