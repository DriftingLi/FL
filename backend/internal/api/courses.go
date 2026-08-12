// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// CourseHandler 学员侧课程 handler。
type CourseHandler struct {
	svc *service.CourseService
}

// NewCourseHandler 创建学员侧课程 handler。
func NewCourseHandler(svc *service.CourseService) *CourseHandler {
	return &CourseHandler{svc: svc}
}

// RegisterCoursesRoutes 注册 /api/courses 蓝图（学员侧课程浏览与学习进度）。
func RegisterCoursesRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.CourseService) {
	h := NewCourseHandler(svc)

	// 公开访问
	rg.GET("/courses", h.ListCourses)
	rg.GET("/chapter/:chapter_id/slides", h.GetChapterSlides)

	// 需要登录
	auth := rg.Group("", middleware.JWTAuth(sess))
	auth.GET("/course/:course_id", h.GetCourseDetail)
	auth.GET("/course/:course_id/chapter/:chapter_id", h.GetChapterDetail)
	auth.POST("/chapter/:chapter_id/slides/regenerate", h.RegenerateChapterSlides)
	auth.POST("/course/:course_id/progress", h.UpdateStudyProgress)
}

// ListCourses 课程列表 GET /api/courses（公开访问，可按专业方向/等级过滤）
func (h *CourseHandler) ListCourses(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 12)
	specialtyID := queryIntPtr(c, "specialty_id")
	levelID := queryIntPtr(c, "level_id")
	response.Success(c, h.svc.GetCourses(page, pageSize, specialtyID, levelID))
}

// GetChapterSlides 章节幻灯片 GET /api/chapter/:chapter_id/slides（公开访问）
func (h *CourseHandler) GetChapterSlides(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	result, err := h.svc.GetChapterSlides(chapterID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetCourseDetail 课程详情（含章节与学习进度）GET /api/course/:course_id
func (h *CourseHandler) GetCourseDetail(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	courseID, err := strconv.Atoi(c.Param("course_id"))
	if err != nil {
		response.BadRequest(c, "课程ID无效")
		return
	}
	result, err := h.svc.GetCourseDetail(courseID, studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetChapterDetail 章节详情 GET /api/course/:course_id/chapter/:chapter_id
func (h *CourseHandler) GetChapterDetail(c *gin.Context) {
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
	result, err := h.svc.GetChapterDetail(courseID, chapterID, studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// RegenerateChapterSlides 重新生成幻灯片 POST /api/chapter/:chapter_id/slides/regenerate
func (h *CourseHandler) RegenerateChapterSlides(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	result, err := h.svc.RegenerateChapterSlides(chapterID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "幻灯片重新生成成功", result)
}

// UpdateStudyProgress 更新学习进度 POST /api/course/:course_id/progress
func (h *CourseHandler) UpdateStudyProgress(c *gin.Context) {
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
	result, err := h.svc.UpdateStudyProgress(studentID, courseID, chapterID, req.Duration)
	if err != nil {
		response.ServerError(c, "更新进度失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "学习进度更新成功", result)
}
