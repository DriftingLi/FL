// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"net/http"

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

	// 证件作用域（ADR-0047 §4）：显式 credential_id 优先，学员缺省用服务端当前证件。
	// **必须挂在本蓝图自己的 group 上**：挂在共享的 /api group 会隐式作用于其后注册的所有蓝图，
	// 覆盖范围由注册顺序决定（还会给它们各加一次证件查询）。
	g := rg.Group("", middleware.CredentialScoped(rd.CredentialScope))

	// 公开访问
	g.GET("/courses", h.ListCourses)

	// 需要登录
	auth := g.Group("", middleware.JWTAuth(rd.Session))
	auth.GET("/course/:course_id", h.GetCourseDetail)
	auth.GET("/course/:course_id/chapter/:chapter_id", h.GetChapterDetail)
	// 章节幻灯片：与章节详情**同一鉴权面**（#1132 复审）。此前它挂在公开组 ⇒ 任意章节 id 可无凭证
	// 拉取 slides（含未发布 / 未挂载课程的章节）；消费方只有 Web 章节页的 PptViewer，走已鉴权请求层。
	auth.GET("/chapter/:chapter_id/slides", h.GetChapterSlides)
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
// unreadableFaces404 是「一份内容读不到」这件**外显结论**在呈现层的唯一落点
// （ADR-0064 决策 1）：底下四件事实各有载体，这里显式把它们统一答 404 + 同一句话，
// 保持 ADR-0062 决策 3 的「不泄漏是哪一态」。
//
// 与旧形状的区别不是码，而是旧的是「端点只有一格错误面，所以只能统一」，
// 这里是「在 service 层分好档之后**选择**统一」。移动端 #1268 若要区分
// 「未解锁 / 加载失败」，今后从这里删掉一行映射即可打开，不必回 service 层重做错误语义。
var unreadableFaces404 = []error{
	service.ErrCourseNotVisible, // 不在平台上：未发布 / 未挂载
	service.ErrCourseLocked,     // 在平台上、可见，但这个人没兑换
	service.ErrCourseNotFound,   // 课程行真不存在
	service.ErrChapterNotFound,  // 章节行真不存在
}

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
				CredentialID: middleware.CredentialIDPtr(c),
				SpecialtyID:  queryIDPtr(c, "specialty_id"),
				LevelID:      queryIDPtr(c, "level_id"),
				Filter:       f,
			}, nil
		},
		Invoke: func(ctx context.Context, req *courseListReq) (*service.CoursePageResult, error) {
			result, err := h.svc.GetCourses(req.Page, req.PageSize, req.CredentialID, req.SpecialtyID, req.LevelID, req.Filter)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetChapterSlides 章节幻灯片
// @Summary 章节幻灯片
// @Description 需登录，返回章节 PPT 转图片后的 slides（#1132 复审：此前为公开访问）
// @Tags 学员端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节ID"
// @Success 200 {object} response.R{data=service.ChapterSlidesDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /chapter/{chapter_id}/slides [get]
func (h *CourseHandler) GetChapterSlides(c *gin.Context) {
	Endpoint[chapterSlidesReq, service.ChapterSlidesDTO]{
		Parse: func(c *gin.Context) (*chapterSlidesReq, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &chapterSlidesReq{ChapterID: id, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *chapterSlidesReq) (*service.ChapterSlidesDTO, error) {
			return h.svc.GetChapterSlides(req.ChapterID, req.StudentID)
		},
		// 判定不动（票8 逐端点判过，与下面的章节详情同一理由）：可读性判据本身已具名
		// （四条不可读事实 → 404），但「章节不存在」在 course_service.go 里曾是裸
		// errors.New ⇒ 换成 regenerate 那张「哨兵 404 + 其余 500」的表会把真 404 答成 500。
		// service 侧把这两条升成哨兵后一并换表（DB 故障今天仍被伪装成 404，登记为已知残留）。
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).
		WithSentinels(http.StatusNotFound, unreadableFaces404...).Handle(c)
}

// GetCourseDetail 课程详情
// @Summary 课程详情（含学习进度）
// @Description 需登录，返回课程信息 + 章节 + 学员维度进度/是否已选/完成章节/最后位置（ADR-0017）；未登录时 last_* 为空。可见性（ADR-0058）：未发布 / 未挂载课程按「不存在」返回
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
		// 判定不动：GetCourseDetail 不看权益（详情是发现面，只判可见性），其「课程不存在」是裸
		// errors.New ⇒ 本端点没有任何具名哨兵可映射，换表只会把真 404 与 DB 故障一起答成 500。
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).
		WithSentinels(http.StatusNotFound, unreadableFaces404...).Handle(c)
}

