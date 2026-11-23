// 类型分流与题库目录的用例（#2760）。
//
// 两组判据，各自对应页面上的一个承诺：
//
// 类型分流：
//  1. 三个目录入口都能打开，页头高亮当前类
//  2. 类型**从路径决定**并透传给上游（/video 发出的请求里必须有 course_type=video）
//  3. 类型页里的筛选/翻页/清筛选链接都留在这个类型里（不掉回「全部课程」）
//  4. 课程卡与详情页带类型徽标（「全部课程」里也看得出形式）
//  5. 详情页的「返回目录」能回到**原类型页**
//
// 题库目录：
//  6. 按证件渲染题量 + 题型分布 + 去练习链接（带预选分类）
//  7. 空题库是空态，上游故障是错误态（两者必须可区分）
//  8. 页面不渲染任何题目内容（只承诺汇总）
package main

import (
	"strings"
	"testing"
)

// ---- 类型分流 ----

// TestCourseTypeDirectoriesRender 三个目录入口都 200，且高亮各自那一类。
func TestCourseTypeDirectoriesRender(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.courses["*"] = map[string]any{"courses": []any{}, "page": 1, "pages": 0, "total": 0}
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	for _, tc := range []struct {
		path     string
		wantHead string
		// 该类在 typenav 里应是「当前项」（span），不是链接
		wantCurrent string
	}{
		{"/courses", "全部课程", "全部课程"},
		{"/video", "视频课程", "视频课程"},
		{"/illustrated", "图文课程", "图文课程"},
	} {
		rec := get(t, handler, tc.path, nil)
		if rec.Code != 200 {
			t.Fatalf("%s 状态码 = %d: %s", tc.path, rec.Code, rec.Body)
		}
		body := rec.Body.String()
		// 页头高亮：当前类必须是 <span aria-current="page">，不是可点链接
		if !strings.Contains(body, `<span class="typenav-item typenav-current" aria-current="page">`+tc.wantCurrent+`</span>`) {
			t.Errorf("%s 的类型导航没有把「%s」标成当前项:\n%s", tc.path, tc.wantCurrent, body)
		}
	}
}

// TestCourseTypePassedToUpstream 类型从路径解析并透传给上游（含分类选项，见下一条）。
func TestCourseTypePassedToUpstream(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.courses["*"] = map[string]any{"courses": []any{}, "page": 1, "pages": 0, "total": 0}
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	// /video ⇒ courses 与 categories 两条请求都要带 course_type=video
	fake.queries = nil
	get(t, handler, "/video", nil)
	var sawCourses, sawCategories bool
	for i, req := range fake.requests {
		switch req.URL.Path {
		case pathCourses:
			sawCourses = true
			if got := fake.queries[i].Get("course_type"); got != "video" {
				t.Errorf("目录请求 course_type = %q, want video", got)
			}
		case pathCategories:
			sawCategories = true
			// 分类选项也必须带上类型：类型页的证件下拉只该列该类型有公开课的分类，
			// 不带就会拿到「全部课程数」，下拉里写 6、点进去只有 2（#2760 US-4）。
			if got := fake.queries[i].Get("course_type"); got != "video" {
				t.Errorf("分类请求 course_type = %q, want video", got)
			}
		}
	}
	if !sawCourses || !sawCategories {
		t.Fatalf("没有打到两条上游（courses=%v categories=%v）", sawCourses, sawCategories)
	}

	// /courses（全部课程）⇒ **不带** course_type（省略即全部，契约口径）
	fake.requests, fake.queries = nil, nil
	get(t, handler, "/courses", nil)
	for i, req := range fake.requests {
		if req.URL.Path == pathCourses || req.URL.Path == pathCategories {
			if _, ok := fake.queries[i]["course_type"]; ok {
				t.Errorf("%s 不该带 course_type（全部课程 = 省略该参数）: %s", req.URL.Path, fake.queries[i].Encode())
			}
		}
	}
}

