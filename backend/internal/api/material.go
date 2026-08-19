// Package api 实现 HTTP handlers。
// 本文件：学习资料聚合（ADR-0018 低成本路径）—— chapter_file 附件视图，
// /api/materials 与 /api/student/materials 同数据（清单别名）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// MaterialHandler 学习资料 handler。
type MaterialHandler struct {
	svc *service.MaterialService
}

// NewMaterialHandler 创建学习资料 handler。
func NewMaterialHandler(svc *service.MaterialService) *MaterialHandler {
	return &MaterialHandler{svc: svc}
}

// RegisterMaterialRoutes 注册 /api/materials 蓝图（JWT + hrwai_user）。
func RegisterMaterialRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.MaterialService) {
	h := NewMaterialHandler(svc)

	g := rg.Group("", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/materials?course_id=&page=&page_size= 资料列表
	g.GET("/materials", h.List)
	// GET /api/materials/:id 资料详情
	g.GET("/materials/:id", h.Get)
	// GET /api/materials/:id/download 获取下载地址（file_url 为静态直链）
	g.GET("/materials/:id/download", h.Download)
	// GET /api/student/materials 学员可访问资料（同列表，清单别名）
	g.GET("/student/materials", h.List)
}

// List 资料列表 GET /api/materials
func (h *MaterialHandler) List(c *gin.Context) {
	resp, err := h.svc.ListMaterials(
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20),
		atoiDefault(c.Query("course_id"), 0))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Get 资料详情 GET /api/materials/:id
func (h *MaterialHandler) Get(c *gin.Context) {
	id, err := pathInt(c, "id", "资料 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.svc.GetMaterial(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, resp)
}

// Download 下载地址 GET /api/materials/:id/download
func (h *MaterialHandler) Download(c *gin.Context) {
	id, err := pathInt(c, "id", "资料 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.svc.GetMaterial(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, gin.H{"file_url": resp.FileURL, "file_name": resp.FileName})
}
