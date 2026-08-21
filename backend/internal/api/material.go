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

// List 资料列表
// @Summary 学习资料列表
// @Description 基于 chapter_file 的聚合视图，支持按 course_id 过滤；与 /student/materials 同数据
// @Tags 学员端-资料
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id query int false "课程ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /materials [get]
// @Router /student/materials [get]
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

// Get 资料详情
// @Summary 资料详情
// @Description 查询单个资料详情
// @Tags 学员端-资料
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "资料ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /materials/{id} [get]
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

// Download 资料下载地址
// @Summary 资料下载地址
// @Description 返回 file_url/file_name 直链（静态资源）
// @Tags 学员端-资料
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "资料ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /materials/{id}/download [get]
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
