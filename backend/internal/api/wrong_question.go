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

// RegisterWrongQuestionRoutes 注册 /api/wrong-questions 蓝图。
func RegisterWrongQuestionRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewWrongQuestionService(db)

	g := rg.Group("/wrong-questions", middleware.JWTAuth(cfg), middleware.RoleRequired("student"))

	// GET /api/wrong-questions  错题列表（分页+过滤）
	g.GET("", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		qType := c.Query("type")
		var kpID *int
		if s := c.Query("knowledge_point_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				kpID = &id
			}
		}
		var minWrongCount *int
		if s := c.Query("min_wrong_count"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				minWrongCount = &v
			}
		}
		response.Success(c, svc.GetWrongQuestions(studentID, page, pageSize, qType, kpID, minWrongCount))
	})

	// POST /api/wrong-questions/:question_id/redo  重做错题
	g.POST("/:question_id/redo", func(c *gin.Context) {
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
		result, err := svc.RedoWrongQuestion(studentID, questionID, req.UserAnswer)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/wrong-questions/:question_id/remove  移出错题本
	g.POST("/:question_id/remove", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		questionID, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		result, err := svc.RemoveWrongQuestion(studentID, questionID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "已移出错题本", result)
	})

	// GET /api/wrong-questions/stats  错题统计
	g.GET("/stats", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		response.Success(c, svc.GetStats(studentID))
	})

	// GET /api/wrong-questions/export  导出错题本（纯文本附件）
	g.GET("/export", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		data := svc.ExportWrongQuestions(studentID)
		text := service.FormatWrongQuestionsText(data)
		c.Header("Content-Disposition", "attachment; filename=wrong_questions.txt")
		c.Data(200, "text/plain; charset=utf-8", []byte(text))
	})
}
