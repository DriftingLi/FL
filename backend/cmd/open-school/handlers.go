package main

import (
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// app 把「上游客户端 + 渲染器 + 配置」装配起来。
// 全部依赖显式注入：测试用 httptest 假上游替换 upstream，不必起真后端。
type app struct {
	upstream  *upstream
	renderer  *renderer
	learnBase string
	log       *zap.Logger
}

// routes 装配路由（#2752 决议 + #2760 类型分流）：
//
//	/              公开目录（首页默认就是「全部课程」）
//	/courses       公开目录（全部课程）
//	/video         视频课程目录
//	/illustrated   图文课程目录（含 PPT）
//	/question-banks 证件题库目录
//	/courses/{id}  课程详情（正式分享地址，不带浏览条件）
//	/healthz       健康检查
//
// 三个课程目录共用同一个 handler，靠 courseTypeOfPath 决定筛哪一类 —— 用 Go 1.22+ 的
// 方法+路径模式：不需要第三方路由，且 405 由标准库给（方法不对不会掉进目录页）。
func (a *app) routes() http.Handler {
	mux := http.NewServeMux()

	// 静态资源只服务这两个具名文件：不挂目录，避免把 templates/ 里的源模板
	// （含未转义的占位）暴露出去。
	mux.HandleFunc("GET "+pathHTMX, serveEmbedded("static/htmx.min.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET "+pathStyleCSS, serveEmbedded("static/style.css", "text/css; charset=utf-8"))
	// 许可证与上游说明单独可达：自托管分发 htmx 时要求随附许可。
	mux.HandleFunc("GET /assets/htmx-LICENSE.txt", serveEmbedded("static/htmx-LICENSE.txt", "text/plain; charset=utf-8"))

	mux.HandleFunc("GET /{$}", a.directory)
	mux.HandleFunc("GET /courses", a.directory)
	mux.HandleFunc("GET /video", a.directory)
	mux.HandleFunc("GET /illustrated", a.directory)
	mux.HandleFunc("GET /question-banks", a.questionBanks)
	mux.HandleFunc("GET /courses/{courseID}", a.detail)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		// 只报进程自身：**不**探测上游。健康检查一旦连带后端，后端抖动会让
		// 编排器重启这个无关的静态前台，故障面积反而变大。
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

// courseTypeOfPath 路径 → 课程类型筛选（#2760）。空串 = 全部课程。
//
// 判据只此一处：路由表与 directory 各自写一遍「/video 是视频」就会出现两处漂移。
func courseTypeOfPath(path string) string {
	for courseType, base := range courseTypePaths {
		if base == path {
			return courseType
		}
	}
	return ""
}

// serveEmbedded 返回一个只读内嵌文件的 handler。
func serveEmbedded(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := embeddedFiles.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		// 静态资源带版本（htmx 2.0.11 固定），可以长期缓存；改文件记得改 query 里的版本。
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(body)
	}
}

// isHTMXFragment 判断这次请求该只回片段。
//
// 关键判据是 HX-History-Restore-Request：htmx 在浏览器前进/后退时
// 用同一个 URL 重新拉一次（historyCacheSize 内是缓存，缓外就是真请求），
// 这次要的是**整页**——回片段会让整个页面被一段 <section /> 替换掉。
// htmx 在 restore 时默认不带 HX-Request，但我们额外把 historyRestoreAsHxRequest
// 保持 false，也只依赖标准行为；两个头都判一遍更难被配置漂移带跑。
func isHTMXFragment(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}
	return r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-History-Restore-Request") != "true"
}

// directory 处理 /、/courses、/video、/illustrated（#2760：类型决定筛哪一类）。
func (a *app) directory(w http.ResponseWriter, r *http.Request) {
	q := parseCourseQuery(r.URL.Query())
	// 类型来自路径而非查询串（见 courseTypePaths）：/video 与 /courses 的唯一差别就是这一格。
	q.CourseType = courseTypeOfPath(r.URL.Path)
	view := directoryView{
		Query:      q,
		RetryURL:   q.href(),
		Back:       q.href(),
		Categories: []categoryDTO{},
		Head:       directoryHead(q.CourseType),
	}

	courses, courseErr := a.upstream.fetchCourses(r.Context(), q)
	// 分类失败不单独报错态：分类是筛选器，课程列表才是主体。
	// 只有列表也失败时才算「页面加载失败」，否则降级成「只有全部课程可选」。
	categories, categoryErr := a.upstream.fetchCategories(r.Context(), q.CourseType)

	switch {
	case courseErr != nil && !isNotFound(courseErr):
		// 上游坏了：错误态（可重试），**不是**空态也不是 404。
		view.Error = "课程列表暂时加载不出来，请稍后重试。"
		view.Courses = []courseSummaryDTO{}
		view.Page, view.Pages = 1, 0
	case courseErr == nil:
		view.Courses = courses.Courses
		view.Page = courses.Page
		view.Pages = courses.Pages
	}

	if categoryErr != nil {
		// 分类降级：留空列表 + 不提错。列表成功时页面仍然可用（关键词搜索照常）。
		a.log.Warn("分类加载失败，筛选器降级为空", zap.Error(categoryErr))
		view.Categories = []categoryDTO{}
	} else {
		view.Categories = categories
	}

	// 分页链接带完整筛选态：翻页不能丢掉关键词与分类。
	view.PrevURL, view.NextURL = "", ""
	if view.Pages > 0 && view.Page > 1 {
		prev := q
		prev.Page = view.Page - 1
		view.PrevURL = prev.href()
	}
	if view.Pages > 0 && view.Page < view.Pages {
		next := q
		next.Page = view.Page + 1
		view.NextURL = next.href()
	}
	// 回写「服务端钳制后的实际页码」：用户手改 page=999 时 htmx 后续请求要跟着实际页码走。
	view.Query.Page = view.Page
	// Back 也必须用钳制后的筛选态重算：卡片链接靠它带筛选进详情，
	// 若沿用请求里的原始 q.page=999，用户点进详情再返回会落回那个越界页码。
	view.Back = view.Query.href()

	if isHTMXFragment(r) {
		// 片段响应带 HX-Push-Url：浏览器地址栏用**服务端规范化后**的地址。
		// 表单会把空字段（category_id=、page=1）一并提交，原样 push 进历史会让
		// 地址栏攒一堆 `category_id=&page=1`；规范化后只保留真实生效的条件，
		// 后退/刷新时服务端按同一个地址还原同一筛选。
		w.Header().Set("HX-Push-Url", view.Query.canonical())
		a.renderer.render(w, http.StatusOK, "results", view)
		return
	}
	a.renderer.render(w, http.StatusOK, "directory", view)
}

// questionBanks 处理 /question-banks（#2760 US-11/US-12）：按证件看题库覆盖。
//
// 判据形态：目录本身就是「汇总」，没有筛选也没有分页 —— 证件总数是有限集（当前 8 个），
// 给它做筛选器只会多一层空态。上游已按分类聚合，这里只负责渲染与「去练习」的跳转链。
func (a *app) questionBanks(w http.ResponseWriter, r *http.Request) {
	banks, err := a.upstream.fetchQuestionBanks(r.Context())
	view := questionBankView{
		Head:  questionBanksHead(),
		Banks: []questionBankRow{},
	}
	if err != nil {
		// 错误态（可重试），**不是**空态：把「服务坏了」说成「没有题库」会让访客
		// 以为这个平台没题可练 —— 与课程目录同一判据（#2753 契约第 4 条）。
		a.log.Warn("题库目录加载失败", zap.Error(err))
		view.Error = "题库目录暂时加载不出来，请稍后重试。"
		a.renderer.render(w, http.StatusBadGateway, "question_banks", view)
		return
	}
	view.Banks = make([]questionBankRow, 0, len(banks))
	for _, b := range banks {
		view.Banks = append(view.Banks, questionBankRow{
			CategoryID:   b.CategoryID,
			CategoryName: b.CategoryName,
			Total:        b.Total,
			// 题型分布按固定顺序摊平成切片：模板里遍历 map 顺序不稳定，
			// 「单选题 2 · 判断题 1」每次刷新换序会让人以为数据在变。
			Types: questionTypeCounts(b.ByType),
			// 去练习的地址：学员站题库页 + 预选分类（#2760 US-13）。
			PracticeURL: a.practiceURL(b.CategoryID),
		})
	}
	a.renderer.render(w, http.StatusOK, "question_banks", view)
}

// practiceURL 生成「去练习」的整页导航地址（#2760 US-13）。
//
// 与 learnURL 同一口径（#2751）：基址**只**来自配置，不从请求头推断。
// 带上 category_id 让学员站题库页加载后预选该分类；未登录时由学员站的既有登录流程
// 接管，它保留 redirect 参数，登录后回到同一个带参地址。
func (a *app) practiceURL(categoryID int) string {
	return a.learnBase + "/training/question-bank?category_id=" + strconv.Itoa(categoryID)
}

// courseTypeTabs 顶部类型导航（#2760 US-1/US-2）：三个入口，当前页高亮。
//
// 每次切换都是**整页**导航而不是 htmx 局部刷新：类型是另一个目录（路径不同），
// 换目录时标题/og:* 都要跟着变，局部替换会让分享元数据停在上一类上。
func schoolNavigation(path string) []courseTypeTab {
	tabs := []courseTypeTab{
		{Label: "全部课程", Href: courseTypePaths[""]},
		{Label: "视频课程", Href: courseTypePaths["video"]},
		{Label: "图文课程", Href: courseTypePaths["illustrated"]},
		{Label: "题库", Href: "/question-banks"},
	}
	for i := range tabs {
		tabs[i].Selected = tabs[i].Href == path
	}
	return tabs
}

// parseCourseID 解析路径上的课程 ID。只认正整数：
// /courses/0、/courses/abc、/courses/1e3 一律当「课程不存在」。
func parseCourseID(raw string) (int, bool) {
	id, err := strconv.Atoi(raw)
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

// detail 处理 /courses/{courseID}。
func (a *app) detail(w http.ResponseWriter, r *http.Request) {
	courseID, ok := parseCourseID(r.PathValue("courseID"))
	if !ok {
		a.renderMissing(w, defaultCatalogPath)
		return
	}
	back := a.safeBack(r.URL.Query().Get("back"))

	course, err := a.upstream.fetchCourseDetail(r.Context(), courseID)
	if err != nil {
		if isNotFound(err) {
			a.renderMissing(w, back)
			return
		}
		a.log.Warn("课程详情加载失败", zap.Int("course_id", courseID), zap.Error(err))
		// 5xx / 网络错误：可重试错误态。状态码用 502 而不是 500：
		// 错在上游，本进程只是渲染。
		a.renderer.render(w, http.StatusBadGateway, "error_page", errorView{
			Head:     errorHead(),
			RetryURL: courseRetryURL(courseID, back),
			Back:     back,
		})
		return
	}

	view := detailView{
		Head:       courseHead(course),
		Back:       back,
		Learn:      a.learnURL(course.CourseID),
		Course:     course,
		Points:     derefInt(course.PointsPrice),
		Free:       derefInt(course.PointsPrice) == 0,
		ChapterCnt: len(course.Chapters),
	}
	a.renderer.render(w, http.StatusOK, "detail", view)
}

// renderMissing 渲染「课程不存在」页（状态码 404，不是 200）。
func (a *app) renderMissing(w http.ResponseWriter, back string) {
	a.renderer.render(w, http.StatusNotFound, "missing_page", missingView{
		Head: missingHead(),
		Back: back,
	})
}

// defaultCatalogPath 是「返回目录」在没有任何筛选线索时的落点。
const defaultCatalogPath = "/courses"

// safeBack 校验 back 查询参数，只接受**本站**的目录地址。
//
// 防线（按顺序）：
//  1. 必须是相对地址：绝对 URL（http://evil.com）、协议相对地址（//evil.com）、
//     带 scheme 的都拒——否则就是一个开放重定向。
//  2. 不能带主机、不能带用户信息、不能带片段（片段在服务端没用，往往是伪装的残渣）。
//  3. 路径必须是 / 或 /courses：只放行目录页，其它路径（/healthz 之类）没有「返回」语义。
//  4. 反斜杠与换行一并拒：浏览器的 URL 解析对 `\` 的宽容度与 Go 不同，
//     `/\/evil.com` 在某些客户端会被归一成 `//evil.com`。
func (a *app) safeBack(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultCatalogPath
	}
	if strings.ContainsAny(raw, "\\\r\n\t") {
		return defaultCatalogPath
	}
	u, err := url.Parse(raw)
	if err != nil {
		return defaultCatalogPath
	}
	if u.IsAbs() || u.Host != "" || u.User != nil || u.Fragment != "" {
		return defaultCatalogPath
	}
	// 只放行三个课程目录的路径（#2760）：类型页也是目录页，详情页的「返回目录」
	// 必须能回到 /video 或 /illustrated，否则从视频页点进详情再返回就掉回全部课程。
	if _, ok := courseTypePathsByBase[u.Path]; !ok {
		return defaultCatalogPath
	}
	// 只回传自己也认可的筛选参数：back 里的未知参数（utm_*、跳转类参数）
	// 不跟着传递，避免把一个被塞进来的参数一路带着走。
	q := parseCourseQuery(u.Query())
	// 类型从**路径**恢复（#2760）：类型不写在查询串里（见 courseTypePaths），
	// 所以只从 query 解析出来的 q.CourseType 是空的——直接 href() 会让
	// 「从视频页进详情再返回」掉回全部课程。路径在此处已通过白名单校验。
	q.CourseType = courseTypePathsByBase[u.Path]
	return q.href()
}

// courseRetryURL 错误态的重试地址：回到同一个详情页，保留 back。
func courseRetryURL(courseID int, back string) string {
	u := "/courses/" + strconv.Itoa(courseID)
	if back != "" && back != defaultCatalogPath {
		return u + "?back=" + url.QueryEscape(back)
	}
	return u
}

// learnURL 生成「进入学习」的整页导航地址。
//
// #2751 决议：基址**只**来自 OPEN_SCHOOL_LEARN_BASE，不从请求头推断
// （Host / X-Forwarded-Host 都是攻击者可控的输入），也不在这里探测角色或权益：
// 公开面只负责把人送到受信任学员站的既有入口，是否解锁由学员站自己判。
func (a *app) learnURL(courseID int) string {
	return a.learnBase + "/training/course/" + strconv.Itoa(courseID)
}

// isNotFound 判断错误链里是否有 404 哨兵（用 errors.Is 而不是比字符串：
// 上游客户端以后若给 404 包一层上下文，这里不必跟着改）。
func isNotFound(err error) bool {
	return errors.Is(err, errCourseNotFound)
}

// courseTypeQuestionTypeLabels 题型的中文标签（#2760）。顺序即展示顺序。
//
// 只登记**当前真的会出现的**三种（题库域 ValidQuestionTypes 的前三），
// 认不出的题型按原始值显示：后端加了新题型时，显示原值比显示「未知」更容易排查。
var questionTypeDisplayOrder = []struct {
	Key   string
	Label string
}{
	{"single_choice", "单选题"},
	{"multi_choice", "多选题"},
	{"true_false", "判断题"},
	{"fault_image", "故障图题"},
	{"short_answer", "简答题"},
}

// questionTypeCounts 把 by_type 映射摊成有序切片（只出 >0 的题型）。
//
// 已登记的题型按 questionTypeDisplayOrder 的固定顺序展示；**没登记的新题型也要出**，
// 按 key 的字母序排在已知题型之后、标签显示原始 key。理由：上游 Total 统计的是全部
// 题库池题目（含新题型），若这里静默丢掉未知题型，页面就会出现「总计 N 题」但题型
// 分布加起来小于 N 的自相矛盾——那正是「只返回各题型数量」这条规格被打破的形状。
func questionTypeCounts(byType map[string]int64) []questionTypeCount {
	out := make([]questionTypeCount, 0, len(byType))
	seen := make(map[string]bool, len(byType))
	for _, entry := range questionTypeDisplayOrder {
		if n, ok := byType[entry.Key]; ok && n > 0 {
			out = append(out, questionTypeCount{Label: entry.Label, Count: n})
			seen[entry.Key] = true
		}
	}
	// 剩余未登记的题型：按 key 排序，保证同一份数据每次渲染出的顺序一致
	// （Go map 遍历随机，直接 range 会让页面每次刷新换序）。
	rest := make([]string, 0, len(byType))
	for key, n := range byType {
		if !seen[key] && n > 0 {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		out = append(out, questionTypeCount{Label: key, Count: byType[key]})
	}
	return out
}

// directoryHead 目录页的分享元数据。og:url 带**类型路径**但不带筛选参数（#2760）：
// 分享出去的永远是「这一类的公开目录首屏」——不是分享者当时的筛选视图，
// 也（因为带路径）不会把「视频课程」的链接说成「全部课程」。
func directoryHead(courseType string) headMeta {
	// 按类型给不同的标题与描述：三类目录分享出去时，预览卡片要能自己说清是哪一类。
	switch courseType {
	case "video":
		return headMeta{
			SiteName:    siteName,
			Title:       "视频课程 · " + siteName,
			Description: "按分类浏览以视频讲授的叉车维修与安全操作公开培训课程，查看章节目录后进入学员站学习。",
			OGTitle:     "视频课程 · " + siteName,
			OGURL:       courseTypePaths["video"],
		}
	case "illustrated":
		return headMeta{
			SiteName:    siteName,
			Title:       "图文课程 · " + siteName,
			Description: "按分类浏览图文与 PPT 形式的叉车维修与安全操作公开培训课程，查看章节目录后进入学员站学习。",
			OGTitle:     "图文课程 · " + siteName,
			OGURL:       courseTypePaths["illustrated"],
		}
	default:
		return headMeta{
			SiteName:    siteName,
			Title:       "全部课程 · " + siteName,
			Description: "按分类浏览叉车维修与安全操作的公开培训课程，可分视频课程与图文课程，查看章节目录后进入学员站学习。",
			OGTitle:     "全部课程 · " + siteName,
			OGURL:       courseTypePaths[""],
		}
	}
}

// questionBanksHead 题库目录页的分享元数据。
func questionBanksHead() headMeta {
	return headMeta{
		SiteName:    siteName,
		Title:       "证件题库目录 · " + siteName,
		Description: "按目标证件查看题库题量与题型分布，确认练习覆盖情况后进入学员站练习。",
		OGTitle:     "证件题库目录 · " + siteName,
		OGURL:       "/question-banks",
	}
}

// courseHead 详情页的分享元数据：title / description / og:* 全部服务端直出。
func courseHead(course courseDetailDTO) headMeta {
	title := course.Name + " · " + siteName
	description := metaText(course.Description, 120)
	if description == "" {
		description = "查看《" + course.Name + "》的课程介绍与章节目录。"
	}
	return headMeta{
		SiteName:    siteName,
		Title:       title,
		Description: description,
		OGTitle:     course.Name,
		OGImage:     course.CoverImage,
		// og:url 用不带查询串的规范地址：详情页的 back 参数是浏览上下文，
		// 分享出去会把别人的筛选态也带上。
		OGURL: "/courses/" + strconv.Itoa(course.CourseID),
	}
}

func missingHead() headMeta {
	return headMeta{
		SiteName:    siteName,
		Title:       "课程不存在 · " + siteName,
		Description: "该课程可能已下架，或链接已失效。可以返回公开课程目录继续浏览。",
		OGTitle:     "课程不存在 · " + siteName,
		OGURL:       defaultCatalogPath,
		Robots:      "noindex",
	}
}

func errorHead() headMeta {
	return headMeta{
		SiteName:    siteName,
		Title:       "课程暂时加载不出来 · " + siteName,
		Description: "公开课程接口暂时不可用，可以稍后重试或返回课程目录。",
		OGTitle:     "课程暂时加载不出来 · " + siteName,
		OGURL:       defaultCatalogPath,
		Robots:      "noindex",
	}
}
