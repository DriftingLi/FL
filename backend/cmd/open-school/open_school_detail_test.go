package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestDetailRendersEntitlementAndChapters 覆盖第 3 项前半：详情渲染权益与章节元数据；
// points_price 缺省（nil）时显示免费。
func TestDetailRendersEntitlementAndChapters(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["11"] = detailFixture()
	application, _ := newTestApp(t, fake)

	rec := get(t, application.routes(), "/courses/11", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("详情状态码 = %d，期望 200", rec.Code)
	}
	body := rec.Body.String()
	mustContain(t, body,
		"叉车安全操作入门",
		"100 积分解锁",
		"进入学习",
		"第一章 班前检查", // 章节元数据
		"第二章 行驶与堆垛",
		"图文", // content_type → 中文标签
		"视频",
		"12 分钟", // duration
		"安全操作",  // 分类
	)
	// duration=0 的章节不渲染「0 分钟」
	if strings.Contains(body, "0 分钟") {
		t.Errorf("duration=0 的章节渲染出了「0 分钟」")
	}

	// 缺省 points_price → 免费
	free := detailFixture()
	delete(free, "points_price")
	fake.detail["12"] = free
	rec = get(t, application.routes(), "/courses/12", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("免费课状态码 = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "免费") {
		t.Errorf("points_price 缺省时页面未显示「免费」\n%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "0 积分解锁") {
		t.Errorf("points_price 缺省时错误渲染成「0 积分解锁」")
	}
}

// TestDetailNotFound 覆盖第 4 项：上游 404 → 「课程不存在」页（状态 404）+ 返回目录链接。
func TestDetailNotFound(t *testing.T) {
	fake := newFakeUpstream(t)
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	for _, target := range []string{"/courses/999", "/courses/abc", "/courses/0", "/courses/-3"} {
		rec := get(t, handler, target, nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s 状态码 = %d，期望 404", target, rec.Code)
			continue
		}
		body := rec.Body.String()
		mustContain(t, body, "课程不存在", `href="/courses"`)
	}
}

// TestUpstreamFailureRendersErrorState 覆盖第 5 项：上游 500 / 连接失败 → 错误态
// （非 404、非空态）。
func TestUpstreamFailureRendersErrorState(t *testing.T) {
	t.Run("详情上游 500", func(t *testing.T) {
		fake := newFakeUpstream(t)
		fake.detailStatus = http.StatusInternalServerError
		application, _ := newTestApp(t, fake)

		rec := get(t, application.routes(), "/courses/11", nil)
		if rec.Code == http.StatusNotFound {
			t.Fatalf("上游 500 被渲染成 404（状态码 %d）", rec.Code)
		}
		if rec.Code != http.StatusBadGateway {
			t.Errorf("状态码 = %d，期望 502", rec.Code)
		}
		body := rec.Body.String()
		mustContain(t, body, "课程暂时加载不出来", "重试")
		// 错误态不能长得像「没这门课」
		if strings.Contains(body, "课程不存在") {
			t.Errorf("错误态里出现了「课程不存在」，会被误读成 404")
		}
	})

	t.Run("目录上游 500", func(t *testing.T) {
		fake := newFakeUpstream(t)
		fake.coursesStatus = http.StatusInternalServerError
		fake.categoriesStatus = http.StatusInternalServerError
		application, _ := newTestApp(t, fake)

		rec := get(t, application.routes(), "/courses", nil)
		body := rec.Body.String()
		mustContain(t, body, "课程列表暂时加载不出来", "重试")
		// 错误态不能渲染成空态文案
		if strings.Contains(body, "没有符合条件的课程") {
			t.Errorf("上游错误被渲染成空态（「没有符合条件的课程」）")
		}
	})

	t.Run("目录连接被断开", func(t *testing.T) {
		fake := newFakeUpstream(t)
		fake.dropConnection = true
		application, _ := newTestApp(t, fake)

		rec := get(t, application.routes(), "/courses", nil)
		body := rec.Body.String()
		mustContain(t, body, "课程列表暂时加载不出来", "重试")
		if rec.Code == http.StatusNotFound {
			t.Errorf("网络错误被渲染成 404")
		}
	})

	t.Run("上游成功信封但 data 为 null", func(t *testing.T) {
		fake := newFakeUpstream(t)
		fake.detail["11"] = nil
		fake.detailStatus = 0
		// 直接构造一个 data=null 的 200 响应：detail 为 nil 时假上游回 404，
		// 所以这里用 courses 路径模拟契约漂移。
		fake.courses["*"] = nil
		application, _ := newTestApp(t, fake)

		rec := get(t, application.routes(), "/courses", nil)
		mustContain(t, rec.Body.String(), "课程列表暂时加载不出来")
	})
}

// TestShareMetadata 覆盖第 6 项：title/description/og:url 存在，且 og:url 不含筛选参数。
func TestShareMetadata(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["11"] = detailFixture()
	fake.categories = map[string]any{"categories": []map[string]any{
		{"id": 3, "name": "安全操作", "course_count": 9},
	}}
	fake.courses["*"] = catalogFixtures()
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	detail := get(t, handler, "/courses/11?back=%2Fcourses%3Fcategory_id%3D3", nil).Body.String()
	mustContain(t, detail,
		"<title>叉车安全操作入门 · 叉车维修培训</title>",
		`<meta name="description" content="从班前检查到停车落锁的完整流程。">`,
		`<meta property="og:title" content="叉车安全操作入门">`,
		`<meta property="og:url" content="/courses/11">`,
		`<meta property="og:image" content="https://cdn.example.com/c11.jpg">`,
		`<meta property="og:description"`,
		`<meta property="og:type" content="website">`,
	)
	// 分享地址不带筛选参数
	if strings.Contains(detail, `og:url" content="/courses/11?`) {
		t.Errorf("og:url 带了查询串：\n%s", detail)
	}

	// 目录页有自身 title / description，og:url 不带筛选
	directory := get(t, handler, "/?category_id=3&q=%E5%AE%89%E5%85%A8", nil).Body.String()
	mustContain(t, directory,
		"<title>全部课程 · 叉车维修培训</title>",
		`<meta name="description"`,
		`<meta property="og:url" content="/courses">`,
	)
	if strings.Contains(directory, `og:url" content="/courses?`) {
		t.Errorf("目录页 og:url 带了筛选参数")
	}

	// 无封面时不出 og:image
	free := detailFixture()
	free["cover_image"] = ""
	delete(free, "points_price")
	fake.detail["12"] = free
	noCover := get(t, handler, "/courses/12", nil).Body.String()
	if strings.Contains(noCover, `property="og:image"`) {
		t.Errorf("没有 cover_image 时仍输出了 og:image")
	}
}

// TestBackParameterValidation 覆盖第 7 项：back 只接受本站本地地址，外部地址被拒。
func TestBackParameterValidation(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["11"] = detailFixture()
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	// 放行：本站目录地址（带回筛选）
	ok := get(t, handler, "/courses/11?back="+url.QueryEscape("/courses?category_id=3&q=安全&page=2"), nil).Body.String()
	mustContain(t, ok, `href="/courses?category_id=3&amp;page=2&amp;q=%E5%AE%89%E5%85%A8"`)

	// 拒绝的外部地址：都要退回 /courses
	rejected := []string{
		"https://evil.example.com/courses",
		"//evil.example.com/courses",
		"http://evil.example.com",
		"javascript:alert(1)",
		"/admin/users",
		"/healthz",
		"/courses/11",
		"\\\\evil.example.com",
		"/\\evil.example.com",
		"/courses#frag",
		"courses?q=x", // 相对路径但不是本站目录地址
	}
	for _, back := range rejected {
		body := get(t, handler, "/courses/11?back="+url.QueryEscape(back), nil).Body.String()
		if strings.Contains(body, "evil.example.com") {
			t.Errorf("back=%q 被放行到页面里", back)
		}
		mustContain(t, body, `<a href="/courses">← 返回课程目录</a>`)
	}
}

// TestLearnLink 覆盖第 8 项：学习链接指向 OPEN_SCHOOL_LEARN_BASE + /training/course/{id}。
func TestLearnLink(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["11"] = detailFixture()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	const learnBase = "http://training.localhost:18091"
	application := &app{
		upstream:  newUpstream(server.URL),
		renderer:  newRenderer(zap.NewNop()),
		learnBase: learnBase,
		log:       zap.NewNop(),
	}

	body := get(t, application.routes(), "/courses/11", nil).Body.String()
	mustContain(t, body, `id="start-learning" href="http://training.localhost:18091/training/course/11"`)
	// 不得从请求头推断：带一个伪造的 Host / X-Forwarded-Host 也不该改变链接
	req := httptest.NewRequest(http.MethodGet, "/courses/11", nil)
	req.Host = "evil.example.com"
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	application.routes().ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "evil.example.com") {
		t.Errorf("学习链接被请求头影响：\n%s", rec.Body.String())
	}
	mustContain(t, rec.Body.String(), "http://training.localhost:18091/training/course/11")
}

// TestNoPrivateContent 覆盖第 9 项：页面不含章节正文/私有数据。
func TestNoPrivateContent(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["11"] = detailFixture()
	application, _ := newTestApp(t, fake)

	body := get(t, application.routes(), "/courses/11", nil).Body.String()
	leaks := []string{
		"私有正文-MUST-NOT-LEAK",
		"private-must-not-leak",
		"/static/uploads/",
		"file_url",
		"content_type",
		`"content":`,
	}
	for _, leak := range leaks {
		if strings.Contains(body, leak) {
			t.Errorf("页面泄漏私有数据 %q\n%s", leak, body)
		}
	}
	// 章节元数据仍然要在（锁的是「不泄漏」，不是「不显示章节」）
	mustContain(t, body, "第一章 班前检查", "图文")
}

// TestHealthz 健康检查只报自身、不探测上游。
func TestHealthz(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.coursesStatus = http.StatusInternalServerError // 上游坏掉也不影响 healthz
	application, _ := newTestApp(t, fake)

	rec := get(t, application.routes(), "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz 状态码 = %d，期望 200", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Errorf("healthz 体 = %q，期望 ok", got)
	}
	if len(fake.requests) != 0 {
		t.Errorf("healthz 探测了上游 %d 次，期望 0", len(fake.requests))
	}
}

// TestStaticAssetsEmbedded 静态资源本地内嵌：htmx 与许可证都从二进制里出。
func TestStaticAssetsEmbedded(t *testing.T) {
	application, _ := newTestApp(t, newFakeUpstream(t))
	handler := application.routes()

	js := get(t, handler, pathHTMX, nil)
	if js.Code != http.StatusOK {
		t.Fatalf("htmx 状态码 = %d", js.Code)
	}
	if !strings.Contains(js.Body.String(), "htmx") {
		t.Errorf("htmx 响应体不含 htmx")
	}
	if ct := js.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("htmx Content-Type = %q", ct)
	}

	css := get(t, handler, pathStyleCSS, nil)
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), ":root") {
		t.Errorf("样式表异常：状态 %d", css.Code)
	}

	license := get(t, handler, "/assets/htmx-LICENSE.txt", nil)
	if license.Code != http.StatusOK || !strings.Contains(license.Body.String(), "Zero-Clause BSD") {
		t.Errorf("许可证不可达：状态 %d", license.Code)
	}

	// 页面不引外网资源（CDN 一律不许）
	page := get(t, handler, "/courses", nil).Body.String()
	for _, src := range []string{"//unpkg.com", "cdn.jsdelivr", "//cdnjs", "https://cdn."} {
		if strings.Contains(page, src) {
			t.Errorf("页面引用了外网资源 %q", src)
		}
	}
	mustContain(t, page, `src="/assets/htmx.min.js"`, `/assets/style.css`)
}
