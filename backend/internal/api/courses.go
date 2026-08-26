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

// ListCourses 课程列表
// @Summary 课程列表
// @Description 公开访问，支持按专业方向 specialty_id / 等级 level_id / 目标证件 credential_id / 热门精品 filter=hot|featured|all 过滤，分页返回
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(12)
// @Param specialty_id query int false "专业方向ID"
// @Param level_id query int false "等级ID"
// @Param credential_id query int false "目标证件ID"
// @Param filter query string false "热门/精品筛选 hot|featured|all" default(all)
// @Success 200 {object} response.R{data=service.CoursePageResult} "success"
// @Router /courses [get]
func (h *CourseHandler) ListCourses(c *gin.Context) {
	Endpoint[courseListReq, service.CoursePageResult]{
		Parse: func(c *gin.Context) (*courseListReq, error) {
			f := c.Query("filter")
			if f != "" && f != "hot" && f != "featured" && f != "all" {
				return nil, badRequest("filter 仅支持 hot|featured|all")
			}
			if f == "" {
				f = "all"
			}
			return &courseListReq{
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 12),
				CredentialID: queryIDPtr(c, "credential_id"),
				SpecialtyID:  queryIDPtr(c, "specialty_id"),
				LevelID:      queryIDPtr(c, "level_id"),
				Filter:       f,
			}, nil
		},
		Invoke: func(ctx context.Context, req *courseListReq) (*service.CoursePageResult, error) {
			result := h.svc.GetCourses(req.Page, req.PageSize, req.CredentialID, req.SpecialtyID, req.LevelID, req.Filter)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *courseListReq, resp *service.CoursePageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetChapterSlides 章节幻灯片
// @Summary 章节幻灯片
// @Description 公开访问，返回章节 PPT 转图片后的 slides
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=service.ChapterSlidesDTO} "success"
// @Failure 404 {object} response.R "章节不存在"
// @Router /chapter/{chapter_id}/slides [get]
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

// GetCourseDetail 课程详情
// @Summary 课程详情（含学习进度）
// @Description 需登录，返回课程信息 + 章节 + 学员维度进度/是否已选/完成章节/最后位置（ADR-0017）；未登录时 last_* 为空
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Success 200 {object} response.R{data=service.CourseDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /course/{course_id} [get]
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

// GetChapterDetail 章节详情
// @Summary 章节详情
// @Description 需登录，返回章节详情 + 相邻章节 + 学习状态
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=service.ChapterDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /course/{course_id}/chapter/{chapter_id} [get]
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

// RegenerateChapterSlides 重新生成幻灯片
// @Summary 重新生成幻灯片
// @Description 需登录，触发章节 PPT 重新转 slides（异步）
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=service.ChapterSlidesDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /chapter/{chapter_id}/slides/regenerate [post]
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

// UpdateStudyProgress 更新学习进度
// @Summary 上报学习进度
// @Description 需登录，上报章节学习时长/播放位置/完成态（ADR-0017）；body 支持 chapter_id/duration/duration_seconds/video_position/completed
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程ID"
// @Param body body object true "进度" example({"chapter_id":1,"duration_seconds":120,"video_position":60,"completed":false})
// @Success 200 {object} response.R{data=service.StudyProgressDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /course/{course_id}/progress [post]
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
				ChapterID       *int `json:"chapter_id"`
				Duration        int  `json:"duration"`
				DurationSeconds int  `json:"duration_seconds"`
				VideoPosition   *int `json:"video_position"`
				Completed       bool `json:"completed"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			if body.Duration < 0 || body.DurationSeconds < 0 {
				return nil, badRequest("学习时长不能为负数")
			}
			if body.VideoPosition != nil && *body.VideoPosition < 0 {
				return nil, badRequest("播放位置不能为负数")
			}
			chapterID := 0
			if body.ChapterID != nil {
				chapterID = *body.ChapterID
			}
			return &studyProgressReq{StudentID: studentID, CourseID: courseID, Input: service.StudyProgressInput{
				ChapterID:     chapterID,
				Duration:      body.Duration,
				DurationSecs:  body.DurationSeconds,
				VideoPosition: body.VideoPosition,
				Completed:     body.Completed,
			}}, nil
		},
		Invoke: func(ctx context.Context, req *studyProgressReq) (*service.StudyProgressDTO, error) {
			return h.svc.UpdateStudyProgress(req.StudentID, req.CourseID, req.Input)
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

// studyProgressReq 学习进度请求（上报参数经 service.StudyProgressInput 承载，ADR-0017）。
type studyProgressReq struct {
	StudentID int
	CourseID  int
	Input     service.StudyProgressInput
}

// courseListReq 学员端课程列表查询参数。
type courseListReq struct {
	Page         int
	PageSize     int
	CredentialID *int
	SpecialtyID  *int
	LevelID      *int
	Filter       string
}
