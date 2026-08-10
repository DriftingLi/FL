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

// GradingHandler 阅卷 handler。
type GradingHandler struct {
	svc *service.GradingService
}

// NewGradingHandler 创建阅卷 handler。
func NewGradingHandler(svc *service.GradingService) *GradingHandler {
	return &GradingHandler{svc: svc}
}

// RegisterGradingRoutes 注册 /api/grading 蓝图（导师阅卷）。
func RegisterGradingRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.GradingService) {
	h := NewGradingHandler(svc)

	g := rg.Group("/grading", middleware.JWTAuth(sess), middleware.RoleRequired("tutor", "admin"))

	// GET /api/grading/participants  已提交参与记录列表
	g.GET("/participants", h.GetSubmittedParticipants)
	// GET /api/grading/participants/:participant_id  参与记录详情
	g.GET("/participants/:participant_id", h.GetParticipantDetail)
	// POST /api/grading/participants/:participant_id/confirm-objective  批量确认客观题
	g.POST("/participants/:participant_id/confirm-objective", h.BatchConfirmObjective)
	// POST /api/grading/:answer_id/grade  首次阅卷
	g.POST("/:answer_id/grade", h.GradeAnswer)
	// POST /api/grading/:answer_id/regrade  复核
	g.POST("/:answer_id/regrade", h.RegradeAnswer)
	// POST /api/grading/:answer_id/confirm-ai  确认 AI 评分
	g.POST("/:answer_id/confirm-ai", h.ConfirmAIGrading)
	// POST /api/grading/:answer_id/ai-grade  AI 重新评分
	g.POST("/:answer_id/ai-grade", h.AIGradeAnswer)
	// GET /api/grading/stats  阅卷统计
	g.GET("/stats", h.GetGradingStats)
}

// GetSubmittedParticipants 已提交参与记录列表 GET /api/grading/participants
func (h *GradingHandler) GetSubmittedParticipants(c *gin.Context) {
	var sessionID *int
	if s := c.Query("session_id"); s != "" {
		if id, err := strconv.Atoi(s); err == nil {
			sessionID = &id
		}
	}
	result, err := h.svc.GetSubmittedParticipants(sessionID)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetParticipantDetail 参与记录详情 GET /api/grading/participants/:participant_id
func (h *GradingHandler) GetParticipantDetail(c *gin.Context) {
	participantID, err := strconv.Atoi(c.Param("participant_id"))
	if err != nil {
		response.BadRequest(c, "参与记录ID无效")
		return
	}
	result, err := h.svc.GetParticipantDetail(participantID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// BatchConfirmObjective 批量确认客观题 POST /api/grading/participants/:participant_id/confirm-objective
func (h *GradingHandler) BatchConfirmObjective(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	graderID, _ := uid.(int)
	participantID, err := strconv.Atoi(c.Param("participant_id"))
	if err != nil {
		response.BadRequest(c, "参与记录ID无效")
		return
	}
	result, err := h.svc.BatchConfirmObjective(participantID, graderID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	count, _ := result["confirmed_count"].(int)
	response.SuccessWithMsg(c, "已确认"+strconv.Itoa(count)+"道客观题", result)
}

// GradeAnswer 首次阅卷 POST /api/grading/:answer_id/grade
func (h *GradingHandler) GradeAnswer(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	graderID, _ := uid.(int)
	answerID, err := strconv.Atoi(c.Param("answer_id"))
	if err != nil {
		response.BadRequest(c, "答题记录ID无效")
		return
	}
	var req struct {
		Score   float64 `json:"score"`
		Comment string  `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.GradeAnswer(answerID, req.Score, graderID, req.Comment)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "阅卷成功", result)
}

// RegradeAnswer 复核 POST /api/grading/:answer_id/regrade
func (h *GradingHandler) RegradeAnswer(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	graderID, _ := uid.(int)
	answerID, err := strconv.Atoi(c.Param("answer_id"))
	if err != nil {
		response.BadRequest(c, "答题记录ID无效")
		return
	}
	var req struct {
		Score   float64 `json:"score"`
		Comment string  `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.RegradeAnswer(answerID, req.Score, graderID, req.Comment)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "复核成功", result)
}

// ConfirmAIGrading 确认 AI 评分 POST /api/grading/:answer_id/confirm-ai
func (h *GradingHandler) ConfirmAIGrading(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	graderID, _ := uid.(int)
	answerID, err := strconv.Atoi(c.Param("answer_id"))
	if err != nil {
		response.BadRequest(c, "答题记录ID无效")
		return
	}
	result, err := h.svc.ConfirmAIGrading(answerID, graderID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "AI评分确认成功", result)
}

// AIGradeAnswer AI 重新评分 POST /api/grading/:answer_id/ai-grade
func (h *GradingHandler) AIGradeAnswer(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	userID, _ := uid.(int)
	answerID, err := strconv.Atoi(c.Param("answer_id"))
	if err != nil {
		response.BadRequest(c, "答题记录ID无效")
		return
	}
	result, err := h.svc.AIGradeAnswer(answerID, &userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "AI评分成功", result)
}

// GetGradingStats 阅卷统计 GET /api/grading/stats
func (h *GradingHandler) GetGradingStats(c *gin.Context) {
	var sessionID *int
	if s := c.Query("session_id"); s != "" {
		if id, err := strconv.Atoi(s); err == nil {
			sessionID = &id
		}
	}
	response.Success(c, h.svc.GetGradingStats(sessionID))
}
