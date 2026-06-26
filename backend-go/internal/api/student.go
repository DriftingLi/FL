// Package api 实现 HTTP handlers。
package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterStudentRoutes 注册 /api/student 蓝图。
// 对应 Python app/api/student.py。
func RegisterStudentRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewStudentService(db)

	g := rg.Group("/student", middleware.JWTAuth(cfg), middleware.RoleRequired("student"))

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
}
