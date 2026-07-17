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

// RegisterLevelExamRoutes 注册 /api/level-exam 蓝图（等级考试与晋级）。
func RegisterLevelExamRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewLevelExamService(db, newAIService(cfg, db))

	g := rg.Group("/level-exam", middleware.JWTAuth(cfg))

	// ===== 场次管理（管理员） =====

	// GET /api/level-exam/sessions  场次列表
	g.GET("/sessions", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		status := c.Query("status")
		role, _ := c.Get(string(middleware.CtxUserRole))
		roleStr, _ := role.(string)
		includeParticipants := roleStr == "tutor" || roleStr == "admin"
		response.Success(c, svc.ListSessions(page, pageSize, status, includeParticipants))
	})

	// POST /api/level-exam/sessions  创建场次
	g.POST("/sessions", middleware.RoleRequired("admin"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateSession(data, &userID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "考试场次创建成功", result)
	})

	// PUT /api/level-exam/sessions/:session_id/status  更新状态
	g.PUT("/sessions/:session_id/status", middleware.RoleRequired("admin"), func(c *gin.Context) {
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
		result, err := svc.UpdateSessionStatus(sessionID, req.Status)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "状态更新成功", result)
	})

	// GET /api/level-exam/sessions/:session_id  场次详情
	g.GET("/sessions/:session_id", func(c *gin.Context) {
		sessionID, err := strconv.Atoi(c.Param("session_id"))
		if err != nil {
			response.BadRequest(c, "场次ID无效")
			return
		}
		result, err := svc.GetSessionDetail(sessionID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// PUT /api/level-exam/sessions/:session_id  更新场次
	g.PUT("/sessions/:session_id", middleware.RoleRequired("admin"), func(c *gin.Context) {
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
		result, err := svc.UpdateSession(sessionID, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "考试场次更新成功", result)
	})

	// DELETE /api/level-exam/sessions/:session_id  删除场次
	g.DELETE("/sessions/:session_id", middleware.RoleRequired("admin"), func(c *gin.Context) {
		sessionID, err := strconv.Atoi(c.Param("session_id"))
		if err != nil {
			response.BadRequest(c, "场次ID无效")
			return
		}
		if err := svc.DeleteSession(sessionID); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "考试场次删除成功", nil)
	})

	// ===== 学员考试流程 =====

	// GET /api/level-exam/available  可用考试列表
	g.GET("/available", middleware.RoleRequired("student"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		result, err := svc.GetAvailableExams(studentID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/level-exam/history  学员考试历史
	g.GET("/history", middleware.RoleRequired("student"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		response.Success(c, svc.GetStudentHistory(studentID, page, pageSize))
	})

	// POST /api/level-exam/sessions/:session_id/enter  进入考试
	g.POST("/sessions/:session_id/enter", middleware.RoleRequired("student"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		sessionID, err := strconv.Atoi(c.Param("session_id"))
		if err != nil {
			response.BadRequest(c, "场次ID无效")
			return
		}
		result, err := svc.EnterExam(sessionID, studentID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "进入考试成功", result)
	})

	// POST /api/level-exam/participants/:participant_id/save  保存答案
	g.POST("/participants/:participant_id/save", middleware.RoleRequired("student"), func(c *gin.Context) {
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
		if err := svc.SaveAnswer(participantID, studentID, req.Answers, req.RemainingTime); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "答案保存成功", nil)
	})

	// POST /api/level-exam/participants/:participant_id/submit  交卷
	g.POST("/participants/:participant_id/submit", middleware.RoleRequired("student"), func(c *gin.Context) {
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
			_ = svc.SaveAnswer(participantID, studentID, req.Answers, rt)
		}
		result, err := svc.SubmitExam(participantID, studentID, req.IsTimeout)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "交卷成功", result)
	})

	// GET /api/level-exam/participants/:participant_id/result  查看结果
	g.GET("/participants/:participant_id/result", middleware.RoleRequired("student"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		participantID, err := strconv.Atoi(c.Param("participant_id"))
		if err != nil {
			response.BadRequest(c, "参与记录ID无效")
			return
		}
		result, err := svc.GetResult(participantID, studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})
}
