// Package api 实现 HTTP handlers。
package api

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterPracticeModeRoutes 注册 /api/practice-mode 蓝图（题库练习）。
func RegisterPracticeModeRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewPracticeModeService(db, newAIService(cfg, db))

	g := rg.Group("/practice-mode", middleware.JWTAuth(cfg), middleware.RoleRequired("student"))

	// GET /api/practice-mode/free  随机练习抽题（count 控制题量）
	g.GET("/free", func(c *gin.Context) {
		qType := c.Query("type")
		var kpID *int
		if s := c.Query("knowledge_point_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				kpID = &id
			}
		}
		count := atoiDefault(c.Query("count"), 20)
		result, err := svc.GetFreeQuestions(qType, kpID, count)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/sequential  顺序练习（开始/续练，返回当前批次+进度）
	g.GET("/sequential", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		result, err := svc.StartSequential(studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/sequential-progress  顺序练习进度（卡片展示用）
	g.GET("/sequential-progress", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		response.Success(c, svc.GetSequentialProgress(studentID))
	})

	// POST /api/practice-mode/progress  保存练习游标和答题状态（支持顺序/专项/章节练习）
	g.POST("/progress", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		var req struct {
			Index        int              `json:"index"`
			PracticeMode string           `json:"practice_mode"`
			Total        int              `json:"total"`
			AnswersState json.RawMessage  `json:"answers_state"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		if err := svc.SaveProgress(studentID, req.Index, req.PracticeMode, req.Total, req.AnswersState); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, map[string]any{"saved": true, "index": req.Index})
	})

	// GET /api/practice-mode/progress?mode=xxx  查询任意模式的练习进度（断点续练用）
	g.GET("/progress", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		mode := c.Query("mode")
		if mode == "" {
			mode = "sequential"
		}
		response.Success(c, svc.GetProgress(studentID, mode))
	})

	// GET /api/practice-mode/category  章节练习：按课程分类抽题
	g.GET("/category", func(c *gin.Context) {
		category := c.Query("category")
		if category == "" {
			response.BadRequest(c, "请指定课程分类")
			return
		}
		count := atoiDefault(c.Query("count"), 0) // 0=全部
		result, err := svc.GetCategoryQuestions(category, count)
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
