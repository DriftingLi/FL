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

// RegisterTutorRoutes 注册 /api/tutor 蓝图（导师管理章节与文件）。
// 对应 Python app/api/tutor.py。
func RegisterTutorRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewTutorService(db, cfg.UploadFolder, service.NewFileService(cfg.UploadFolder))

	g := rg.Group("/tutor", middleware.JWTAuth(cfg), middleware.RoleRequired("tutor"))

	// GET /api/tutor/courses  导师课程列表
	g.GET("/courses", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		response.Success(c, svc.GetCourses(nil, page, pageSize))
	})

	// GET /api/tutor/course/:course_id/chapters  课程章节列表（含文件）
	g.GET("/course/:course_id/chapters", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := svc.GetCourseChapters(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/tutor/chapter/:chapter_id/upload  上传章节文件
	g.POST("/chapter/:chapter_id/upload", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		file, err := c.FormFile("file")
		if err != nil {
			response.BadRequest(c, "未找到上传文件")
			return
		}
		if file.Filename == "" {
			response.BadRequest(c, "未选择文件")
			return
		}
		src, err := file.Open()
		if err != nil {
			response.ServerError(c, "文件上传失败")
			return
		}
		defer src.Close()
		content, err := io.ReadAll(src)
		if err != nil {
			response.ServerError(c, "文件上传失败")
			return
		}
		result, err := svc.UploadChapterFile(chapterID, file.Filename, content)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "文件上传成功", result)
	})

	// PUT /api/tutor/chapter/:chapter_id  更新章节信息
	g.PUT("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateChapterInfo(chapterID, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "章节更新成功", result)
	})

	// DELETE /api/tutor/file/:file_id  删除章节文件
	g.DELETE("/file/:file_id", func(c *gin.Context) {
		fileID, err := strconv.Atoi(c.Param("file_id"))
		if err != nil {
			response.BadRequest(c, "文件ID无效")
			return
		}
		result, err := svc.DeleteChapterFileByID(fileID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "文件删除成功", result)
	})

	// POST /api/tutor/files/batch-delete  批量删除文件
	g.POST("/files/batch-delete", func(c *gin.Context) {
		var req struct {
			FileIDs []int `json:"file_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.FileIDs) == 0 {
			response.BadRequest(c, "请选择要删除的文件")
			return
		}
		result := svc.BatchDeleteChapterFiles(req.FileIDs)
		count, _ := result["success_count"].(int)
		response.SuccessWithMsg(c, "成功删除"+strconv.Itoa(count)+"个文件", result)
	})
}
