package faq

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// faqErrStatus 帮助中心哨兵→状态码表（ADR-0024 口径：按哨兵映射，不比对文案）。
// 校验类错误（空标题/空答案/标识格式/**标识已占用**）不是「资源不存在」语义，走 fallback 400。
// 标识冲突本可表达为 409，但本仓从未使用 409，且 renderStatus 的单一咽喉里没有 409 分支
// ——域表里放 409 会被静默渲染成 500（pkg/httpx/endpoint.go:130 的已知坑）。要引入 409 得同时动
// response 单点与前端状态映射，那是独立一拍，不在本票范围。
var faqErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrFaqCategoryNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrFaqEntryNotFound, Status: http.StatusNotFound},
	},
	Fallback: http.StatusBadRequest,
}

// handler 帮助中心 handler（#1079）：学员端只读 + 管理端 CRUD。
type handler struct {
	svc *Service
}

// newHandler 创建帮助中心 handler。
func newHandler(svc *Service) *handler { return &handler{svc: svc} }

// RegisterRoutes 注册帮助中心两条蓝图：
//   - 学员面 /api/faq（只读整页，能力点 faq.read）；
//   - 管理面 /api/admin/faq（分类与条目 CRUD，能力点 faq.manage）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/faq", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapFaqRead))
	g.GET("", h.List)

	ag := rg.Group("/admin/faq", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapFaqManage))
	ag.GET("/categories", h.AdminListCategories)
	ag.POST("/categories", h.AdminCreateCategory)
	ag.PUT("/categories/:id", h.AdminUpdateCategory)
	ag.DELETE("/categories/:id", h.AdminDeleteCategory)
	ag.GET("/entries", h.AdminListEntries)
	ag.POST("/entries", h.AdminCreateEntry)
	ag.PUT("/entries/:id", h.AdminUpdateEntry)
	ag.DELETE("/entries/:id", h.AdminDeleteEntry)
}

