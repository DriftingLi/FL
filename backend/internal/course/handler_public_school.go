// 本文件：公开网校读面的 HTTP 出口（#2753 实施地图节点 B 的接口面）。
//
// 为什么不并进 handler.go：那边注册的是既有学员 Web 消费的读面（`/api/courses` 等），
// 一经改动前端与移动端都会跟着动。公开网校是**另一个应用**的消费面，契约独立演进——
// 分文件注册让「改公开网校的判据/字段」与「改既有学员站读面」在 diff 上也分得开。
//
// 注册位置：挂在既有 `RegisterRoutes` 里（同一蓝图 / 同一 service），不新增域注册项，
// 因此路由总数常量会 +3（见 internal/api/routes_registry_test.go）。
package course

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/httpx"
)

// registerPublicSchoolRoutes 注册公开网校读面四条（匿名可读，无鉴权、无证件作用域）。
//
// 挂 `rg` 而不是带 CredentialScoped 的 `g`：公开网校按**分类**找课，账号的当前证件不参与
// （#2753 A 节点口径）。挂上证件作用域会让 Cookie 里的当前证件改变公开目录内容 —— 同一条
// 分享链接对不同人显示不同结果，那正是公开面不该有的事。
func registerPublicSchoolRoutes(rg *gin.RouterGroup, h *handler) {
	rg.GET("/public-school/categories", h.ListPublicSchoolCategories)
	rg.GET("/public-school/courses", h.ListPublicSchoolCourses)
	rg.GET("/public-school/courses/:course_id", h.GetPublicSchoolCourseDetail)
	// 证件题库目录（#2760）：独立于课程目录，只有题库的证件也要在这里出现。
	rg.GET("/public-school/question-banks", h.ListPublicQuestionBanks)
}

// parsePublicSchoolCourseType 解析 `course_type` 查询参数（#2760）。
//
// 三种输入：缺省 / 空串 ⇒ 全部课程（返回 ""）；合法值 ⇒ 该类型；
// 其余（含拼错、大小写不符）⇒ 400。**不静默回退**成「全部」：访客点的是「视频课程」，
// 参数写错却给全部课程，页面标签与内容就对不上了——那是比 400 更坏的错误。
func parsePublicSchoolCourseType(c *gin.Context) (string, error) {
	raw, provided := c.GetQuery("course_type")
	if !provided {
		return "", nil
	}
	if raw == "" {
		return "", nil
	}
	if !IsValidCourseType(raw) {
		return "", httpx.BadRequest("课程类型无效")
	}
	return raw, nil
}

