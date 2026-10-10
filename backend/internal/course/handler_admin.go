// 本文件：课程域 HTTP 出口之二 —— 管理端课程 / 章节 CRUD（/api/admin/course*、/api/admin/chapter*）。
// 原住 internal/api/admin.go:48-59 与 :82-408 的九条课程管理端点，波 4d 按裁决 D1 搬进课程域包：
// 否则管理域包会长期替课程域持有路由（ADR-0070「目录即射程」）。
// 本域两条蓝图分居两文件，故 handler 类型与注册函数都带 Admin 后缀（同包不能有两个 RegisterRoutes）。
// 装配点：internal/api/routes_registry.go 调 course.RegisterAdminRoutes(api, rd.Session, deps.AdminCourseSvc)。
package course

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/sortorder"
	"forklift-training/pkg/httpx"
)

// adminHandler 管理端课程 / 章节 CRUD（原 internal/api/admin.go 的 AdminHandler 课程面）。
type adminHandler struct {
	courseSvc *AdminService
}

// newAdminHandler 创建管理端课程 / 章节 handler。
func newAdminHandler(courseSvc *AdminService) *adminHandler {
	return &adminHandler{courseSvc: courseSvc}
}

// RegisterAdminRoutes 注册 /api/admin 下的课程与章节管理端点（组级 JWTAuth + 逐片能力守卫，#1639）。
func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, svc *AdminService) {
	h := newAdminHandler(svc)

	g := rg.Group("/admin", middleware.JWTAuth(session))

	// 课程读面被内容生成页共用（生成前要选课程与章节），故 content.generate 一并作为候选（#1639）。
	read := g.Group("", middleware.CapabilityRequired(authz.CapCourseManage, authz.CapContentGenerate))
	read.GET("/courses", h.ListCourses)
	read.GET("/course/:course_id", h.GetCourseDetail)

	// ===== 课程与章节写面 =====
	write := g.Group("", middleware.CapabilityRequired(authz.CapCourseManage))
	write.POST("/course", h.CreateCourse)
	write.PUT("/course/:course_id", h.UpdateCourse)
	write.PUT("/course/:course_id/sort", h.SwapCourseSort)
	write.DELETE("/course/:course_id", h.DeleteCourse)
	write.POST("/course/:course_id/chapter", h.CreateChapter)
	write.PUT("/chapter/:chapter_id", h.UpdateChapter)
	write.DELETE("/chapter/:chapter_id", h.DeleteChapter)
}