// List 帮助中心（学员端）
// @Summary 帮助中心
// @Description 一次返回全部分类与其下已发布条目（按 sort_order 升序）；搜索走端上过滤
// @Tags 学员端-帮助中心
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=faq.FaqResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /faq [get]
func (h *handler) List(c *gin.Context) {
	httpx.Endpoint[struct{}, FaqResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*FaqResult, error) {
			return h.svc.ListPublished()
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminListCategories 分类清单（管理端）
// @Summary 帮助中心分类清单
// @Description 返回全部分类（含停用）与各自的条目计数
// @Tags 管理端-帮助中心
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=faq.AdminFaqCategoriesResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Router /admin/faq/categories [get]
func (h *handler) AdminListCategories(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminFaqCategoriesResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqCategoriesResult, error) {
			items, err := h.svc.AdminListCategories()
			if err != nil {
				return nil, err
			}
			return &AdminFaqCategoriesResult{Categories: items}, nil
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// faqCategoryBody 分类请求体（PUT 为整体替换语义，字段一律按提交值落库）。
type faqCategoryBody struct {
	Code      string `json:"code"`
	Title     string `json:"title"`
	SortOrder int    `json:"sort_order"`
	Enabled   bool   `json:"enabled"`
}

// AdminCreateCategory 新建分类
// @Summary 新建帮助中心分类
// @Tags 管理端-帮助中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "分类" example({"code":"account","title":"账号与登录","sort_order":1,"enabled":true})
// @Success 200 {object} response.R{data=faq.AdminFaqCategoryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 400 {object} response.R "分类标识已存在"
// @Router /admin/faq/categories [post]
func (h *handler) AdminCreateCategory(c *gin.Context) {
	var body faqCategoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	httpx.Endpoint[struct{}, AdminFaqCategoryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqCategoryDTO, error) {
			return h.svc.AdminCreateCategory(FaqCategoryInput{
				Code: body.Code, Title: body.Title, SortOrder: body.SortOrder, Enabled: body.Enabled,
			})
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminUpdateCategory 改分类
// @Summary 改帮助中心分类
// @Description 整体替换：提交什么就落什么
// @Tags 管理端-帮助中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Param body body object true "分类"
// @Success 200 {object} response.R{data=faq.AdminFaqCategoryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "分类不存在"
// @Failure 400 {object} response.R "分类标识已存在"
// @Router /admin/faq/categories/{id} [put]
func (h *handler) AdminUpdateCategory(c *gin.Context) {
	id, err := httpx.PathInt(c, "id", "分类 ID 无效")
	if err != nil {
		response.BadRequest(c, "分类 ID 无效")
		return
	}
	var body faqCategoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	httpx.Endpoint[struct{}, AdminFaqCategoryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqCategoryDTO, error) {
			return h.svc.AdminUpdateCategory(id, FaqCategoryInput{
				Code: body.Code, Title: body.Title, SortOrder: body.SortOrder, Enabled: body.Enabled,
			})
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminDeleteCategory 删分类（连带删除其下全部条目）
// @Summary 删帮助中心分类
// @Description ⚠️ 外键 CASCADE：其下条目一并删除。管理端确认框必须写明这一点。
// @Tags 管理端-帮助中心
// @Produce json
// @Security BearerAuth
// @Param id path int true "分类 ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "分类不存在"
// @Router /admin/faq/categories/{id} [delete]
func (h *handler) AdminDeleteCategory(c *gin.Context) {
	id, err := httpx.PathInt(c, "id", "分类 ID 无效")
	if err != nil {
		response.BadRequest(c, "分类 ID 无效")
		return
	}
	httpx.Endpoint[struct{}, struct{}]{
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			if err := h.svc.AdminDeleteCategory(id); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminListEntries 条目清单（管理端）
// @Summary 帮助中心条目清单
// @Description 返回全部条目（含未发布），可按分类过滤
// @Tags 管理端-帮助中心
// @Produce json
// @Security BearerAuth
// @Param category_id query int false "按分类过滤"
// @Success 200 {object} response.R{data=faq.AdminFaqEntriesResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/faq/entries [get]
func (h *handler) AdminListEntries(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminFaqEntriesResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqEntriesResult, error) {
			items, err := h.svc.AdminListEntries(httpx.QueryIntPtr(c, "category_id"))
			if err != nil {
				return nil, err
			}
			return &AdminFaqEntriesResult{Entries: items}, nil
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// faqEntryBody 条目请求体（PUT 为整体替换语义）。
type faqEntryBody struct {
	CategoryID int    `json:"category_id"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	SortOrder  int    `json:"sort_order"`
	Published  bool   `json:"published"`
}

// AdminCreateEntry 新建条目
// @Summary 新建帮助中心条目
// @Tags 管理端-帮助中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "条目"
// @Success 200 {object} response.R{data=faq.AdminFaqEntryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "分类不存在"
// @Router /admin/faq/entries [post]
func (h *handler) AdminCreateEntry(c *gin.Context) {
	var body faqEntryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	httpx.Endpoint[struct{}, AdminFaqEntryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqEntryDTO, error) {
			return h.svc.AdminCreateEntry(FaqEntryInput{
				CategoryID: body.CategoryID, Question: body.Question, Answer: body.Answer,
				SortOrder: body.SortOrder, Published: body.Published,
			})
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminUpdateEntry 改条目
// @Summary 改帮助中心条目
// @Description 整体替换：提交什么就落什么
// @Tags 管理端-帮助中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "条目 ID"
// @Param body body object true "条目"
// @Success 200 {object} response.R{data=faq.AdminFaqEntryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "条目或分类不存在"
// @Router /admin/faq/entries/{id} [put]
func (h *handler) AdminUpdateEntry(c *gin.Context) {
	id, err := httpx.PathInt(c, "id", "条目 ID 无效")
	if err != nil {
		response.BadRequest(c, "条目 ID 无效")
		return
	}
	var body faqEntryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	httpx.Endpoint[struct{}, AdminFaqEntryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminFaqEntryDTO, error) {
			return h.svc.AdminUpdateEntry(id, FaqEntryInput{
				CategoryID: body.CategoryID, Question: body.Question, Answer: body.Answer,
				SortOrder: body.SortOrder, Published: body.Published,
			})
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}

// AdminDeleteEntry 删条目
// @Summary 删帮助中心条目
// @Tags 管理端-帮助中心
// @Produce json
// @Security BearerAuth
// @Param id path int true "条目 ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "条目不存在"
// @Router /admin/faq/entries/{id} [delete]
func (h *handler) AdminDeleteEntry(c *gin.Context) {
	id, err := httpx.PathInt(c, "id", "条目 ID 无效")
	if err != nil {
		response.BadRequest(c, "条目 ID 无效")
		return
	}
	httpx.Endpoint[struct{}, struct{}]{
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			if err := h.svc.AdminDeleteEntry(id); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}
