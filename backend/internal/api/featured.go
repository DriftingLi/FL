// Package api 实现 HTTP handlers。
package api

import (
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterFeaturedRoutes 注册内容精选路由（公开 + 管理端）。
func RegisterFeaturedRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	fileSvc := service.NewFileService(cfg.UploadFolder)
	svc := service.NewFeaturedService(db, fileSvc)

	// ===== 公开接口（无鉴权）=====

	// GET /api/featured-contents  内容精选列表（仅已发布）
	rg.GET("/featured-contents", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		category := c.Query("category")
		response.Success(c, svc.GetPublicList(page, pageSize, category))
	})

	// GET /api/featured-content/:id  内容精选详情（含相关资讯 + 上/下一篇）
	rg.GET("/featured-content/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "内容ID无效")
			return
		}
		result, err := svc.GetPublicDetail(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// ===== 管理端接口（需 admin 角色）=====

	g := rg.Group("/admin", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// GET /api/admin/featured-contents  管理端列表（含草稿）
	g.GET("/featured-contents", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		category := c.Query("category")
		status := c.Query("status")
		response.Success(c, svc.AdminList(page, pageSize, category, status))
	})

	// GET /api/admin/featured-content/:id  管理端详情
	g.GET("/featured-content/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "内容ID无效")
			return
		}
		result, err := svc.AdminDetail(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/admin/featured-content  创建内容精选
	g.POST("/featured-content", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.Create(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "内容创建成功", result)
	})

	// PUT /api/admin/featured-content/:id  更新内容精选
	g.PUT("/featured-content/:id", func(c *gin.Context) {
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
		result, err := svc.Update(id, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "内容更新成功", result)
	})

	// DELETE /api/admin/featured-content/:id  删除内容精选
	g.DELETE("/featured-content/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "内容ID无效")
			return
		}
		result, err := svc.Delete(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "内容删除成功", result)
	})

	// POST /api/admin/featured-content/:id/publish  发布内容精选
	g.POST("/featured-content/:id/publish", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "内容ID无效")
			return
		}
		result, err := svc.Publish(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "内容发布成功", result)
	})

	// POST /api/admin/featured-content/upload-image  上传图片（Markdown 编辑器内嵌 + 封面）
	// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
	g.POST("/featured-content/upload-image", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(200, gin.H{
				"msg":  "未找到上传文件",
				"code": 1,
				"data": gin.H{
					"errFiles": []string{},
					"succMap":  map[string]string{},
				},
			})
			return
		}
		if file.Filename == "" {
			c.JSON(200, gin.H{
				"msg":  "未选择文件",
				"code": 1,
				"data": gin.H{
					"errFiles": []string{},
					"succMap":  map[string]string{},
				},
			})
			return
		}
		src, err := file.Open()
		if err != nil {
			c.JSON(200, gin.H{
				"msg":  "文件打开失败",
				"code": 1,
				"data": gin.H{
					"errFiles": []string{file.Filename},
					"succMap":  map[string]string{},
				},
			})
			return
		}
		defer src.Close()
		content, err := io.ReadAll(src)
		if err != nil {
			c.JSON(200, gin.H{
				"msg":  "文件读取失败",
				"code": 1,
				"data": gin.H{
					"errFiles": []string{file.Filename},
					"succMap":  map[string]string{},
				},
			})
			return
		}
		// 校验图片格式与大小
		if ok, msg := fileSvc.ValidateImageFile(file.Filename, file.Size); !ok {
			c.JSON(200, gin.H{
				"msg":  msg,
				"code": 1,
				"data": gin.H{
					"errFiles": []string{file.Filename},
					"succMap":  map[string]string{},
				},
			})
			return
		}
		url, err := svc.SaveImage(content, file.Filename)
		if err != nil {
			c.JSON(200, gin.H{
				"msg":  "文件保存失败",
				"code": 1,
				"data": gin.H{
					"errFiles": []string{file.Filename},
					"succMap":  map[string]string{},
				},
			})
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
	})
}
