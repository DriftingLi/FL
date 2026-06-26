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

// RegisterPracticeRoutes 注册 /api/practice 蓝图（叉车实操模拟记录）。
// 对应 Python app/api/practice.py。
func RegisterPracticeRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewPracticeService(db)

	g := rg.Group("/practice", middleware.JWTAuth(cfg))

	// POST /api/practice/record  保存实操记录
	g.POST("/record", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil || data == nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		if _, ok := data["practice_type"].(string); !ok || data["practice_type"] == "" {
			response.BadRequest(c, "请指定实操类型")
			return
		}
		result, err := svc.SaveRecord(studentID, data)
		if err != nil {
			response.ServerError(c, "保存失败: "+err.Error())
			return
		}
		response.SuccessWithMsg(c, "保存成功", result)
	})

	// GET /api/practice/record/:record_id  记录详情（admin 可查任意，学员仅查本人）
	g.GET("/record/:record_id", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		role, _ := c.Get(string(middleware.CtxUserRole))
		studentID, _ := uid.(int)
		roleStr, _ := role.(string)
		recordID, err := strconv.Atoi(c.Param("record_id"))
		if err != nil {
			response.BadRequest(c, "记录ID无效")
			return
		}
		queryStudentID := studentID
		if roleStr == "admin" {
			queryStudentID = 0 // 0 表示不限制
		}
		result, err := svc.GetRecord(recordID, queryStudentID)
		if err != nil {
			response.NotFound(c, "记录不存在")
			return
		}
		response.Success(c, result)
	})

	// GET /api/practice/records  学员本人的实操记录分页
	g.GET("/records", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		practiceType := c.Query("practice_type")
		result := svc.GetRecords(studentID, page, pageSize, practiceType)
		response.Success(c, result)
	})

	// GET /api/practice/stats  学员本人的实操统计
	g.GET("/stats", func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		studentID, _ := uid.(int)
		result := svc.GetStats(studentID)
		response.Success(c, result)
	})

	// GET /api/practice/admin/stats  管理员实操统计
	g.GET("/admin/stats", middleware.RoleRequired("admin"), func(c *gin.Context) {
		response.Success(c, svc.GetAdminStats())
	})

	// GET /api/practice/admin/records  管理员实操记录列表
	g.GET("/admin/records", middleware.RoleRequired("admin"), func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		practiceType := c.Query("practice_type")
		var studentID *int
		if s := c.Query("student_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				studentID = &id
			}
		}
		result := svc.GetAdminRecords(page, pageSize, practiceType, studentID)
		response.Success(c, result)
	})
}

// atoiDefault 字符串转 int，失败或为空时返回默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