// GetChapterDetail 章节详情
// @Summary 章节详情
// @Description 需登录，返回章节详情 + 相邻章节 + 学习状态。可见性（ADR-0058）：章节跟随所属课程，未发布 / 未挂载一律按「不存在」返回
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
		// 判定不动：同上面幻灯片一处（「章节不存在」「章节不属于该课程」在 service 侧都是裸
		// errors.New，前者该 404、后者该 400，现在都被这条表压成 404；升哨兵后一并换表）。
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).
		WithSentinels(http.StatusNotFound, unreadableFaces404...).Handle(c)
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
// @Failure 404 {object} response.R "章节不存在或对本学员不可读（未发布/未挂载/未兑换）"
// @Failure 500 {object} response.R "幻灯片生成失败（无 PPT 文件 / 转图失败 / 权益查询失败）"
// @Router /chapter/{chapter_id}/slides/regenerate [post]
func (h *CourseHandler) RegenerateChapterSlides(c *gin.Context) {
	Endpoint[chapterSlidesReq, service.ChapterSlidesDTO]{
		Parse: func(c *gin.Context) (*chapterSlidesReq, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &chapterSlidesReq{ChapterID: id, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *chapterSlidesReq) (*service.ChapterSlidesDTO, error) {
			return h.svc.RegenerateChapterSlides(req.ChapterID, req.StudentID)
		},
		// 只有「被可读性判据拦下」（未发布 / 未挂载 / 未兑换）才是「不存在」；下游失败
		// （该章节没有 PPT、PPT 转图失败）与权益查询查不动一律 500。
		// 旧形状 WithSuccess(ok, 404) 把门禁与下游故障压成同一个 404 ⇒ 门禁从外部不可分辨、
		// 无测可建（ADR-0062 复核登记 #4），并把 DB 故障答成「这个章节不存在」。
		ErrStatus: &errStatusTable{entries: []errStatusEntry{
			// 「读不到」的四件事实统一答 404（呈现层显式决定，见 unreadableFaces404 注释）。
			{sentinel: service.ErrCourseNotVisible, status: http.StatusNotFound},
			{sentinel: service.ErrCourseLocked, status: http.StatusNotFound},
			{sentinel: service.ErrCourseNotFound, status: http.StatusNotFound},
			{sentinel: service.ErrChapterNotFound, status: http.StatusNotFound},
		}},
		Render: func(c *gin.Context, _ *chapterSlidesReq, resp *service.ChapterSlidesDTO) {
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
		ErrStatus: &errStatusTable{entries: []errStatusEntry{
			// 不可读（未发布 / 未挂载 / 未兑换）按 404，与另外三条内容路径同判
			// （ADR-0062 决策 3）；其余错误保持既有「更新进度失败: + 原文」500 形状。
			// 「读不到」的四件事实统一答 404（呈现层显式决定，见 unreadableFaces404 注释）。
			{sentinel: service.ErrCourseNotVisible, status: http.StatusNotFound},
			{sentinel: service.ErrCourseLocked, status: http.StatusNotFound},
			{sentinel: service.ErrCourseNotFound, status: http.StatusNotFound},
			{sentinel: service.ErrChapterNotFound, status: http.StatusNotFound},
			{sentinel: nil, status: http.StatusInternalServerError, errPrefix: "更新进度失败: "},
		}},
		Render: func(c *gin.Context, _ *studyProgressReq, resp *service.StudyProgressDTO) {
			response.SuccessWithMsg(c, "学习进度更新成功", resp)
		},
	}.Handle(c)
}

// chapterSlidesReq 章节幻灯片请求（chapter_id）。
type chapterSlidesReq struct {
	ChapterID int
	StudentID int
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
