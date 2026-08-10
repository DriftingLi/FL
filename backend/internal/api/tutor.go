// Package api 实现 HTTP handlers。
package api

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/response"
)

// RegisterTutorRoutes 注册 /api/tutor 蓝图（导师管理章节与文件）。
// 导师不建课：课程创建/编辑由管理员负责（/api/admin/course*），导师仅管理章节。
func RegisterTutorRoutes(rg *gin.RouterGroup, deps *Deps) {
	svc := deps.TutorSvc
	fileSvc := deps.FileSvc

	g := rg.Group("/tutor", middleware.JWTAuth(deps.Session), middleware.RoleRequired("tutor"))

	// GET /api/tutor/courses  导师课程列表
	g.GET("/courses", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		response.Success(c, svc.GetCourses(nil, page, pageSize))
	})

	// GET /api/tutor/grading-stats  导师仪表盘阅卷统计（按天分组）
	//   query: days=7|30（其他值回退为 7）
	g.GET("/grading-stats", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		tutorID, _ := uid.(int)
		days := atoiDefault(c.Query("days"), 7)
		response.Success(c, svc.GetGradingStats(tutorID, days))
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

	// GET /api/tutor/chapter/:chapter_id  章节详情（含上下章ID + 文件列表）
	g.GET("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := svc.GetChapterDetail(chapterID)
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

	// POST /api/tutor/upload-image  上传图文 Markdown 中的图片（Vditor 格式）
	// form 字段：file（图片）、chapter_id（可选，用于按章节分目录存储 images/chapters/<chapterId>/）
	// 返回 Vditor 期望的响应格式：{ msg: "", code: 0, data: { errFiles: [], succMap: { "name": "url" } } }
	g.POST("/upload-image", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(200, gin.H{"msg": "未找到上传文件", "code": 1, "data": gin.H{"errFiles": []string{}, "succMap": map[string]string{}}})
			return
		}
		if file.Filename == "" {
			c.JSON(200, gin.H{"msg": "未选择文件", "code": 1, "data": gin.H{"errFiles": []string{}, "succMap": map[string]string{}}})
			return
		}
		src, err := file.Open()
		if err != nil {
			c.JSON(200, gin.H{"msg": "文件打开失败", "code": 1, "data": gin.H{"errFiles": []string{file.Filename}, "succMap": map[string]string{}}})
			return
		}
		defer src.Close()
		content, err := io.ReadAll(src)
		if err != nil {
			c.JSON(200, gin.H{"msg": "文件读取失败", "code": 1, "data": gin.H{"errFiles": []string{file.Filename}, "succMap": map[string]string{}}})
			return
		}
		if ok, msg := fileSvc.ValidateImageFile(file.Filename, file.Size); !ok {
			c.JSON(200, gin.H{"msg": msg, "code": 1, "data": gin.H{"errFiles": []string{file.Filename}, "succMap": map[string]string{}}})
			return
		}
		// 按章节分目录存储，便于删除章节时按前缀清理（历史旧目录孤儿文件不处理）
		// chapter_id 支持 query（Vditor 走 URL）与 form（直接 multipart）两种传递方式
		subfolder := "images/chapters"
		chapterIDStr := c.Query("chapter_id")
		if chapterIDStr == "" {
			chapterIDStr = c.PostForm("chapter_id")
		}
		if chapterID, err := strconv.Atoi(chapterIDStr); err == nil && chapterID > 0 {
			subfolder = fmt.Sprintf("images/chapters/%d", chapterID)
		}
		url, err := fileSvc.SaveFile(content, file.Filename, subfolder)
		if err != nil {
			c.JSON(200, gin.H{"msg": "文件保存失败", "code": 1, "data": gin.H{"errFiles": []string{file.Filename}, "succMap": map[string]string{}}})
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
