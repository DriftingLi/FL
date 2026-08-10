// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// AdminHandler 管理员后台 handler。
type AdminHandler struct {
	adminSvc      *service.AdminService
	courseSvc     *service.AdminCourseService
	authSvc       *service.AuthService
	aiConfigSvc   *service.AIConfigService
	contentGenSvc *service.ContentGenerateService
}

// NewAdminHandler 创建管理员后台 handler。
func NewAdminHandler(adminSvc *service.AdminService, courseSvc *service.AdminCourseService, authSvc *service.AuthService, aiConfigSvc *service.AIConfigService, contentGenSvc *service.ContentGenerateService) *AdminHandler {
	return &AdminHandler{
		adminSvc: adminSvc, courseSvc: courseSvc, authSvc: authSvc,
		aiConfigSvc: aiConfigSvc, contentGenSvc: contentGenSvc,
	}
}

// RegisterAdminRoutes 注册 /api/admin 蓝图（管理员后台）。
func RegisterAdminRoutes(rg *gin.RouterGroup, deps *Deps) {
	h := NewAdminHandler(deps.AdminSvc, deps.AdminCourseSvc, deps.AuthSvc, deps.AIConfigSvc, deps.ContentGenSvc)

	g := rg.Group("/admin", middleware.JWTAuth(deps.Session), middleware.RoleRequired("admin"))

	// ===== AI 配置（多配置管理 + 功能绑定）=====
	registerSettingsRoutes(g, h.aiConfigSvc, deps.DB)

	// ===== 课程管理 =====
	g.GET("/courses", h.ListCourses)
	g.POST("/course", h.CreateCourse)
	g.GET("/course/:course_id", h.GetCourseDetail)
	g.PUT("/course/:course_id", h.UpdateCourse)
	g.PUT("/course/:course_id/sort", h.SwapCourseSort)
	g.DELETE("/course/:course_id", h.DeleteCourse)
	g.POST("/course/:course_id/chapter", h.CreateChapter)
	g.PUT("/chapter/:chapter_id", h.UpdateChapter)
	g.DELETE("/chapter/:chapter_id", h.DeleteChapter)
	g.POST("/course/generate-content", h.GenerateContent)
	g.GET("/course/generate-content/:task_id", h.GetGenerationTask)

	// ===== HRWAI 用户管理(统一) =====
	// 合并原学员管理与评估用户管理两套接口,操作 hrwai_users 表。
	// 旧路由 /admin/students、/admin/student/* 保留为兼容别名,前端已切到 /admin/hrwai-users/*。
	g.GET("/hrwai-users", h.ListHrwaiUsers)
	g.POST("/hrwai-users", h.CreateHrwaiUser)
	g.PUT("/hrwai-users/:id", h.UpdateHrwaiUser)
	g.PUT("/hrwai-users/:id/password", h.ResetHrwaiUserPassword)
	g.PUT("/hrwai-users/:id/status", h.ToggleHrwaiUserStatus)
	g.DELETE("/hrwai-users/:id", h.DeleteHrwaiUser)

	// ===== 导师管理 =====
	g.GET("/tutors", h.ListTutors)
	g.POST("/tutor", h.CreateTutor)
	g.DELETE("/tutor/:tutor_id", h.DeleteTutor)
	g.PUT("/tutor/:tutor_id/password", h.ResetTutorPassword)
	g.PUT("/tutor/:tutor_id/status", h.ToggleTutorStatus)

	// ===== 统计看板 =====
	g.GET("/statistics", h.GetStatistics)
}

// ListCourses 课程列表 GET /api/admin/courses
func (h *AdminHandler) ListCourses(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	keyword := c.Query("keyword")
	specialtyID := queryIntPtr(c, "specialty_id")
	levelID := queryIntPtr(c, "level_id")
	response.Success(c, h.courseSvc.GetCourses(page, pageSize, keyword, specialtyID, levelID))
}

// CreateCourse 创建课程 POST /api/admin/course
func (h *AdminHandler) CreateCourse(c *gin.Context) {
	var data map[string]any
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.courseSvc.CreateCourse(data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "课程创建成功", result)
}

