// Package api 实现 HTTP handlers。
package api

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/response"
)

// RegisterPracticeModeRoutes 注册 /api/practice-mode 蓝图（题库练习）。
func RegisterPracticeModeRoutes(rg *gin.RouterGroup, deps *Deps) {
	svc := deps.PracticeModeSvc
	cfg := deps.Cfg

	g := rg.Group("/practice-mode", middleware.JWTAuth(cfg), middleware.RoleRequired("hrwai_user"))

	// GET /api/practice-mode/free  随机练习抽题（count 控制题量）
	g.GET("/free", func(c *gin.Context) {
		qType := c.Query("type")
		count := atoiDefault(c.Query("count"), 20)
		result, err := svc.GetFreeQuestions(qType, count)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice-mode/tag  标签练习开始/续练（按题库标签，count 控制题量）
	g.GET("/tag", func(c *gin.Context) {
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
		result, err := svc.StartTagPractice(studentID, tagID, count)
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

	// POST /api/practice-mode/progress  保存练习游标和答题状态（支持顺序/专项/标签练习）
	g.POST("/progress", func(c *gin.Context) {
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