// TestCourseTypePageKeepsTypeInLinks 类型页里的筛选、翻页、清筛选都留在本类型（不掉回全部课程）。
func TestCourseTypePageKeepsTypeInLinks(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.courses["*"] = map[string]any{
		"courses": []map[string]any{{
			"course_id": 21, "name": "视频课", "course_type": "video",
			"category": map[string]any{"id": 3, "name": "安全操作"},
		}},
		"page": 2, "pages": 4, "total": 40,
	}
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	rec := get(t, handler, "/video?category_id=3&q=%E6%B6%B2%E5%8E%8B", nil)
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	// 表单 action 与 htmx 目标都要指向 /video（不是 /courses）
	if !strings.Contains(body, `action="/video"`) || !strings.Contains(body, `hx-get="/video"`) {
		t.Errorf("视频页的筛选表单没指向 /video:\n%s", body)
	}
	// 分页链接带类型路径
	if !strings.Contains(body, `href="/video?`) {
		t.Errorf("视频页的翻页链接没保留 /video 前缀:\n%s", body)
	}
	// 清除筛选回到 /video（不带查询串）
	if !strings.Contains(body, `href="/video"`) {
		t.Errorf("视频页的「清除筛选」没回到 /video:\n%s", body)
	}
	// 反例：不该出现裸的 /courses 作为筛选目标
	if strings.Contains(body, `action="/courses"`) || strings.Contains(body, `hx-get="/courses"`) {
		t.Errorf("视频页的筛选表单掉回了 /courses:\n%s", body)
	}
}

// TestCourseTypeBadgeOnCardsAndDetail 「全部课程」里的卡片与详情页都带类型徽标。
func TestCourseTypeBadgeOnCardsAndDetail(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.courses["*"] = map[string]any{
		"courses": []map[string]any{{
			"course_id": 21, "name": "视频课", "course_type": "video",
			"category": map[string]any{"id": 3, "name": "安全操作"},
		}, {
			"course_id": 22, "name": "图文课", "course_type": "illustrated",
			"category": map[string]any{"id": 3, "name": "安全操作"},
		}},
		"page": 1, "pages": 1, "total": 2,
	}
	fake.detail["21"] = map[string]any{
		"course_id": 21, "name": "视频课", "course_type": "video", "chapter_count": 1,
		"category": map[string]any{"id": 3, "name": "安全操作"},
		"chapters": []map[string]any{{"chapter_id": 1, "title": "第一章", "order_num": 1, "content_type": "video"}},
	}
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	// 目录卡：两类各一个徽标
	body := get(t, handler, "/courses", nil).Body.String()
	for _, want := range []string{"视频课程", "图文课程"} {
		if !strings.Contains(body, want) {
			t.Errorf("「全部课程」的卡片缺少类型徽标 %q:\n%s", want, body)
		}
	}
	// 徽标带类型类名（样式钩子）：两张卡要能长得不一样
	if !strings.Contains(body, "tag-type-video") || !strings.Contains(body, "tag-type-illustrated") {
		t.Errorf("类型徽标缺少样式钩子（tag-type-*）:\n%s", body)
	}

	// 详情页也要有
	detail := get(t, handler, "/courses/21", nil).Body.String()
	if !strings.Contains(detail, "tag-type-video") || !strings.Contains(detail, "视频课程") {
		t.Errorf("详情页缺少类型徽标:\n%s", detail)
	}
}

// TestDetailBackReturnsToTypeDirectory 详情页的「返回目录」能回到原类型页（#2760 US-6）。
func TestDetailBackReturnsToTypeDirectory(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.detail["21"] = map[string]any{
		"course_id": 21, "name": "视频课", "course_type": "video",
		"category": map[string]any{"id": 3, "name": "安全操作"},
		"chapters": []any{},
	}
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	// back 指向类型页 + 筛选
	rec := get(t, handler, "/courses/21?back=%2Fvideo%3Fcategory_id%3D3", nil)
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/video?category_id=3"`) {
		t.Errorf("详情页的「返回目录」没回到 /video?category_id=3:\n%s", body)
	}

	// 恶意/外部 back 仍被挡（类型页也是白名单，但白名单之外一律回默认目录）
	rec = get(t, handler, "/courses/21?back=http%3A%2F%2Fevil.com%2Fvideo", nil)
	body = rec.Body.String()
	if strings.Contains(body, "evil.com") {
		t.Errorf("外部 back 被渲染进了页面:\n%s", body)
	}
	if !strings.Contains(body, `href="/courses"`) {
		t.Errorf("被挡的 back 应回默认目录 /courses:\n%s", body)
	}
}

