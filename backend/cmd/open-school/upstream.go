package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 四条只读上游端点的路径（契约由 #2753 节点 B 与 #2760 在 backend/internal/course 实现）。
// 路径写死在这里、不拼参数：应用侧一旦「自己造路径」，契约漂移就没法在编译期发现，
// 只能在联调时逐个 404 抓。
const (
	pathCategories    = "/api/public-school/categories"
	pathCourses       = "/api/public-school/courses"
	pathQuestionBanks = "/api/public-school/question-banks"
)

// errCourseNotFound 上游 404 的唯一哨兵：目录/详情两个接口的 404 都归到这里。
var errCourseNotFound = errors.New("课程不存在")

// errUpstream 上游不可用（5xx、网络错误、响应体读不出、信封不合契约）。
// 调用侧据此渲染「加载失败」错误态——**不得**退化成空态或 404，
// 那是把「服务坏了」说成「没有这门课」（#2753 契约第 4 条）。
var errUpstream = errors.New("公开接口暂不可用")

// upstream 是只读 API 客户端。
//
// 刻意不带任何身份：不转发 Cookie、不转发 Authorization、不读请求头推断的地址
// （#2751：不探测角色/权益，也不从 Host/X-Forwarded-* 拼学员站地址）。
type upstream struct {
	base   string
	client *http.Client
}

func newUpstream(base string) *upstream {
	return &upstream{
		base: base,
		// 5s 是「用户还在等」的上限：公开面抢的是公网首屏，超时后给可重试错误态，
		// 比让浏览器转圈更符合预期。
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// envelope 是后端统一信封 {code, message, data}。Data 延迟解析：
// 只有业务成功时才需要反序列化到具体 DTO。
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// get 发起一次 GET 并把 data 解到 dst。
//
// 返回约定：
//   - 上游 404 → errCourseNotFound
//   - 其余非 200、网络错误、信封不合契约 → errUpstream
//   - 成功 → nil（dst 已填充）
func (u *upstream) get(ctx context.Context, path string, query url.Values, dst any) error {
	endpoint := u.base + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	// 用无 body 的 NewRequestWithContext：ctx 来自入站请求，浏览器断开时上游请求同步取消，
	// 不给后端攒无人认领的查询。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: 构造上游请求失败", errUpstream)
	}
	req.Header.Set("Accept", "application/json")
	// 只用匿名凭据：公开面没有登录态，任何凭据头传上去都是越权面。
	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", errUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return errCourseNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: 上游状态码 %d", errUpstream, resp.StatusCode)
	}
	// 上限 4MiB：公开面一页最多 60 条摘要，真超了说明契约变了，宁可报错也不要无界读。
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%w: 读取上游响应失败", errUpstream)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%w: 响应不是 JSON 信封", errUpstream)
	}
	if env.Code != http.StatusOK {
		return fmt.Errorf("%w: 信封 code=%d message=%s", errUpstream, env.Code, env.Message)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		// 200 但 data 为 null：契约里成功面不该出现，按上游异常处理而不是当成空列表——
		// 空列表会渲染成「没有课程」，掩盖后端 bug。
		return fmt.Errorf("%w: 成功信封 data 为 null", errUpstream)
	}
	if dst == nil {
		return nil
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		return fmt.Errorf("%w: data 结构不符契约", errUpstream)
	}
	return nil
}

// categoryDTO 分类行。course_count 由服务端按公开口径算好，应用侧不再筛。
type categoryDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	CourseCount int    `json:"course_count"`
}

// courseSummaryDTO 目录卡片：只含公开字段。
// 结构里刻意没有 content / file_url：上游即使多给了（泄漏或契约漂移），
// 解到这里就丢了，模板没有任何路径能把它渲染出去。
type courseSummaryDTO struct {
	CourseID int    `json:"course_id"`
	Name     string `json:"name"`
	// CourseType 课程主类型（#2760）：video / illustrated。卡片上要打标，
	// 访客在「全部课程」里也得看得出这门课是什么形式。
	CourseType  string      `json:"course_type"`
	Description string      `json:"description"`
	CoverImage  string      `json:"cover_image"`
	Category    categoryDTO `json:"category"`
	PointsPrice *int        `json:"points_price"`
}

// chapterDTO 章节元数据。只有标题/序号/类型/时长，没有正文与文件地址。
type chapterDTO struct {
	ChapterID   int    `json:"chapter_id"`
	Title       string `json:"title"`
	OrderNum    int    `json:"order_num"`
	ContentType string `json:"content_type"`
	Duration    int    `json:"duration"`
}