// ListPublicSchoolCategories 公开网校分类筛选选项
// @Summary 公开网校分类选项（按证件口径，空项隐藏）
// @Description 只返回已启用、映射到有效启用证件的培训一级分类，且只含已发布公开课数 > 0 的分类（空项隐藏）。可选 `course_type` 收口：只列该类型有公开课的分类。公开读面，匿名可读，不受账号当前证件影响。
// @Tags 学员端-课程
// @Produce json
// @Param course_type query string false "课程类型（video / illustrated；省略=全部课程）"
// @Success 200 {object} response.R{data=course.PublicSchoolCategoryListDTO} "success"
// @Failure 400 {object} response.R "课程类型无效"
// @Router /public-school/categories [get]
func (h *handler) ListPublicSchoolCategories(c *gin.Context) {
	httpx.Endpoint[string, PublicSchoolCategoryListDTO]{
		Parse: func(c *gin.Context) (*string, error) {
			courseType, err := parsePublicSchoolCourseType(c)
			if err != nil {
				return nil, err
			}
			return &courseType, nil
		},
		Invoke: func(ctx context.Context, courseType *string) (*PublicSchoolCategoryListDTO, error) {
			return h.svc.ListPublicSchoolCategories(ctx, *courseType)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// ListPublicSchoolCourses 公开网校课程目录
// @Summary 公开网校课程目录（类型 + 分类 + 关键词，先筛后分页）
// @Description 只返回已发布且分类满足证件口径的课程。`course_type` 按课程主类型分流（video / illustrated，省略=全部）。关键词 `q` 匹配公开名称与简介（先筛后分页）。`category_id` 指向不合格分类时返回空集。公开读面，匿名可读。
// @Tags 学员端-课程
// @Produce json
// @Param course_type query string false "课程类型（video / illustrated；省略=全部课程）"
// @Param category_id query int false "一级分类ID"
// @Param q query string false "关键词（匹配公开名称与简介）"
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.R{data=course.PublicSchoolCoursePageDTO} "success"
// @Failure 400 {object} response.R "课程类型或分类ID无效"
// @Router /public-school/courses [get]
func (h *handler) ListPublicSchoolCourses(c *gin.Context) {
	httpx.Endpoint[publicSchoolListHTTPReq, PublicSchoolCoursePageDTO]{
		Parse: func(c *gin.Context) (*publicSchoolListHTTPReq, error) {
			courseType, err := parsePublicSchoolCourseType(c)
			if err != nil {
				return nil, err
			}
			categoryID := httpx.QueryIDPtr(c, "category_id")
			if _, provided := c.GetQuery("category_id"); provided && categoryID == nil {
				return nil, httpx.BadRequest("分类ID无效")
			}
			return &publicSchoolListHTTPReq{
				page: httpx.QueryIntDefault(c, "page", 1), pageSize: httpx.QueryIntDefault(c, "page_size", 12),
				categoryID: categoryID, keyword: c.Query("q"), courseType: courseType,
			}, nil
		},
		Invoke: func(ctx context.Context, req *publicSchoolListHTTPReq) (*PublicSchoolCoursePageDTO, error) {
			return h.svc.ListPublicSchoolCourses(ctx, publicSchoolListRequest{
				page: req.page, pageSize: req.pageSize, categoryID: req.categoryID,
				keyword: req.keyword, courseType: req.courseType,
			})
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetPublicSchoolCourseDetail 公开网校课程详情
// @Summary 公开网校课程详情（公开字段 + 已发布章节目录）
// @Description 公开字段仅承诺课程 ID、名称、简介、封面、一级分类、积分价格，以及已发布章节的 ID/标题/序号/类型/时长。未发布、分类不合格或不存在统一 404「课程不存在」。公开读面，匿名可读。
// @Tags 学员端-课程
// @Produce json
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=course.PublicSchoolCourseDetailDTO} "success"
// @Failure 400 {object} response.R "课程ID无效"
// @Failure 404 {object} response.R "课程不存在"
// @Router /public-school/courses/{course_id} [get]
func (h *handler) GetPublicSchoolCourseDetail(c *gin.Context) {
	httpx.Endpoint[int, PublicSchoolCourseDetailDTO]{
		Parse: func(c *gin.Context) (*int, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			return &id, err
		},
		Invoke: func(ctx context.Context, id *int) (*PublicSchoolCourseDetailDTO, error) {
			return h.svc.GetPublicSchoolCourseDetail(ctx, *id)
		},
		// 三态（未发布 / 分类不合格 / 行不存在）统一答 404 同一句文案：不泄漏是哪一态
		// （与既有公开详情同一呈现决定，ADR-0062 决策 3 / ADR-0064 决策 1）。
	}.WithSentinelsMsg(http.StatusNotFound, "课程不存在", unreadableFaces404...).Handle(c)
}

// publicSchoolListHTTPReq 目录查询参数的 handler 侧容器（解析与 service 入参分开：
// service 入参不需要知道 gin，也不需要「是否 provided」这类 HTTP 事实）。
type publicSchoolListHTTPReq struct {
	page       int
	pageSize   int
	categoryID *int
	keyword    string
	courseType string
}

// ListPublicQuestionBanks 公开网校证件题库目录
// @Summary 公开网校证件题库目录（按证件汇总题量与题型分布）
// @Description 按题目的一级培训分类聚合题库池题目：只统计已发布且排除来源标记（真题）的题目，分类须已启用且映射到已启用证件。**与是否有公开课无关**——只有题库的证件也会出现。只返回汇总（分类 ID/显示名/总题数/各题型数量），不含任何题干、选项、答案或解析。公开读面，匿名可读。
// @Tags 学员端-课程
// @Produce json
// @Success 200 {object} response.R{data=course.PublicQuestionBankListDTO} "success"
// @Router /public-school/question-banks [get]
func (h *handler) ListPublicQuestionBanks(c *gin.Context) {
	httpx.Endpoint[struct{}, PublicQuestionBankListDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*PublicQuestionBankListDTO, error) {
			return h.svc.ListPublicQuestionBanks(ctx)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}
