// Package api 实现 HTTP handlers。
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

// FeaturedHandler 内容精选 handler。
type FeaturedHandler struct {
	svc     *service.FeaturedService
	fileSvc *service.FileStore
}

// NewFeaturedHandler 创建内容精选 handler。
func NewFeaturedHandler(svc *service.FeaturedService, fileSvc *service.FileStore) *FeaturedHandler {
	return &FeaturedHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterFeaturedRoutes 注册内容精选路由（公开 + 管理端）。
func RegisterFeaturedRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.FeaturedService, fileSvc *service.FileStore) {
	h := NewFeaturedHandler(svc, fileSvc)

	// ===== 公开接口（无鉴权）=====
	rg.GET("/featured-contents", h.GetPublicList)
	rg.GET("/featured-content/:id", h.GetPublicDetail)
	rg.POST("/featured-content/:id/view", h.IncrementViewCount)

	// ===== 管理端接口（需 admin 角色）=====
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapContentManage))
	g.GET("/featured-contents", h.AdminList)
	g.GET("/featured-content/:id", h.AdminDetail)
	g.POST("/featured-content", h.Create)
	g.PUT("/featured-content/:id", h.Update)
	g.DELETE("/featured-content/:id", h.Delete)
	g.POST("/featured-content/:id/publish", h.Publish)
	g.POST("/featured-content/upload-image", h.UploadImage)
}

