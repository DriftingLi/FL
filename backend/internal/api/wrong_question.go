// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// WrongQuestionHandler 错题本 handler。
type WrongQuestionHandler struct {
	svc *service.WrongQuestionService
}

// NewWrongQuestionHandler 创建错题本 handler。
func NewWrongQuestionHandler(svc *service.WrongQuestionService) *WrongQuestionHandler {
	return &WrongQuestionHandler{svc: svc}
}

// RegisterWrongQuestionRoutes 注册 /api/wrong-questions 蓝图。
func RegisterWrongQuestionRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.WrongQuestionService) {
	h := NewWrongQuestionHandler(svc)

	g := rg.Group("/wrong-questions", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/wrong-questions  错题列表（分页+过滤）
	g.GET("", h.List)
	// POST /api/wrong-questions/:question_id/redo  重做错题
	g.POST("/:question_id/redo", h.Redo)
	// POST /api/wrong-questions/:question_id/remove  移出错题本
	g.POST("/:question_id/remove", h.Remove)
	// GET /api/wrong-questions/stats  错题统计
	g.GET("/stats", h.GetStats)
	// GET /api/wrong-questions/export  导出错题本（纯文本附件）
	g.GET("/export", h.Export)
}

// List 错题列表 GET /api/wrong-questions（分页+过滤）
func (h *WrongQuestionHandler) List(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	qType := c.Query("type")
	minWrongCount := queryIntPtr(c, "min_wrong_count")
	response.Success(c, h.svc.GetWrongQuestions(studentID, page, pageSize, qType, minWrongCount))
}

// Redo 重做错题 POST /api/wrong-questions/:question_id/redo
func (h *WrongQuestionHandler) Redo(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	questionID, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	var req struct {
		UserAnswer interface{} `json:"user_answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.RedoWrongQuestion(studentID, questionID, req.UserAnswer)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// Remove 移出错题本 POST /api/wrong-questions/:question_id/remove
func (h *WrongQuestionHandler) Remove(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	questionID, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	result, err := h.svc.RemoveWrongQuestion(studentID, questionID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "已移出错题本", result)
}

// GetStats 错题统计 GET /api/wrong-questions/stats
func (h *WrongQuestionHandler) GetStats(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	response.Success(c, h.svc.GetStats(studentID))
}

// Export 导出错题本（纯文本附件）GET /api/wrong-questions/export
func (h *WrongQuestionHandler) Export(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	data := h.svc.ExportWrongQuestions(studentID)
	text := service.FormatWrongQuestionsText(data)
	c.Header("Content-Disposition", "attachment; filename=wrong_questions.txt")
	c.Data(200, "text/plain; charset=utf-8", []byte(text))
}
