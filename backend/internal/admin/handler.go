// 本文件：管理域 HTTP 出口之一（/api/admin 蓝图：hrwai 用户、讲师、统计、内容生成）。
// 招聘者面见 handler_recruiter.go；课程与章节管理端在 internal/course/handler_admin.go（波 4d 裁决 D1）。
// 装配点：internal/api/routes_registry.go 调 admin.RegisterRoutes(api, rd.Session, deps.AdminSvc, deps.AuthSvc, deps.AIConfigSvc, deps.ContentGenSvc)。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/auth"
	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// adminHandler 管理域 handler 之一（hrwai 用户、讲师、统计、内容生成）。
type adminHandler struct {
	adminSvc      *Service
	authSvc       *auth.Service
	aiConfigSvc   *aiassistant.ConfigService
	contentGenSvc *service.ContentGenerateService
}

// newHandler 创建管理员后台 handler。
func newHandler(adminSvc *Service, authSvc *auth.Service, aiConfigSvc *aiassistant.ConfigService, contentGenSvc *service.ContentGenerateService) *adminHandler {
	return &adminHandler{
		adminSvc: adminSvc, authSvc: authSvc,
		aiConfigSvc: aiConfigSvc, contentGenSvc: contentGenSvc,
	}
}

// RegisterRoutes 注册 /api/admin 蓝图（管理员后台：hrwai 用户、讲师、统计、内容生成）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, adminSvc *Service, authSvc *auth.Service, aiConfigSvc *aiassistant.ConfigService, contentGenSvc *service.ContentGenerateService) {
	h := newHandler(adminSvc, authSvc, aiConfigSvc, contentGenSvc)

	g := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAdminAccess))

	// ===== AI 配置（多配置管理 + 功能绑定）=====
	aiassistant.RegisterAdminRoutes(g, aiConfigSvc)

	// ===== 课程内容生成（留驻：实现仍在 internal/service，未随课程域搬包）=====
	// 原 :48-59 的九条课程 / 章节管理端点已搬进 internal/course/handler_admin.go（波 4d 裁决 D1）；
	// 这两条生成面（含 service.ContentGenerateService）属管理域射程，路由留在本蓝图，
	// 与课程域那份注册按完整路径并存（gin 只按完整路径匹配，两组同名 /admin 不冲突）。
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

