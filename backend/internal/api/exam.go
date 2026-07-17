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

// RegisterExamRoutes 注册 /api/exam 蓝图（课程考核，数据源为 JSON 题库）。
func RegisterExamRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewExamService(db)

	g := rg.Group("/exam", middleware.JWTAuth(cfg))

	// GET /api/exam/:course_id  获取课程考核题目
	g.GET("/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := svc.GetExamQuestions(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// POST /api/exam/:course_id/submit  提交考核答案
	g.POST("/:course_id/submit", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var req struct {
			Answers map[string]interface{} `json:"answers"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.Answers) == 0 {
			response.BadRequest(c, "请提交答案")
			return
		}
		result, err := svc.SubmitExam(studentID, courseID, req.Answers)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "交卷成功", result)
	})

	// GET /api/exam/:course_id/result  获取最近一次考核结果
	g.GET("/:course_id/result", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, _ := svc.GetExamResult(studentID, courseID)
		if result == nil {
			c.JSON(200, response.R{Code: 200, Message: "暂无考核记录", Data: nil})
			return
		}
		response.Success(c, result)
	})

	// GET /api/exam/history  获取学员考核历史
	g.GET("/history", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		response.Success(c, svc.GetExamHistory(studentID))
	})
}
