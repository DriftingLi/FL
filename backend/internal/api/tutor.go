// Package api 实现 HTTP handlers。
package api

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// TutorHandler 导师端 handler。
type TutorHandler struct {
	svc     *service.TutorService
	fileSvc *service.FileService
}

// NewTutorHandler 创建导师端 handler。
func NewTutorHandler(svc *service.TutorService, fileSvc *service.FileService) *TutorHandler {
	return &TutorHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterTutorRoutes 注册 /api/tutor 蓝图。
func RegisterTutorRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.TutorService, fileSvc *service.FileService) {
	h := NewTutorHandler(svc, fileSvc)

	g := rg.Group("/tutor", middleware.JWTAuth(rd.Session), middleware.RoleRequired("tutor"))

	// GET /api/tutor/courses  导师课程列表
	g.GET("/courses", h.ListCourses)
	// GET /api/tutor/grading-stats  导师仪表盘阅卷统计（按天分组，query: days=7|30）
	g.GET("/grading-stats", h.GetGradingStats)
	// GET /api/tutor/course/:course_id/chapters  课程章节列表（含文件）
	g.GET("/course/:course_id/chapters", h.GetCourseChapters)
	// GET /api/tutor/chapter/:chapter_id  章节详情（含上下章ID + 文件列表）
	g.GET("/chapter/:chapter_id", h.GetChapterDetail)
	// POST /api/tutor/chapter/:chapter_id/upload  上传章节文件
	g.POST("/chapter/:chapter_id/upload", h.UploadChapterFile)
	// POST /api/tutor/upload-image  上传图文 Markdown 中的图片（Vditor 格式）
	g.POST("/upload-image", h.UploadImage)
	// PUT /api/tutor/chapter/:chapter_id  更新章节信息
	g.PUT("/chapter/:chapter_id", h.UpdateChapterInfo)
	// DELETE /api/tutor/file/:file_id  删除章节文件
	g.DELETE("/file/:file_id", h.DeleteChapterFile)
	// POST /api/tutor/files/batch-delete  批量删除文件
	g.POST("/files/batch-delete", h.BatchDeleteChapterFiles)
}

// ListCourses 导师课程列表 GET /api/tutor/courses
func (h *TutorHandler) ListCourses(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	specialtyID := queryIDPtr(c, "specialty_id")
	levelID := queryIDPtr(c, "level_id")
	response.Success(c, h.svc.GetCourses(page, pageSize, specialtyID, levelID))
}

// GetGradingStats 导师仪表盘阅卷统计 GET /api/tutor/grading-stats（按天分组）
func (h *TutorHandler) GetGradingStats(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	tutorID, _ := uid.(int)
	days := atoiDefault(c.Query("days"), 7)
	response.Success(c, h.svc.GetGradingStats(tutorID, days))
}

// GetCourseChapters 课程章节列表（含文件）GET /api/tutor/course/:course_id/chapters
func (h *TutorHandler) GetCourseChapters(c *gin.Context) {
	courseID, err := strconv.Atoi(c.Param("course_id"))
	if err != nil {
		response.BadRequest(c, "课程ID无效")
		return
	}
	result, err := h.svc.GetCourseChapters(courseID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetChapterDetail 章节详情（含上下章ID + 文件列表）GET /api/tutor/chapter/:chapter_id
func (h *TutorHandler) GetChapterDetail(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	result, err := h.svc.GetChapterDetail(chapterID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// UploadChapterFile 上传章节文件 POST /api/tutor/chapter/:chapter_id/upload
func (h *TutorHandler) UploadChapterFile(c *gin.Context) {
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
	result, err := h.svc.UploadChapterFile(chapterID, file.Filename, content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "文件上传成功", result)
}

// UploadImage 上传图文 Markdown 中的图片（Vditor 格式）POST /api/tutor/upload-image
// form 字段：file（图片）、chapter_id（可选，用于按章节分目录存储 images/chapters/<chapterId>/）
// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
func (h *TutorHandler) UploadImage(c *gin.Context) {
	// 按章节分目录存储，便于删除章节时按前缀清理（历史旧目录孤儿文件不处理）
	// chapter_id 支持 query（Vditor 走 URL）与 form（直接 multipart）两种传递方式
	uploadVditorImage(c, h.fileSvc, func(content []byte, filename string) (string, error) {
		subfolder := "images/chapters"
		chapterIDStr := c.Query("chapter_id")
		if chapterIDStr == "" {
			chapterIDStr = c.PostForm("chapter_id")
		}
		if chapterID, err := strconv.Atoi(chapterIDStr); err == nil && chapterID > 0 {
			subfolder = fmt.Sprintf("images/chapters/%d", chapterID)
		}
		return h.fileSvc.SaveFile(content, filename, subfolder)
	})
}

// UpdateChapterInfo 更新章节信息 PUT /api/tutor/chapter/:chapter_id
func (h *TutorHandler) UpdateChapterInfo(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	var data service.ChapterInput
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateChapterInfo(chapterID, &data)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "章节更新成功", result)
}

// DeleteChapterFile 删除章节文件 DELETE /api/tutor/file/:file_id
func (h *TutorHandler) DeleteChapterFile(c *gin.Context) {
	fileID, err := strconv.Atoi(c.Param("file_id"))
	if err != nil {
		response.BadRequest(c, "文件ID无效")
		return
	}
	result, err := h.svc.DeleteChapterFileByID(fileID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "文件删除成功", result)
}

// BatchDeleteChapterFiles 批量删除文件 POST /api/tutor/files/batch-delete
func (h *TutorHandler) BatchDeleteChapterFiles(c *gin.Context) {
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
	result := h.svc.BatchDeleteChapterFiles(req.FileIDs)
	response.SuccessWithMsg(c, "成功删除"+strconv.Itoa(result.SuccessCount)+"个文件", result)
}
