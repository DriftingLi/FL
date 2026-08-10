// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

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
func RegisterLevelExamRoutes(rg *gin.RouterGroup, deps *Deps) {
	h := NewLevelExamHandler(deps.LevelExamSvc)

	g := rg.Group("/level-exam", middleware.JWTAuth(deps.Session))

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

// ListSessions 场次列表 GET /api/level-exam/sessions
func (h *LevelExamHandler) ListSessions(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	status := c.Query("status")
	role, _ := c.Get(string(middleware.CtxUserRole))
	roleStr, _ := role.(string)
	includeParticipants := roleStr == "tutor" || roleStr == "admin"
	response.Success(c, h.svc.ListSessions(page, pageSize, status, includeParticipants))
}

// CreateSession 创建场次 POST /api/level-exam/sessions
func (h *LevelExamHandler) CreateSession(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	userID, _ := uid.(int)
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateSession(data, &userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "考试场次创建成功", result)
}

// UpdateSessionStatus 更新状态 PUT /api/level-exam/sessions/:session_id/status
func (h *LevelExamHandler) UpdateSessionStatus(c *gin.Context) {
	sessionID, err := strconv.Atoi(c.Param("session_id"))
	if err != nil {
		response.BadRequest(c, "场次ID无效")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		response.BadRequest(c, "状态不能为空")
		return
	}
	result, err := h.svc.UpdateSessionStatus(sessionID, req.Status)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "状态更新成功", result)
}

// GetSessionDetail 场次详情 GET /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) GetSessionDetail(c *gin.Context) {
	sessionID, err := strconv.Atoi(c.Param("session_id"))
	if err != nil {
		response.BadRequest(c, "场次ID无效")
		return
	}
	result, err := h.svc.GetSessionDetail(sessionID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// UpdateSession 更新场次 PUT /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) UpdateSession(c *gin.Context) {
	sessionID, err := strconv.Atoi(c.Param("session_id"))
	if err != nil {
		response.BadRequest(c, "场次ID无效")
		return
	}
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateSession(sessionID, data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "考试场次更新成功", result)
}

// DeleteSession 删除场次 DELETE /api/level-exam/sessions/:session_id
func (h *LevelExamHandler) DeleteSession(c *gin.Context) {
	sessionID, err := strconv.Atoi(c.Param("session_id"))
	if err != nil {
		response.BadRequest(c, "场次ID无效")
		return
	}
	if err := h.svc.DeleteSession(sessionID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "考试场次删除成功", nil)
}

// GetAvailableExams 可用考试列表 GET /api/level-exam/available
func (h *LevelExamHandler) GetAvailableExams(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	result, err := h.svc.GetAvailableExams(studentID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetStudentHistory 学员考试历史 GET /api/level-exam/history
func (h *LevelExamHandler) GetStudentHistory(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	response.Success(c, h.svc.GetStudentHistory(studentID, page, pageSize))
}

// EnterExam 进入考试 POST /api/level-exam/sessions/:session_id/enter
func (h *LevelExamHandler) EnterExam(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	sessionID, err := strconv.Atoi(c.Param("session_id"))
	if err != nil {
		response.BadRequest(c, "场次ID无效")
		return
	}
	result, err := h.svc.EnterExam(sessionID, studentID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "进入考试成功", result)
}

// SaveAnswer 保存答案 POST /api/level-exam/participants/:participant_id/save
func (h *LevelExamHandler) SaveAnswer(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	participantID, err := strconv.Atoi(c.Param("participant_id"))
	if err != nil {
		response.BadRequest(c, "参与记录ID无效")
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
	if err := h.svc.SaveAnswer(participantID, studentID, req.Answers, req.RemainingTime); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "答案保存成功", nil)
}

// SubmitExam 交卷 POST /api/level-exam/participants/:participant_id/submit
func (h *LevelExamHandler) SubmitExam(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	participantID, err := strconv.Atoi(c.Param("participant_id"))
	if err != nil {
		response.BadRequest(c, "参与记录ID无效")
		return
	}
	var req struct {
		IsTimeout     bool                   `json:"is_timeout"`
		Answers       map[string]interface{} `json:"answers"`
		RemainingTime *int                   `json:"remaining_time"`
	}
	_ = c.ShouldBindJSON(&req)
	// 若同时提交了答案，先保存
	if req.Answers != nil {
		rt := 0
		if req.RemainingTime != nil {
			rt = *req.RemainingTime
		}
		_ = h.svc.SaveAnswer(participantID, studentID, req.Answers, rt)
	}
	result, err := h.svc.SubmitExam(participantID, studentID, req.IsTimeout)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "交卷成功", result)
}

// GetResult 查看结果 GET /api/level-exam/participants/:participant_id/result
func (h *LevelExamHandler) GetResult(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	participantID, err := strconv.Atoi(c.Param("participant_id"))
	if err != nil {
		response.BadRequest(c, "参与记录ID无效")
		return
	}
	result, err := h.svc.GetResult(participantID, studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}
