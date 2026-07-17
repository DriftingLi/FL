// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterGradingRoutes 注册 /api/grading 蓝图（导师阅卷）。
func RegisterGradingRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewGradingService(db, newAIService(cfg, db))

	g := rg.Group("/grading", middleware.JWTAuth(cfg), middleware.RoleRequired("tutor", "admin"))

	// GET /api/grading/participants  已提交参与记录列表
	g.GET("/participants", func(c *gin.Context) {
		var sessionID *int
		if s := c.Query("session_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				sessionID = &id
			}
		}
		result, err := svc.GetSubmittedParticipants(sessionID)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/grading/participants/:participant_id  参与记录详情
	g.GET("/participants/:participant_id", func(c *gin.Context) {
		participantID, err := strconv.Atoi(c.Param("participant_id"))
		if err != nil {
			response.BadRequest(c, "参与记录ID无效")
			return
		}
		result, err := svc.GetParticipantDetail(participantID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/grading/participants/:participant_id/confirm-objective  批量确认客观题
	g.POST("/participants/:participant_id/confirm-objective", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		graderID, _ := uid.(int)
		participantID, err := strconv.Atoi(c.Param("participant_id"))
		if err != nil {
			response.BadRequest(c, "参与记录ID无效")
			return
		}
		result, err := svc.BatchConfirmObjective(participantID, graderID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		count, _ := result["confirmed_count"].(int)
		response.SuccessWithMsg(c, "已确认"+strconv.Itoa(count)+"道客观题", result)
	})

	// POST /api/grading/:answer_id/grade  首次阅卷
	g.POST("/:answer_id/grade", func(c *gin.Context) {
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
		result, err := svc.GradeAnswer(answerID, req.Score, graderID, req.Comment)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "阅卷成功", result)
	})

	// POST /api/grading/:answer_id/regrade  复核
	g.POST("/:answer_id/regrade", func(c *gin.Context) {
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
		result, err := svc.RegradeAnswer(answerID, req.Score, graderID, req.Comment)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "复核成功", result)
	})

	// POST /api/grading/:answer_id/confirm-ai  确认 AI 评分
	g.POST("/:answer_id/confirm-ai", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		graderID, _ := uid.(int)
		answerID, err := strconv.Atoi(c.Param("answer_id"))
		if err != nil {
			response.BadRequest(c, "答题记录ID无效")
			return
		}
		result, err := svc.ConfirmAIGrading(answerID, graderID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "AI评分确认成功", result)
	})

	// POST /api/grading/:answer_id/ai-grade  AI 重新评分
	g.POST("/:answer_id/ai-grade", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		answerID, err := strconv.Atoi(c.Param("answer_id"))
		if err != nil {
			response.BadRequest(c, "答题记录ID无效")
			return
		}
		result, err := svc.AIGradeAnswer(answerID, &userID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "AI评分成功", result)
	})

	// GET /api/grading/stats  阅卷统计
	g.GET("/stats", func(c *gin.Context) {
		var sessionID *int
		if s := c.Query("session_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				sessionID = &id
			}
		}
		response.Success(c, svc.GetGradingStats(sessionID))
	})
}
