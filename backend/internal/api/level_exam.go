// Package api 实现 HTTP handlers。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// LevelExamHandler 等级考试 handler。
type LevelExamHandler struct {
	svc *service.LevelExamService
}

// NewLevelExamHandler 创建等级考试 handler。
func NewLevelExamHandler(svc *service.LevelExamService) *LevelExamHandler {
	return &LevelExamHandler{svc: svc}
}

// RegisterLevelExamRoutes 注册 /api/level-exam 蓝图（等级考试与晋级）。
func RegisterLevelExamRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.LevelExamService) {
	h := NewLevelExamHandler(svc)

	g := rg.Group("/level-exam", middleware.JWTAuth(rd.Session))

	// ===== 场次管理（管理员） =====
	g.GET("/sessions", h.ListSessions)
	g.POST("/sessions", middleware.RoleRequired("admin"), h.CreateSession)
	g.PUT("/sessions/:session_id/status", middleware.RoleRequired("admin"), h.UpdateSessionStatus)
	g.GET("/sessions/:session_id", h.GetSessionDetail)
	g.PUT("/sessions/:session_id", middleware.RoleRequired("admin"), h.UpdateSession)
	g.DELETE("/sessions/:session_id", middleware.RoleRequired("admin"), h.DeleteSession)

	// ===== 学员考试流程 =====
	g.GET("/available", middleware.RoleRequired("hrwai_user"), h.GetAvailableExams)
	g.GET("/history", middleware.RoleRequired("hrwai_user"), h.GetStudentHistory)
	g.POST("/sessions/:session_id/enter", middleware.RoleRequired("hrwai_user"), h.EnterExam)
	g.POST("/participants/:participant_id/save", middleware.RoleRequired("hrwai_user"), h.SaveAnswer)
	g.POST("/participants/:participant_id/submit", middleware.RoleRequired("hrwai_user"), h.SubmitExam)
	g.GET("/participants/:participant_id/result", middleware.RoleRequired("hrwai_user"), h.GetResult)
}

// listSessionsReq 场次列表请求（query + 角色上下文）。
type listSessionsReq struct {
	Page                int
	PageSize            int
	Status              string
	IncludeParticipants bool
}

