// Package api 实现 HTTP handlers。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// StudentHandler 学员端 handler。
type StudentHandler struct {
	svc *service.StudentService
}

// NewStudentHandler 创建学员端 handler。
func NewStudentHandler(svc *service.StudentService) *StudentHandler {
	return &StudentHandler{svc: svc}
}

// RegisterStudentRoutes 注册 /api/student 蓝图。
func RegisterStudentRoutes(rg *gin.RouterGroup, deps *Deps) {
	h := NewStudentHandler(deps.StudentSvc)

	g := rg.Group("/student", middleware.JWTAuth(deps.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/student/profile  学员信息+学习统计+课程进度
	g.GET("/profile", h.GetProfile)
	// GET /api/student/records  学员学习记录分页
	g.GET("/records", h.GetRecords)
	// GET /api/student/study-stats  学员仪表盘学习统计（按天分组，query: days=7|30）
	g.GET("/study-stats", h.GetStudyStats)
}

// GetProfile 学员信息+学习统计+课程进度 GET /api/student/profile
func (h *StudentHandler) GetProfile(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	result, err := h.svc.GetProfile(studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetRecords 学员学习记录分页 GET /api/student/records
func (h *StudentHandler) GetRecords(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	result := h.svc.GetRecords(studentID, page, pageSize, startDate, endDate)
	response.Success(c, result)
}

// GetStudyStats 学员仪表盘学习统计（按天分组）GET /api/student/study-stats
func (h *StudentHandler) GetStudyStats(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	days := atoiDefault(c.Query("days"), 7)
	result := h.svc.GetStudyStats(studentID, days)
	response.Success(c, result)
}
