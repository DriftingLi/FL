// Package api 实现 HTTP handlers。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

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
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
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
// @Description 仅已发布内容，支持按分类过滤
// @Tags 学员端-精选内容
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param category query string false "分类"
// @Success 200 {object} response.R "success"
// @Router /featured-contents [get]
func (h *FeaturedHandler) GetPublicList(c *gin.Context) {
	Endpoint[featuredListReq, service.FeaturedContentPageResult]{
		Parse: func(c *gin.Context) (*featuredListReq, error) {
			return &featuredListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
				Category: c.Query("category"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *featuredListReq) (*service.FeaturedContentPageResult, error) {
			result := h.svc.GetPublicList(req.Page, req.PageSize, req.Category)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *featuredListReq, resp *service.FeaturedContentPageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetPublicDetail 精选内容详情
// @Summary 精选内容详情（公开）
// @Description 含相关资讯与上下篇；no_view=1 时不计 view_count
// @Tags 学员端-精选内容
// @Accept json
// @Produce json
// @Param id path int true "内容ID"
// @Param no_view query string false "1 不计数"
// @Success 200 {object} response.R "success"
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
		Render: func(c *gin.Context, _ *featuredDetailReq, resp *service.FeaturedContentDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
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
		Render: func(c *gin.Context, _ *featuredIDReq, resp *viewCountResp, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, gin.H{"content_id": resp.ID, "view_count": resp.Count})
		},
	}.Handle(c)
}

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
			result := h.svc.AdminList(req.Page, req.PageSize, req.Category, req.Status)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *adminFeaturedListReq, resp *service.FeaturedContentPageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// AdminDetail 管理端详情 GET /api/admin/featured-content/:id
func (h *FeaturedHandler) AdminDetail(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.AdminDetail(req.ID)
		},
		Render: func(c *gin.Context, _ *featuredIDReq, resp *service.FeaturedContentAdminDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Create 创建内容精选 POST /api/admin/featured-content
func (h *FeaturedHandler) Create(c *gin.Context) {
	Endpoint[service.FeaturedContentInput, service.FeaturedContentAdminDetailDTO]{
		Parse: func(c *gin.Context) (*service.FeaturedContentInput, error) {
			return bindJSONMsg[service.FeaturedContentInput](c, "请求数据无效")
		},
		Invoke: func(ctx context.Context, req *service.FeaturedContentInput) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Create(*req)
		},
		Render: func(c *gin.Context, _ *service.FeaturedContentInput, resp *service.FeaturedContentAdminDetailDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "内容创建成功", resp)
		},
	}.Handle(c)
}

// Update 更新内容精选 PUT /api/admin/featured-content/:id
func (h *FeaturedHandler) Update(c *gin.Context) {
	Endpoint[featuredUpdateReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedUpdate,
		Invoke: func(ctx context.Context, req *featuredUpdateReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Update(req.ID, req.Input)
		},
		Render: func(c *gin.Context, _ *featuredUpdateReq, resp *service.FeaturedContentAdminDetailDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "内容更新成功", resp)
		},
	}.Handle(c)
}

// Delete 删除内容精选 DELETE /api/admin/featured-content/:id
func (h *FeaturedHandler) Delete(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedDeleteResult]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedDeleteResult, error) {
			return h.svc.Delete(req.ID)
		},
		Render: func(c *gin.Context, _ *featuredIDReq, resp *service.FeaturedDeleteResult, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "内容删除成功", resp)
		},
	}.Handle(c)
}

// Publish 发布内容精选 POST /api/admin/featured-content/:id/publish
func (h *FeaturedHandler) Publish(c *gin.Context) {
	Endpoint[featuredIDReq, service.FeaturedContentAdminDetailDTO]{
		Parse: parseFeaturedID,
		Invoke: func(ctx context.Context, req *featuredIDReq) (*service.FeaturedContentAdminDetailDTO, error) {
			return h.svc.Publish(req.ID)
		},
		Render: func(c *gin.Context, _ *featuredIDReq, resp *service.FeaturedContentAdminDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "内容发布成功", resp)
		},
	}.Handle(c)
}

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