// ListSessions 场次列表 GET /api/level-exam/sessions
func (h *LevelExamHandler) ListSessions(c *gin.Context) {
	Endpoint[listSessionsReq, service.LevelExamSessionListDTO]{
		Parse: func(c *gin.Context) (*listSessionsReq, error) {
			role, _ := c.Get(string(middleware.CtxUserRole))
			roleStr, _ := role.(string)
			return &listSessionsReq{
				Page:                atoiDefault(c.Query("page"), 1),
				PageSize:            atoiDefault(c.Query("page_size"), 20),
				Status:              c.Query("status"),
				IncludeParticipants: roleStr == "tutor" || roleStr == "admin",
			}, nil
		},
		Invoke: func(ctx context.Context, req *listSessionsReq) (*service.LevelExamSessionListDTO, error) {
			return h.svc.ListSessions(req.Page, req.PageSize, req.Status, req.IncludeParticipants), nil
		},
		Render: func(c *gin.Context, _ *listSessionsReq, resp *service.LevelExamSessionListDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// createSessionReq 创建场次请求（body map + 操作者 ID）。
type createSessionReq struct {
	Data   map[string]any
	UserID int
}

// CreateSession 创建场次 POST /api/level-exam/sessions
func (h *LevelExamHandler) CreateSession(c *gin.Context) {
	Endpoint[createSessionReq, service.LevelExamSessionDTO]{
		Parse: func(c *gin.Context) (*createSessionReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var data map[string]any
			if err := c.ShouldBindJSON(&data); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &createSessionReq{Data: data, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *createSessionReq) (*service.LevelExamSessionDTO, error) {
			return h.svc.CreateSession(req.Data, &req.UserID)
		},
		Render: func(c *gin.Context, _ *createSessionReq, resp *service.LevelExamSessionDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "考试场次创建成功", resp)
		},
	}.Handle(c)
}

// updateSessionStatusReq 更新状态请求（路径 ID + body status）。
type updateSessionStatusReq struct {
	SessionID int
	Status    string
}

// UpdateSessionStatus 更新状态 PUT /api/level-exam/sessions/:session_id/status
func (h *LevelExamHandler) UpdateSessionStatus(c *gin.Context) {
	Endpoint[updateSessionStatusReq, service.LevelExamSessionDTO]{
		Parse: func(c *gin.Context) (*updateSessionStatusReq, error) {
			sessionID, err := pathInt(c, "session_id", "场次ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				Status string `json:"status"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
				return nil, badRequest("状态不能为空")
			}
			return &updateSessionStatusReq{SessionID: sessionID, Status: req.Status}, nil
		},
		Invoke: func(ctx context.Context, req *updateSessionStatusReq) (*service.LevelExamSessionDTO, error) {
			return h.svc.UpdateSessionStatus(req.SessionID, req.Status)
		},
		Render: func(c *gin.Context, _ *updateSessionStatusReq, resp *service.LevelExamSessionDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "状态更新成功", resp)
		},
	}.Handle(c)
}

// GetSessionDetail 场次详情 GET /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) GetSessionDetail(c *gin.Context) {
	Endpoint[struct {
		SessionID int
	}, service.LevelExamSessionDTO]{
		Parse: func(c *gin.Context) (*struct {
			SessionID int
		}, error) {
			sessionID, err := pathInt(c, "session_id", "场次ID无效")
			if err != nil {
				return nil, err
			}
			return &struct {
				SessionID int
			}{SessionID: sessionID}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			SessionID int
		}) (*service.LevelExamSessionDTO, error) {
			return h.svc.GetSessionDetail(req.SessionID)
		},
		Render: func(c *gin.Context, _ *struct {
			SessionID int
		}, resp *service.LevelExamSessionDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// updateSessionReq 更新场次请求（路径 ID + body map）。
type updateSessionReq struct {
	SessionID int
	Data      map[string]any
}

// UpdateSession 更新场次 PUT /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) UpdateSession(c *gin.Context) {
	Endpoint[updateSessionReq, service.LevelExamSessionDTO]{
		Parse: func(c *gin.Context) (*updateSessionReq, error) {
			sessionID, err := pathInt(c, "session_id", "场次ID无效")
			if err != nil {
				return nil, err
			}
			var data map[string]any
			if err := c.ShouldBindJSON(&data); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &updateSessionReq{SessionID: sessionID, Data: data}, nil
		},
		Invoke: func(ctx context.Context, req *updateSessionReq) (*service.LevelExamSessionDTO, error) {
			return h.svc.UpdateSession(req.SessionID, req.Data)
		},
		Render: func(c *gin.Context, _ *updateSessionReq, resp *service.LevelExamSessionDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "考试场次更新成功", resp)
		},
	}.Handle(c)
}

// DeleteSession 删除场次 DELETE /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) DeleteSession(c *gin.Context) {
	Endpoint[struct {
		SessionID int
	}, struct{}]{
		Parse: func(c *gin.Context) (*struct {
			SessionID int
		}, error) {
			sessionID, err := pathInt(c, "session_id", "场次ID无效")
			if err != nil {
				return nil, err
			}
			return &struct {
				SessionID int
			}{SessionID: sessionID}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			SessionID int
		}) (*struct{}, error) {
			if err := h.svc.DeleteSession(req.SessionID); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *struct {
			SessionID int
		}, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "考试场次删除成功", nil)
		},
	}.Handle(c)
}

// studentCtxReq 携带学员 ID 的请求（来自上下文）。
type studentCtxReq struct {
	StudentID int
}