// @Summary 启动课程内容异步生成
// @Description 管理员为指定课程的章节启动 AI 内容生成，返回 task_id 供轮询
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "生成请求 {course_id,chapter_ids}"
// @Success 201 {object} response.R{data=service.GenerateContentResultDTO} "生成任务已启动"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/course/generate-content [post]
// GenerateContent 异步生成课程内容 POST /api/admin/course/generate-content
func (h *adminHandler) GenerateContent(c *gin.Context) {
	httpx.Endpoint[generateContentReq, service.GenerateContentResultDTO]{
		Parse: func(c *gin.Context) (*generateContentReq, error) {
			var req struct {
				CourseID   int   `json:"course_id"`
				ChapterIDs []int `json:"chapter_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || req.CourseID == 0 {
				return nil, httpx.BadRequest("请选择课程")
			}
			if len(req.ChapterIDs) == 0 {
				return nil, httpx.BadRequest("请选择至少一个章节")
			}
			return &generateContentReq{CourseID: req.CourseID, ChapterIDs: req.ChapterIDs, UserID: c.GetInt("user_id")}, nil
		},
		Invoke: func(ctx context.Context, req *generateContentReq) (*service.GenerateContentResultDTO, error) {
			taskID, err := h.contentGenSvc.StartGeneration(req.CourseID, req.ChapterIDs, req.UserID)
			if err != nil {
				return nil, err
			}
			return &service.GenerateContentResultDTO{TaskID: taskID}, nil
		},
	}.WithSuccess(httpx.Created("生成任务已启动"), http.StatusBadRequest).Handle(c)
}

// @Summary 查询内容生成任务状态
// @Description 前端轮询生成进度（pending/processing/completed/failed + 逐章节结果）
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.R{data=service.GenTaskStatus} "success"
// @Failure 400 {object} response.R "task_id 无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "生成任务不存在"
// @Router /admin/course/generate-content/{task_id} [get]
// GetGenerationTask 查询生成任务状态（前端轮询）GET /api/admin/course/generate-content/:task_id
func (h *adminHandler) GetGenerationTask(c *gin.Context) {
	httpx.Endpoint[taskIDParam, service.GenTaskStatus]{
		Parse: func(c *gin.Context) (*taskIDParam, error) {
			return &taskIDParam{TaskID: c.Param("task_id")}, nil
		},
		Invoke: func(ctx context.Context, req *taskIDParam) (*service.GenTaskStatus, error) {
			return h.contentGenSvc.GetTaskStatus(req.TaskID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(service.ErrGenTaskNotFound, http.StatusNotFound).
		WithSentinel(service.ErrGenTaskIDInvalid, http.StatusBadRequest).Handle(c)
}

// @Summary HRWAI 用户列表
// @Description 管理员分页查询 hrwai_users（账号/昵称/手机号模糊搜索）
// @Tags 管理端-用户
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（账号/昵称/手机号）"
// @Success 200 {object} response.R{data=admin.HrwaiUserPageResult} "success"
// @Failure 400 {object} response.R "查询用户列表失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users [get]
// ListHrwaiUsers HRWAI 用户列表 GET /api/admin/hrwai-users
func (h *adminHandler) ListHrwaiUsers(c *gin.Context) {
	httpx.Endpoint[hrwaiUserListReq, HrwaiUserPageResult]{
		Parse: func(c *gin.Context) (*hrwaiUserListReq, error) {
			return &hrwaiUserListReq{
				Page:     httpx.QueryIntDefault(c, "page", 1),
				PageSize: httpx.QueryIntDefault(c, "page_size", 20),
				Keyword:  c.Query("keyword"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *hrwaiUserListReq) (*HrwaiUserPageResult, error) {
			return h.adminSvc.ListHrwaiUsers(req.Page, req.PageSize, req.Keyword)
		},
		ErrStatus: httpx.ErrStatusAllMsg(http.StatusBadRequest, "查询用户列表失败"),
	}.Handle(c)
}

// @Summary 新增 HRWAI 用户
// @Description 管理员创建 hrwai_users 账号（account / username 缺省时后端生成）
// @Tags 管理端-用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "创建请求 {phone,password,account,username,email,company}"
// @Success 201 {object} response.R{data=admin.HrwaiUserCreatedDTO} "用户添加成功"
// @Failure 400 {object} response.R "参数错误/手机号已注册"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users [post]
// CreateHrwaiUser 新增 HRWAI 用户 POST /api/admin/hrwai-users
func (h *adminHandler) CreateHrwaiUser(c *gin.Context) {
	httpx.Endpoint[createHrwaiUserReq, HrwaiUserCreatedDTO]{
		Parse: func(c *gin.Context) (*createHrwaiUserReq, error) {
			return httpx.BindJSON[createHrwaiUserReq](c)
		},
		Invoke: func(ctx context.Context, req *createHrwaiUserReq) (*HrwaiUserCreatedDTO, error) {
			u, err := h.adminSvc.CreateHrwaiUser(req.Phone, req.Password, req.Account, req.Username, req.Email, req.Company)
			if err != nil {
				return nil, err
			}
			dto := NewHrwaiUserCreatedDTO(u)
			return &dto, nil
		},
	}.WithSuccess(httpx.Created("用户添加成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 更新 HRWAI 用户资料
// @Description 管理员更新用户昵称/邮箱/单位/状态（不含密码），响应 data 为 null
// @Tags 管理端-用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param body body object false "更新请求 {username,email,company,status}"
// @Success 200 {object} response.R "用户资料已更新"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users/{id} [put]
// UpdateHrwaiUser 更新 HRWAI 用户资料(不含密码) PUT /api/admin/hrwai-users/:id
func (h *adminHandler) UpdateHrwaiUser(c *gin.Context) {
	httpx.Endpoint[updateHrwaiUserReq, struct{}]{
		Parse: func(c *gin.Context) (*updateHrwaiUserReq, error) {
			id, err := httpx.PathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Username string `json:"username"`
				Email    string `json:"email"`
				Company  string `json:"company"`
				Status   int16  `json:"status"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			if body.Status != 0 && body.Status != 1 {
				return nil, httpx.BadRequest("状态值非法(仅支持 0/1)")
			}
			return &updateHrwaiUserReq{ID: id, Username: body.Username, Email: body.Email, Company: body.Company, Status: body.Status}, nil
		},
		Invoke: func(ctx context.Context, req *updateHrwaiUserReq) (*struct{}, error) {
			if err := h.adminSvc.UpdateHrwaiUser(req.ID, req.Username, req.Email, req.Company, req.Status); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("用户资料已更新"), http.StatusBadRequest).Handle(c)
}

// @Summary 重置 HRWAI 用户密码
// @Description 管理员重置用户口令，响应 data 为 null（不回显口令）
// @Tags 管理端-用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param body body object false "重置请求 {password}"
// @Success 200 {object} response.R "密码已重置"
// @Failure 400 {object} response.R "参数错误/密码长度非法"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "用户不存在"
// @Router /admin/hrwai-users/{id}/password [put]
// ResetHrwaiUserPassword 重置 HRWAI 用户密码 PUT /api/admin/hrwai-users/:id/password
func (h *adminHandler) ResetHrwaiUserPassword(c *gin.Context) {
	httpx.Endpoint[resetPasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*resetPasswordReq, error) {
			id, err := httpx.PathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			if len(body.Password) < 6 || len(body.Password) > 20 {
				return nil, httpx.BadRequest("密码长度需为 6-20 个字符")
			}
			return &resetPasswordReq{ID: id, Password: body.Password}, nil
		},
		Invoke: func(ctx context.Context, req *resetPasswordReq) (*struct{}, error) {
			if err := h.adminSvc.ResetHrwaiUserPassword(ctx, req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("密码已重置"), http.StatusInternalServerError).
		WithSentinel(model.ErrHrwaiUserNotFound, http.StatusNotFound).
		WithSentinel(ErrInvalidHrwaiUserID, http.StatusBadRequest).Handle(c)
}

// @Summary 切换 HRWAI 用户启用/禁用状态
// @Description 管理员切换用户状态，返回切换后的新状态
// @Tags 管理端-用户
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} response.R{data=admin.StatusResultDTO} "用户已启用/已禁用"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "用户不存在"
// @Router /admin/hrwai-users/{id}/status [put]
// ToggleHrwaiUserStatus 切换 HRWAI 用户启用/禁用状态 PUT /api/admin/hrwai-users/:id/status
func (h *adminHandler) ToggleHrwaiUserStatus(c *gin.Context) {
	httpx.Endpoint[idParam, StatusResultDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*StatusResultDTO, error) {
			next, err := h.adminSvc.ToggleHrwaiUserStatus(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			return &StatusResultDTO{Status: int(next)}, nil
		},
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: model.ErrHrwaiUserNotFound, Status: http.StatusNotFound},
			{Sentinel: ErrInvalidHrwaiUserID, Status: http.StatusBadRequest},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
		Render: func(c *gin.Context, _ *idParam, resp *StatusResultDTO) {
			msg := "用户已启用"
			if resp.Status == 0 {
				msg = "用户已禁用"
			}
			response.SuccessWithMsg(c, msg, resp)
		},
	}.Handle(c)
}

// @Summary 删除 HRWAI 用户
// @Description 管理员删除用户，响应 data 为 null
// @Tags 管理端-用户
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} response.R "用户删除成功"
// @Failure 400 {object} response.R "删除失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users/{id} [delete]
// DeleteHrwaiUser 删除 HRWAI 用户 DELETE /api/admin/hrwai-users/:id
func (h *adminHandler) DeleteHrwaiUser(c *gin.Context) {
	httpx.Endpoint[idParam, struct{}]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*struct{}, error) {
			if err := h.adminSvc.DeleteHrwaiUser(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("用户删除成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 导师列表
// @Description 管理员分页查询导师（用户名/姓名模糊搜索）
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键字（用户名/姓名）"
// @Success 200 {object} response.R{data=admin.TutorListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/tutors [get]
// ListTutors 导师列表 GET /api/admin/tutors
func (h *adminHandler) ListTutors(c *gin.Context) {
	httpx.Endpoint[tutorListReq, TutorListDTO]{
		Parse: func(c *gin.Context) (*tutorListReq, error) {
			return &tutorListReq{
				Page:     httpx.QueryIntDefault(c, "page", 1),
				PageSize: httpx.QueryIntDefault(c, "page_size", 10),
				Keyword:  c.Query("keyword"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *tutorListReq) (*TutorListDTO, error) {
			return h.adminSvc.GetTutors(req.Page, req.PageSize, req.Keyword)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 添加导师
// @Description 管理员为导师建号（用户名/密码/姓名必填）
// @Tags 管理端-导师
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "建号请求 {username,password,name}"
// @Success 201 {object} response.R{data=auth.TutorRegisterResultDTO} "讲师添加成功"
// @Failure 400 {object} response.R "参数错误/用户名已被注册"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/tutor [post]
// CreateTutor 添加导师 POST /api/admin/tutor
func (h *adminHandler) CreateTutor(c *gin.Context) {
	httpx.Endpoint[createTutorReq, auth.TutorRegisterResultDTO]{
		Parse: func(c *gin.Context) (*createTutorReq, error) {
			req, err := httpx.BindJSON[createTutorReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" || req.Name == "" {
				return nil, httpx.BadRequest("用户名、密码和姓名不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *createTutorReq) (*auth.TutorRegisterResultDTO, error) {
			return h.authSvc.TutorRegister(req.Username, req.Password, req.Name)
		},
	}.WithSuccess(httpx.Created("讲师添加成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 删除导师
// @Description 管理员删除导师账号，返回被删除的 tutor_id
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "导师 ID"
// @Success 200 {object} response.R{data=admin.TutorDeletedDTO} "讲师删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id} [delete]
// DeleteTutor 删除导师 DELETE /api/admin/tutor/:tutor_id
func (h *adminHandler) DeleteTutor(c *gin.Context) {
	httpx.Endpoint[idParam, TutorDeletedDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*TutorDeletedDTO, error) {
			return h.adminSvc.DeleteTutor(req.ID)
		},
	}.WithSuccess(httpx.OkMsg("讲师删除成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrTutorNotFound, http.StatusNotFound).
		WithSentinel(ErrInvalidTutorID, http.StatusBadRequest).Handle(c)
}

// @Summary 重置导师密码
// @Description 管理员重置导师口令，响应 data 为 null
// @Tags 管理端-导师
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "导师 ID"
// @Param body body object false "重置请求 {password}"
// @Success 200 {object} response.R "密码已重置"
// @Failure 400 {object} response.R "参数错误/密码长度非法"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/password [put]
// ResetTutorPassword 重置导师密码 PUT /api/admin/tutor/:tutor_id/password
func (h *adminHandler) ResetTutorPassword(c *gin.Context) {
	httpx.Endpoint[resetPasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*resetPasswordReq, error) {
			id, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			if len(body.Password) < 6 || len(body.Password) > 20 {
				return nil, httpx.BadRequest("密码长度需为 6-20 个字符")
			}
			return &resetPasswordReq{ID: id, Password: body.Password}, nil
		},
		Invoke: func(ctx context.Context, req *resetPasswordReq) (*struct{}, error) {
			if err := h.adminSvc.ResetTutorPassword(ctx, req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("密码已重置"), http.StatusInternalServerError).
		WithSentinel(model.ErrTutorNotFound, http.StatusNotFound).
		WithSentinel(ErrInvalidTutorID, http.StatusBadRequest).Handle(c)
}

// @Summary 切换导师启用/禁用状态
// @Description 管理员切换导师状态，返回切换后的新状态
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "导师 ID"
// @Success 200 {object} response.R{data=admin.StatusResultDTO} "讲师已启用/已禁用"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/status [put]
// ToggleTutorStatus 切换导师启用/禁用状态 PUT /api/admin/tutor/:tutor_id/status
func (h *adminHandler) ToggleTutorStatus(c *gin.Context) {
	httpx.Endpoint[idParam, StatusResultDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*StatusResultDTO, error) {
			next, err := h.adminSvc.ToggleTutorStatus(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			return &StatusResultDTO{Status: next}, nil
		},
		// 与 ToggleHrwaiUserStatus 同一判定：admin_service 的「讲师不存在」是裸 errors.New，
		// 无哨兵可名 ⇒ 本批不动（参数错误那半边已随票8 归位 400）。
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: model.ErrTutorNotFound, Status: http.StatusNotFound},
			{Sentinel: ErrInvalidTutorID, Status: http.StatusBadRequest},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
		Render: func(c *gin.Context, _ *idParam, resp *StatusResultDTO) {
			msg := "讲师已启用"
			if resp.Status == 0 {
				msg = "讲师已禁用"
			}
			response.SuccessWithMsg(c, msg, resp)
		},
	}.Handle(c)
}

// @Summary 统计看板
// @Description 管理员查看学员/课程/学习时长概览与课程统计
// @Tags 管理端-统计
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=admin.AdminStatisticsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/statistics [get]
// GetStatistics 统计看板 GET /api/admin/statistics
func (h *adminHandler) GetStatistics(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminStatisticsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminStatisticsDTO, error) {
			return h.adminSvc.GetStatistics(), nil
		},
	}.Handle(c)
}

// ===== Endpoint 请求类型（吸收原 handler 内联 struct / 路径参数） =====

// idParam 路径整型 ID 请求（course_id / chapter_id / id / tutor_id / file_id 等）。
type idParam struct {
	ID int
}

// taskIDParam 字符串任务 ID 请求（生成任务轮询走字符串 task_id）。
type taskIDParam struct {
	TaskID string
}

// generateContentReq 异步生成课程内容请求。
type generateContentReq struct {
	CourseID   int
	ChapterIDs []int
	UserID     int
}

// createHrwaiUserReq 新增 HRWAI 用户请求体。
type createHrwaiUserReq struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Account  string `json:"account"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Company  string `json:"company"`
}

// updateHrwaiUserReq 更新 HRWAI 用户资料（路径 id + body）。
type updateHrwaiUserReq struct {
	ID       int
	Username string
	Email    string
	Company  string
	Status   int16
}

// resetPasswordReq 重置密码请求（路径 id/tutor_id + body password）。
type resetPasswordReq struct {
	ID       int
	Password string
}

// createTutorReq 添加导师请求体。
type createTutorReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// hrwaiUserListReq HRWAI 用户列表查询参数。
type hrwaiUserListReq struct {
	Page     int
	PageSize int
	Keyword  string
}

// tutorListReq 导师列表查询参数。
type tutorListReq struct {
	Page     int
	PageSize int
	Keyword  string
}
