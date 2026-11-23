package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// 本文件的测试全部走 httptest + 假上游：不起真后端、不连库。
// 假上游按 #2753 契约的三条路径回信封，并在需要时故意多塞私有字段
// （content / file_url），用来锁「页面不渲染正文与文件地址」。

// fakeUpstream 是可编程的假后端。
type fakeUpstream struct {
	t *testing.T

	categories map[string]any
	// courses 按查询串决定返回什么（键是排序后的 rawQuery），未命中就用 defaultCourses。
	courses map[string]any
	// detail 按课程 ID 决定返回什么；值为 nil 表示 404。
	detail map[string]any

	// 非 200 的响应体
	categoriesStatus int
	coursesStatus    int
	detailStatus     int
	// questionBanks 是题库目录的 data 段（#2760）。
	questionBanks       map[string]any
	questionBanksStatus int
	// 连接失败模拟：handler 直接 panic 掉连接（用 CloseClientConnection 更贴近，见下）
	dropConnection bool

	// 记录收到的请求（供「关键词/分类/分页透传」断言）
	requests []*http.Request
	queries  []url.Values
}

func newFakeUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	return &fakeUpstream{
		t:          t,
		categories: map[string]any{},
		courses:    map[string]any{},
		detail:     map[string]any{},
	}
}

// envelope 生成契约里的成功信封。
func successEnvelope(data any) map[string]any {
	return map[string]any{"code": 200, "message": "success", "data": data}
}