// GetAvailableExams 可用考试列表 GET /api/level-exam/available
func (h *LevelExamHandler) GetAvailableExams(c *gin.Context) {
	Endpoint[studentCtxReq, []service.LevelExamAvailableDTO]{
		Parse: func(c *gin.Context) (*studentCtxReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &studentCtxReq{StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *studentCtxReq) (*[]service.LevelExamAvailableDTO, error) {
			result, err := h.svc.GetAvailableExams(req.StudentID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *studentCtxReq, resp *[]service.LevelExamAvailableDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// studentHistoryReq 考试历史请求（学员 ID + 分页）。
type studentHistoryReq struct {
	StudentID int
	Page      int
	PageSize  int
}

// GetStudentHistory 学员考试历史 GET /api/level-exam/history
func (h *LevelExamHandler) GetStudentHistory(c *gin.Context) {
	Endpoint[studentHistoryReq, service.LevelExamHistoryDTO]{
		Parse: func(c *gin.Context) (*studentHistoryReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &studentHistoryReq{
				StudentID: studentID,
				Page:      atoiDefault(c.Query("page"), 1),
				PageSize:  atoiDefault(c.Query("page_size"), 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *studentHistoryReq) (*service.LevelExamHistoryDTO, error) {
			return h.svc.GetStudentHistory(req.StudentID, req.Page, req.PageSize), nil
		},
		Render: func(c *gin.Context, _ *studentHistoryReq, resp *service.LevelExamHistoryDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// enterExamReq 进入考试请求（路径 session_id + 学员 ID）。
type enterExamReq struct {
	SessionID int
	StudentID int
}

// EnterExam 进入考试 POST /api/level-exam/sessions/:session_id/enter
func (h *LevelExamHandler) EnterExam(c *gin.Context) {
	Endpoint[enterExamReq, service.LevelExamDataDTO]{
		Parse: func(c *gin.Context) (*enterExamReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			sessionID, err := pathInt(c, "session_id", "场次ID无效")
			if err != nil {
				return nil, err
			}
			return &enterExamReq{SessionID: sessionID, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *enterExamReq) (*service.LevelExamDataDTO, error) {
			return h.svc.EnterExam(req.SessionID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *enterExamReq, resp *service.LevelExamDataDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "进入考试成功", resp)
		},
	}.Handle(c)
}

// saveAnswerReq 保存答案请求（路径 participant_id + 学员 ID + body）。
type saveAnswerReq struct {
	ParticipantID int
	StudentID     int
	Answers       map[string]any
	RemainingTime int
}

// SaveAnswer 保存答案 POST /api/level-exam/participants/:participant_id/save
func (h *LevelExamHandler) SaveAnswer(c *gin.Context) {
	Endpoint[saveAnswerReq, struct{}]{
		Parse: func(c *gin.Context) (*saveAnswerReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			participantID, err := pathInt(c, "participant_id", "参与记录ID无效")
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
			return &saveAnswerReq{
				ParticipantID: participantID,
				StudentID:     studentID,
				Answers:       req.Answers,
				RemainingTime: req.RemainingTime,
			}, nil
		},
		Invoke: func(ctx context.Context, req *saveAnswerReq) (*struct{}, error) {
			if err := h.svc.SaveAnswer(req.ParticipantID, req.StudentID, req.Answers, req.RemainingTime); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *saveAnswerReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "答案保存成功", nil)
		},
	}.Handle(c)
}

// submitExamReq 交卷请求（路径 participant_id + 学员 ID + body）。
type submitExamReq struct {
	ParticipantID int
	StudentID     int
	IsTimeout     bool
	Answers       map[string]any
	RemainingTime *int
}

// SubmitExam 交卷 POST /api/level-exam/participants/:participant_id/submit
func (h *LevelExamHandler) SubmitExam(c *gin.Context) {
	Endpoint[submitExamReq, service.LevelExamParticipantDTO]{
		Parse: func(c *gin.Context) (*submitExamReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			participantID, err := pathInt(c, "participant_id", "参与记录ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				IsTimeout     bool           `json:"is_timeout"`
				Answers       map[string]any `json:"answers"`
				RemainingTime *int           `json:"remaining_time"`
			}
			_ = c.ShouldBindJSON(&req)
			return &submitExamReq{
				ParticipantID: participantID,
				StudentID:     studentID,
				IsTimeout:     req.IsTimeout,
				Answers:       req.Answers,
				RemainingTime: req.RemainingTime,
			}, nil
		},
		Invoke: func(ctx context.Context, req *submitExamReq) (*service.LevelExamParticipantDTO, error) {
			if req.Answers != nil {
				rt := 0
				if req.RemainingTime != nil {
					rt = *req.RemainingTime
				}
				_ = h.svc.SaveAnswer(req.ParticipantID, req.StudentID, req.Answers, rt)
			}
			return h.svc.SubmitExam(req.ParticipantID, req.StudentID, req.IsTimeout)
		},
		Render: func(c *gin.Context, _ *submitExamReq, resp *service.LevelExamParticipantDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "交卷成功", resp)
		},
	}.Handle(c)
}

// getResultReq 查看结果请求（路径 participant_id + 学员 ID）。
type getResultReq struct {
	ParticipantID int
	StudentID     int
}

// GetResult 查看结果 GET /api/level-exam/participants/:participant_id/result
func (h *LevelExamHandler) GetResult(c *gin.Context) {
	Endpoint[getResultReq, service.LevelExamResultDTO]{
		Parse: func(c *gin.Context) (*getResultReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			participantID, err := pathInt(c, "participant_id", "参与记录ID无效")
			if err != nil {
				return nil, err
			}
			return &getResultReq{ParticipantID: participantID, StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *getResultReq) (*service.LevelExamResultDTO, error) {
			return h.svc.GetResult(req.ParticipantID, req.StudentID)
		},
		Render: func(c *gin.Context, _ *getResultReq, resp *service.LevelExamResultDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}
