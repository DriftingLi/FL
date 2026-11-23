package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// 模板与静态资源全部编进二进制：公开面是单文件部署物，不依赖工作目录，
// 也不会出现「忘了拷 static 目录」这类只在生产炸的故障。
//
//go:embed templates/*.html static
var embeddedFiles embed.FS

// siteName 出现在页头与分享元数据。
const siteName = "叉车维修培训"

// 静态资源只有两个：本应用自己的样式表与本地内嵌的 htmx（2.0.11，零外网依赖）。
const (
	pathStyleCSS = "/assets/style.css"
	pathHTMX     = "/assets/htmx.min.js"
)

// headMeta 是全站 <head> 的唯一数据源。
// 为什么单独抽出来：分享卡片（og:*）与 <title> 必须由服务端直出，
// 这样爬虫与 IM 预览拿到的是完整标签，不依赖 JS。
type headMeta struct {
	SiteName    string
	Title       string
	Description string
	OGTitle     string
	OGImage     string
	OGURL       string
	// Robots 非空时输出 robots 元标签（404 / 错误页 noindex：
	// 让「课程不存在」不被搜索引擎当正文收录）。
	Robots string
}

// directoryView 目录页（整页）与结果片段共用。
// 片段模板只读其中一部分字段，多出来的字段对片段无害——
// 刻意不拆成两个结构：拆开就得在两处保持字段同步，漏一处就是渲染空值。
type directoryView struct {
	Head       headMeta
	Query      courseQuery
	Categories []categoryDTO
	Back       string
	Courses    []courseSummaryDTO
	Page       int
	Pages      int
	PrevURL    string
	NextURL    string

	// Error 非空 = 上游故障，渲染错误态（不是空态）。
	Error    string
	RetryURL string
}

// courseTypeTab 顶部类型导航的一项。
type courseTypeTab struct {
	Label string
	Href  string
	// Selected 当前页即这一类（渲染成不可点的当前项，带 aria-current）。
	Selected bool
}

// questionBankView 题库目录整页（#2760）。
type questionBankView struct {
	Head  headMeta
	Banks []questionBankRow
	// Error 非空 = 上游故障，渲染错误态。
	Error string
}

// questionBankRow 题库目录的一行（渲染就绪形态：题型已摊成有序切片）。
type questionBankRow struct {
	CategoryID   int
	CategoryName string
	Total        int
	Types        []questionTypeCount
	// PracticeURL 去学员站练习的地址（带预选分类）。
	PracticeURL string
}

// questionTypeCount 一种题型的题量。
type questionTypeCount struct {
	Label string
	Count int64
}

// FiltersActive 是否有筛选条件：决定「清除筛选」入口是否出现。
func (v directoryView) FiltersActive() bool {
	return v.Query.Keyword != "" || v.Query.CategoryID > 0
}

// SelectedCategory 当前选中的分类 ID（用于 <option selected>）；0 = 全部。
func (v directoryView) SelectedCategory() int {
	return v.Query.CategoryID
}

// Base 该筛选态对应的目录路径（不含查询串）。
//
// 「清除筛选」与表单 action 都用它：类型页清筛选后应留在本类型，而不是掉回全部课程。
//
// 为什么是方法而不是 courseQuery 上的方法：html/template 只能调用**导出**的字段与方法，
// 而 courseQuery 的字段全是小写（它是内部筛选态，不该被模板乱读）。在视图上开一个
// 导出的 Base，模板拿得到路径、拿不到内部字段。
func (v directoryView) Base() string {
	return v.Query.directoryBase()
}

// detailView 详情页。
type detailView struct {
	Head  headMeta
	Back  string
	Learn string

	Course     courseDetailDTO
	Points     int
	Free       bool
	ChapterCnt int
}

// missingView / errorView 是两个「非课程」页面的数据。
// 分开建结构而不是复用 detailView：404 页不该带任何课程字段，
// 复用会让模板里有路径能把半个课程渲染出来。
type missingView struct {
	Head headMeta
	Back string
}

type errorView struct {
	Head     headMeta
	RetryURL string
	Back     string
}

type renderer struct {
	templates *template.Template
	log       *zap.Logger
}

// funcs 是模板里唯一允许的业务逻辑（值映射与取整）。
// 复杂判断一律留在 Go 侧：模板里的表达式没有测试，错了只会静默渲染成空串。
var funcs = template.FuncMap{
	// pointsOf 把可缺省的点数价格摊平成 int（nil / 缺省 = 0 = 免费）。
	"pointsOf":    derefInt,
	"chapterType": chapterTypeLabel,
	"metaText":    metaText,
	// courseTypeLabel 课程主类型的显示名（#2760）：video → 视频课程、illustrated → 图文课程。
	"courseTypeLabel":  courseTypeLabel,
	"schoolNavigation": schoolNavigation,
}

// courseTypeLabel 课程主类型的显示名。
//
// 认不出的类型返回原始值：上游加了新类型却还没在这里登记时，显示原值比显示「课程」
// 更容易看出「这是新分类」，也和 chapterTypeLabel 的既有口径一致。
func courseTypeLabel(raw string) string {
	switch raw {
	case "video":
		return "视频课程"
	case "illustrated":
		return "图文课程"
	case "":
		return ""
	default:
		return raw
	}
}

// derefInt 解引用可缺省的整数字段：nil 与 0 同义（免费）。
//
// 为什么单独提一个函数：模板闭包与 Go 侧（详情页的 Free / Points 判定）必须用
// 同一条「缺省 = 免费」判据，抄两份就会在某个字段上分叉。
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// newRenderer 解析全部模板。解析失败直接 panic：模板是编译产物的一部分，
// 编进二进制的东西坏了属于启动期就该炸的错，不该等到第一个用户访问才 500。
func newRenderer(log *zap.Logger) *renderer {
	parsed, err := template.New("open-school").Funcs(funcs).ParseFS(embeddedFiles, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("模板解析失败：%v", err))
	}
	return &renderer{templates: parsed, log: log}
}

// render 先渲染到内存再落盘：模板执行到一半出错时还能返回一个干净的 500，
// 而不是先写出一截半截 HTML 再把错误码塞进已经发出去的响应体。
func (r *renderer) render(w http.ResponseWriter, status int, name string, view any) {
	var buf bytes.Buffer
	if err := r.templates.ExecuteTemplate(&buf, name, view); err != nil {
		r.log.Error("模板渲染失败", zap.String("template", name), zap.Error(err))
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	// 公开页按请求参数变化，必须 no-store：否则浏览器后退会拿到上一个筛选的缓存。
	header.Set("Cache-Control", "no-store")
	// 同一 URL 会因请求头返回整页或片段，代理必须分开缓存键。
	header.Set("Vary", "HX-Request, HX-History-Restore-Request")
	header.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// metaText 把简介压成适合 <meta> / og:description 的一行：折叠所有空白（含换行，
// 否则属性里会留一串空格）并按字符数截断（按字节截会把中文截成乱码）。
func metaText(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

// chapterTypeLabel 把 content_type 映射成中文标签。
// 认不出的类型原样显示：那是后端新增了正文形态，显示原始值比显示「未知」更容易排查。
func chapterTypeLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "text":
		return "图文"
	case "video":
		return "视频"
	case "document":
		return "文档"
	case "ppt":
		return "课件"
	case "":
		return ""
	default:
		return raw
	}
}
