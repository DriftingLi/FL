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

// RegisterPracticeModeRoutes 注册 /api/practice-mode 蓝图（题库练习）。
// 对应 Python app/api/practice_mode.py。
func RegisterPracticeModeRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewPracticeModeService(db, newAIService(cfg, db))

	g := rg.Group("/practice-mode", middleware.JWTAuth(cfg), middleware.RoleRequired("student"))

	// GET /api/practice-mode/free  自由练习随机抽题
	g.GET("/free", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		qType := c.Query("type")
		var kpID *int
		if s := c.Query("knowledge_point_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				kpID = &id
			}
		}
		result, err := svc.GetFreeQuestions(studentID, qType, kpID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/knowledge-point  按知识点练习
	g.GET("/knowledge-point", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		kpIDStr := c.Query("knowledge_point_id")
		if kpIDStr == "" {
			response.BadRequest(c, "请指定知识点")
			return
		}
		kpID, err := strconv.Atoi(kpIDStr)
		if err != nil {
			response.BadRequest(c, "知识点ID无效")
			return
		}
		count := atoiDefault(c.Query("count"), 0)
		randomOrder := c.Query("random") == "true" || c.Query("random") == "True"
		result, err := svc.GetKnowledgePointPractice(studentID, kpID, count, randomOrder)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/knowledge-point-progress  知识点进度
	g.GET("/knowledge-point-progress", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		var kpID *int
		if s := c.Query("knowledge_point_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				kpID = &id
			}
		}
		result, err := svc.GetKnowledgePointProgress(studentID, kpID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/practice-mode/submit  提交答案
	g.POST("/submit", func(c *gin.Context) {
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
		result, err := svc.SubmitAnswer(studentID, req.QuestionID, req.UserAnswer, req.PracticeType)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/stats  练习统计
	g.GET("/stats", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		response.Success(c, svc.GetStats(studentID))
	})

	// GET /api/practice-mode/history  练习历史
	g.GET("/history", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		qType := c.Query("type")
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")
		response.Success(c, svc.GetHistory(studentID, page, pageSize, qType, startDate, endDate))
	})
}