// GetPublicList 精选内容列表
// @Summary 精选内容列表（公开）
// @Description 仅已发布内容，支持按分类与排序（latest 按时间/hot 按浏览量）过滤
// @Tags 学员端-精选内容
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param category query string false "分类"
// @Param sort query string false "排序 latest|hot" default(latest)
// @Success 200 {object} response.R "success"
// @Router /featured-contents [get]
func (h *FeaturedHandler) GetPublicList(c *gin.Context) {
	Endpoint[featuredListReq, service.FeaturedContentPageResult]{
		Parse: func(c *gin.Context) (*featuredListReq, error) {
			return &featuredListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
				Category: c.Query("category"),
				Sort:     c.Query("sort"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *featuredListReq) (*service.FeaturedContentPageResult, error) {
			result, err := h.svc.GetPublicList(req.Page, req.PageSize, req.Category, req.Sort)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetPublicDetail 精选内容详情
// @Summary 精选内容详情（公开）
// @Description 含相关资讯与上下篇；no_view=1 时不计 view_count
// @Tags 学员端-精选内容
// @Accept json
// @Produce json
// @Param id path int true "内容ID"
// @Param no_view query string false "1 不计数"
// @Success 200 {object} response.R{data=service.FeaturedContentDetailDTO} "success"
// @Failure 404 {object} response.R "不存在"
// @Router /featured-content/{id} [get]
func (h *FeaturedHandler) GetPublicDetail(c *gin.Context) {
	Endpoint[featuredDetailReq, service.FeaturedContentDetailDTO]{
		Parse: func(c *gin.Context) (*featuredDetailReq, error) {
			id, err := pathInt(c, "id", "内容ID无效")
			if err != nil {
				return nil, err
			}
			return &featuredDetailReq{ID: id, CountView: c.Query("no_view") != "1"}, nil
		},
		Invoke: func(ctx context.Context, req *featuredDetailReq) (*service.FeaturedContentDetailDTO, error) {
			return h.svc.GetPublicDetail(req.ID, req.CountView)
		},
	}.WithSuccess(okMsg("success"), http.StatusNotFound).Handle(c)
}

// IncrementViewCount 精选阅读量
// @Summary 增加精选阅读量
// @Description 客户端 hydration 后计数
// @Tags 学员端-精选内容
// @Accept json
// @Produce json
// @Param id path int true "内容ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "不存在"
// @Router /featured-content/{id}/view [post]
func (h *FeaturedHandler) IncrementViewCount(c *gin.Context) {
	Endpoint[featuredIDReq, viewCountResp]{
		Parse: func(c *gin.Context) (*featuredIDReq, error) {
			id, err := pathInt(c, "id", "内容ID无效")
			if err != nil {
				return nil, err
			}
			return &featuredIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *featuredIDReq) (*viewCountResp, error) {
			count, err := h.svc.IncrementViewCount(req.ID)
			if err != nil {
				return nil, err
			}
			return &viewCountResp{ID: req.ID, Count: count}, nil
		},
		// 判定不动（票8 逐端点判过）：FeaturedService.IncrementViewCount 的「内容不存在」是裸
		// errors.New，自增失败的驱动错误原样上抛 ⇒ api 侧无哨兵可分档，改判会把真 404 变 500。
		ErrStatus: errStatusAll(http.StatusNotFound),
		Render: func(c *gin.Context, _ *featuredIDReq, resp *viewCountResp) {
			response.Success(c, gin.H{"content_id": resp.ID, "view_count": resp.Count})
		},
	}.Handle(c)
}

// @Summary 精选内容列表（管理端）
// @Description 管理端列表（含草稿），支持分类与状态过滤
// @Tags 管理端-精选内容
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param category query string false "分类"
// @Param status query string false "状态"
// @Success 200 {object} response.R{data=service.FeaturedContentPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/featured-contents [get]
// AdminList 管理端列表（含草稿）GET /api/admin/featured-contents
func (h *FeaturedHandler) AdminList(c *gin.Context) {
	Endpoint[adminFeaturedListReq, service.FeaturedContentPageResult]{
		Parse: func(c *gin.Context) (*adminFeaturedListReq, error) {
			return &adminFeaturedListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
				Category: c.Query("category"),
				Status:   c.Query("status"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *adminFeaturedListReq) (*service.FeaturedContentPageResult, error) {
			result, err := h.svc.AdminList(req.Page, req.PageSize, req.Category, req.Status)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 精选内容详情（管理端）
// @Description 管理端详情（列表项 + 正文，无相关资讯与上下篇）
// @Tags 管理端-精选内容
// @Produce json
// @Security BearerAuth
// @Param id path int true "内容 ID"
// @Success 200 {object} response.R{data=service.FeaturedContentAdminDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "内容不存在"
// @Router /admin/featured-content/{id} [get]
// AdminDetail 管理端详情 GET /api/admin/featured-content/:id
func (h *FeaturedHandler) AdminDetail(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.AdminDetail(req.ID)
		},
	}.WithSuccess(okMsg("success"), http.StatusNotFound).Handle(c)
}

// @Summary 创建精选内容
// @Description 管理员创建内容（status 缺省为草稿）
// @Tags 管理端-精选内容
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "内容输入 {title,category,summary,cover_image,content,source,status,sort_order}"
// @Success 201 {object} response.R{data=service.FeaturedContentAdminDetailDTO} "内容创建成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/featured-content [post]
// Create 创建内容精选 POST /api/admin/featured-content
func (h *FeaturedHandler) Create(c *gin.Context) {
	Endpoint[service.FeaturedContentInput, service.FeaturedContentAdminDetailDTO]{
		Parse: func(c *gin.Context) (*service.FeaturedContentInput, error) {
			return bindJSONMsg[service.FeaturedContentInput](c, "请求数据无效")
		},
		Invoke: func(ctx context.Context, req *service.FeaturedContentInput) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Create(*req)
		},
	}.WithSuccess(created("内容创建成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 更新精选内容
// @Description 管理员更新内容（未携带的字段保留现状）
// @Tags 管理端-精选内容
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "内容 ID"
// @Param body body object false "内容输入 {title,category,summary,cover_image,content,source,status,sort_order}"
// @Success 200 {object} response.R{data=service.FeaturedContentAdminDetailDTO} "内容更新成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/featured-content/{id} [put]
// Update 更新内容精选 PUT /api/admin/featured-content/:id
func (h *FeaturedHandler) Update(c *gin.Context) {
	Endpoint[featuredUpdateReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedUpdate,
		Invoke: func(ctx context.Context, req *featuredUpdateReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Update(req.ID, req.Input)
		},
	}.WithSuccess(okMsg("内容更新成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 删除精选内容
// @Description 管理员删除内容，返回被删除的 content_id
// @Tags 管理端-精选内容
// @Produce json
// @Security BearerAuth
// @Param id path int true "内容 ID"
// @Success 200 {object} response.R{data=service.FeaturedDeleteResult} "内容删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "内容不存在"
// @Router /admin/featured-content/{id} [delete]
// Delete 删除内容精选 DELETE /api/admin/featured-content/:id
func (h *FeaturedHandler) Delete(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedDeleteResult]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedDeleteResult, error) {
			return h.svc.Delete(req.ID)
		},
	}.WithSuccess(okMsg("内容删除成功"), http.StatusNotFound).Handle(c)
}

// @Summary 发布精选内容
// @Description 草稿 → 已发布，返回发布后的内容
// @Tags 管理端-精选内容
// @Produce json
// @Security BearerAuth
// @Param id path int true "内容 ID"
// @Success 200 {object} response.R{data=service.FeaturedContentAdminDetailDTO} "内容发布成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "内容不存在"
// @Router /admin/featured-content/{id}/publish [post]
// Publish 发布内容精选 POST /api/admin/featured-content/:id/publish
func (h *FeaturedHandler) Publish(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Publish(req.ID)
		},
	}.WithSuccess(okMsg("内容发布成功"), http.StatusNotFound).Handle(c)
}

// @Summary 上传精选内容图片
// @Description Markdown 编辑器内嵌图 + 封面上传；返回 Vditor 约定的**非统一信封**响应 {msg,code,data:{errFiles,succMap}}（code=0 表示成功）
// @Tags 管理端-精选内容
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "图片文件"
// @Success 200 {object} map[string]any "Vditor 响应（非统一信封）"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/featured-content/upload-image [post]
// UploadImage 上传图片（Markdown 编辑器内嵌 + 封面）POST /api/admin/featured-content/upload-image
// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
func (h *FeaturedHandler) UploadImage(c *gin.Context) {
	// 封面/内嵌图存 featured 目录（FeaturedService.SaveImage 单点）
	uploadVditorImage(c, h.fileSvc, h.svc.SaveImage)
}

// featuredListReq 公开内容精选列表查询参数。
type featuredListReq struct {
	Page     int
	PageSize int
	Category string
	Sort     string
}

// adminFeaturedListReq 管理端内容精选列表查询参数。
type adminFeaturedListReq struct {
	Page     int
	PageSize int
	Category string
	Status   string
}

// featuredDetailReq 内容详情请求（id + countView）。
type featuredDetailReq struct {
	ID        int
	CountView bool
}

// featuredIDReq 仅 ID 的请求（详情/删除/发布/计数）。
type featuredIDReq struct {
	ID int
}

// featuredUpdateReq 更新请求（id + typed input）。
type featuredUpdateReq struct {
	ID    int
	Input service.FeaturedContentUpdateInput
}

// viewCountResp 计数响应（content_id + view_count）。
type viewCountResp struct {
	ID    int
	Count int
}

func parseFeaturedID(c *gin.Context) (*featuredIDReq, error) {
	id, err := pathInt(c, "id", "内容ID无效")
	if err != nil {
		return nil, err
	}
	return &featuredIDReq{ID: id}, nil
}

func parseFeaturedUpdate(c *gin.Context) (*featuredUpdateReq, error) {
	id, err := pathInt(c, "id", "内容ID无效")
	if err != nil {
		return nil, err
	}
	input, err := bindJSONMsg[service.FeaturedContentUpdateInput](c, "请求数据无效")
	if err != nil {
		return nil, err
	}
	return &featuredUpdateReq{ID: id, Input: *input}, nil
}
