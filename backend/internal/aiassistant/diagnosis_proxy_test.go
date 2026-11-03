// 外部诊断回答配图白名单代理测试（ADR-0091 / 票 2237 B 节）：
// 钉死五道闸（scheme+host+前缀 / 剥 ?# / 逐段白名单 / 图片扩展名 / 解一次再转义）
// 与回源行为（不跟随重定向、非 200、未配置）。
package aiassistant

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

const (
	testOSSBasePath = "/forklift/repair_img"
	testOSSHostPath = "hrwai-guangzhou.oss-cn-beijing.aliyuncs.com"
)

// newOSSFake 起一个假 OSS：任意路径都回 PNG 字节，并记录最近一次请求路径。
func newOSSFake(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var lastPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG-bytes"))
	}))
	t.Cleanup(server.Close)
	return server, &lastPath
}

// TestDiagnosisOSSProxyPath_Whitelist 五道闸：正例与全部反例。
func TestDiagnosisOSSProxyPath_Whitelist(t *testing.T) {
	base := "https://" + testOSSHostPath + testOSSBasePath

	ok := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "林德型号目录",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/linde/Linde_A-005-01__image-20260824100446677.png",
			want: diagnosisImageRoute + "linde/Linde_A-005-01__image-20260824100446677.png",
		},
		{
			name: "中力规范名目录",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/images_all/制动系统异响.png",
			want: diagnosisImageRoute + "images_all/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F%E5%BC%82%E5%93%8D.png",
		},
		{
			name: "带查询串（剥掉）",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/linde/a.jpg?v=1",
			want: diagnosisImageRoute + "linde/a.jpg",
		},
		{
			name: "百分号编码文件名（解一次再转义，不双重编码）",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/linde/%E6%9E%97%E5%BE%B7__a.jpg",
			want: diagnosisImageRoute + "linde/%E6%9E%97%E5%BE%B7__a.jpg",
		},
		{
			name: "大写扩展名",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/linde/a.PNG",
			want: diagnosisImageRoute + "linde/a.PNG",
		},
		{
			name: "webp 放行",
			in:   "https://" + testOSSHostPath + testOSSBasePath + "/images_all/a.webp",
			want: diagnosisImageRoute + "images_all/a.webp",
		},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			got, valid := diagnosisOSSProxyPath(base, c.in)
			if !valid || got != c.want {
				t.Fatalf("应放行并归一:\n got=%q valid=%v\nwant=%q", got, valid, c.want)
			}
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"http 明文", "http://" + testOSSHostPath + testOSSBasePath + "/linde/a.png"},
		{"同形异域", "https://evil-" + testOSSHostPath + testOSSBasePath + "/linde/a.png"},
		{"别的 host", "https://example.com" + testOSSBasePath + "/linde/a.png"},
		{"前缀外", "https://" + testOSSHostPath + "/create/linde/a.png"},
		{"前缀只有父级", "https://" + testOSSHostPath + "/forklift/linde/a.png"},
		{"前缀子串伪装", "https://" + testOSSHostPath + testOSSBasePath + "-evil/linde/a.png"},
		{"穿越", "https://" + testOSSHostPath + testOSSBasePath + "/linde/../../etc/passwd.png"},
		{"编码穿越（解一次后落网）", "https://" + testOSSHostPath + testOSSBasePath + "/linde/%2e%2e/%2e%2e/etc/passwd.png"},
		{"二次编码残留 %", "https://" + testOSSHostPath + testOSSBasePath + "/linde/%252e%252e/a.png"},
		{"pdf 不放行（图片语法里只会是坏图）", "https://" + testOSSHostPath + testOSSBasePath + "/linde/a.pdf"},
		{"无扩展名", "https://" + testOSSHostPath + testOSSBasePath + "/linde/a"},
		{"空路径", "https://" + testOSSHostPath},
		{"相对路径", testOSSBasePath + "/linde/a.png"},
		{"空串", ""},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if got, valid := diagnosisOSSProxyPath(base, c.in); valid {
				t.Fatalf("应拒绝，got %q", got)
			}
		})
	}

	// 基准为空/非法：一律不放行（配置没给 OSS 基准就没有任何图可服务）
	for _, base := range []string{"", "   ", "http://" + testOSSHostPath + testOSSBasePath, "not-a-url"} {
		t.Run("基准非法 "+base, func(t *testing.T) {
			if _, valid := diagnosisOSSProxyPath(base, "https://"+testOSSHostPath+testOSSBasePath+"/linde/a.png"); valid {
				t.Fatal("基准非法时应拒绝")
			}
		})
	}
}