// GetCourseDetail 课程详情 GET /api/admin/course/:course_id
func (h *AdminHandler) GetCourseDetail(c *gin.Context) {
	courseID, err := strconv.Atoi(c.Param("course_id"))
	if err != nil {
		response.BadRequest(c, "课程ID无效")
		return
	}
	result, err := h.courseSvc.GetCourseDetail(courseID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// UpdateCourse 更新课程 PUT /api/admin/course/:course_id
func (h *AdminHandler) UpdateCourse(c *gin.Context) {
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
	result, err := h.courseSvc.UpdateCourse(courseID, data)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "课程更新成功", result)
}

// SwapCourseSort 交换课程排序 PUT /api/admin/course/:course_id/sort（同一方向+等级组内，body: {"swap_with": <id>}）
func (h *AdminHandler) SwapCourseSort(c *gin.Context) {
	courseID, err := strconv.Atoi(c.Param("course_id"))
	if err != nil {
		response.BadRequest(c, "课程ID无效")
		return
	}
	var body struct {
		SwapWith int `json:"swap_with"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
		response.BadRequest(c, "swap_with 参数无效")
		return
	}
	if err := h.courseSvc.SwapCourseSort(courseID, body.SwapWith); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "排序已交换", nil)
}

// DeleteCourse 删除课程 DELETE /api/admin/course/:course_id
func (h *AdminHandler) DeleteCourse(c *gin.Context) {
	courseID, err := strconv.Atoi(c.Param("course_id"))
	if err != nil {
		response.BadRequest(c, "课程ID无效")
		return
	}
	result, err := h.courseSvc.DeleteCourse(courseID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "课程删除成功", result)
}

// CreateChapter 创建章节 POST /api/admin/course/:course_id/chapter
func (h *AdminHandler) CreateChapter(c *gin.Context) {
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
	result, err := h.courseSvc.CreateChapter(courseID, data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "章节创建成功", result)
}

// UpdateChapter 更新章节 PUT /api/admin/chapter/:chapter_id
func (h *AdminHandler) UpdateChapter(c *gin.Context) {
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
	result, err := h.courseSvc.UpdateChapter(chapterID, data)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "章节更新成功", result)
}

// DeleteChapter 删除章节 DELETE /api/admin/chapter/:chapter_id
func (h *AdminHandler) DeleteChapter(c *gin.Context) {
	chapterID, err := strconv.Atoi(c.Param("chapter_id"))
	if err != nil {
		response.BadRequest(c, "章节ID无效")
		return
	}
	result, err := h.courseSvc.DeleteChapter(chapterID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "章节删除成功", result)
}

// GenerateContent 异步生成课程内容 POST /api/admin/course/generate-content
func (h *AdminHandler) GenerateContent(c *gin.Context) {
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
	taskID, err := h.contentGenSvc.StartGeneration(req.CourseID, req.ChapterIDs, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "生成任务已启动", map[string]any{"task_id": taskID})
}

// GetGenerationTask 查询生成任务状态（前端轮询）GET /api/admin/course/generate-content/:task_id
func (h *AdminHandler) GetGenerationTask(c *gin.Context) {
	taskID := c.Param("task_id")
	status, err := h.contentGenSvc.GetTaskStatus(taskID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, status)
}

// ListHrwaiUsers HRWAI 用户列表 GET /api/admin/hrwai-users
func (h *AdminHandler) ListHrwaiUsers(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	keyword := c.Query("keyword")
	result, err := h.adminSvc.ListHrwaiUsers(page, pageSize, keyword)
	if err != nil {
		response.BadRequest(c, "查询用户列表失败")
		return
	}
	response.Success(c, result)
}

// CreateHrwaiUser 新增 HRWAI 用户 POST /api/admin/hrwai-users
func (h *AdminHandler) CreateHrwaiUser(c *gin.Context) {
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
	user, err := h.adminSvc.CreateHrwaiUser(req.Phone, req.Password, req.Name, req.Email, req.Company)
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
}

// UpdateHrwaiUser 更新 HRWAI 用户资料(不含密码) PUT /api/admin/hrwai-users/:id
func (h *AdminHandler) UpdateHrwaiUser(c *gin.Context) {
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
	if err := h.adminSvc.UpdateHrwaiUser(id, req.Name, req.Email, req.Company, req.Status); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "用户资料已更新", nil)
}

// ResetHrwaiUserPassword 重置 HRWAI 用户密码 PUT /api/admin/hrwai-users/:id/password
func (h *AdminHandler) ResetHrwaiUserPassword(c *gin.Context) {
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
	if err := h.adminSvc.ResetHrwaiUserPassword(id, req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "密码已重置", nil)
}

// ToggleHrwaiUserStatus 切换 HRWAI 用户启用/禁用状态 PUT /api/admin/hrwai-users/:id/status
func (h *AdminHandler) ToggleHrwaiUserStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "用户ID无效")
		return
	}
	next, err := h.adminSvc.ToggleHrwaiUserStatus(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	msg := "用户已启用"
	if next == 0 {
		msg = "用户已禁用"
	}
	response.SuccessWithMsg(c, msg, map[string]any{"status": next})
}

// DeleteHrwaiUser 删除 HRWAI 用户 DELETE /api/admin/hrwai-users/:id
func (h *AdminHandler) DeleteHrwaiUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "用户ID无效")
		return
	}
	if err := h.adminSvc.DeleteHrwaiUser(id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "用户删除成功", nil)
}

// ListTutors 导师列表 GET /api/admin/tutors
func (h *AdminHandler) ListTutors(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 10)
	keyword := c.Query("keyword")
	response.Success(c, h.adminSvc.GetTutors(page, pageSize, keyword))
}

// CreateTutor 添加导师 POST /api/admin/tutor
func (h *AdminHandler) CreateTutor(c *gin.Context) {
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
	result, err := h.authSvc.TutorRegister(req.Username, req.Password, req.Name)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "导师添加成功", result)
}

// DeleteTutor 删除导师 DELETE /api/admin/tutor/:tutor_id
func (h *AdminHandler) DeleteTutor(c *gin.Context) {
	tutorID, err := strconv.Atoi(c.Param("tutor_id"))
	if err != nil {
		response.BadRequest(c, "导师ID无效")
		return
	}
	result, err := h.adminSvc.DeleteTutor(tutorID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "导师删除成功", result)
}

// ResetTutorPassword 重置导师密码 PUT /api/admin/tutor/:tutor_id/password
func (h *AdminHandler) ResetTutorPassword(c *gin.Context) {
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
	if err := h.adminSvc.ResetTutorPassword(tutorID, req.Password); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "密码已重置", nil)
}

// ToggleTutorStatus 切换导师启用/禁用状态 PUT /api/admin/tutor/:tutor_id/status
func (h *AdminHandler) ToggleTutorStatus(c *gin.Context) {
	tutorID, err := strconv.Atoi(c.Param("tutor_id"))
	if err != nil {
		response.BadRequest(c, "导师ID无效")
		return
	}
	next, err := h.adminSvc.ToggleTutorStatus(tutorID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	msg := "导师已启用"
	if next == 0 {
		msg = "导师已禁用"
	}
	response.SuccessWithMsg(c, msg, map[string]any{"status": next})
}

// GetStatistics 统计看板 GET /api/admin/statistics
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	response.Success(c, h.adminSvc.GetStatistics())
}
