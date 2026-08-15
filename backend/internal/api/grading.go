// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
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
func RegisterGradingRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.GradingService) {
	h := NewGradingHandler(svc)

	g := rg.Group("/grading", middleware.JWTAuth(rd.Session), middleware.RoleRequired("tutor", "admin"))

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
	Endpoint[struct{}, []service.GradingParticipantDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.GradingParticipantDTO, error) {
			result, err := h.svc.GetSubmittedParticipants(queryIDPtr(c, "session_id"))
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.GradingParticipantDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetParticipantDetail 参与记录详情 GET /api/grading/participants/:participant_id
func (h *GradingHandler) GetParticipantDetail(c *gin.Context) {
	Endpoint[participantIDReq, service.GradingParticipantDetailDTO]{
		Parse: func(c *gin.Context) (*participantIDReq, error) {
			id, err := pathInt(c, "participant_id", "参与记录ID无效")
			if err != nil {
				return nil, err
			}
			return &participantIDReq{ParticipantID: id}, nil
		},
		Invoke: func(ctx context.Context, req *participantIDReq) (*service.GradingParticipantDetailDTO, error) {
			return h.svc.GetParticipantDetail(req.ParticipantID)
		},
		Render: func(c *gin.Context, _ *participantIDReq, resp *service.GradingParticipantDetailDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// BatchConfirmObjective 批量确认客观题 POST /api/grading/participants/:participant_id/confirm-objective
func (h *GradingHandler) BatchConfirmObjective(c *gin.Context) {
	Endpoint[batchConfirmReq, map[string]any]{
		Parse: func(c *gin.Context) (*batchConfirmReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			graderID, _ := uid.(int)
			participantID, err := pathInt(c, "participant_id", "参与记录ID无效")
			if err != nil {
				return nil, err
			}
			return &batchConfirmReq{ParticipantID: participantID, GraderID: graderID}, nil
		},
		Invoke: func(ctx context.Context, req *batchConfirmReq) (*map[string]any, error) {
			result, err := h.svc.BatchConfirmObjective(req.ParticipantID, req.GraderID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *batchConfirmReq, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			count, _ := (*resp)["confirmed_count"].(int)
			response.SuccessWithMsg(c, "已确认"+strconv.Itoa(count)+"道客观题", resp)
		},
	}.Handle(c)
}

// GradeAnswer 首次阅卷 POST /api/grading/:answer_id/grade
func (h *GradingHandler) GradeAnswer(c *gin.Context) {
	Endpoint[gradeReq, service.LevelExamAnswerDTO]{
		Parse: func(c *gin.Context) (*gradeReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			graderID, _ := uid.(int)
			answerID, err := pathInt(c, "answer_id", "答题记录ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Score   float64 `json:"score"`
				Comment string  `json:"comment"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &gradeReq{AnswerID: answerID, Score: body.Score, GraderID: graderID, Comment: body.Comment}, nil
		},
		Invoke: func(ctx context.Context, req *gradeReq) (*service.LevelExamAnswerDTO, error) {
			return h.svc.GradeAnswer(req.AnswerID, req.Score, req.GraderID, req.Comment)
		},
		Render: func(c *gin.Context, _ *gradeReq, resp *service.LevelExamAnswerDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "阅卷成功", resp)
		},
	}.Handle(c)
}

// RegradeAnswer 复核 POST /api/grading/:answer_id/regrade
func (h *GradingHandler) RegradeAnswer(c *gin.Context) {
	Endpoint[gradeReq, service.LevelExamAnswerDTO]{
		Parse: func(c *gin.Context) (*gradeReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			graderID, _ := uid.(int)
			answerID, err := pathInt(c, "answer_id", "答题记录ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Score   float64 `json:"score"`
				Comment string  `json:"comment"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &gradeReq{AnswerID: answerID, Score: body.Score, GraderID: graderID, Comment: body.Comment}, nil
		},
		Invoke: func(ctx context.Context, req *gradeReq) (*service.LevelExamAnswerDTO, error) {
			return h.svc.RegradeAnswer(req.AnswerID, req.Score, req.GraderID, req.Comment)
		},
		Render: func(c *gin.Context, _ *gradeReq, resp *service.LevelExamAnswerDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "复核成功", resp)
		},
	}.Handle(c)
}

// ConfirmAIGrading 确认 AI 评分 POST /api/grading/:answer_id/confirm-ai
func (h *GradingHandler) ConfirmAIGrading(c *gin.Context) {
	Endpoint[gradingIDReq, service.LevelExamAnswerDTO]{
		Parse: func(c *gin.Context) (*gradingIDReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			graderID, _ := uid.(int)
			answerID, err := pathInt(c, "answer_id", "答题记录ID无效")
			if err != nil {
				return nil, err
			}
			return &gradingIDReq{AnswerID: answerID, GraderID: graderID}, nil
		},
		Invoke: func(ctx context.Context, req *gradingIDReq) (*service.LevelExamAnswerDTO, error) {
			return h.svc.ConfirmAIGrading(req.AnswerID, req.GraderID)
		},
		Render: func(c *gin.Context, _ *gradingIDReq, resp *service.LevelExamAnswerDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "AI评分确认成功", resp)
		},
	}.Handle(c)
}

// AIGradeAnswer AI 重新评分 POST /api/grading/:answer_id/ai-grade
func (h *GradingHandler) AIGradeAnswer(c *gin.Context) {
	Endpoint[gradingIDReq, service.LevelExamAnswerDTO]{
		Parse: func(c *gin.Context) (*gradingIDReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			answerID, err := pathInt(c, "answer_id", "答题记录ID无效")
			if err != nil {
				return nil, err
			}
			return &gradingIDReq{AnswerID: answerID, GraderID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *gradingIDReq) (*service.LevelExamAnswerDTO, error) {
			return h.svc.AIGradeAnswer(req.AnswerID, &req.GraderID)
		},
		Render: func(c *gin.Context, _ *gradingIDReq, resp *service.LevelExamAnswerDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "AI评分成功", resp)
		},
	}.Handle(c)
}

// GetGradingStats 阅卷统计 GET /api/grading/stats
func (h *GradingHandler) GetGradingStats(c *gin.Context) {
	Endpoint[struct{}, map[string]any]{
		Invoke: func(ctx context.Context, _ *struct{}) (*map[string]any, error) {
			result := h.svc.GetGradingStats(queryIDPtr(c, "session_id"))
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *map[string]any, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// participantIDReq 参与记录 ID 请求。
type participantIDReq struct {
	ParticipantID int
}

// batchConfirmReq 批量确认客观题请求。
type batchConfirmReq struct {
	ParticipantID int
	GraderID      int
}

// gradeReq 阅卷/复核请求。
type gradeReq struct {
	AnswerID int
	Score    float64
	GraderID int
	Comment  string
}

// gradingIDReq 阅卷 ID 请求（confirm-ai / ai-grade）。
type gradingIDReq struct {
	AnswerID int
	GraderID int
}
