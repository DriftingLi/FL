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

// MockExamHandler 模拟考试 handler。
type MockExamHandler struct {
	svc *service.MockExamService
}

// NewMockExamHandler 创建模拟考试 handler。
func NewMockExamHandler(svc *service.MockExamService) *MockExamHandler {
	return &MockExamHandler{svc: svc}
}

// RegisterMockExamRoutes 注册 /api/mock-exam 蓝图。
func RegisterMockExamRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.MockExamService) {
	h := NewMockExamHandler(svc)

	g := rg.Group("/mock-exam", middleware.JWTAuth(sess), middleware.RoleRequired("hrwai_user"))

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

// Start 开始模拟考试 POST /api/mock-exam/start
func (h *MockExamHandler) Start(c *gin.Context) {
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
	result, err := h.svc.Start(studentID, req.Count, req.Duration)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "模拟考试开始", result)
}

// SaveProgress 保存进度 POST /api/mock-exam/:mock_exam_id/save
func (h *MockExamHandler) SaveProgress(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
	if err != nil {
		response.BadRequest(c, "考试ID无效")
		return
	}
	var req struct {
		Answers       map[string]interface{} `json:"answers"`
		RemainingTime int                    `json:"remaining_time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if err := h.svc.SaveProgress(mockExamID, studentID, req.Answers, req.RemainingTime); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "进度保存成功", nil)
}

// Resume 恢复考试 GET /api/mock-exam/:mock_exam_id/resume
func (h *MockExamHandler) Resume(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
	if err != nil {
		response.BadRequest(c, "考试ID无效")
		return
	}
	result, err := h.svc.Resume(mockExamID, studentID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// Submit 交卷 POST /api/mock-exam/:mock_exam_id/submit
func (h *MockExamHandler) Submit(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
	if err != nil {
		response.BadRequest(c, "考试ID无效")
		return
	}
	result, err := h.svc.Submit(mockExamID, studentID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "交卷成功", result)
}

// GetResult 获取结果 GET /api/mock-exam/:mock_exam_id/result
func (h *MockExamHandler) GetResult(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
	if err != nil {
		response.BadRequest(c, "考试ID无效")
		return
	}
	result, err := h.svc.GetResult(mockExamID, studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetHistory 历史记录 GET /api/mock-exam/history
func (h *MockExamHandler) GetHistory(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	response.Success(c, h.svc.GetHistory(studentID, page, pageSize))
}
