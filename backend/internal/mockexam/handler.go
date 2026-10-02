// Package mockexam 模拟考试域 HTTP 出口（/api/mock-exam 蓝图，ADR-0070 的 handler.go 形态）。
package mockexam

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// handler 模拟考试 handler。
type handler struct {
	svc *Service
}

// newHandler 创建模拟考试 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/mock-exam 蓝图。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, credRes middleware.CredentialResolver, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/mock-exam", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapMockExamTake), middleware.CredentialScoped(credRes))

	// POST /api/mock-exam/start  开始模拟考试（count 题量 + duration 时长）
	g.POST("/start", h.Start)
	// POST /api/mock-exam/:mock_exam_id/save  保存进度
	g.POST("/:mock_exam_id/save", h.SaveProgress)
	// GET /api/mock-exam/:mock_exam_id/resume  恢复考试
	g.GET("/:mock_exam_id/resume", h.Resume)
	// POST /api/mock-exam/:mock_exam_id/submit  交卷
	g.POST("/:mock_exam_id/submit", h.Submit)
	// GET /api/mock-exam/:mock_exam_id/result  获取结果
	g.GET("/:mock_exam_id/result", h.GetResult)
	// GET /api/mock-exam/history  历史记录
	g.GET("/history", h.GetHistory)
}

// startReq 开始模拟考试请求（学员 ID + body count/duration + query credential_id）。
type startReq struct {
	StudentID    int
	Count        int
	Duration     int
	CredentialID *int
}

// Start 开始模拟考试
// @Summary 开始模拟考试
// @Description 创建模拟考试会话，count 题量、duration 时长（默认 90 分钟）；credential_id 可选（经拦截器注入当前证件，按证件分区抽题）
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID"
// @Param body body object false "参数" example({"count":20,"duration":90})
// @Success 200 {object} response.R{data=MockExamStartDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/start [post]
func (h *handler) Start(c *gin.Context) {
	httpx.Endpoint[startReq, MockExamStartDTO]{
		Parse: func(c *gin.Context) (*startReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				Count           int `json:"count"`
				Duration        int `json:"duration"`
				QuestionCount   int `json:"question_count"`
				DurationMinutes int `json:"duration_minutes"`
			}
			_ = c.ShouldBindJSON(&req)
			count := req.Count
			if count <= 0 {
				count = req.QuestionCount
			}
			duration := req.Duration
			if duration <= 0 {
				duration = req.DurationMinutes
			}
			if duration == 0 {
				duration = 90
			}
			return &startReq{StudentID: studentID, Count: count, Duration: duration, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *startReq) (*MockExamStartDTO, error) {
			return h.svc.Start(req.StudentID, req.Count, req.Duration, req.CredentialID)
		},
	}.WithSuccess(httpx.OkMsg("模拟考试开始"), http.StatusBadRequest).Handle(c)
}

// saveProgressReq 保存进度请求（路径 mock_exam_id + 学员 ID + body）。
type saveProgressReq struct {
	MockExamID    int
	StudentID     int
	Answers       map[string]any
	RemainingTime int
}

// SaveProgress 保存模拟考试进度
// @Summary 保存模拟考试进度
// @Description 保存作答与剩余时间
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Param body body object true "作答" example({"answers":{},"remaining_time":3600})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/{mock_exam_id}/save [post]
func (h *handler) SaveProgress(c *gin.Context) {
	httpx.Endpoint[saveProgressReq, struct{}]{
		Parse: func(c *gin.Context) (*saveProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			mockExamID, err := httpx.PathInt(c, "mock_exam_id", "考试ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				Answers       map[string]any `json:"answers"`
				RemainingTime int            `json:"remaining_time"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &saveProgressReq{
				MockExamID:    mockExamID,
				StudentID:     studentID,
				Answers:       req.Answers,
				RemainingTime: req.RemainingTime,
			}, nil
		},
		Invoke: func(ctx context.Context, req *saveProgressReq) (*struct{}, error) {
			if err := h.svc.SaveProgress(req.MockExamID, req.StudentID, req.Answers, req.RemainingTime); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("进度保存成功"), http.StatusBadRequest).Handle(c)
}

// mockExamIDReq 模拟考试请求（路径 mock_exam_id + 学员 ID）。
type mockExamIDReq struct {
	MockExamID int
	StudentID  int
}

// Resume 恢复模拟考试
// @Summary 恢复模拟考试
// @Description 恢复未交卷的模拟考试，返回题目与进度
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Success 200 {object} response.R{data=MockExamResumeDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/{mock_exam_id}/resume [get]
func (h *handler) Resume(c *gin.Context) {
	httpx.Endpoint[mockExamIDReq, MockExamResumeDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*MockExamResumeDTO, error) {
			return h.svc.Resume(req.MockExamID, req.StudentID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusBadRequest).Handle(c)
}

// Submit 模拟考试交卷
// @Summary 模拟考试交卷
// @Description 交卷并触发判分
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Success 200 {object} response.R{data=MockExamSubmitDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/{mock_exam_id}/submit [post]
func (h *handler) Submit(c *gin.Context) {
	httpx.Endpoint[mockExamIDReq, MockExamSubmitDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*MockExamSubmitDTO, error) {
			return h.svc.Submit(req.MockExamID, req.StudentID)
		},
	}.WithSuccess(httpx.OkMsg("交卷成功"), http.StatusBadRequest).Handle(c)
}

// GetResult 模拟考试结果
// @Summary 模拟考试结果
// @Description 查询已交卷模拟考试的结果与解析
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Success 200 {object} response.R{data=MockExamResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /mock-exam/{mock_exam_id}/result [get]
func (h *handler) GetResult(c *gin.Context) {
	httpx.Endpoint[mockExamIDReq, MockExamResultDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*MockExamResultDTO, error) {
			return h.svc.GetResult(req.MockExamID, req.StudentID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(ErrMockExamNotFound, http.StatusNotFound).Handle(c)
}

// GetHistory 模拟考试历史
// @Summary 模拟考试历史
// @Description 分页查询模拟考试历史记录（按当前证件分区：credential_id 可选，经拦截器注入当前证件）
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=MockExamHistoryDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/history [get]
func (h *handler) GetHistory(c *gin.Context) {
	httpx.Endpoint[mockExamHistoryReq, MockExamHistoryDTO]{
		Parse: func(c *gin.Context) (*mockExamHistoryReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &mockExamHistoryReq{
				StudentID:    studentID,
				CredentialID: middleware.CredentialIDPtr(c),
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *mockExamHistoryReq) (*MockExamHistoryDTO, error) {
			return h.svc.GetHistory(req.StudentID, req.CredentialID, req.Page, req.PageSize)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// mockExamHistoryReq 历史列表请求（学员 ID + 当前证件 + 分页）。
// 证件来自 CredentialScoped 中间件（显式 query 优先，否则服务端当前证件），
// 与 Start 同源；nil = 未设置，按不分区处理。
type mockExamHistoryReq struct {
	StudentID    int
	CredentialID *int
	Page         int
	PageSize     int
}

// parseMockExamID 解析 mock_exam_id 路径参数与学员 ID。
func (h *handler) parseMockExamID(c *gin.Context) (*mockExamIDReq, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := httpx.PathInt(c, "mock_exam_id", "考试ID无效")
	if err != nil {
		return nil, err
	}
	return &mockExamIDReq{MockExamID: mockExamID, StudentID: studentID}, nil
}
