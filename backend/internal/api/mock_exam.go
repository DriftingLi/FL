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

// RegisterMockExamRoutes 注册 /api/mock-exam 蓝图。
func RegisterMockExamRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewMockExamService(db, newAIService(cfg, db))

	g := rg.Group("/mock-exam", middleware.JWTAuth(cfg), middleware.RoleRequired("student"))

	// POST /api/mock-exam/start  开始模拟考试（count 题量 + duration 时长）
	g.POST("/start", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		var req struct {
			Count    int `json:"count"`
			Duration int `json:"duration"`
		}
		_ = c.ShouldBindJSON(&req)
		if req.Duration == 0 {
			req.Duration = 90
		}
		result, err := svc.Start(studentID, req.Count, req.Duration)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "模拟考试开始", result)
	})

	// POST /api/mock-exam/:mock_exam_id/save  保存进度
	g.POST("/:mock_exam_id/save", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
		if err != nil {
			response.BadRequest(c, "考试ID无效")
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
		if err := svc.SaveProgress(mockExamID, studentID, req.Answers, req.RemainingTime); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "进度保存成功", nil)
	})

	// GET /api/mock-exam/:mock_exam_id/resume  恢复考试
	g.GET("/:mock_exam_id/resume", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
		if err != nil {
			response.BadRequest(c, "考试ID无效")
			return
		}
		result, err := svc.Resume(mockExamID, studentID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/mock-exam/:mock_exam_id/submit  交卷
	g.POST("/:mock_exam_id/submit", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
		if err != nil {
			response.BadRequest(c, "考试ID无效")
			return
		}
		result, err := svc.Submit(mockExamID, studentID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "交卷成功", result)
	})

	// GET /api/mock-exam/:mock_exam_id/result  获取结果
	g.GET("/:mock_exam_id/result", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		mockExamID, err := strconv.Atoi(c.Param("mock_exam_id"))
		if err != nil {
			response.BadRequest(c, "考试ID无效")
			return
		}
		result, err := svc.GetResult(mockExamID, studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/mock-exam/history  历史记录
	g.GET("/history", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		response.Success(c, svc.GetHistory(studentID, page, pageSize))
	})
}

// newAIService 在蓝图内构造 AIService，集中配置依赖。
func newAIService(cfg *config.Config, db *gorm.DB) *service.AIService {
	apiKey := cfg.ZhipuAPIKey
	if apiKey == "" {
		apiKey = cfg.OpenAIAPIKey
	}
	return service.NewAIService(db, apiKey, cfg.ZhipuBaseURL, cfg.ZhipuModel)
}
