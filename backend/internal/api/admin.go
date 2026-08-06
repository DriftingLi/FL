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

// RegisterAdminRoutes 注册 /api/admin 蓝图（管理员后台）。
// aiConfigSvc 由上层 NewRouter 创建并传入，便于 AI 助手模块复用同一实例。
func RegisterAdminRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB, aiConfigSvc *service.AIConfigService) {
	adminSvc := service.NewAdminService(db)
	courseSvc := service.NewAdminCourseService(db)
	authSvc := service.NewAuthService(db, cfg.JWTSecretKey, cfg.JWTExpiry(),
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student)

	// AI 多配置 service + 课程内容生成 service
	aiSvc := service.NewAIService(db, aiConfigSvc)
	contentGenSvc := service.NewContentGenerateService(db, aiSvc)

	g := rg.Group("/admin", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// ===== AI 配置（多配置管理 + 功能绑定）=====
	registerSettingsRoutes(g, aiConfigSvc, db)

	// ===== 课程管理 =====

	// GET /api/admin/courses  课程列表
	g.GET("/courses", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		keyword := c.Query("keyword")
		category := c.Query("category")
		specialtyID := queryIntPtr(c, "specialty_id")
		levelID := queryIntPtr(c, "level_id")
		response.Success(c, courseSvc.GetCourses(page, pageSize, keyword, category, specialtyID, levelID))
	})

	// POST /api/admin/course  创建课程
	g.POST("/course", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.CreateCourse(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "课程创建成功", result)
	})

	// GET /api/admin/course/:course_id  课程详情
	g.GET("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := courseSvc.GetCourseDetail(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// PUT /api/admin/course/:course_id  更新课程
	g.PUT("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.UpdateCourse(courseID, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程更新成功", result)
	})

	// DELETE /api/admin/course/:course_id  删除课程
	g.DELETE("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := courseSvc.DeleteCourse(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程删除成功", result)
	})

	// POST /api/admin/course/:course_id/chapter  创建章节
	g.POST("/course/:course_id/chapter", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.CreateChapter(courseID, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "章节创建成功", result)
	})

	// PUT /api/admin/chapter/:chapter_id  更新章节
	g.PUT("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.UpdateChapter(chapterID, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "章节更新成功", result)
	})

	// DELETE /api/admin/chapter/:chapter_id  删除章节
	g.DELETE("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := courseSvc.DeleteChapter(chapterID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "章节删除成功", result)
	})

	// POST /api/admin/course/generate-content  异步生成课程内容
	g.POST("/course/generate-content", func(c *gin.Context) {
		var req struct {
			CourseID   int   `json:"course_id"`
			ChapterIDs []int `json:"chapter_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.CourseID == 0 {
			response.BadRequest(c, "请选择课程")
			return
		}
		if len(req.ChapterIDs) == 0 {
			response.BadRequest(c, "请选择至少一个章节")
			return
		}
		userID := c.GetInt("user_id")
		taskID, err := contentGenSvc.StartGeneration(req.CourseID, req.ChapterIDs, userID)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "生成任务已启动", map[string]any{"task_id": taskID})
	})

	// GET /api/admin/course/generate-content/:task_id  查询生成任务状态（前端轮询）
	g.GET("/course/generate-content/:task_id", func(c *gin.Context) {
		taskID := c.Param("task_id")
		status, err := contentGenSvc.GetTaskStatus(taskID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, status)
	})

	// ===== HRWAI 用户管理(统一) =====
	// 合并原学员管理与评估用户管理两套接口,操作 hrwai_users 表。
	// 旧路由 /admin/students、/admin/student/* 保留为兼容别名,前端已切到 /admin/hrwai-users/*。

	// GET /api/admin/hrwai-users  HRWAI 用户列表
	g.GET("/hrwai-users", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		keyword := c.Query("keyword")
		result, err := adminSvc.ListHrwaiUsers(page, pageSize, keyword)
		if err != nil {
			response.BadRequest(c, "查询用户列表失败")
			return
		}
		response.Success(c, result)
	})

	// POST /api/admin/hrwai-users  新增 HRWAI 用户
	g.POST("/hrwai-users", func(c *gin.Context) {
		var req struct {
			Phone    string `json:"phone"`
			Password string `json:"password"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Company  string `json:"company"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		user, err := adminSvc.CreateHrwaiUser(req.Phone, req.Password, req.Name, req.Email, req.Company)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "用户添加成功", map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"name":     user.Name,
			"nickname": user.Nickname,
			"phone":    user.Phone,
		})
	})

	// PUT /api/admin/hrwai-users/:id  更新 HRWAI 用户资料(不含密码)
	g.PUT("/hrwai-users/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "用户ID无效")
			return
		}
		var req struct {
			Name    string `json:"name"`
			Email   string `json:"email"`
			Company string `json:"company"`
			Status  int16  `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Name == "" {
			response.BadRequest(c, "姓名不能为空")
			return
		}
		if req.Status != 0 && req.Status != 1 {
			response.BadRequest(c, "状态值非法(仅支持 0/1)")
			return
		}
		if err := adminSvc.UpdateHrwaiUser(id, req.Name, req.Email, req.Company, req.Status); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "用户资料已更新", nil)
	})

	// PUT /api/admin/hrwai-users/:id/password  重置 HRWAI 用户密码
	g.PUT("/hrwai-users/:id/password", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "用户ID无效")
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.Password) < 6 || len(req.Password) > 20 {
			response.BadRequest(c, "密码长度需为 6-20 个字符")
			return
		}
		if err := adminSvc.ResetHrwaiUserPassword(id, req.Password); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "密码已重置", nil)
	})

	// PUT /api/admin/hrwai-users/:id/status  切换 HRWAI 用户启用/禁用状态
	g.PUT("/hrwai-users/:id/status", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "用户ID无效")
			return
		}
		next, err := adminSvc.ToggleHrwaiUserStatus(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		msg := "用户已启用"
		if next == 0 {
			msg = "用户已禁用"
		}
		response.SuccessWithMsg(c, msg, map[string]any{"status": next})
	})

	// DELETE /api/admin/hrwai-users/:id  删除 HRWAI 用户
	g.DELETE("/hrwai-users/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "用户ID无效")
			return
		}
		if err := adminSvc.DeleteHrwaiUser(id); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "用户删除成功", nil)
	})

	// ===== 导师管理 =====

	// GET /api/admin/tutors  导师列表
	g.GET("/tutors", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		keyword := c.Query("keyword")
		response.Success(c, adminSvc.GetTutors(page, pageSize, keyword))
	})

	// POST /api/admin/tutor  添加导师
	g.POST("/tutor", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Username == "" || req.Password == "" || req.Name == "" {
			response.BadRequest(c, "用户名、密码和姓名不能为空")
			return
		}
		result, err := authSvc.TutorRegister(req.Username, req.Password, req.Name)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "导师添加成功", result)
	})

	// DELETE /api/admin/tutor/:tutor_id  删除导师
	g.DELETE("/tutor/:tutor_id", func(c *gin.Context) {
		tutorID, err := strconv.Atoi(c.Param("tutor_id"))
		if err != nil {
			response.BadRequest(c, "导师ID无效")
			return
		}
		result, err := adminSvc.DeleteTutor(tutorID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "导师删除成功", result)
	})

	// PUT /api/admin/tutor/:tutor_id/password  重置导师密码
	g.PUT("/tutor/:tutor_id/password", func(c *gin.Context) {
		tutorID, err := strconv.Atoi(c.Param("tutor_id"))
		if err != nil {
			response.BadRequest(c, "导师ID无效")
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.Password) < 6 || len(req.Password) > 20 {
			response.BadRequest(c, "密码长度需为 6-20 个字符")
			return
		}
		if err := adminSvc.ResetTutorPassword(tutorID, req.Password); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "密码已重置", nil)
	})

	// PUT /api/admin/tutor/:tutor_id/status  切换导师启用/禁用状态
	g.PUT("/tutor/:tutor_id/status", func(c *gin.Context) {
		tutorID, err := strconv.Atoi(c.Param("tutor_id"))
		if err != nil {
			response.BadRequest(c, "导师ID无效")
			return
		}
		next, err := adminSvc.ToggleTutorStatus(tutorID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		msg := "导师已启用"
		if next == 0 {
			msg = "导师已禁用"
		}
		response.SuccessWithMsg(c, msg, map[string]any{"status": next})
	})

	// ===== 统计看板 =====

	// GET /api/admin/statistics  统计看板
	g.GET("/statistics", func(c *gin.Context) {
		response.Success(c, adminSvc.GetStatistics())
	})
}
