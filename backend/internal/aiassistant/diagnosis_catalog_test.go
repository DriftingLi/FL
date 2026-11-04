// v2 引导只读面 service 的单元测试（票 2242）：五个 GET 的形状、集合归一、错误映射。
//
// 与 diagnosis_adapter_test.go 的分工：那边测 SSE 对话通道，这边测一次性 JSON 的引导面。
// 共用一份 fake 上游的思路一致：路径逐字匹配，字段名照契约写，**不按我方 DTO 回编码**
// （否则「上游改了键名」会伪装成「字段为空」而恒绿）。
package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// catalogFake 引导面假上游：按路径返回固定 JSON，并记录最近一次的 query 与鉴权头。
type catalogFake struct {
	t        *testing.T
	routes   map[string]string // path → 响应体
	status   int               // 非 0 时统一走失败分支
	detail   string
	lastPath string
	lastQry  string
	lastKey  string
}

func (f *catalogFake) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastPath = r.URL.Path
		f.lastQry = r.URL.RawQuery
		f.lastKey = r.Header.Get("X-API-Key")
		if f.status != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"detail":"` + f.detail + `"}`))
			return
		}
		body, ok := f.routes[r.URL.Path]
		if !ok {
			f.t.Errorf("未预期的引导面路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
}

func newCatalogService(t *testing.T, fake *catalogFake) (*DiagnosisCatalogService, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)
	return NewDiagnosisCatalogService(server.URL, "test-key", zap.NewNop()), server
}

// TestDiagnosisCatalogIntents 品牌卡与高频问题：六个字段逐个透传，picker 原样保留
// （第二层渲系统卡还是型号卡由它决定，map 层不能吞）。
func TestDiagnosisCatalogIntents(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{
		"/api/v2/intents": `{"brands":[
			{"id":"zhongli","name":"中力","tagline":"电动仓储车","icon":"🚜","cls":"cap-brand-zl","picker":"system"},
			{"id":"linde","name":"林德","tagline":"德国进口","icon":"🏭","cls":"cap-brand-ld","picker":"model"}
		],"faq":[{"q":"液压泵压力多少正常","a":"16~18MPa"}]}`,
	}}
	svc, _ := newCatalogService(t, fake)
	out, err := svc.Intents(context.Background())
	if err != nil {
		t.Fatalf("拉引导面失败: %v", err)
	}
	if len(out.Brands) != 2 {
		t.Fatalf("brands 应有 2 条，got %d", len(out.Brands))
	}
	if out.Brands[0].Picker != "system" || out.Brands[1].Picker != "model" {
		t.Fatalf("picker 应原样透传（第二层渲卡片形态靠它分流），got %q / %q",
			out.Brands[0].Picker, out.Brands[1].Picker)
	}
	if out.Brands[1].Cls != "cap-brand-ld" || out.Brands[1].Icon != "🏭" {
		t.Fatalf("cls/icon 应原样透传，got %+v", out.Brands[1])
	}
	if len(out.FAQ) != 1 || out.FAQ[0].Q != "液压泵压力多少正常" {
		t.Fatalf("faq 不符: %+v", out.FAQ)
	}
	if fake.lastKey != "test-key" {
		t.Fatalf("引导面必须带 X-API-Key，got %q", fake.lastKey)
	}
}

// TestDiagnosisCatalogIntentsEmpty 上游回 null / 缺字段时集合归一成空集（判据 5）。
func TestDiagnosisCatalogIntentsEmpty(t *testing.T) {
	for _, body := range []string{
		`{"brands":null,"faq":null}`,
		`{}`,
	} {
		fake := &catalogFake{t: t, routes: map[string]string{"/api/v2/intents": body}}
		svc, _ := newCatalogService(t, fake)
		out, err := svc.Intents(context.Background())
		if err != nil {
			t.Fatalf("%s: 拉引导面失败: %v", body, err)
		}
		if out.Brands == nil || out.FAQ == nil {
			t.Fatalf("%s: 集合应归一成空集而不是 nil，got brands=%v faq=%v", body, out.Brands, out.FAQ)
		}
		if len(out.Brands) != 0 || len(out.FAQ) != 0 {
			t.Fatalf("%s: 期望空集，got %+v", body, out)
		}
	}
}

// TestDiagnosisCatalogSystems brand 走 query 而不是路径段；icon 只在有值时出现。
func TestDiagnosisCatalogSystems(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{
		"/api/v2/systems": `{"items":[{"id":"hyd","name":"液压系统","icon":"🔧"},{"id":"elec","name":"电气系统"}]}`,
	}}
	svc, _ := newCatalogService(t, fake)
	out, err := svc.Systems(context.Background(), "zhongli")
	if err != nil {
		t.Fatalf("拉系统列表失败: %v", err)
	}
	if fake.lastQry != "brand=zhongli" {
		t.Fatalf("brand 应作为 query 发出，got %q", fake.lastQry)
	}
	if len(out.Items) != 2 || out.Items[0].Icon != "🔧" || out.Items[1].Icon != "" {
		t.Fatalf("items 不符: %+v", out.Items)
	}
	// 空 icon 必须从 wire 上消失（契约里林德型号没有 icon 字段）。
	raw, _ := json.Marshal(out.Items[1])
	if want := `{"id":"elec","name":"电气系统"}`; string(raw) != want {
		t.Fatalf("无 icon 的条目不应出现 icon 键，got %s", raw)
	}
}

// TestDiagnosisCatalogFaults second 为空时不发该参数（= 品牌根列表），非空时原样带上。
func TestDiagnosisCatalogFaults(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{
		"/api/v2/faults": `{"items":[{"id":"f1","name":"仪表报故障码"}]}`,
	}}
	svc, _ := newCatalogService(t, fake)

	if _, err := svc.Faults(context.Background(), "linde", ""); err != nil {
		t.Fatalf("拉故障现象失败: %v", err)
	}
	if fake.lastQry != "brand=linde" {
		t.Fatalf("second 为空时不应发该参数，got %q", fake.lastQry)
	}
	if _, err := svc.Faults(context.Background(), "linde", "E16"); err != nil {
		t.Fatalf("拉故障现象失败: %v", err)
	}
	if fake.lastQry != "brand=linde&second=E16" {
		t.Fatalf("second 非空时应原样带上，got %q", fake.lastQry)
	}
}

// TestDiagnosisCatalogCodeHint 五态与 samples：非 format 态不带 samples 时归一成空集。
func TestDiagnosisCatalogCodeHint(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantSt   string
		wantSamp int
	}{
		{"known", `{"status":"known","message":"该码指向液压压力过低","fix":"","samples":[]}`, "known", 0},
		{"unknown", `{"status":"unknown","message":"格式正确但未收录","fix":""}`, "unknown", 0},
		{"format", `{"status":"format","message":"不是合法故障码","fix":"","samples":["E01","E012"]}`, "format", 2},
		{"none", `{"status":"none","message":"","fix":""}`, "none", 0},
		{"typo", `{"status":"typo","message":"疑似笔误","fix":"E16","samples":[]}`, "typo", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &catalogFake{t: t, routes: map[string]string{"/api/v2/code_hint": c.body}}
			svc, _ := newCatalogService(t, fake)
			out, err := svc.CodeHint(context.Background(), "E1")
			if err != nil {
				t.Fatalf("故障码体检失败: %v", err)
			}
			if out.Status != c.wantSt {
				t.Fatalf("status = %q, want %q", out.Status, c.wantSt)
			}
			if out.Samples == nil {
				t.Fatal("samples 应归一成空集而不是 nil")
			}
			if len(out.Samples) != c.wantSamp {
				t.Fatalf("samples 应有 %d 条，got %v", c.wantSamp, out.Samples)
			}
		})
	}
}

// TestDiagnosisCatalogCodeHintEmptyQuery q 为空时不发该参数（上游回 status=none）。
func TestDiagnosisCatalogCodeHintEmptyQuery(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{
		"/api/v2/code_hint": `{"status":"none","message":"","fix":""}`,
	}}
	svc, _ := newCatalogService(t, fake)
	if _, err := svc.CodeHint(context.Background(), ""); err != nil {
		t.Fatalf("故障码体检失败: %v", err)
	}
	if fake.lastQry != "" {
		t.Fatalf("q 为空时不应发 query，got %q", fake.lastQry)
	}
}

// TestDiagnosisCatalogHealthProjection 健康检查只投影两字段、不发 Key、永不返 error。
func TestDiagnosisCatalogHealthProjection(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{
		// 真实 /api/health 的形状：20 个字段，其中四个是对方内部拓扑。
		"/api/health": `{"ok":true,"app_configured":true,"key_fp8":"a1b2c3d4","data_dir":"/data/forklift",
			"workflow_id_head":"wf_1234","memory_mode":"redis","uptime_s":3600,"version":"2.0.1"}`,
	}}
	svc, _ := newCatalogService(t, fake)
	out := svc.Health(context.Background())
	if !out.OK || !out.AppConfigured {
		t.Fatalf("健康检查应如实投影 ok/app_configured，got %+v", out)
	}
	if fake.lastKey != "" {
		t.Fatalf("上游 /api/health 无鉴权，不应发 X-API-Key，got %q", fake.lastKey)
	}
	// 只投影两字段：序列化出去不能带任何对方内部拓扑。
	raw, _ := json.Marshal(out)
	if want := `{"ok":true,"app_configured":true}`; string(raw) != want {
		t.Fatalf("健康检查只该投影两字段（Key 指纹 / 数据目录 / 工作流 ID 都是对方内部拓扑），got %s", raw)
	}
}

// TestDiagnosisCatalogHealthNeverErrors 连不上 / 非 200 / 坏 JSON 一律回零值而不是 error。
func TestDiagnosisCatalogHealthNeverErrors(t *testing.T) {
	// 上游 500
	fake := &catalogFake{t: t, status: http.StatusInternalServerError, detail: "boom"}
	svc, _ := newCatalogService(t, fake)
	if out := svc.Health(context.Background()); out.OK || out.AppConfigured {
		t.Fatalf("上游 500 时应回零值，got %+v", out)
	}

	// 坏 JSON
	fake = &catalogFake{t: t, routes: map[string]string{"/api/health": `not-json`}}
	svc, _ = newCatalogService(t, fake)
	if out := svc.Health(context.Background()); out.OK {
		t.Fatalf("坏 JSON 时应回零值，got %+v", out)
	}

	// 未配置基址（部署态）：连请求都不该发
	if out := NewDiagnosisCatalogService("", "k", zap.NewNop()).Health(context.Background()); out.OK != false {
		t.Fatalf("未配置基址时应回零值，got %+v", out)
	}
}

// TestDiagnosisCatalogNotConfigured 基址或 Key 为空时四个数据面方法都报「未配置」，
// 由 handler 映射成 502（部署态不是故障，但也不该伪装成空数据）。
func TestDiagnosisCatalogNotConfigured(t *testing.T) {
	for _, svc := range []*DiagnosisCatalogService{
		NewDiagnosisCatalogService("", "test-key", zap.NewNop()),
		NewDiagnosisCatalogService("http://x.test", "   ", zap.NewNop()),
	} {
		ctx := context.Background()
		if _, err := svc.Intents(ctx); !errors.Is(err, ErrDiagnosisNotConfigured) {
			t.Fatalf("Intents 应报未配置，got %v", err)
		}
		if _, err := svc.Systems(ctx, "zhongli"); !errors.Is(err, ErrDiagnosisNotConfigured) {
			t.Fatalf("Systems 应报未配置，got %v", err)
		}
		if _, err := svc.Faults(ctx, "zhongli", ""); !errors.Is(err, ErrDiagnosisNotConfigured) {
			t.Fatalf("Faults 应报未配置，got %v", err)
		}
		if _, err := svc.CodeHint(ctx, "E01"); !errors.Is(err, ErrDiagnosisNotConfigured) {
			t.Fatalf("CodeHint 应报未配置，got %v", err)
		}
	}
}

// TestDiagnosisCatalogStatusMapping 429 单独成类（前端要能区分「等会儿再试」与「服务坏了」），
// 其余非 200 一律 502 那一类。
func TestDiagnosisCatalogStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		detail string
		wantRL bool
	}{
		{http.StatusUnauthorized, "missing api key", false},
		{http.StatusForbidden, "ip not allowed", false},
		{http.StatusBadRequest, "bad param", false},
		{http.StatusTooManyRequests, "rate limited", true},
		{http.StatusBadGateway, "upstream boom", false},
		{http.StatusInternalServerError, "internal", false},
	}
	for _, c := range cases {
		fake := &catalogFake{t: t, status: c.status, detail: c.detail}
		svc, _ := newCatalogService(t, fake)
		_, err := svc.Systems(context.Background(), "zhongli")
		if err == nil {
			t.Fatalf("%d: 期望报错", c.status)
		}
		if got := errors.Is(err, ErrDiagnosisRateLimited); got != c.wantRL {
			t.Fatalf("%d: ErrDiagnosisRateLimited = %v, want %v（err=%v）", c.status, got, c.wantRL, err)
		}
	}
}

// TestDiagnosisCatalogBadJSON 上游回非 JSON 时不把解析错误裸抛给前端。
func TestDiagnosisCatalogBadJSON(t *testing.T) {
	fake := &catalogFake{t: t, routes: map[string]string{"/api/v2/intents": `<html>502 Bad Gateway</html>`}}
	svc, _ := newCatalogService(t, fake)
	_, err := svc.Intents(context.Background())
	if err == nil {
		t.Fatal("坏 JSON 应报错")
	}
	if errors.Is(err, ErrDiagnosisRateLimited) || errors.Is(err, ErrDiagnosisNotConfigured) {
		t.Fatalf("坏 JSON 不该被归进限流或未配置，got %v", err)
	}
}

// TestDiagnosisCatalogNoRedirect 上游 302 到别处必须显式失败：凭证与白名单只对声明的上游成立
// （ADR-0091 决策 8 同款）。引导面与对话面共用这条纪律。
func TestDiagnosisCatalogNoRedirect(t *testing.T) {
	var reached bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer elsewhere.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/api/v2/systems", http.StatusFound)
	}))
	defer redirector.Close()

	svc := NewDiagnosisCatalogService(redirector.URL, "test-key", zap.NewNop())
	if _, err := svc.Systems(context.Background(), "zhongli"); err == nil {
		t.Fatal("跟随 302 到非白名单上游应当报错")
	}
	if reached {
		t.Fatal("请求被 302 带到了另一个上游——凭证会跟着漏出去")
	}
}
