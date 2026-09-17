package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// faqErrStatus 帮助中心哨兵→状态码表（ADR-0024 口径：按哨兵映射，不比对文案）。
// 校验类错误（空标题/空答案/标识格式/**标识已占用**）不是「资源不存在」语义，走 fallback 400。
// 标识冲突本可表达为 409，但本仓从未使用 409，且 renderStatus 的单一咽喉里没有 409 分支
// ——域表里放 409 会被静默渲染成 500（endpoint.go:130 的已知坑）。要引入 409 得同时动
// response 单点与前端状态映射，那是独立一拍，不在本票范围。
var faqErrStatus = &errStatusTable{
	entries: []errStatusEntry{
		{service.ErrFaqCategoryNotFound, http.StatusNotFound},
		{service.ErrFaqEntryNotFound, http.StatusNotFound},
	},
	fallback: http.StatusBadRequest,
}

// FaqHandler 帮助中心 handler（#1079）：学员端只读 + 管理端 CRUD。
type FaqHandler struct {
	svc *service.FaqService
}

// NewFaqHandler 创建帮助中心 handler。
func NewFaqHandler(svc *service.FaqService) *FaqHandler { return &FaqHandler{svc: svc} }

// RegisterFaqRoutes 注册帮助中心两条蓝图：
//   - 学员面 /api/faq（只读整页，能力点 faq.read）；
//   - 管理面 /api/admin/faq（分类与条目 CRUD，能力点 faq.manage）。
func RegisterFaqRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.FaqService) {
	h := NewFaqHandler(svc)

	g := rg.Group("/faq", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapFaqRead))
	g.GET("", h.List)

	ag := rg.Group("/admin/faq", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapFaqManage))
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
// @Success 200 {object} response.R{data=service.FaqResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /faq [get]
func (h *FaqHandler) List(c *gin.Context) {
	Endpoint[struct{}, service.FaqResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.FaqResult, error) {
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
// @Success 200 {object} response.R{data=service.AdminFaqCategoriesResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Router /admin/faq/categories [get]
func (h *FaqHandler) AdminListCategories(c *gin.Context) {
	Endpoint[struct{}, service.AdminFaqCategoriesResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqCategoriesResult, error) {
			items, err := h.svc.AdminListCategories()
			if err != nil {
				return nil, err
			}
			return &service.AdminFaqCategoriesResult{Categories: items}, nil
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
// @Success 200 {object} response.R{data=service.AdminFaqCategoryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 400 {object} response.R "分类标识已存在"
// @Router /admin/faq/categories [post]
func (h *FaqHandler) AdminCreateCategory(c *gin.Context) {
	var body faqCategoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	Endpoint[struct{}, service.AdminFaqCategoryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqCategoryDTO, error) {
			return h.svc.AdminCreateCategory(service.FaqCategoryInput{
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
// @Success 200 {object} response.R{data=service.AdminFaqCategoryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "分类不存在"
// @Failure 400 {object} response.R "分类标识已存在"
// @Router /admin/faq/categories/{id} [put]
func (h *FaqHandler) AdminUpdateCategory(c *gin.Context) {
	id, err := pathInt(c, "id", "分类 ID 无效")
	if err != nil {
		response.BadRequest(c, "分类 ID 无效")
		return
	}
	var body faqCategoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	Endpoint[struct{}, service.AdminFaqCategoryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqCategoryDTO, error) {
			return h.svc.AdminUpdateCategory(id, service.FaqCategoryInput{
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
func (h *FaqHandler) AdminDeleteCategory(c *gin.Context) {
	id, err := pathInt(c, "id", "分类 ID 无效")
	if err != nil {
		response.BadRequest(c, "分类 ID 无效")
		return
	}
	Endpoint[struct{}, struct{}]{
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
// @Success 200 {object} response.R{data=service.AdminFaqEntriesResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/faq/entries [get]
func (h *FaqHandler) AdminListEntries(c *gin.Context) {
	Endpoint[struct{}, service.AdminFaqEntriesResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqEntriesResult, error) {
			items, err := h.svc.AdminListEntries(queryIntPtr(c, "category_id"))
			if err != nil {
				return nil, err
			}
			return &service.AdminFaqEntriesResult{Entries: items}, nil
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
// @Success 200 {object} response.R{data=service.AdminFaqEntryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "分类不存在"
// @Router /admin/faq/entries [post]
func (h *FaqHandler) AdminCreateEntry(c *gin.Context) {
	var body faqEntryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	Endpoint[struct{}, service.AdminFaqEntryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqEntryDTO, error) {
			return h.svc.AdminCreateEntry(service.FaqEntryInput{
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
// @Success 200 {object} response.R{data=service.AdminFaqEntryDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 404 {object} response.R "条目或分类不存在"
// @Router /admin/faq/entries/{id} [put]
func (h *FaqHandler) AdminUpdateEntry(c *gin.Context) {
	id, err := pathInt(c, "id", "条目 ID 无效")
	if err != nil {
		response.BadRequest(c, "条目 ID 无效")
		return
	}
	var body faqEntryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	Endpoint[struct{}, service.AdminFaqEntryDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminFaqEntryDTO, error) {
			return h.svc.AdminUpdateEntry(id, service.FaqEntryInput{
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
func (h *FaqHandler) AdminDeleteEntry(c *gin.Context) {
	id, err := pathInt(c, "id", "条目 ID 无效")
	if err != nil {
		response.BadRequest(c, "条目 ID 无效")
		return
	}
	Endpoint[struct{}, struct{}]{
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			if err := h.svc.AdminDeleteEntry(id); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: faqErrStatus,
	}.Handle(c)
}