// TestDiagnosisOSSProxyPath_BaseWithSubpath 基准自带子路径时按它做前缀（不写死常量）。
func TestDiagnosisOSSProxyPath_BaseWithSubpath(t *testing.T) {
	base := "https://" + testOSSHostPath + "/other_prefix"
	if got, valid := diagnosisOSSProxyPath(base, "https://"+testOSSHostPath+"/other_prefix/x/a.png"); !valid {
		t.Fatalf("应按基准前缀放行，got %q", got)
	}
	if _, valid := diagnosisOSSProxyPath(base, "https://"+testOSSHostPath+testOSSBasePath+"/x/a.png"); valid {
		t.Fatal("基准前缀之外的路径应拒绝")
	}
}

// TestDiagnosisProxyOpenOSSImage 回源：路径折叠正确、Content-Type 透出、非法子路径不发请求。
func TestDiagnosisProxyOpenOSSImage(t *testing.T) {
	server, lastPath := newOSSFake(t)
	// 假 OSS 是 http，直接构造 service 让它指向它（白名单判定已在上面单独覆盖）
	svc := &DiagnosisProxyService{
		ossBase: server.URL + testOSSBasePath,
		client:  server.Client(),
		logger:  zap.NewNop(),
	}

	body, ct, err := svc.OpenOSSImage(context.Background(), "linde/Linde_A-005-01__image.png")
	if err != nil {
		t.Fatalf("回源失败: %v", err)
	}
	defer func() { _ = body.Close() }()
	raw, _ := io.ReadAll(body)
	if !strings.HasPrefix(string(raw), "\x89PNG") {
		t.Fatalf("字节不符: %q", raw)
	}
	if ct != "image/png" {
		t.Fatalf("Content-Type 应透出，got %q", ct)
	}
	if want := testOSSBasePath + "/linde/Linde_A-005-01__image.png"; *lastPath != want {
		t.Fatalf("上游路径不符:\n got=%q\nwant=%q", *lastPath, want)
	}

	// 中文目录名：入参是转义形态（handler 的 *filepath 带的是原始 path）。
	// 上游 req.URL.Path 是 Go 解码后的形态，故这里比解码值（真正的转义发生在
	// resolveOSSSubpath 的出参上，见 TestResolveOSSSubpathEscapes）。
	*lastPath = ""
	if _, _, err := svc.OpenOSSImage(context.Background(), "images_all/%E5%88%B6%E5%8A%A8.png"); err != nil {
		t.Fatalf("中文目录回源失败: %v", err)
	}
	if want := testOSSBasePath + "/images_all/制动.png"; *lastPath != want {
		t.Fatalf("中文目录上游路径不符:\n got=%q\nwant=%q", *lastPath, want)
	}
}

// TestResolveOSSSubpathEscapes 出参必须是逐段转义形态（交给上游的恒为合法 URL 路径）。
func TestResolveOSSSubpathEscapes(t *testing.T) {
	ok := []struct {
		name string
		in   string
		want string
	}{
		{"中文目录", "images_all/制动系统异响.png", "images_all/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F%E5%BC%82%E5%93%8D.png"},
		{"已是转义形态解一次再转义", "images_all/%E5%88%B6%E5%8A%A8.png", "images_all/%E5%88%B6%E5%8A%A8.png"},
		{"普通名", "linde/Linde_A-005-01__image.png", "linde/Linde_A-005-01__image.png"},
		{"前导斜杠被剥", "/linde/a.png", "linde/a.png"},
	}
	for _, c := range ok {
		t.Run(c.name, func(t *testing.T) {
			got, valid := resolveOSSSubpath(c.in)
			if !valid || got != c.want {
				t.Fatalf("got %q valid=%v want %q", got, valid, c.want)
			}
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"空", ""},
		{"只有斜杠", "/"},
		{"穿越", "linde/../../etc/passwd.png"},
		{"编码穿越", "linde/%2e%2e/%2e%2e/etc/passwd.png"},
		{"二次编码残留 %", "linde/%252e%252e/a.png"},
		{"pdf 不放行", "linde/a.pdf"},
		{"无扩展名", "linde/a"},
		// 已知代价（ADR-0091 决策 2 复用 staticSegmentPattern）：原名里的括号/空格不在
		// 段白名单字符集内，这类图会被降级成文字。安全优先，记为已知损失而不是漏洞。
		{"原名含括号", "linde/型号__原名(1).jpg"},
		{"原名含空格", "images_all/带 空格.png"},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			if got, valid := resolveOSSSubpath(c.in); valid {
				t.Fatalf("应拒绝，got %q", got)
			}
		})
	}
}

