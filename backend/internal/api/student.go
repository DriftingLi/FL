// Package api 实现 HTTP handlers。
package api

import (
	"context"

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
func RegisterStudentRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.StudentService) {
	h := NewStudentHandler(svc)

	g := rg.Group("/student", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/student/profile  学员信息+学习统计+课程进度
	g.GET("/profile", h.GetProfile)
	// GET /api/student/records  学员学习记录分页
	g.GET("/records", h.GetRecords)
	// GET /api/student/study-stats  学员仪表盘学习统计（按天分组，query: days=7|30）
	g.GET("/study-stats", h.GetStudyStats)
}

// GetProfile 学员信息+学习统计+课程进度 GET /api/student/profile
func (h *StudentHandler) GetProfile(c *gin.Context) {
	Endpoint[struct{}, service.StudentProfileDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.StudentProfileDTO, error) {
			return h.svc.GetProfile(middleware.CurrentUserID(c))
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.StudentProfileDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetRecords 学员学习记录分页 GET /api/student/records
func (h *StudentHandler) GetRecords(c *gin.Context) {
	Endpoint[struct{}, service.StudyRecordPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.StudyRecordPageResult, error) {
			result := h.svc.GetRecords(middleware.CurrentUserID(c),
				atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 10),
				c.Query("start_date"), c.Query("end_date"))
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.StudyRecordPageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetStudyStats 学员仪表盘学习统计（按天分组）GET /api/student/study-stats
func (h *StudentHandler) GetStudyStats(c *gin.Context) {
	Endpoint[struct{}, service.StudyDailyStatsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.StudyDailyStatsDTO, error) {
			return h.svc.GetStudyStats(middleware.CurrentUserID(c), atoiDefault(c.Query("days"), 7)), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.StudyDailyStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}
