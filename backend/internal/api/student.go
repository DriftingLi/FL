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
	// GET /api/student/courses  我的课程（含继续学习 top1，ADR-0017）
	g.GET("/courses", h.GetStudentCourses)
	// GET /api/student/courses/:course_id  单课程学习详情（每章状态与播放位置）
	g.GET("/courses/:course_id", h.GetStudentCourseDetail)
}

// GetProfile 学员信息+学习统计+课程进度 GET /api/student/profile
func (h *StudentHandler) GetProfile(c *gin.Context) {
	Endpoint[studentUserIDReq, service.StudentProfileDTO]{
		Parse: func(c *gin.Context) (*studentUserIDReq, error) {
			return &studentUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *studentUserIDReq) (*service.StudentProfileDTO, error) {
			return h.svc.GetProfile(req.UserID)
		},
		Render: func(c *gin.Context, _ *studentUserIDReq, resp *service.StudentProfileDTO, err error) {
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
	Endpoint[studyRecordsReq, service.StudyRecordPageResult]{
		Parse: func(c *gin.Context) (*studyRecordsReq, error) {
			return &studyRecordsReq{
				UserID:    middleware.CurrentUserID(c),
				Page:      atoiDefault(c.Query("page"), 1),
				PageSize:  atoiDefault(c.Query("page_size"), 10),
				StartDate: c.Query("start_date"),
				EndDate:   c.Query("end_date"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *studyRecordsReq) (*service.StudyRecordPageResult, error) {
			result := h.svc.GetRecords(req.UserID, req.Page, req.PageSize, req.StartDate, req.EndDate)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *studyRecordsReq, resp *service.StudyRecordPageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// studentUserIDReq 仅带学员 ID 的请求。
type studentUserIDReq struct {
	UserID int
}

// studyRecordsReq 学习记录分页请求。
type studyRecordsReq struct {
	UserID    int
	Page      int
	PageSize  int
	StartDate string
	EndDate   string
}

// studyStatsReq 学习统计请求（days 窗口）。
type studyStatsReq struct {
	UserID int
	Days   int
}

// GetStudyStats 学员仪表盘学习统计（按天分组）GET /api/student/study-stats
func (h *StudentHandler) GetStudyStats(c *gin.Context) {
	Endpoint[studyStatsReq, service.StudyDailyStatsDTO]{
		Parse: func(c *gin.Context) (*studyStatsReq, error) {
			return &studyStatsReq{UserID: middleware.CurrentUserID(c), Days: atoiDefault(c.Query("days"), 7)}, nil
		},
		Invoke: func(ctx context.Context, req *studyStatsReq) (*service.StudyDailyStatsDTO, error) {
			return h.svc.GetStudyStats(req.UserID, req.Days), nil
		},
		Render: func(c *gin.Context, _ *studyStatsReq, resp *service.StudyDailyStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// studentCourseReq 单课程学习状态请求。
type studentCourseReq struct {
	UserID   int
	CourseID int
}

// GetStudentCourses 我的课程 GET /api/student/courses
func (h *StudentHandler) GetStudentCourses(c *gin.Context) {
	Endpoint[studentUserIDReq, service.StudentCoursesDTO]{
		Parse: func(c *gin.Context) (*studentUserIDReq, error) {
			return &studentUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *studentUserIDReq) (*service.StudentCoursesDTO, error) {
			return h.svc.GetStudentCourses(req.UserID)
		},
		Render: func(c *gin.Context, _ *studentUserIDReq, resp *service.StudentCoursesDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetStudentCourseDetail 单课程学习详情 GET /api/student/courses/:course_id
func (h *StudentHandler) GetStudentCourseDetail(c *gin.Context) {
	Endpoint[studentCourseReq, service.StudentCourseDetailDTO]{
		Parse: func(c *gin.Context) (*studentCourseReq, error) {
			courseID, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &studentCourseReq{UserID: middleware.CurrentUserID(c), CourseID: courseID}, nil
		},
		Invoke: func(ctx context.Context, req *studentCourseReq) (*service.StudentCourseDetailDTO, error) {
			return h.svc.GetStudentCourseDetail(req.UserID, req.CourseID)
		},
		Render: func(c *gin.Context, _ *studentCourseReq, resp *service.StudentCourseDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}
