// Package api 实现 HTTP handlers。
package api

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// PracticeModeHandler 题库练习 handler。
type PracticeModeHandler struct {
	svc *service.PracticeModeService
}

// NewPracticeModeHandler 创建题库练习 handler。
func NewPracticeModeHandler(svc *service.PracticeModeService) *PracticeModeHandler {
	return &PracticeModeHandler{svc: svc}
}

// RegisterPracticeModeRoutes 注册 /api/practice-mode 蓝图（题库练习）。
func RegisterPracticeModeRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.PracticeModeService) {
	h := NewPracticeModeHandler(svc)

	g := rg.Group("/practice-mode", middleware.JWTAuth(sess), middleware.RoleRequired("hrwai_user"))

	g.GET("/free", h.GetFreeQuestions)
	g.GET("/tag", h.StartTagPractice)
	g.GET("/sequential", h.StartSequential)
	g.GET("/sequential-progress", h.GetSequentialProgress)
	g.POST("/progress", h.SaveProgress)
	g.GET("/progress", h.GetProgress)
	g.POST("/submit", h.SubmitAnswer)
	g.GET("/stats", h.GetStats)
	g.GET("/history", h.GetHistory)
}

// GetFreeQuestions 随机练习抽题 GET /api/practice-mode/free（count 控制题量）
func (h *PracticeModeHandler) GetFreeQuestions(c *gin.Context) {
	qType := c.Query("type")
	count := atoiDefault(c.Query("count"), 20)
	result, err := h.svc.GetFreeQuestions(qType, count)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// StartTagPractice 标签练习开始/续练 GET /api/practice-mode/tag（按题库标签，count 控制题量）
func (h *PracticeModeHandler) StartTagPractice(c *gin.Context) {
	tagIDStr := c.Query("tag_id")
	if tagIDStr == "" {
		response.BadRequest(c, "请指定题库标签")
		return
	}
	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil || tagID <= 0 {
		response.BadRequest(c, "题库标签ID无效")
		return
	}
	count := atoiDefault(c.Query("count"), 0) // 0=全部
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	result, err := h.svc.StartTagPractice(studentID, tagID, count)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// StartSequential 顺序练习（开始/续练，返回当前批次+进度）GET /api/practice-mode/sequential
func (h *PracticeModeHandler) StartSequential(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	result, err := h.svc.StartSequential(studentID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetSequentialProgress 顺序练习进度（卡片展示用）GET /api/practice-mode/sequential-progress
func (h *PracticeModeHandler) GetSequentialProgress(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	response.Success(c, h.svc.GetSequentialProgress(studentID))
}

// SaveProgress 保存练习游标和答题状态（支持顺序/专项/标签练习）POST /api/practice-mode/progress
func (h *PracticeModeHandler) SaveProgress(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	var req struct {
		Index        int             `json:"index"`
		PracticeMode string          `json:"practice_mode"`
		Total        int             `json:"total"`
		AnswersState json.RawMessage `json:"answers_state"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if err := h.svc.SaveProgress(studentID, req.Index, req.PracticeMode, req.Total, req.AnswersState); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, map[string]any{"saved": true, "index": req.Index})
}

// GetProgress 查询任意模式的练习进度（断点续练用）GET /api/practice-mode/progress?mode=xxx
func (h *PracticeModeHandler) GetProgress(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	mode := c.Query("mode")
	if mode == "" {
		mode = "sequential"
	}
	response.Success(c, h.svc.GetProgress(studentID, mode))
}

// SubmitAnswer 提交答案 POST /api/practice-mode/submit
func (h *PracticeModeHandler) SubmitAnswer(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	var req struct {
		QuestionID   int         `json:"question_id"`
		UserAnswer   interface{} `json:"user_answer"`
		PracticeType string      `json:"practice_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	if req.QuestionID == 0 {
		response.BadRequest(c, "题目ID不能为空")
		return
	}
	if req.PracticeType == "" {
		req.PracticeType = "free"
	}
	result, err := h.svc.SubmitAnswer(studentID, req.QuestionID, req.UserAnswer, req.PracticeType)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

// GetStats 练习统计 GET /api/practice-mode/stats
func (h *PracticeModeHandler) GetStats(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	response.Success(c, h.svc.GetStats(studentID))
}

// GetHistory 练习历史 GET /api/practice-mode/history
func (h *PracticeModeHandler) GetHistory(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	qType := c.Query("type")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	response.Success(c, h.svc.GetHistory(studentID, page, pageSize, qType, startDate, endDate))
}
