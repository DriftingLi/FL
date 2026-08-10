// Package api 实现 HTTP handlers。
package api

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// FeaturedHandler 内容精选 handler。
type FeaturedHandler struct {
	svc     *service.FeaturedService
	fileSvc *service.FileService
}

// NewFeaturedHandler 创建内容精选 handler。
func NewFeaturedHandler(svc *service.FeaturedService, fileSvc *service.FileService) *FeaturedHandler {
	return &FeaturedHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterFeaturedRoutes 注册内容精选路由（公开 + 管理端）。
func RegisterFeaturedRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.FeaturedService, fileSvc *service.FileService) {
	h := NewFeaturedHandler(svc, fileSvc)

	// ===== 公开接口（无鉴权）=====
	rg.GET("/featured-contents", h.GetPublicList)
	rg.GET("/featured-content/:id", h.GetPublicDetail)
	rg.POST("/featured-content/:id/view", h.IncrementViewCount)

	// ===== 管理端接口（需 admin 角色）=====
	g := rg.Group("/admin", middleware.JWTAuth(sess), middleware.RoleRequired("admin"))
	g.GET("/featured-contents", h.AdminList)
	g.GET("/featured-content/:id", h.AdminDetail)
	g.POST("/featured-content", h.Create)
	g.PUT("/featured-content/:id", h.Update)
	g.DELETE("/featured-content/:id", h.Delete)
	g.POST("/featured-content/:id/publish", h.Publish)
	g.POST("/featured-content/upload-image", h.UploadImage)
}

// GetPublicList 内容精选列表（仅已发布）GET /api/featured-contents
func (h *FeaturedHandler) GetPublicList(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	category := c.Query("category")
	response.Success(c, h.svc.GetPublicList(page, pageSize, category))
}

// GetPublicDetail 内容精选详情（含相关资讯 + 上/下一篇）GET /api/featured-content/:id
// 带 no_view=1 时不改变 view_count（SSR/爬虫路径）；不带参数保持既有计数行为。
func (h *FeaturedHandler) GetPublicDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	countView := c.Query("no_view") != "1"
	result, err := h.svc.GetPublicDetail(id, countView)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// IncrementViewCount 客户端阅读量计数（真实浏览器 hydration 后调用）POST /api/featured-content/:id/view
func (h *FeaturedHandler) IncrementViewCount(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	count, err := h.svc.IncrementViewCount(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, gin.H{"content_id": id, "view_count": count})
}

// AdminList 管理端列表（含草稿）GET /api/admin/featured-contents
func (h *FeaturedHandler) AdminList(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	category := c.Query("category")
	status := c.Query("status")
	response.Success(c, h.svc.AdminList(page, pageSize, category, status))
}

// AdminDetail 管理端详情 GET /api/admin/featured-content/:id
func (h *FeaturedHandler) AdminDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	result, err := h.svc.AdminDetail(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// Create 创建内容精选 POST /api/admin/featured-content
func (h *FeaturedHandler) Create(c *gin.Context) {
	var data map[string]any
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.Create(data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "内容创建成功", result)
}

// Update 更新内容精选 PUT /api/admin/featured-content/:id
func (h *FeaturedHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	var data map[string]any
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.Update(id, data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "内容更新成功", result)
}

// Delete 删除内容精选 DELETE /api/admin/featured-content/:id
func (h *FeaturedHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	result, err := h.svc.Delete(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "内容删除成功", result)
}

// Publish 发布内容精选 POST /api/admin/featured-content/:id/publish
func (h *FeaturedHandler) Publish(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "内容ID无效")
		return
	}
	result, err := h.svc.Publish(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "内容发布成功", result)
}

// vditorFeatureError 构造 Vditor 期望的错误响应体。
func vditorFeatureError(msg string, errFiles []string) gin.H {
	return gin.H{"msg": msg, "code": 1, "data": gin.H{"errFiles": errFiles, "succMap": map[string]string{}}}
}

// UploadImage 上传图片（Markdown 编辑器内嵌 + 封面）POST /api/admin/featured-content/upload-image
// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
func (h *FeaturedHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(200, vditorFeatureError("未找到上传文件", []string{}))
		return
	}
	if file.Filename == "" {
		c.JSON(200, vditorFeatureError("未选择文件", []string{}))
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(200, vditorFeatureError("文件打开失败", []string{file.Filename}))
		return
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		c.JSON(200, vditorFeatureError("文件读取失败", []string{file.Filename}))
		return
	}
	// 校验图片格式与大小
	if ok, msg := h.fileSvc.ValidateImageFile(file.Filename, file.Size); !ok {
		c.JSON(200, vditorFeatureError(msg, []string{file.Filename}))
		return
	}
	url, err := h.svc.SaveImage(content, file.Filename)
	if err != nil {
		c.JSON(200, vditorFeatureError("文件保存失败", []string{file.Filename}))
		return
	}
	c.JSON(200, gin.H{
		"msg":  "",
		"code": 0,
		"data": gin.H{
			"errFiles": []string{},
			"succMap":  map[string]string{file.Filename: url},
		},
	})
}