// ---- 题库目录 ----

// questionBankFixtures 两份证件题库：一个有分布、一个只有一种题型。
func questionBankFixtures() map[string]any {
	return map[string]any{
		"question_banks": []map[string]any{
			{
				"category_id": 1, "category_name": "叉车司机N1证", "total": 477,
				"by_type": map[string]int64{"single_choice": 189, "multi_choice": 100, "true_false": 188},
			},
			{
				"category_id": 2, "category_name": "低压电工证", "total": 2,
				"by_type": map[string]int64{"single_choice": 2},
			},
		},
	}
}

// TestQuestionBanksRendersAggregates 题库目录渲染题量、题型分布与带预选分类的练习链接。
func TestQuestionBanksRendersAggregates(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.questionBanks = questionBankFixtures()
	application, _ := newTestApp(t, fake)
	handler := application.routes()

	rec := get(t, handler, "/question-banks", nil)
	if rec.Code != 200 {
		t.Fatalf("状态码 = %d: %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()

	for _, want := range []string{
		"叉车司机N1证", "477 道题",
		// 题型按固定顺序摊平（不是 map 遍历的随机序）
		"单选题 189", "多选题 100", "判断题 188",
		"低压电工证", "2 道题",
		// 练习链接带预选分类（#2760 US-13）
		defaultLearnBase + "/training/question-bank?category_id=1",
		defaultLearnBase + "/training/question-bank?category_id=2",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("题库目录缺少 %q:\n%s", want, body)
		}
	}

	// 题型顺序：单选题必须在多选题之前（固定顺序，不是 map 序）
	si := strings.Index(body, "单选题 189")
	mi := strings.Index(body, "多选题 100")
	if si < 0 || mi < 0 || si > mi {
		t.Errorf("题型不是按固定顺序渲染（单选应在多选之前）: si=%d mi=%d", si, mi)
	}
}

// TestQuestionBanksKeepsUnknownQuestionTypes 未知题型也要出现在题型分布里
// （Total 含它，若分布缺项就自相矛盾）。
func TestQuestionBanksKeepsUnknownQuestionTypes(t *testing.T) {
	got := questionTypeCounts(map[string]int64{
		"single_choice": 2,
		"new_type_z":    3,
		"new_type_a":    1,
	})
	if len(got) != 3 {
		t.Fatalf("应保留全部 3 种题型（含 2 个未登记的），实得 %+v", got)
	}
	// 已登记的按固定顺序在前，未登记的按 key 字母序在后
	if got[0].Label != "单选题" || got[1].Label != "new_type_a" || got[2].Label != "new_type_z" {
		t.Fatalf("题型顺序错：%+v", got)
	}
	if got[1].Count != 1 || got[2].Count != 3 {
		t.Fatalf("未登记题型的题量错：%+v", got)
	}
}

// TestQuestionBanksSeparatesEmptyFromError 空态与错误态必须可区分
// （#2753 契约第 4 条：把「服务坏了」说成「没有题库」是错的）。
func TestQuestionBanksSeparatesEmptyFromError(t *testing.T) {
	// ① 空题库：200 + 空态文案
	empty := newFakeUpstream(t)
	empty.questionBanks = map[string]any{"question_banks": []any{}}
	appEmpty, _ := newTestApp(t, empty)
	rec := get(t, appEmpty.routes(), "/question-banks", nil)
	if rec.Code != 200 {
		t.Fatalf("空题库状态码 = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "题库还在补充中") {
		t.Errorf("空题库应渲染空态:\n%s", body)
	}
	if strings.Contains(body, "题库目录暂时加载不出来") {
		t.Errorf("空题库不该渲染错误态:\n%s", body)
	}

	// ② 上游故障：502 + 错误态文案（不是空态）
	broken := newFakeUpstream(t)
	broken.questionBanksStatus = 500
	appBroken, _ := newTestApp(t, broken)
	rec = get(t, appBroken.routes(), "/question-banks", nil)
	if rec.Code != 502 {
		t.Fatalf("上游故障状态码 = %d, want 502", rec.Code)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "题库目录暂时加载不出来") {
		t.Errorf("上游故障应渲染错误态:\n%s", body)
	}
	if strings.Contains(body, "题库还在补充中") {
		t.Errorf("上游故障被渲染成了空态（把服务坏了说成没有题库）:\n%s", body)
	}
}

// TestQuestionBanksExposesNoQuestionContent 页面只渲染汇总，任何题目内容都不出现。
func TestQuestionBanksExposesNoQuestionContent(t *testing.T) {
	fake := newFakeUpstream(t)
	// 上游（假设性地）多给了题目内容：解到 questionBankDTO 就该丢掉，
	// 模板也没有任何路径能把它渲染出去。
	fake.questionBanks = map[string]any{
		"question_banks": []map[string]any{{
			"category_id": 1, "category_name": "叉车司机N1证", "total": 1,
			"by_type": map[string]int64{"single_choice": 1},
			"content": "这是一道不该出现的题干", "answer": "A", "explanation": "不该出现的解析",
			"questions": []map[string]any{{"content": "内层题干"}},
		}},
	}
	application, _ := newTestApp(t, fake)
	body := get(t, application.routes(), "/question-banks", nil).Body.String()

	for _, leak := range []string{"不该出现的题干", "内层题干", "不该出现的解析", `>A<`} {
		if strings.Contains(body, leak) {
			t.Errorf("题库目录渲染了题目内容 %q:\n%s", leak, body)
		}
	}
}

// TestQuestionBanksUsesConfiguredLearnBase 练习链接的基址只来自配置（不从 Host 推断）。
//
// 判据：把 learnBase 换成另一个域，页面上的链接必须整体跟着变——
// 若实现里有从请求头拼地址的路径，这里就会露出旧的 defaultLearnBase。
func TestQuestionBanksUsesConfiguredLearnBase(t *testing.T) {
	fake := newFakeUpstream(t)
	fake.questionBanks = questionBankFixtures()
	application, _ := newTestApp(t, fake)
	application.learnBase = "https://training.example.test"
	handler := application.routes()

	// 带一个别有用心的 Host / X-Forwarded-Host：都不能影响链接基址
	rec := get(t, handler, "/question-banks", map[string]string{
		"Host":              "evil.example.com",
		"X-Forwarded-Host":  "evil.example.com",
		"X-Forwarded-Proto": "http",
	})
	body := rec.Body.String()
	if !strings.Contains(body, "https://training.example.test/training/question-bank?category_id=1") {
		t.Errorf("练习链接没有用配置的 learnBase:\n%s", body)
	}
	if strings.Contains(body, "evil.example.com") {
		t.Errorf("请求头影响了练习链接基址（开放重定向面）:\n%s", body)
	}
}

// TestQuestionBanksNavPresent 题库页也带类型导航（三个课程目录 + 题库在同一导航里可达）。
func TestQuestionBanksNavPresent(t *testing.T) {
	fake := newFakeUpstream(t)
	application, _ := newTestApp(t, fake)
	body := get(t, application.routes(), "/question-banks", nil).Body.String()

	for _, href := range []string{`href="/courses"`, `href="/video"`, `href="/illustrated"`} {
		if !strings.Contains(body, href) {
			t.Errorf("题库页缺少主导航入口 %s:\n%s", href, body)
		}
	}
	if !strings.Contains(body, `<span class="typenav-item typenav-current" aria-current="page">题库</span>`) {
		t.Error("题库页应在主导航标记当前页")
	}
}