func (f *fakeUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, r)
	f.queries = append(f.queries, r.URL.Query())

	switch {
	case r.URL.Path == pathCategories:
		if f.categoriesStatus != 0 {
			w.WriteHeader(f.categoriesStatus)
			_, _ = w.Write([]byte(`{"code":500,"message":"boom","data":null}`))
			return
		}
		writeJSON(w, successEnvelope(f.categories))
	case r.URL.Path == pathCourses:
		if f.coursesStatus != 0 {
			w.WriteHeader(f.coursesStatus)
			_, _ = w.Write([]byte(`{"code":500,"message":"boom","data":null}`))
			return
		}
		if f.dropConnection {
			// 直接关掉底层连接：客户端会拿到 EOF，等价于网络错误。
			if hijacker, ok := w.(http.Hijacker); ok {
				if conn, _, err := hijacker.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
		}
		data, ok := f.courses[r.URL.RawQuery]
		if !ok {
			data = f.courses["*"]
		}
		writeJSON(w, successEnvelope(data))
	case strings.HasPrefix(r.URL.Path, pathCourses+"/"):
		id := strings.TrimPrefix(r.URL.Path, pathCourses+"/")
		if f.detailStatus != 0 {
			w.WriteHeader(f.detailStatus)
			_, _ = w.Write([]byte(`{"code":500,"message":"boom","data":null}`))
			return
		}
		data, ok := f.detail[id]
		if !ok || data == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":404,"message":"课程不存在","data":null}`))
			return
		}
		writeJSON(w, successEnvelope(data))
	case r.URL.Path == pathQuestionBanks:
		if f.questionBanksStatus != 0 {
			w.WriteHeader(f.questionBanksStatus)
			_, _ = w.Write([]byte(`{"code":500,"message":"boom","data":null}`))
			return
		}
		if f.questionBanks == nil {
			// 与真上游同形：没有合格题库时 data 是 {"question_banks": []}，不是 null
			writeJSON(w, successEnvelope(map[string]any{"question_banks": []any{}}))
			return
		}
		writeJSON(w, successEnvelope(f.questionBanks))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"no route","data":null}`))
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// newTestApp 装配一个指向假上游的应用。
func newTestApp(t *testing.T, fake http.Handler) (*app, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	application := &app{
		upstream:  newUpstream(server.URL),
		renderer:  newRenderer(zap.NewNop()),
		learnBase: defaultLearnBase,
		log:       zap.NewNop(),
	}
	return application, server
}

// get 发一个 GET 并返回 recorder。
func get(t *testing.T, h http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// 目录夹具：两个分类、三门课（含免费与积分课）。
func catalogFixtures() map[string]any {
	return map[string]any{
		"courses": []map[string]any{
			{
				"course_id": 11, "name": "叉车安全操作入门",
				"description":  "从班前检查到停车落锁的完整流程。",
				"cover_image":  "https://cdn.example.com/c11.jpg",
				"category":     map[string]any{"id": 3, "name": "安全操作"},
				"points_price": 100,
			},
			{
				"course_id": 12, "name": "叉车电控系统检修",
				"description": "电控故障的排查顺序与工量具。",
				"cover_image": "",
				"category":    map[string]any{"id": 4, "name": "维修技术"},
				// points_price 缺省：免费
			},
		},
		"page": 1, "pages": 2, "total": 14,
	}
}

// detailFixture 含私有字段 content / file_url：页面必须一个都不渲染。
func detailFixture() map[string]any {
	return map[string]any{
		"course_id": 11, "name": "叉车安全操作入门",
		"description":   "从班前检查到停车落锁的完整流程。",
		"cover_image":   "https://cdn.example.com/c11.jpg",
		"category":      map[string]any{"id": 3, "name": "安全操作"},
		"points_price":  100,
		"chapter_count": 2,
		"chapters": []map[string]any{
			{"chapter_id": 101, "title": "第一章 班前检查", "order_num": 1,
				"content_type": "text", "duration": 12,
				// 私有字段：正文与文件地址
				"content":  "私有正文-MUST-NOT-LEAK",
				"file_url": "/static/uploads/private-must-not-leak.mp4",
			},
			{"chapter_id": 102, "title": "第二章 行驶与堆垛", "order_num": 2,
				"content_type": "video", "duration": 0},
		},
	}
}

// TestDirectoryPageAndFragment 覆盖第 1 项：整页含分类选项与课程卡片；
// HX-Request 只回片段（不含 <html>）。
func TestDirectoryPageAndFragment(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{
		{"id": 3, "name": "安全操作", "course_count": 9},
		{"id": 4, "name": "维修技术", "course_count": 5},
	}}
	fake.courses["*"] = catalogFixtures()
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	full := get(t, handler, "/", nil)
	if full.Code != http.StatusOK {
		t.Fatalf("整页状态码 = %d，期望 200", full.Code)
	}
	body := full.Body.String()
	mustContain(t, body, "<!doctype html>", "<html", "</html>", `id="results"`)
	// 分类选项
	mustContain(t, body, "安全操作（9）", "维修技术（5）", "全部分类")
	// 课程卡片
	mustContain(t, body, "叉车安全操作入门", "叉车电控系统检修", "100 积分解锁", "免费")
	mustContain(t, body, "/courses/11?back=", "/courses/12?back=")
	// 导览语
	mustContain(t, body, "<h1>全部课程</h1>")

	fragment := get(t, handler, "/courses?q=%E5%AE%89%E5%85%A8", map[string]string{"HX-Request": "true"})
	if fragment.Code != http.StatusOK {
		t.Fatalf("片段状态码 = %d，期望 200", fragment.Code)
	}
	frag := fragment.Body.String()
	if strings.Contains(strings.ToLower(frag), "<html") {
		t.Errorf("片段里出现了 <html>：\n%s", frag)
	}
	if strings.Contains(strings.ToLower(frag), "<!doctype") || strings.Contains(frag, "<body") {
		t.Errorf("片段里出现了文档骨架：\n%s", frag)
	}
	mustContain(t, frag, `id="results"`, "叉车安全操作入门")
	// 片段响应头必须声明 Vary：同一 URL 会因请求头返回两种体。
	if vary := fragment.Header().Get("Vary"); !strings.Contains(vary, "HX-Request") {
		t.Errorf("片段的 Vary = %q，应含 HX-Request", vary)
	}
}

// TestHistoryRestoreReturnsFullPage 覆盖「历史恢复请求要返回整页，不能返回片段」。
func TestHistoryRestoreReturnsFullPage(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{}}
	fake.courses["*"] = catalogFixtures()
	application, _ := newTestApp(t, fake)

	// htmx 打开 historyRestoreAsHxRequest 时会同时带两个头；被配置改坏也不该回片段。
	rec := get(t, application.routes(), "/courses?category_id=3", map[string]string{
		"HX-Request":                 "true",
		"HX-History-Restore-Request": "true",
	})
	body := rec.Body.String()
	mustContain(t, body, "<!doctype html>", "</html>")
}

// TestQueryPassthrough 覆盖第 2 项：关键词与分类进入上游查询串；分页参数透传。
func TestQueryPassthrough(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{}}
	fake.courses["*"] = map[string]any{"courses": []map[string]any{}, "page": 3, "pages": 5, "total": 50}
	application, _ := newTestApp(t, fake)

	rec := get(t, application.routes(), "/courses?q=%E7%94%B5%E6%8E%A7&category_id=4&page=3&page_size=12", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rec.Code)
	}

	var got url.Values
	for i, r := range fake.requests {
		if r.URL.Path == pathCourses {
			got = fake.queries[i]
		}
	}
	if got == nil {
		t.Fatal("假上游没收到 /api/public-school/courses 请求")
	}
	// 关键词必须原样透传（由服务端匹配，应用侧不过滤整页）。
	if got.Get("q") != "电控" {
		t.Errorf("上游 q = %q，期望 电控", got.Get("q"))
	}
	if got.Get("category_id") != "4" {
		t.Errorf("上游 category_id = %q，期望 4", got.Get("category_id"))
	}
	if got.Get("page") != "3" {
		t.Errorf("上游 page = %q，期望 3", got.Get("page"))
	}
	if got.Get("page_size") != "12" {
		t.Errorf("上游 page_size = %q，期望 12", got.Get("page_size"))
	}

	// 应用侧不得自己过滤整页：上游返回两条、关键词对不上任何一条，也要照原样渲染
	// （筛选归服务端，应用侧再筛一次就会漏结果）。
	fake.courses["*"] = catalogFixtures()
	rec = get(t, application.routes(), "/courses?q=zzz-no-match-in-page", nil)
	if !strings.Contains(rec.Body.String(), "叉车安全操作入门") {
		t.Errorf("应用侧疑似自己过滤了整页：上游给了两条，页面没有渲染\n%s", rec.Body.String())
	}
}

// TestPagingClampAndLinks 覆盖分页链接带完整筛选态、以及钳制用户手改的非法值。
func TestPagingClampAndLinks(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{}}
	// 有课才谈得上翻页：空列表时页面走空态，不渲染 pager（这是有意的口径）。
	page := catalogFixtures()
	page["page"] = 2
	page["pages"] = 4
	page["total"] = 40
	fake.courses["*"] = page
	application, _ := newTestApp(t, fake)

	rec := get(t, application.routes(), "/courses?q=%E5%AE%89%E5%85%A8&category_id=3&page=2", nil)
	body := rec.Body.String()
	// 上一页/下一页都要保住 q 与 category_id
	mustContain(t, body, "category_id=3", "page=1", "page=3", "第 2 / 4 页")

	// page_size 超上限 → 回退默认 12，且不会把 99999 传给上游
	get(t, application.routes(), "/courses?page_size=99999", nil)
	last := fake.queries[len(fake.queries)-2] // 最后一条是 categories
	if last.Get("page_size") != "12" {
		t.Errorf("page_size 钳制失败：上游收到 %q，期望 12", last.Get("page_size"))
	}
	// page=abc → 回退 1
	get(t, application.routes(), "/courses?page=abc", nil)
	if got := fake.queries[len(fake.queries)-2].Get("page"); got != "1" {
		t.Errorf("page 回退失败：上游收到 %q，期望 1", got)
	}
}

// TestCategoryOnlyFragmentUpdate 覆盖「只改 select、不提交表单」也要出片段。
//
// 页面用 hx-trigger="submit, change from:find select, input changed delay:400ms from:find input[name=q]"
// 触发；浏览器脚本（含 E/F 联调的验收脚本）可能走 select.form.requestSubmit()，
// 也可能只改值再看片段刷新——两条路对服务端是同一个形状：带 HX-Request 的 GET。
// 这条用例锁「服务端只凭 HX-Request + URL 上的 category_id 就能出正确的片段」。
func TestCategoryOnlyFragmentUpdate(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{
		{"id": 3, "name": "安全操作", "course_count": 9},
	}}
	fake.courses["*"] = catalogFixtures()
	application, _ := newTestApp(t, fake)

	// 模拟「只改 select」：URL 上只有 category_id，没有 q、没有 page。
	rec := get(t, application.routes(), "/courses?category_id=3", map[string]string{"HX-Request": "true"})
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d，期望 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(strings.ToLower(body), "<html") {
		t.Errorf("select 变更回了整页而不是片段")
	}
	mustContain(t, body, `id="results"`, "叉车安全操作入门")
	// 地址栏用服务端规范化后的地址：空壳参数不该被 push 进历史
	if got := rec.Header().Get("HX-Push-Url"); got != "/courses?category_id=3" {
		t.Errorf("HX-Push-Url = %q，期望 /courses?category_id=3", got)
	}
	// 分类确实进了上游查询串
	var upstreamQuery url.Values
	for i, rq := range fake.requests {
		if rq.URL.Path == pathCourses {
			upstreamQuery = fake.queries[i]
		}
	}
	if upstreamQuery.Get("category_id") != "3" {
		t.Errorf("上游 category_id = %q，期望 3", upstreamQuery.Get("category_id"))
	}
	// 表单会把空字段一并提交：category_id=&page=1 这类空壳也要被规范化掉
	rec = get(t, application.routes(), "/courses?category_id=&page=1&q=", map[string]string{"HX-Request": "true"})
	if got := rec.Header().Get("HX-Push-Url"); got != "/courses" {
		t.Errorf("空壳参数未规范化：HX-Push-Url = %q，期望 /courses", got)
	}
}

// TestDirectoryCardsCarryFilterBack 锁定跳转链：目录页渲染时 Back 必须被赋成
// **当前筛选视图**的地址，卡片链接的 back 才不是空串——这是「点进详情再返回
// 恢复原筛选」整条链的前半段（后半段见 TestBackParameterValidation）。
func TestDirectoryCardsCarryFilterBack(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.categories = map[string]any{"categories": []map[string]any{}}
	page := catalogFixtures()
	page["page"] = 2
	page["pages"] = 4
	fake.courses["*"] = page
	application, _ := newTestApp(t, fake)

	// 断言方式：按浏览器的方式解析卡片 href，再解 back —— 而不是比字面量。
	// 模板的 urlquery 会把 back 整体再转义一次（空格变 %25），字面量比对既脆
	// 又测不到真正关心的东西：浏览器点进去之后，服务端收到的 back 是什么。
	body := get(t, application.routes(), "/courses?category_id=3&q=%E5%AE%89%E5%85%A8&page=2", nil).Body.String()
	back := cardBackHref(t, body, 11)
	if back == "" {
		t.Fatalf("卡片 11 的 back 是空串（directoryView.Back 没赋值？）\n%s", body)
	}
	inner, err := url.Parse(back)
	if err != nil {
		t.Fatalf("back 解析失败：%v", err)
	}
	if inner.Path != "/courses" {
		t.Errorf("back 路径 = %q，期望 /courses", inner.Path)
	}
	q := inner.Query()
	if q.Get("category_id") != "3" || q.Get("q") != "安全" || q.Get("page") != "2" {
		t.Errorf("back 丢了筛选态：category_id=%q q=%q page=%q",
			q.Get("category_id"), q.Get("q"), q.Get("page"))
	}

	// 越界页码被钳制后，back 也要跟着用实际页码，而不是请求里的 999
	body = get(t, application.routes(), "/courses?category_id=3&page=999", nil).Body.String()
	clamped := cardBackHref(t, body, 11)
	if strings.Contains(clamped, "page=999") {
		t.Errorf("back 沿用了越界页码 999：%q", clamped)
	}
	inner, err = url.Parse(clamped)
	if err != nil {
		t.Fatalf("钳制后的 back 解析失败：%v", err)
	}
	if got := inner.Query().Get("page"); got != "2" {
		t.Errorf("back 未回写钳制后的实际页码：page=%q，期望 2", got)
	}
}

// cardBackHref 从渲染出的 HTML 里取出某门课卡片链接的 back 值（浏览器视角：
// 先解 &amp; 再解 href 查询串），拿不到返回空串。
func cardBackHref(t *testing.T, body string, courseID int) string {
	t.Helper()
	prefix := fmt.Sprintf(`/courses/%d?back=`, courseID)
	start := strings.Index(body, prefix)
	if start < 0 {
		return ""
	}
	rest := body[start+len(prefix):]
	end := strings.IndexAny(rest, `"`)
	if end < 0 {
		return ""
	}
	// 模板输出的 &amp; 是 HTML 实体，浏览器会先还原成 &；这里照做。
	raw := strings.ReplaceAll(rest[:end], "&amp;", "&")
	u, err := url.Parse("/x?back=" + raw)
	if err != nil {
		return ""
	}
	return u.Query().Get("back")
}

// mustContain 断言 body 含全部子串。
func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("页面缺少 %q", want)
		}
	}
}