// courseDetailDTO 详情页数据。
type courseDetailDTO struct {
	CourseID int    `json:"course_id"`
	Name     string `json:"name"`
	// CourseType 课程主类型（#2760）：详情页要显示「这是一门视频课」。
	CourseType   string       `json:"course_type"`
	Description  string       `json:"description"`
	CoverImage   string       `json:"cover_image"`
	Category     categoryDTO  `json:"category"`
	PointsPrice  *int         `json:"points_price"`
	ChapterCount int          `json:"chapter_count"`
	Chapters     []chapterDTO `json:"chapters"`
}

// questionBankDTO 证件题库目录的一行（#2760）：只有汇总，没有题目内容。
type questionBankDTO struct {
	CategoryID   int              `json:"category_id"`
	CategoryName string           `json:"category_name"`
	Total        int              `json:"total"`
	ByType       map[string]int64 `json:"by_type"`
}

// questionBanksData 题库目录信封的 data 段。
type questionBanksData struct {
	QuestionBanks []questionBankDTO `json:"question_banks"`
}

type categoriesData struct {
	Categories []categoryDTO `json:"categories"`
}

type coursesData struct {
	Courses []courseSummaryDTO `json:"courses"`
	Page    int                `json:"page"`
	Pages   int                `json:"pages"`
	Total   int64              `json:"total"`
}

// fetchCategories 取分类。失败时调用侧渲染错误态：分类选项缺失时，
// 目录页的「分类筛选」会变成空下拉，那本身也是一种误导，不静默吞。
//
// courseType 非空时透传给上游（#2760）：类型页的证件下拉只该列该类型有公开课的分类，
// 在应用侧自己过滤会拿到「course_count 是全部课程数」的错数（下拉里写 6、点进去只有 2）。
func (u *upstream) fetchCategories(ctx context.Context, courseType string) ([]categoryDTO, error) {
	var data categoriesData
	var query url.Values
	if courseType != "" {
		query = url.Values{}
		query.Set("course_type", courseType)
	}
	if err := u.get(ctx, pathCategories, query, &data); err != nil {
		return nil, err
	}
	return data.Categories, nil
}

// fetchCourses 取一页课程。course_type / category_id / q / page / page_size 原样透传给上游：
// 关键词匹配与「先筛后分页」都在服务端做（契约第 2 条），应用侧不整页过滤。
func (u *upstream) fetchCourses(ctx context.Context, q courseQuery) (coursesData, error) {
	values := url.Values{}
	// 类型筛选（#2760）：属于哪一类，由 URL 的路径（/video、/illustrated）决定。
	if q.CourseType != "" {
		values.Set("course_type", q.CourseType)
	}
	if q.CategoryID > 0 {
		values.Set("category_id", strconv.Itoa(q.CategoryID))
	}
	if q.Keyword != "" {
		values.Set("q", q.Keyword)
	}
	values.Set("page", strconv.Itoa(q.Page))
	values.Set("page_size", strconv.Itoa(q.PageSize))

	var data coursesData
	if err := u.get(ctx, pathCourses, values, &data); err != nil {
		return coursesData{}, err
	}
	if data.Courses == nil {
		data.Courses = []courseSummaryDTO{}
	}
	return data, nil
}

// fetchQuestionBanks 取证件题库目录（#2760）。不筛不排：上游已按分类聚合并按 sort_order 出。
//
// 错误归口与另两条一致：404 也按 errUpstream 处理——题库目录是**集合**端点，
// 「404」在这里只可能是路径漂移（比空集更该响）。
func (u *upstream) fetchQuestionBanks(ctx context.Context) ([]questionBankDTO, error) {
	var data questionBanksData
	if err := u.get(ctx, pathQuestionBanks, nil, &data); err != nil {
		return nil, err
	}
	if data.QuestionBanks == nil {
		data.QuestionBanks = []questionBankDTO{}
	}
	return data.QuestionBanks, nil
}

// fetchCourseDetail 取课程详情。404 归 errCourseNotFound，其余归 errUpstream。
func (u *upstream) fetchCourseDetail(ctx context.Context, courseID int) (courseDetailDTO, error) {
	var data courseDetailDTO
	path := pathCourses + "/" + strconv.Itoa(courseID)
	if err := u.get(ctx, path, nil, &data); err != nil {
		return courseDetailDTO{}, err
	}
	if data.Chapters == nil {
		data.Chapters = []chapterDTO{}
	}
	return data, nil
}

// courseQuery 是目录页的筛选态，同时也是 URL 与 htmx 局部刷新的唯一来源。
type courseQuery struct {
	Keyword    string
	CategoryID int
	Page       int
	PageSize   int
	// CourseType 课程主类型（#2760）："video" / "illustrated" / 空串（全部课程）。
	//
	// **它不进查询串**：类型由路径决定（/video、/illustrated、/），不写成 ?course_type=video。
	// 理由见 directoryBase —— 类型是「哪个目录」而不是「这个目录里的一个筛选项」，
	// 放进查询串会让 /courses?course_type=video 和 /video 变成同内容的两个地址。
	CourseType string
	// ExplicitPage 请求里带了 page 参数（用来决定第 1 页要不要写进 URL）。
	ExplicitPage bool
}