// TestDiagnosisProxyOpenOSSImage_Rejects 非法子路径与未配置：不发请求、给中文错误。
func TestDiagnosisProxyOpenOSSImage_Rejects(t *testing.T) {
	server, lastPath := newOSSFake(t)
	svc := &DiagnosisProxyService{
		ossBase: server.URL + testOSSBasePath,
		client:  server.Client(),
		logger:  zap.NewNop(),
	}
	for _, bad := range []string{"", "/", "linde/a.pdf", "linde/../../etc/passwd.png", "linde/%2e%2e/a.png", "linde/a"} {
		*lastPath = ""
		if _, _, err := svc.OpenOSSImage(context.Background(), bad); err == nil {
			t.Fatalf("非法子路径应拒绝: %q", bad)
		}
		if *lastPath != "" {
			t.Fatalf("非法子路径不应发上游请求: %q", bad)
		}
	}

	unconf := &DiagnosisProxyService{logger: zap.NewNop()}
	if _, _, err := unconf.OpenOSSImage(context.Background(), "linde/a.png"); err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("未配置应给友好错误，got %v", err)
	}
}

// TestDiagnosisProxyOpenOSSImage_UpstreamFail 上游非 200 与网络错误都收成中文人话。
func TestDiagnosisProxyOpenOSSImage_UpstreamFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	svc := &DiagnosisProxyService{
		ossBase: server.URL + testOSSBasePath,
		client:  server.Client(),
		logger:  zap.NewNop(),
	}
	if _, _, err := svc.OpenOSSImage(context.Background(), "linde/a.png"); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("非 200 应报不存在，got %v", err)
	}
}

// TestDiagnosisProxyNoRedirect 302 不得被跟随（ADR-0091 决策 8：白名单只对声明的 host 成立）。
func TestDiagnosisProxyNoRedirect(t *testing.T) {
	// 目标服务器：若被跟随，就会在这里留下命中记录
	var followed bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		followed = true
		_, _ = w.Write([]byte("secret"))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/x.png", http.StatusFound)
	}))
	defer redirector.Close()

	svc := &DiagnosisProxyService{
		ossBase: redirector.URL + testOSSBasePath,
		client: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		logger: zap.NewNop(),
	}
	if _, _, err := svc.OpenOSSImage(context.Background(), "linde/a.png"); err == nil {
		t.Fatal("302 应作为非 200 失败，而不是被跟随")
	}
	if followed {
		t.Fatal("不跟随重定向：目标服务器不应被命中")
	}
}

// TestDiagnosisProxyOpenOSSImageEscapesConsistency 归一产物必须是本站路由参数能解回去的形状：
// adapter 拼出的路径去掉前缀后，喂回 OpenOSSImage 得到同一上游路径（编码往返闭环）。
func TestDiagnosisProxyOpenOSSImageEscapesConsistency(t *testing.T) {
	base := "https://" + testOSSHostPath + testOSSBasePath
	for _, name := range []string{
		"linde/Linde_A-005-01__image.png",
		"images_all/制动系统异响.png",
		"linde/image-20260824100446677.jpg",
	} {
		raw := base + "/" + name
		proxyPath, ok := diagnosisOSSProxyPath(base, raw)
		if !ok {
			t.Fatalf("应放行: %q", name)
		}
		sub := strings.TrimPrefix(proxyPath, diagnosisImageRoute)
		back, ok := diagnosisOSSProxyPath(base, base+"/"+sub)
		if !ok || back != proxyPath {
			t.Fatalf("编码往返不闭环:\n name=%q\n proxy=%q\n back=%q ok=%v", name, proxyPath, back, ok)
		}
		// 路由参数（转义形态）经 resolveOSSSubpath 应还原成同一上游路径
		upstream, ok := resolveOSSSubpath(sub)
		if !ok {
			t.Fatalf("路由参数应可解析: %q", sub)
		}
		if want := strings.TrimPrefix(proxyPath, diagnosisImageRoute); upstream != want {
			t.Fatalf("子路径还原不符:\n got=%q\nwant=%q", upstream, want)
		}
	}
}
