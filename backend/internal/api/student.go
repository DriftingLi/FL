// Package api 实现 HTTP handlers。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/pkg/response"
)

// RegisterStudentRoutes 注册 /api/student 蓝图。
func RegisterStudentRoutes(rg *gin.RouterGroup, deps *Deps) {
	svc := deps.StudentSvc

	g := rg.Group("/student", middleware.JWTAuth(deps.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/student/profile  学员信息+学习统计+课程进度
	g.GET("/profile", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		result, err := svc.GetProfile(studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// GET /api/student/records  学员学习记录分页
	g.GET("/records", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		startDate := c.Query("start_date")
		endDate := c.Query("end_date")
		result := svc.GetRecords(studentID, page, pageSize, startDate, endDate)
		response.Success(c, result)
	})

	// GET /api/student/study-stats  学员仪表盘学习统计（按天分组）
	//   query: days=7|30（其他值回退为 7）
	g.GET("/study-stats", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		days := atoiDefault(c.Query("days"), 7)
		result := svc.GetStudyStats(studentID, days)
		response.Success(c, result)
	})
}
