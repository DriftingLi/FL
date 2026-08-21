// Package api 实现 HTTP handlers。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// MockExamHandler 模拟考试 handler。
type MockExamHandler struct {
	svc *service.MockExamService
}

// NewMockExamHandler 创建模拟考试 handler。
func NewMockExamHandler(svc *service.MockExamService) *MockExamHandler {
	return &MockExamHandler{svc: svc}
}

// RegisterMockExamRoutes 注册 /api/mock-exam 蓝图。
func RegisterMockExamRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.MockExamService) {
	h := NewMockExamHandler(svc)

	g := rg.Group("/mock-exam", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

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

// startReq 开始模拟考试请求（学员 ID + body count/duration）。
type startReq struct {
	StudentID int
	Count     int
	Duration  int
}

// Start 开始模拟考试
// @Summary 开始模拟考试
// @Description 创建模拟考试会话，count 题量、duration 时长（默认 90 分钟）
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "参数" example({"count":20,"duration":90})
// @Success 200 {object} response.R{data=service.MockExamStartDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/start [post]
func (h *MockExamHandler) Start(c *gin.Context) {
	Endpoint[startReq, service.MockExamStartDTO]{
		Parse: func(c *gin.Context) (*startReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				Count    int `json:"count"`
				Duration int `json:"duration"`
			}
			_ = c.ShouldBindJSON(&req)
			if req.Duration == 0 {
				req.Duration = 90
			}
			return &startReq{StudentID: studentID, Count: req.Count, Duration: req.Duration}, nil
		},
		Invoke: func(ctx context.Context, req *startReq) (*service.MockExamStartDTO, error) {
			return h.svc.Start(req.StudentID, req.Count, req.Duration)
		},
		Render: func(c *gin.Context, _ *startReq, resp *service.MockExamStartDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "模拟考试开始", resp)
		},
	}.Handle(c)
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
func (h *MockExamHandler) SaveProgress(c *gin.Context) {
	Endpoint[saveProgressReq, struct{}]{
		Parse: func(c *gin.Context) (*saveProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			mockExamID, err := pathInt(c, "mock_exam_id", "考试ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				Answers       map[string]any `json:"answers"`
				RemainingTime int            `json:"remaining_time"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
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
		Render: func(c *gin.Context, _ *saveProgressReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "进度保存成功", nil)
		},
	}.Handle(c)
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
// @Success 200 {object} response.R{data=service.MockExamResumeDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/{mock_exam_id}/resume [get]
func (h *MockExamHandler) Resume(c *gin.Context) {
	Endpoint[mockExamIDReq, service.MockExamResumeDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*service.MockExamResumeDTO, error) {
			return h.svc.Resume(req.MockExamID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *mockExamIDReq, resp *service.MockExamResumeDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Submit 模拟考试交卷
// @Summary 模拟考试交卷
// @Description 交卷并触发判分
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Success 200 {object} response.R{data=service.MockExamSubmitDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/{mock_exam_id}/submit [post]
func (h *MockExamHandler) Submit(c *gin.Context) {
	Endpoint[mockExamIDReq, service.MockExamSubmitDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*service.MockExamSubmitDTO, error) {
			return h.svc.Submit(req.MockExamID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *mockExamIDReq, resp *service.MockExamSubmitDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "交卷成功", resp)
		},
	}.Handle(c)
}

// GetResult 模拟考试结果
// @Summary 模拟考试结果
// @Description 查询已交卷模拟考试的结果与解析
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mock_exam_id path int true "模拟考试ID"
// @Success 200 {object} response.R{data=service.MockExamResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在"
// @Router /mock-exam/{mock_exam_id}/result [get]
func (h *MockExamHandler) GetResult(c *gin.Context) {
	Endpoint[mockExamIDReq, service.MockExamResultDTO]{
		Parse: h.parseMockExamID,
		Invoke: func(ctx context.Context, req *mockExamIDReq) (*service.MockExamResultDTO, error) {
			return h.svc.GetResult(req.MockExamID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *mockExamIDReq, resp *service.MockExamResultDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetHistory 模拟考试历史
// @Summary 模拟考试历史
// @Description 分页查询模拟考试历史记录
// @Tags 学员端-模拟考试
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=service.MockExamHistoryDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /mock-exam/history [get]
func (h *MockExamHandler) GetHistory(c *gin.Context) {
	Endpoint[struct {
		StudentID int
		Page      int
		PageSize  int
	}, service.MockExamHistoryDTO]{
		Parse: func(c *gin.Context) (*struct {
			StudentID int
			Page      int
			PageSize  int
		}, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &struct {
				StudentID int
				Page      int
				PageSize  int
			}{
				StudentID: studentID,
				Page:      atoiDefault(c.Query("page"), 1),
				PageSize:  atoiDefault(c.Query("page_size"), 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			StudentID int
			Page      int
			PageSize  int
		}) (*service.MockExamHistoryDTO, error) {
			return h.svc.GetHistory(req.StudentID, req.Page, req.PageSize), nil
		},
		Render: func(c *gin.Context, _ *struct {
			StudentID int
			Page      int
			PageSize  int
		}, resp *service.MockExamHistoryDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// parseMockExamID 解析 mock_exam_id 路径参数与学员 ID。
func (h *MockExamHandler) parseMockExamID(c *gin.Context) (*mockExamIDReq, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := pathInt(c, "mock_exam_id", "考试ID无效")
	if err != nil {
		return nil, err
	}
	return &mockExamIDReq{MockExamID: mockExamID, StudentID: studentID}, nil
}