// @Summary 管理端课程列表
// @Description 管理员课程列表（含草稿），支持关键字/证件/方向/等级/热门精品过滤
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键字"
// @Param credential_id query int false "目标证件 ID"
// @Param specialty_id query int false "专业方向 ID"
// @Param level_id query int false "等级 ID"
// @Param filter query string false "热门/精品筛选 hot|featured|all" default(all)
// @Success 200 {object} response.R{data=CoursePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/courses [get]
// ListCourses 课程列表 GET /api/admin/courses（filter=hot|featured|all，缺省 all）
func (h *adminHandler) ListCourses(c *gin.Context) {
	httpx.Endpoint[adminCourseListReq, CoursePageResult]{
		Parse: func(c *gin.Context) (*adminCourseListReq, error) {
			f := c.Query("filter")
			if f != "" && f != "hot" && f != "featured" && f != "all" {
				return nil, httpx.BadRequest("filter 仅支持 hot|featured|all")
			}
			if f == "" {
				f = "all"
			}
			return &adminCourseListReq{
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 10),
				Keyword:      c.Query("keyword"),
				CredentialID: httpx.QueryIDPtr(c, "credential_id"),
				SpecialtyID:  httpx.QueryIDPtr(c, "specialty_id"),
				LevelID:      httpx.QueryIDPtr(c, "level_id"),
				Filter:       f,
			}, nil
		},
		Invoke: func(ctx context.Context, req *adminCourseListReq) (*CoursePageResult, error) {
			result, err := h.courseSvc.GetCourses(req.Page, req.PageSize, req.Keyword, req.CredentialID, req.SpecialtyID, req.LevelID, req.Filter)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// courseWriteFacts400 是课程两条写面（Create/Update）**共用**的「输入不合法」事实集
// （ADR-0065 决策 3）。挂同一份表，为的是让「同一件输入错误按 HTTP 动词分家」不可能再发生：
// 此前 Create 的默认面是 400（于是 `applyCourseTrainingFields` / `replaceCoursePrerequisites`
// 里的「查不动」被答成参数错误，还连带把驱动原文 `SQL logic error: no such table: …` 外发出去），
// 而 Update 的默认面是 500（于是 14 条输入不合法被答成服务端故障）——两面各错一半。
//
// 三条「引用对象不存在」复用目录域的同一载体（`catalog_specs.go:15-17`），不另起名字。
var courseWriteFacts400 = []error{
	ErrCourseNameRequired, ErrSpecialtyRequired, ErrCourseLevelRequired,
	ErrCourseCredentialIDInvalid, ErrCourseSpecialtyIDInvalid,
	ErrCourseLevelIDInvalid, ErrCertificateTemplateIDInvalid,
	ErrCourseCredentialRefNotFound, model.ErrSpecialtyNotFound,
	model.ErrCourseLevelNotFound, model.ErrCertificateTemplateNotFound,
	ErrCourseTheoryHoursNegative, ErrCoursePracticeHoursNegative,
	ErrCourseSortOrderNegative,
	ErrCoursePrerequisiteSelf, ErrCoursePrerequisiteNotFound,
	ErrCoursePrerequisiteCycle,
}

// @Summary 创建课程
// @Description 管理员创建课程（含培训目录扩展字段）
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "课程输入 {name,description,cover_image,duration,status,...}"
// @Success 201 {object} response.R{data=CourseDTO} "课程创建成功"
// @Failure 400 {object} response.R "输入不合法（挂载必填 / 引用ID无效或不存在 / 数值为负 / 前置课程冲突）"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/course [post]
// CreateCourse 创建课程 POST /api/admin/course
func (h *adminHandler) CreateCourse(c *gin.Context) {
	httpx.Endpoint[CourseInput, CourseDTO]{
		Parse: func(c *gin.Context) (*CourseInput, error) {
			return httpx.BindJSONMsg[CourseInput](c, "请求数据无效")
		},
		Invoke: func(ctx context.Context, req *CourseInput) (*CourseDTO, error) {
			return h.courseSvc.CreateCourse(req)
		},
	}.WithSuccess(httpx.Created("课程创建成功"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, courseWriteFacts400...).Handle(c)
}

// @Summary 管理端课程详情
// @Description 课程字段平铺 + chapters（含停用项与嵌套元数据）
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Success 200 {object} response.R{data=AdminCourseDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /admin/course/{course_id} [get]
// GetCourseDetail 课程详情 GET /api/admin/course/:course_id
func (h *adminHandler) GetCourseDetail(c *gin.Context) {
	httpx.Endpoint[idParam, AdminCourseDetailDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*AdminCourseDetailDTO, error) {
			return h.courseSvc.GetCourseDetail(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}

// @Summary 更新课程
// @Description 管理员更新课程字段（未携带的指针字段保留现状）
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "课程输入 {name,description,cover_image,duration,status,...}"
// @Success 200 {object} response.R{data=CourseDTO} "课程更新成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/course/{course_id} [put]
// UpdateCourse 更新课程 PUT /api/admin/course/:course_id
func (h *adminHandler) UpdateCourse(c *gin.Context) {
	httpx.Endpoint[courseIDInput, CourseDTO]{
		Parse: func(c *gin.Context) (*courseIDInput, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			data, err := httpx.BindJSONMsg[CourseInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &courseIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *courseIDInput) (*CourseDTO, error) {
			return h.courseSvc.UpdateCourse(req.ID, req.Input)
		},
	}.WithSuccess(httpx.OkMsg("课程更新成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).
		// 与 Create 面共用同一份表（ADR-0065 决策 3）：此前这里只挂了「必填」两条，
		// 于是「方向被引用成一张不存在的行」在编辑面上是 500。
		WithSentinels(http.StatusBadRequest, courseWriteFacts400...).Handle(c)
}

// courseSortFacts400 是课程交换排序端的表：目录侧那两件（不支持排序 / 待交换的项不存在）
// 加上课程侧独有的三件（未挂载、跨组、待交换的目标不在组内）。两枚共享哨兵的唯一出处是本包
// 依赖的叶子包 internal/sortorder（波 4d 破 course↔training 环：internal/training/catalog_service.go:18
// 已 import 本包，本包不能反向 import 它，故不能再复用 training.SortFacts400）。
// 其中 ErrEntityNotSortable 与 sortorder.ErrSwapItemNotFound 从课程这条链上**构造不出来**（课程开了排序；
// 两行都在函数里先 First 过）——仍留在共用表里，是因为「这一族的输入事实」应该只有一份清单；
// 真正可达性归零这件事写在这里，而不是靠测试去假装打过它。
var courseSortFacts400 = append(append([]error{}, sortorder.ErrEntityNotSortable, sortorder.ErrSwapItemNotFound),
	ErrCourseNotMountedForSort, ErrCourseSortGroupMismatch,
	ErrCourseSwapTargetNotFound)

// @Summary 交换课程排序
// @Description 同一方向+等级组内交换 sort_order，响应 data 为 null
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "交换请求 {swap_with}"
// @Success 200 {object} response.R "排序已交换"
// @Failure 400 {object} response.R "输入不合法（swap_with 参数无效 / 待交换的课程不存在 / 未挂载 / 跨组）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/course/{course_id}/sort [put]
// SwapCourseSort 交换课程排序 PUT /api/admin/course/:course_id/sort（同一方向+等级组内，body: {"swap_with": <id>}）
func (h *adminHandler) SwapCourseSort(c *gin.Context) {
	httpx.Endpoint[swapCourseSortReq, struct{}]{
		Parse: func(c *gin.Context) (*swapCourseSortReq, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
				return nil, httpx.BadRequest("swap_with 参数无效")
			}
			return &swapCourseSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: func(ctx context.Context, req *swapCourseSortReq) (*struct{}, error) {
			if err := h.courseSvc.SwapCourseSort(req.ID, req.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("排序已交换"), http.StatusInternalServerError).
		// 路径那门课不存在 ⇒ 404（此前落默认面 400：「你换的这门课没有」被说成「参数错了」）。
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).
		WithSentinels(http.StatusBadRequest, courseSortFacts400...).Handle(c)
}

// @Summary 删除课程
// @Description 管理员删除课程，返回被删除的 course_id
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Success 200 {object} response.R{data=DeleteCourseResult} "课程删除成功"
// @Failure 400 {object} response.R "课程ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /admin/course/{course_id} [delete]
// DeleteCourse 删除课程 DELETE /api/admin/course/:course_id
func (h *adminHandler) DeleteCourse(c *gin.Context) {
	httpx.Endpoint[idParam, DeleteCourseResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*DeleteCourseResult, error) {
			return h.courseSvc.DeleteCourse(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("课程删除成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}

// @Summary 创建章节
// @Description 管理员为课程创建章节
// @Tags 管理端-章节
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "章节输入 {title,content,duration,order_num,description}"
// @Success 201 {object} response.R{data=ChapterDTO} "章节创建成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/course/{course_id}/chapter [post]
// CreateChapter 创建章节 POST /api/admin/course/:course_id/chapter
func (h *adminHandler) CreateChapter(c *gin.Context) {
	httpx.Endpoint[chapterIDInput, ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			data, err := httpx.BindJSONMsg[ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*ChapterDTO, error) {
			return h.courseSvc.CreateChapter(req.ID, req.Input)
		},
	}.WithSuccess(httpx.Created("章节创建成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 更新章节
// @Description 管理员更新章节字段
// @Tags 管理端-章节
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节 ID"
// @Param body body object false "章节输入 {title,content,duration,order_num,description}"
// @Success 200 {object} response.R{data=ChapterDTO} "章节更新成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /admin/chapter/{chapter_id} [put]
// UpdateChapter 更新章节 PUT /api/admin/chapter/:chapter_id
func (h *adminHandler) UpdateChapter(c *gin.Context) {
	httpx.Endpoint[chapterIDInput, ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			data, err := httpx.BindJSONMsg[ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*ChapterDTO, error) {
			return h.courseSvc.UpdateChapter(req.ID, req.Input)
		},
	}.WithSuccess(httpx.OkMsg("章节更新成功"), http.StatusInternalServerError).
		WithSentinel(ErrChapterNotFound, http.StatusNotFound).Handle(c)
}

// @Summary 删除章节
// @Description 管理员删除章节，返回被删除的 chapter_id
// @Tags 管理端-章节
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节 ID"
// @Success 200 {object} response.R{data=DeleteChapterResult} "章节删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /admin/chapter/{chapter_id} [delete]
// DeleteChapter 删除章节 DELETE /api/admin/chapter/:chapter_id
func (h *adminHandler) DeleteChapter(c *gin.Context) {
	httpx.Endpoint[idParam, DeleteChapterResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*DeleteChapterResult, error) {
			return h.courseSvc.DeleteChapter(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("章节删除成功"), http.StatusInternalServerError).
		WithSentinel(ErrChapterNotFound, http.StatusNotFound).Handle(c)
}

// ===== Endpoint 请求类型（原 internal/api/admin.go:875-967 的共享私有类型副本）=====

// idParam 路径整型 ID 请求（course_id / chapter_id）。原 internal/api/admin.go:878 的共享私有类型；
// 域包不得引用装配根的私有名，故在本包落一份 1 字段副本 —— 形状与语义逐字一致。
type idParam struct {
	ID int
}

// courseIDInput 路径课程 ID + CourseInput 请求体（创建/更新课程）。原 internal/api/admin.go:888。
type courseIDInput struct {
	ID    int
	Input *CourseInput
}

// chapterIDInput 路径章节 ID + ChapterInput 请求体（创建/更新章节）。原 internal/api/admin.go:894。
type chapterIDInput struct {
	ID    int
	Input *ChapterInput
}

// swapCourseSortReq 交换课程排序请求（路径 course_id + body swap_with）。原 internal/api/admin.go:900。
type swapCourseSortReq struct {
	ID       int
	SwapWith int
}

// adminCourseListReq 管理端课程列表查询参数。原 internal/api/admin.go:945。
type adminCourseListReq struct {
	Page         int
	PageSize     int
	Keyword      string
	CredentialID *int
	SpecialtyID  *int
	LevelID      *int
	Filter       string
}
