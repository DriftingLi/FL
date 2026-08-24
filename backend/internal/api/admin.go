// Package api 实现 HTTP handlers。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
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
func RegisterAdminRoutes(rg *gin.RouterGroup, rd RouterDeps, adminSvc *service.AdminService, courseSvc *service.AdminCourseService, authSvc *service.AuthService, aiConfigSvc *service.AIConfigService, contentGenSvc *service.ContentGenerateService) {
	h := NewAdminHandler(adminSvc, courseSvc, authSvc, aiConfigSvc, contentGenSvc)

	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))

	// ===== AI 配置（多配置管理 + 功能绑定）=====
	NewAIConfigHandler(aiConfigSvc).registerAIConfigRoutes(g)

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
	Endpoint[adminCourseListReq, service.CoursePageResult]{
		Parse: func(c *gin.Context) (*adminCourseListReq, error) {
			return &adminCourseListReq{
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 10),
				Keyword:      c.Query("keyword"),
				CredentialID: queryIDPtr(c, "credential_id"),
				SpecialtyID:  queryIDPtr(c, "specialty_id"),
				LevelID:      queryIDPtr(c, "level_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *adminCourseListReq) (*service.CoursePageResult, error) {
			result := h.courseSvc.GetCourses(req.Page, req.PageSize, req.Keyword, req.CredentialID, req.SpecialtyID, req.LevelID)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *adminCourseListReq, resp *service.CoursePageResult, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// CreateCourse 创建课程 POST /api/admin/course
func (h *AdminHandler) CreateCourse(c *gin.Context) {
	Endpoint[service.CourseInput, service.CourseDTO]{
		Parse: func(c *gin.Context) (*service.CourseInput, error) {
			return bindJSONMsg[service.CourseInput](c, "请求数据无效")
		},
		Invoke: func(ctx context.Context, req *service.CourseInput) (*service.CourseDTO, error) {
			return h.courseSvc.CreateCourse(req)
		},
		Render: func(c *gin.Context, _ *service.CourseInput, resp *service.CourseDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "课程创建成功", resp)
		},
	}.Handle(c)
}

// GetCourseDetail 课程详情 GET /api/admin/course/:course_id
func (h *AdminHandler) GetCourseDetail(c *gin.Context) {
	Endpoint[idParam, service.AdminCourseDetailDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.AdminCourseDetailDTO, error) {
			return h.courseSvc.GetCourseDetail(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.AdminCourseDetailDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UpdateCourse 更新课程 PUT /api/admin/course/:course_id
func (h *AdminHandler) UpdateCourse(c *gin.Context) {
	Endpoint[courseIDInput, service.CourseDTO]{
		Parse: func(c *gin.Context) (*courseIDInput, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			data, err := bindJSONMsg[service.CourseInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &courseIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *courseIDInput) (*service.CourseDTO, error) {
			return h.courseSvc.UpdateCourse(req.ID, req.Input)
		},
		Render: func(c *gin.Context, _ *courseIDInput, resp *service.CourseDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "课程更新成功", resp)
		},
	}.Handle(c)
}

// SwapCourseSort 交换课程排序 PUT /api/admin/course/:course_id/sort（同一方向+等级组内，body: {"swap_with": <id>}）
func (h *AdminHandler) SwapCourseSort(c *gin.Context) {
	Endpoint[swapCourseSortReq, struct{}]{
		Parse: func(c *gin.Context) (*swapCourseSortReq, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
				return nil, badRequest("swap_with 参数无效")
			}
			return &swapCourseSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: func(ctx context.Context, req *swapCourseSortReq) (*struct{}, error) {
			if err := h.courseSvc.SwapCourseSort(req.ID, req.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *swapCourseSortReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "排序已交换", nil)
		},
	}.Handle(c)
}

// DeleteCourse 删除课程 DELETE /api/admin/course/:course_id
func (h *AdminHandler) DeleteCourse(c *gin.Context) {
	Endpoint[idParam, service.DeleteCourseResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.DeleteCourseResult, error) {
			return h.courseSvc.DeleteCourse(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.DeleteCourseResult, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "课程删除成功", resp)
		},
	}.Handle(c)
}

// CreateChapter 创建章节 POST /api/admin/course/:course_id/chapter
func (h *AdminHandler) CreateChapter(c *gin.Context) {
	Endpoint[chapterIDInput, service.ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := pathInt(c, "course_id", "课程ID无效")
			if err != nil {
				return nil, err
			}
			data, err := bindJSONMsg[service.ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*service.ChapterDTO, error) {
			return h.courseSvc.CreateChapter(req.ID, req.Input)
		},
		Render: func(c *gin.Context, _ *chapterIDInput, resp *service.ChapterDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "章节创建成功", resp)
		},
	}.Handle(c)
}

// UpdateChapter 更新章节 PUT /api/admin/chapter/:chapter_id
func (h *AdminHandler) UpdateChapter(c *gin.Context) {
	Endpoint[chapterIDInput, service.ChapterDTO]{
		Parse: func(c *gin.Context) (*chapterIDInput, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			data, err := bindJSONMsg[service.ChapterInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &chapterIDInput{ID: id, Input: data}, nil
		},
		Invoke: func(ctx context.Context, req *chapterIDInput) (*service.ChapterDTO, error) {
			return h.courseSvc.UpdateChapter(req.ID, req.Input)
		},
		Render: func(c *gin.Context, _ *chapterIDInput, resp *service.ChapterDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "章节更新成功", resp)
		},
	}.Handle(c)
}

// DeleteChapter 删除章节 DELETE /api/admin/chapter/:chapter_id
func (h *AdminHandler) DeleteChapter(c *gin.Context) {
	Endpoint[idParam, service.DeleteChapterResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "chapter_id", "章节ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.DeleteChapterResult, error) {
			return h.courseSvc.DeleteChapter(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.DeleteChapterResult, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "章节删除成功", resp)
		},
	}.Handle(c)
}

// GenerateContent 异步生成课程内容 POST /api/admin/course/generate-content
func (h *AdminHandler) GenerateContent(c *gin.Context) {
	Endpoint[generateContentReq, string]{
		Parse: func(c *gin.Context) (*generateContentReq, error) {
			var req struct {
				CourseID   int   `json:"course_id"`
				ChapterIDs []int `json:"chapter_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || req.CourseID == 0 {
				return nil, badRequest("请选择课程")
			}
			if len(req.ChapterIDs) == 0 {
				return nil, badRequest("请选择至少一个章节")
			}
			return &generateContentReq{CourseID: req.CourseID, ChapterIDs: req.ChapterIDs, UserID: c.GetInt("user_id")}, nil
		},
		Invoke: func(ctx context.Context, req *generateContentReq) (*string, error) {
			taskID, err := h.contentGenSvc.StartGeneration(req.CourseID, req.ChapterIDs, req.UserID)
			if err != nil {
				return nil, err
			}
			return &taskID, nil
		},
		Render: func(c *gin.Context, _ *generateContentReq, resp *string, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "生成任务已启动", map[string]any{"task_id": *resp})
		},
	}.Handle(c)
}

// GetGenerationTask 查询生成任务状态（前端轮询）GET /api/admin/course/generate-content/:task_id
func (h *AdminHandler) GetGenerationTask(c *gin.Context) {
	Endpoint[taskIDParam, service.GenTaskStatus]{
		Parse: func(c *gin.Context) (*taskIDParam, error) {
			return &taskIDParam{TaskID: c.Param("task_id")}, nil
		},
		Invoke: func(ctx context.Context, req *taskIDParam) (*service.GenTaskStatus, error) {
			return h.contentGenSvc.GetTaskStatus(req.TaskID)
		},
		Render: func(c *gin.Context, _ *taskIDParam, resp *service.GenTaskStatus, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListHrwaiUsers HRWAI 用户列表 GET /api/admin/hrwai-users
func (h *AdminHandler) ListHrwaiUsers(c *gin.Context) {
	Endpoint[hrwaiUserListReq, service.HrwaiUserPageResult]{
		Parse: func(c *gin.Context) (*hrwaiUserListReq, error) {
			return &hrwaiUserListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
				Keyword:  c.Query("keyword"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *hrwaiUserListReq) (*service.HrwaiUserPageResult, error) {
			return h.adminSvc.ListHrwaiUsers(req.Page, req.PageSize, req.Keyword)
		},
		Render: func(c *gin.Context, _ *hrwaiUserListReq, resp *service.HrwaiUserPageResult, err error) {
			if err != nil {
				response.BadRequest(c, "查询用户列表失败")
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// CreateHrwaiUser 新增 HRWAI 用户 POST /api/admin/hrwai-users
func (h *AdminHandler) CreateHrwaiUser(c *gin.Context) {
	Endpoint[createHrwaiUserReq, model.HrwaiUser]{
		Parse: func(c *gin.Context) (*createHrwaiUserReq, error) {
			return bindJSON[createHrwaiUserReq](c)
		},
		Invoke: func(ctx context.Context, req *createHrwaiUserReq) (*model.HrwaiUser, error) {
			return h.adminSvc.CreateHrwaiUser(req.Phone, req.Password, req.Account, req.Username, req.Email, req.Company)
		},
		Render: func(c *gin.Context, _ *createHrwaiUserReq, resp *model.HrwaiUser, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "用户添加成功", map[string]any{
				"id":       resp.ID,
				"uid":      service.FormatUID(resp.UID),
				"account":  resp.Account,
				"username": resp.Username,
				"phone":    resp.Phone,
			})
		},
	}.Handle(c)
}

// UpdateHrwaiUser 更新 HRWAI 用户资料(不含密码) PUT /api/admin/hrwai-users/:id
func (h *AdminHandler) UpdateHrwaiUser(c *gin.Context) {
	Endpoint[updateHrwaiUserReq, struct{}]{
		Parse: func(c *gin.Context) (*updateHrwaiUserReq, error) {
			id, err := pathInt(c, "id", "用户ID无效")
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
				return nil, badRequest("请求参数错误")
			}
			if body.Status != 0 && body.Status != 1 {
				return nil, badRequest("状态值非法(仅支持 0/1)")
			}
			return &updateHrwaiUserReq{ID: id, Username: body.Username, Email: body.Email, Company: body.Company, Status: body.Status}, nil
		},
		Invoke: func(ctx context.Context, req *updateHrwaiUserReq) (*struct{}, error) {
			if err := h.adminSvc.UpdateHrwaiUser(req.ID, req.Username, req.Email, req.Company, req.Status); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *updateHrwaiUserReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "用户资料已更新", nil)
		},
	}.Handle(c)
}

// ResetHrwaiUserPassword 重置 HRWAI 用户密码 PUT /api/admin/hrwai-users/:id/password
func (h *AdminHandler) ResetHrwaiUserPassword(c *gin.Context) {
	Endpoint[resetPasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*resetPasswordReq, error) {
			id, err := pathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			if len(body.Password) < 6 || len(body.Password) > 20 {
				return nil, badRequest("密码长度需为 6-20 个字符")
			}
			return &resetPasswordReq{ID: id, Password: body.Password}, nil
		},
		Invoke: func(ctx context.Context, req *resetPasswordReq) (*struct{}, error) {
			if err := h.adminSvc.ResetHrwaiUserPassword(req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *resetPasswordReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "密码已重置", nil)
		},
	}.Handle(c)
}

// ToggleHrwaiUserStatus 切换 HRWAI 用户启用/禁用状态 PUT /api/admin/hrwai-users/:id/status
func (h *AdminHandler) ToggleHrwaiUserStatus(c *gin.Context) {
	Endpoint[idParam, int16]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*int16, error) {
			next, err := h.adminSvc.ToggleHrwaiUserStatus(req.ID)
			if err != nil {
				return nil, err
			}
			return &next, nil
		},
		Render: func(c *gin.Context, _ *idParam, resp *int16, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			msg := "用户已启用"
			if *resp == 0 {
				msg = "用户已禁用"
			}
			response.SuccessWithMsg(c, msg, map[string]any{"status": *resp})
		},
	}.Handle(c)
}

// DeleteHrwaiUser 删除 HRWAI 用户 DELETE /api/admin/hrwai-users/:id
func (h *AdminHandler) DeleteHrwaiUser(c *gin.Context) {
	Endpoint[idParam, struct{}]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "用户ID无效")
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
		Render: func(c *gin.Context, _ *idParam, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "用户删除成功", nil)
		},
	}.Handle(c)
}

// ListTutors 导师列表 GET /api/admin/tutors
func (h *AdminHandler) ListTutors(c *gin.Context) {
	Endpoint[tutorListReq, service.TutorListDTO]{
		Parse: func(c *gin.Context) (*tutorListReq, error) {
			return &tutorListReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 10),
				Keyword:  c.Query("keyword"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *tutorListReq) (*service.TutorListDTO, error) {
			return h.adminSvc.GetTutors(req.Page, req.PageSize, req.Keyword), nil
		},
		Render: func(c *gin.Context, _ *tutorListReq, resp *service.TutorListDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// CreateTutor 添加导师 POST /api/admin/tutor
func (h *AdminHandler) CreateTutor(c *gin.Context) {
	Endpoint[createTutorReq, map[string]any]{
		Parse: func(c *gin.Context) (*createTutorReq, error) {
			req, err := bindJSON[createTutorReq](c)
			if err != nil {
				return nil, err
			}
			if req.Username == "" || req.Password == "" || req.Name == "" {
				return nil, badRequest("用户名、密码和姓名不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *createTutorReq) (*map[string]any, error) {
			result, err := h.authSvc.TutorRegister(req.Username, req.Password, req.Name)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *createTutorReq, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "导师添加成功", *resp)
		},
	}.Handle(c)
}

// DeleteTutor 删除导师 DELETE /api/admin/tutor/:tutor_id
func (h *AdminHandler) DeleteTutor(c *gin.Context) {
	Endpoint[idParam, service.TutorDeletedDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "tutor_id", "导师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.TutorDeletedDTO, error) {
			return h.adminSvc.DeleteTutor(req.ID)
		},
		Render: func(c *gin.Context, _ *idParam, resp *service.TutorDeletedDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "导师删除成功", resp)
		},
	}.Handle(c)
}

// ResetTutorPassword 重置导师密码 PUT /api/admin/tutor/:tutor_id/password
func (h *AdminHandler) ResetTutorPassword(c *gin.Context) {
	Endpoint[resetPasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*resetPasswordReq, error) {
			id, err := pathInt(c, "tutor_id", "导师ID无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求参数错误")
			}
			if len(body.Password) < 6 || len(body.Password) > 20 {
				return nil, badRequest("密码长度需为 6-20 个字符")
			}
			return &resetPasswordReq{ID: id, Password: body.Password}, nil
		},
		Invoke: func(ctx context.Context, req *resetPasswordReq) (*struct{}, error) {
			if err := h.adminSvc.ResetTutorPassword(req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *resetPasswordReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "密码已重置", nil)
		},
	}.Handle(c)
}

// ToggleTutorStatus 切换导师启用/禁用状态 PUT /api/admin/tutor/:tutor_id/status
func (h *AdminHandler) ToggleTutorStatus(c *gin.Context) {
	Endpoint[idParam, int]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "tutor_id", "导师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*int, error) {
			next, err := h.adminSvc.ToggleTutorStatus(req.ID)
			if err != nil {
				return nil, err
			}
			return &next, nil
		},
		Render: func(c *gin.Context, _ *idParam, resp *int, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			msg := "导师已启用"
			if *resp == 0 {
				msg = "导师已禁用"
			}
			response.SuccessWithMsg(c, msg, map[string]any{"status": *resp})
		},
	}.Handle(c)
}

// GetStatistics 统计看板 GET /api/admin/statistics
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	Endpoint[struct{}, service.AdminStatisticsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminStatisticsDTO, error) {
			return h.adminSvc.GetStatistics(), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.AdminStatisticsDTO, _ error) {
			response.Success(c, resp)
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

// courseIDInput 路径课程 ID + CourseInput 请求体（创建/更新课程）。
type courseIDInput struct {
	ID    int
	Input *service.CourseInput
}

// chapterIDInput 路径章节 ID + ChapterInput 请求体（创建/更新章节）。
type chapterIDInput struct {
	ID    int
	Input *service.ChapterInput
}

// swapCourseSortReq 交换课程排序请求（路径 course_id + body swap_with）。
type swapCourseSortReq struct {
	ID       int
	SwapWith int
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

// adminCourseListReq 管理端课程列表查询参数。
type adminCourseListReq struct {
	Page         int
	PageSize     int
	Keyword      string
	CredentialID *int
	SpecialtyID  *int
	LevelID      *int
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
