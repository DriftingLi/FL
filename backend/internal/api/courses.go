// Package api 实现 HTTP handlers。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
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
func RegisterCoursesRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.CourseService) {
	h := NewCourseHandler(svc)

	// 公开访问
	rg.GET("/courses", h.ListCourses)
	rg.GET("/chapter/:chapter_id/slides", h.GetChapterSlides)

	// 需要登录
	auth := rg.Group("", middleware.JWTAuth(rd.Session))
	auth.GET("/course/:course_id", h.GetCourseDetail)
	auth.GET("/course/:course_id/chapter/:chapter_id", h.GetChapterDetail)
	auth.POST("/chapter/:chapter_id/slides/regenerate", h.RegenerateChapterSlides)
	auth.POST("/course/:course_id/progress", h.UpdateStudyProgress)
}

// ListCourses 课程列表 GET /api/courses（公开访问，可按专业方向/等级过滤）
func (h *CourseHandler) ListCourses(c *gin.Context) {
	Endpoint[struct{}, service.CoursePageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.CoursePageResult, error) {
			result := h.svc.GetCourses(atoiDefault(c.Query("page"), 1),
				atoiDefault(c.Query("page_size"), 12),
				queryIDPtr(c, "specialty_id"), queryIDPtr(c, "level_id"))
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.CoursePageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetChapterSlides 章节幻灯片 GET /api/chapter/:chapter_id/slides（公开访问）
func (h *CourseHandler) GetChapterSlides(c *gin.Context) {
	Endpoint[chapterSlidesReq, service.ChapterSlidesDTO]{
		Parse: func(c *gin.Context) (*chapterSlidesReq, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &chapterSlidesReq{ChapterID: id}, nil
		},
		Invoke: func(ctx context.Context, req *chapterSlidesReq) (*service.ChapterSlidesDTO, error) {
			return h.svc.GetChapterSlides(req.ChapterID)
		},
		Render: func(c *gin.Context, _ *chapterSlidesReq, resp *service.ChapterSlidesDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetCourseDetail 课程详情（含章节与学习进度）GET /api/course/:course_id
func (h *CourseHandler) GetCourseDetail(c *gin.Context) {
	Endpoint[courseDetailReq, service.CourseDetailDTO]{
		Parse: func(c *gin.Context) (*courseDetailReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &courseDetailReq{CourseID: id, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *courseDetailReq) (*service.CourseDetailDTO, error) {
			return h.svc.GetCourseDetail(req.CourseID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *courseDetailReq, resp *service.CourseDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetChapterDetail 章节详情 GET /api/course/:course_id/chapter/:chapter_id
func (h *CourseHandler) GetChapterDetail(c *gin.Context) {
	Endpoint[chapterDetailReq, service.ChapterDetailDTO]{
		Parse: func(c *gin.Context) (*chapterDetailReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			courseID, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			chapterID, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &chapterDetailReq{CourseID: courseID, ChapterID: chapterID, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *chapterDetailReq) (*service.ChapterDetailDTO, error) {
			return h.svc.GetChapterDetail(req.CourseID, req.ChapterID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *chapterDetailReq, resp *service.ChapterDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// RegenerateChapterSlides 重新生成幻灯片 POST /api/chapter/:chapter_id/slides/regenerate
func (h *CourseHandler) RegenerateChapterSlides(c *gin.Context) {
	Endpoint[chapterSlidesReq, service.ChapterSlidesDTO]{
		Parse: func(c *gin.Context) (*chapterSlidesReq, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &chapterSlidesReq{ChapterID: id}, nil
		},
		Invoke: func(ctx context.Context, req *chapterSlidesReq) (*service.ChapterSlidesDTO, error) {
			return h.svc.RegenerateChapterSlides(req.ChapterID)
		},
		Render: func(c *gin.Context, _ *chapterSlidesReq, resp *service.ChapterSlidesDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "幻灯片重新生成成功", resp)
		},
	}.Handle(c)
}

// UpdateStudyProgress 更新学习进度 POST /api/course/:course_id/progress
func (h *CourseHandler) UpdateStudyProgress(c *gin.Context) {
	Endpoint[studyProgressReq, service.StudyProgressDTO]{
		Parse: func(c *gin.Context) (*studyProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			courseID, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				ChapterID *int `json:"chapter_id"`
				Duration  int  `json:"duration"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			if body.Duration < 0 {
				return nil, badRequest("学习时长不能为负数")
			}
			chapterID := 0
			if body.ChapterID != nil {
				chapterID = *body.ChapterID
			}
			return &studyProgressReq{StudentID: studentID, CourseID: courseID, ChapterID: chapterID, Duration: body.Duration}, nil
		},
		Invoke: func(ctx context.Context, req *studyProgressReq) (*service.StudyProgressDTO, error) {
			return h.svc.UpdateStudyProgress(req.StudentID, req.CourseID, req.ChapterID, req.Duration)
		},
		Render: func(c *gin.Context, _ *studyProgressReq, resp *service.StudyProgressDTO, err error) {
			if err != nil {
				response.ServerError(c, "更新进度失败: "+err.Error())
				return
			}
			response.SuccessWithMsg(c, "学习进度更新成功", resp)
		},
	}.Handle(c)
}

// chapterSlidesReq 章节幻灯片请求（chapter_id）。
type chapterSlidesReq struct {
	ChapterID int
}

// courseDetailReq 课程详情请求（course_id + studentID）。
type courseDetailReq struct {
	CourseID  int
	StudentID int
}

// chapterDetailReq 章节详情请求（course_id + chapter_id + studentID）。
type chapterDetailReq struct {
	CourseID  int
	ChapterID int
	StudentID int
}

// studyProgressReq 学习进度请求。
type studyProgressReq struct {
	StudentID int
	CourseID  int
	ChapterID int
	Duration  int
}
