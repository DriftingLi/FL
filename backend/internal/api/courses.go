// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterCoursesRoutes 注册 /api/courses 蓝图（学员侧课程浏览与学习进度）。
// 对应 Python app/api/courses.py。
func RegisterCoursesRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewCourseService(db, cfg.UploadFolder, service.NewFileService(cfg.UploadFolder))

	// GET /api/courses  课程列表（公开访问）
	rg.GET("/courses", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 12)
		category := c.Query("category")
		response.Success(c, svc.GetCourses(page, pageSize, category))
	})

	// GET /api/chapter/:chapter_id/slides  章节幻灯片（公开访问）
	rg.GET("/chapter/:chapter_id/slides", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := svc.GetChapterSlides(chapterID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// 以下路由需要登录
	auth := rg.Group("", middleware.JWTAuth(cfg))

	// GET /api/course/:course_id  课程详情（含章节与学习进度）
	auth.GET("/course/:course_id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := svc.GetCourseDetail(courseID, studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/course/:course_id/chapter/:chapter_id  章节详情
	auth.GET("/course/:course_id/chapter/:chapter_id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := svc.GetChapterDetail(courseID, chapterID, studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/chapter/:chapter_id/slides/regenerate  重新生成幻灯片
	auth.POST("/chapter/:chapter_id/slides/regenerate", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := svc.RegenerateChapterSlides(chapterID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "幻灯片重新生成成功", result)
	})

	// POST /api/course/:course_id/progress  更新学习进度
	auth.POST("/course/:course_id/progress", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var req struct {
			ChapterID *int `json:"chapter_id"`
			Duration  int  `json:"duration"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Duration < 0 {
			response.BadRequest(c, "学习时长不能为负数")
			return
		}
		chapterID := 0
		if req.ChapterID != nil {
			chapterID = *req.ChapterID
		}
		result, err := svc.UpdateStudyProgress(studentID, courseID, chapterID, req.Duration)
		if err != nil {
			response.ServerError(c, "更新进度失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "学习进度更新成功", result)
	})
}