// 分页口径：与后端 ListPublicCourses 的默认值一致（默认 12、上限 60）。
// 应用侧再钳一遍不是为了防后端，是防**用户手改 URL**：page_size=100000 会被
// htmx 原样回填到下一次请求里，画面当场塌成一条长列表。
const (
	defaultPageSize = 12
	maxPageSize     = 60
)

// parseCourseQuery 从请求查询串解析筛选态。
// 非法值（category_id=abc、page=-3、page_size=99999）一律回退默认，不报 400：
// 公开面的分享链接被人手改坏时，最合适的行为是照常给一页结果，而不是给错误页。
func parseCourseQuery(values url.Values) courseQuery {
	q := courseQuery{
		Keyword:      strings.TrimSpace(values.Get("q")),
		Page:         atoiDefault(values.Get("page"), 1),
		PageSize:     atoiDefault(values.Get("page_size"), defaultPageSize),
		ExplicitPage: values.Get("page") != "",
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > maxPageSize {
		q.PageSize = defaultPageSize
	}
	if id := atoiDefault(values.Get("category_id"), 0); id > 0 {
		q.CategoryID = id
	}
	return q
}

func atoiDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

// courseTypePaths 是三种目录形态的路径前缀（#2760）。
//
// 为什么类型做成**路径**而不是查询参数：类型是「你在哪个目录」，不是「这个目录里的一个
// 筛选项」。做成 ?course_type=video 会立刻生出两个同内容地址（/courses?course_type=video
// 与 /video），分享链接就没法说「这条链是视频课的目录」；而 htmx 每次局部刷新都要把类型
// 编进查询串，一旦某个链接漏了它，访客会从视频页悄悄掉回「全部课程」。
//
// 空串（全部课程）落在 /courses —— 保持 #2753 已定的地址不变。
var courseTypePaths = map[string]string{
	"":            "/courses",
	"video":       "/video",
	"illustrated": "/illustrated",
}

// courseTypePathsByBase 是 courseTypePaths 的反查表（路径 → 类型）。
//
// 两方向都要：正向用于生成链接，反向用于**校验外来 URL**（back 参数白名单）与
// 从请求路径解析类型。分成两张表而不是每次遍历：反查在 safeBack 里是安全判据，
// 拿遍历现推会让「白名单」这个词失去「一张明确的表」这层意思。
var courseTypePathsByBase = map[string]string{
	"/":            "",
	"/courses":     "",
	"/video":       "video",
	"/illustrated": "illustrated",
}

// directoryBase 该筛选态对应的目录路径（类型决定前缀）。
func (q courseQuery) directoryBase() string {
	if base, ok := courseTypePaths[q.CourseType]; ok {
		return base
	}
	// 认不出的类型退回全部课程：应用侧不该出现这个值（路由只挂三种），
	// 但 URL 手改或上游新增类型时，退回「全部」比 404 更可用。
	return courseTypePaths[""]
}

// path 生成带筛选态的路径。base 为空时取该类型对应的目录路径。
func (q courseQuery) path(base string) string {
	if base == "" {
		base = q.directoryBase()
	}
	values := url.Values{}
	if q.CategoryID > 0 {
		values.Set("category_id", strconv.Itoa(q.CategoryID))
	}
	if q.Keyword != "" {
		values.Set("q", q.Keyword)
	}
	if q.Page > 1 {
		values.Set("page", strconv.Itoa(q.Page))
	} else if q.Page == 1 && q.ExplicitPage {
		// 第 1 页通常省略（干净 URL），但「从第 2 页点上一页」时显式写 page=1：
		// 省略会让链接变成 /courses?category_id=3，翻页动作在 URL 上不可见，
		// 也就没法看出来「这是翻回第一页」还是「清掉了筛选」。
		values.Set("page", "1")
	}
	if q.PageSize != defaultPageSize && q.PageSize > 0 {
		values.Set("page_size", strconv.Itoa(q.PageSize))
	}
	if len(values) == 0 {
		return base
	}
	return base + "?" + values.Encode()
}

// href 目录页（含筛选）的地址，页脚/返回链接复用。
func (q courseQuery) href() string {
	return q.path("")
}

// canonical 是「这个筛选视图」的规范地址：第 1 页不写 page。
// 用于 htmx 的 HX-Push-Url——表单会把隐藏字段与空下拉一并提交，原样 push 进历史
// 会让地址栏攒一堆 `category_id=&page=1`；规范化后只留真正生效的条件。
func (q courseQuery) canonical() string {
	trimmed := q
	if trimmed.Page <= 1 {
		trimmed.Page = 0 // path() 不写 page
		trimmed.ExplicitPage = false
	}
	return trimmed.path("")
}
