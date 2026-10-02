// Package student 学员端学习域（ADR-0017）：档案、学习记录、学习统计与课程进度。
// 本包是 internal/<域> 形态的样板之一（ADR-0070）：handler.go 是 HTTP 出口（/api/student 蓝图，JWT + hrwai_user），
// go 是域实现，batch_backfill.go / daily_series.go 是随域助手。
// 装配点：internal/api/routes_registry.go 调 student.RegisterRoutes(api, rd.Session, deps.StudentSvc)。
package student

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// handler 学员端 handler。
type handler struct {
	svc *Service
}

// newHandler 创建学员端 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/student 蓝图（JWT + hrwai_user）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/student", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapStudentAccess))

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

// GetProfile 学员信息+学习统计+课程进度
// @Summary 学员档案
// @Description 学员基本信息 + 学习统计 + 课程进度（角色 hrwai_user）
// @Tags 学员端-学习中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=StudentProfileDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "学员不存在"
// @Router /student/profile [get]
func (h *handler) GetProfile(c *gin.Context) {
	httpx.Endpoint[studentUserIDReq, StudentProfileDTO]{
		Parse: func(c *gin.Context) (*studentUserIDReq, error) {
			return &studentUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *studentUserIDReq) (*StudentProfileDTO, error) {
			return h.svc.GetProfile(req.UserID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(ErrStudentNotFound, http.StatusNotFound).Handle(c)
}

// GetRecords 学员学习记录分页
// @Summary 学习记录分页
// @Description 按学员维度分页查询学习记录，支持按日期过滤
// @Tags 学员端-学习中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param start_date query string false "开始日期 YYYY-MM-DD"
// @Param end_date query string false "结束日期 YYYY-MM-DD"
// @Success 200 {object} response.R{data=StudyRecordPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /student/records [get]
func (h *handler) GetRecords(c *gin.Context) {
	httpx.Endpoint[studyRecordsReq, StudyRecordPageResult]{
		Parse: func(c *gin.Context) (*studyRecordsReq, error) {
			return &studyRecordsReq{
				UserID:    middleware.CurrentUserID(c),
				Page:      httpx.QueryIntDefault(c, "page", 1),
				PageSize:  httpx.QueryIntDefault(c, "page_size", 10),
				StartDate: c.Query("start_date"),
				EndDate:   c.Query("end_date"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *studyRecordsReq) (*StudyRecordPageResult, error) {
			result, err := h.svc.GetRecords(req.UserID, req.Page, req.PageSize, req.StartDate, req.EndDate)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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

// GetStudyStats 学员仪表盘学习统计
// @Summary 学习统计（按天）
// @Description 按天聚合学习时长，用于仪表盘图表；days 仅支持 7 或 30，其他回退 7
// @Tags 学员端-学习中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param days query int false "统计天数" Enums(7,30) default(7)
// @Success 200 {object} response.R{data=StudyDailyStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /student/study-stats [get]
func (h *handler) GetStudyStats(c *gin.Context) {
	httpx.Endpoint[studyStatsReq, StudyDailyStatsDTO]{
		Parse: func(c *gin.Context) (*studyStatsReq, error) {
			return &studyStatsReq{UserID: middleware.CurrentUserID(c), Days: httpx.QueryIntDefault(c, "days", 7)}, nil
		},
		Invoke: func(ctx context.Context, req *studyStatsReq) (*StudyDailyStatsDTO, error) {
			return h.svc.GetStudyStats(req.UserID, req.Days), nil
		},
	}.Handle(c)
}

// studentCourseReq 单课程学习状态请求。
type studentCourseReq struct {
	UserID   int
	CourseID int
}

// GetStudentCourses 我的课程
// @Summary 我的课程
// @Description 学员已产生学习记录的课程列表（按最后学习时间倒序）+ continue_learning 置顶；包含封面/方向/等级/完成章节/最后位置（ADR-0017）
// @Tags 学员端-学习中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=StudentCoursesDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /student/courses [get]
func (h *handler) GetStudentCourses(c *gin.Context) {
	httpx.Endpoint[studentUserIDReq, StudentCoursesDTO]{
		Parse: func(c *gin.Context) (*studentUserIDReq, error) {
			return &studentUserIDReq{UserID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *studentUserIDReq) (*StudentCoursesDTO, error) {
			return h.svc.GetStudentCourses(req.UserID)
		},
		// 不挂 ErrStudentNotFound：GetStudentCourses 只读 study_records/course，从不取 hrwai_users
		// 行 ⇒ 那一档在本端点不可达（虚报档位会让台账的「声明了却测不出」反向锁失去意义）。
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetStudentCourseDetail 单课程学习详情
// @Summary 单课程学习详情
// @Description 指定课程的学习详情，包含每章进度/播放位置/完成状态（progress>=100 为完成）
// @Tags 学员端-学习中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=StudentCourseDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /student/courses/{course_id} [get]
func (h *handler) GetStudentCourseDetail(c *gin.Context) {
	httpx.Endpoint[studentCourseReq, StudentCourseDetailDTO]{
		Parse: func(c *gin.Context) (*studentCourseReq, error) {
			courseID, err := httpx.PathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &studentCourseReq{UserID: middleware.CurrentUserID(c), CourseID: courseID}, nil
		},
		Invoke: func(ctx context.Context, req *studentCourseReq) (*StudentCourseDetailDTO, error) {
			return h.svc.GetStudentCourseDetail(req.UserID, req.CourseID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseNotFound, http.StatusNotFound).Handle(c)
}
