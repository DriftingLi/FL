// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
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
func RegisterAdminRoutes(rg *gin.RouterGroup, rd RouterDeps, adminSvc *service.AdminService, courseSvc *service.AdminCourseService, authSvc *service.AuthService, aiConfigSvc *service.AIConfigService, contentGenSvc *service.ContentGenerateService) {
	h := NewAdminHandler(adminSvc, courseSvc, authSvc, aiConfigSvc, contentGenSvc)

	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapAdminAccess))

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

// @Summary 管理端课程列表
// @Description 管理员课程列表（含草稿），支持关键字/证件/方向/等级/热门精品过滤
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键字"
// @Param credential_id query int false "目标证件 ID"
// @Param specialty_id query int false "专业方向 ID"
// @Param level_id query int false "等级 ID"
// @Param filter query string false "热门/精品筛选 hot|featured|all" default(all)
// @Success 200 {object} response.R{data=service.CoursePageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/courses [get]
// ListCourses 课程列表 GET /api/admin/courses（filter=hot|featured|all，缺省 all）
func (h *AdminHandler) ListCourses(c *gin.Context) {
	Endpoint[adminCourseListReq, service.CoursePageResult]{
		Parse: func(c *gin.Context) (*adminCourseListReq, error) {
			f := c.Query("filter")
			if f != "" && f != "hot" && f != "featured" && f != "all" {
				return nil, badRequest("filter 仅支持 hot|featured|all")
			}
			if f == "" {
				f = "all"
			}
			return &adminCourseListReq{
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 10),
				Keyword:      c.Query("keyword"),
				CredentialID: queryIDPtr(c, "credential_id"),
				SpecialtyID:  queryIDPtr(c, "specialty_id"),
				LevelID:      queryIDPtr(c, "level_id"),
				Filter:       f,
			}, nil
		},
		Invoke: func(ctx context.Context, req *adminCourseListReq) (*service.CoursePageResult, error) {
			result, err := h.courseSvc.GetCourses(req.Page, req.PageSize, req.Keyword, req.CredentialID, req.SpecialtyID, req.LevelID, req.Filter)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 创建课程
// @Description 管理员创建课程（含培训目录扩展字段）
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "课程输入 {name,description,cover_image,duration,status,...}"
// @Success 201 {object} response.R{data=service.CourseDTO} "课程创建成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/course [post]
// CreateCourse 创建课程 POST /api/admin/course
func (h *AdminHandler) CreateCourse(c *gin.Context) {
	Endpoint[service.CourseInput, service.CourseDTO]{
		Parse: func(c *gin.Context) (*service.CourseInput, error) {
			return bindJSONMsg[service.CourseInput](c, "请求数据无效")
		},
		Invoke: func(ctx context.Context, req *service.CourseInput) (*service.CourseDTO, error) {
			return h.courseSvc.CreateCourse(req)
		},
	}.WithSuccess(created("课程创建成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 管理端课程详情
// @Description 课程字段平铺 + chapters（含停用项与嵌套元数据）
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Success 200 {object} response.R{data=service.AdminCourseDetailDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /admin/course/{course_id} [get]
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
	}.WithSuccess(okMsg("success"), http.StatusNotFound).Handle(c)
}

// @Summary 更新课程
// @Description 管理员更新课程字段（未携带的指针字段保留现状）
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "课程输入 {name,description,cover_image,duration,status,...}"
// @Success 200 {object} response.R{data=service.CourseDTO} "课程更新成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /admin/course/{course_id} [put]
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
	}.WithSuccess(okMsg("课程更新成功"), http.StatusNotFound).Handle(c)
}

// @Summary 交换课程排序
// @Description 同一方向+等级组内交换 sort_order，响应 data 为 null
// @Tags 管理端-课程
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "交换请求 {swap_with}"
// @Success 200 {object} response.R "排序已交换"
// @Failure 400 {object} response.R "swap_with 参数无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/course/{course_id}/sort [put]
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
	}.WithSuccess(okMsgNoData("排序已交换"), http.StatusBadRequest).Handle(c)
}

// @Summary 删除课程
// @Description 管理员删除课程，返回被删除的 course_id
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Success 200 {object} response.R{data=service.DeleteCourseResult} "课程删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程不存在"
// @Router /admin/course/{course_id} [delete]
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
	}.WithSuccess(okMsg("课程删除成功"), http.StatusNotFound).Handle(c)
}

// @Summary 创建章节
// @Description 管理员为课程创建章节
// @Tags 管理端-章节
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param course_id path int true "课程 ID"
// @Param body body object false "章节输入 {title,content,duration,order_num,description}"
// @Success 201 {object} response.R{data=service.ChapterDTO} "章节创建成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/course/{course_id}/chapter [post]
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
	}.WithSuccess(created("章节创建成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 更新章节
// @Description 管理员更新章节字段
// @Tags 管理端-章节
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节 ID"
// @Param body body object false "章节输入 {title,content,duration,order_num,description}"
// @Success 200 {object} response.R{data=service.ChapterDTO} "章节更新成功"
// @Failure 400 {object} response.R "请求数据无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /admin/chapter/{chapter_id} [put]
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
	}.WithSuccess(okMsg("章节更新成功"), http.StatusNotFound).Handle(c)
}

// @Summary 删除章节
// @Description 管理员删除章节，返回被删除的 chapter_id
// @Tags 管理端-章节
// @Produce json
// @Security BearerAuth
// @Param chapter_id path int true "章节 ID"
// @Success 200 {object} response.R{data=service.DeleteChapterResult} "章节删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "章节不存在"
// @Router /admin/chapter/{chapter_id} [delete]
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
	}.WithSuccess(okMsg("章节删除成功"), http.StatusNotFound).Handle(c)
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
func (h *AdminHandler) GenerateContent(c *gin.Context) {
	Endpoint[generateContentReq, service.GenerateContentResultDTO]{
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
		Invoke: func(ctx context.Context, req *generateContentReq) (*service.GenerateContentResultDTO, error) {
			taskID, err := h.contentGenSvc.StartGeneration(req.CourseID, req.ChapterIDs, req.UserID)
			if err != nil {
				return nil, err
			}
			return &service.GenerateContentResultDTO{TaskID: taskID}, nil
		},
	}.WithSuccess(created("生成任务已启动"), http.StatusBadRequest).Handle(c)
}

// @Summary 查询内容生成任务状态
// @Description 前端轮询生成进度（pending/processing/completed/failed + 逐章节结果）
// @Tags 管理端-课程
// @Produce json
// @Security BearerAuth
// @Param task_id path string true "任务 ID"
// @Success 200 {object} response.R{data=service.GenTaskStatus} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "任务不存在"
// @Router /admin/course/generate-content/{task_id} [get]
// GetGenerationTask 查询生成任务状态（前端轮询）GET /api/admin/course/generate-content/:task_id
func (h *AdminHandler) GetGenerationTask(c *gin.Context) {
	Endpoint[taskIDParam, service.GenTaskStatus]{
		Parse: func(c *gin.Context) (*taskIDParam, error) {
			return &taskIDParam{TaskID: c.Param("task_id")}, nil
		},
		Invoke: func(ctx context.Context, req *taskIDParam) (*service.GenTaskStatus, error) {
			return h.contentGenSvc.GetTaskStatus(req.TaskID)
		},
	}.WithSuccess(okMsg("success"), http.StatusNotFound).Handle(c)
}

// @Summary HRWAI 用户列表
// @Description 管理员分页查询 hrwai_users（账号/昵称/手机号模糊搜索）
// @Tags 管理端-用户
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（账号/昵称/手机号）"
// @Success 200 {object} response.R{data=service.HrwaiUserPageResult} "success"
// @Failure 400 {object} response.R "查询用户列表失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users [get]
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
		ErrStatus: errStatusAllMsg(http.StatusBadRequest, "查询用户列表失败"),
	}.Handle(c)
}

// @Summary 新增 HRWAI 用户
// @Description 管理员创建 hrwai_users 账号（account / username 缺省时后端生成）
// @Tags 管理端-用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "创建请求 {phone,password,account,username,email,company}"
// @Success 201 {object} response.R{data=service.HrwaiUserCreatedDTO} "用户添加成功"
// @Failure 400 {object} response.R "参数错误/手机号已注册"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/hrwai-users [post]
// CreateHrwaiUser 新增 HRWAI 用户 POST /api/admin/hrwai-users
func (h *AdminHandler) CreateHrwaiUser(c *gin.Context) {
	Endpoint[createHrwaiUserReq, service.HrwaiUserCreatedDTO]{
		Parse: func(c *gin.Context) (*createHrwaiUserReq, error) {
			return bindJSON[createHrwaiUserReq](c)
		},
		Invoke: func(ctx context.Context, req *createHrwaiUserReq) (*service.HrwaiUserCreatedDTO, error) {
			u, err := h.adminSvc.CreateHrwaiUser(req.Phone, req.Password, req.Account, req.Username, req.Email, req.Company)
			if err != nil {
				return nil, err
			}
			dto := service.NewHrwaiUserCreatedDTO(u)
			return &dto, nil
		},
	}.WithSuccess(created("用户添加成功"), http.StatusBadRequest).Handle(c)
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
	}.WithSuccess(okMsgNoData("用户资料已更新"), http.StatusBadRequest).Handle(c)
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
// @Router /admin/hrwai-users/{id}/password [put]
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
			if err := h.adminSvc.ResetHrwaiUserPassword(ctx, req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(okMsgNoData("密码已重置"), http.StatusBadRequest).Handle(c)
}

// @Summary 切换 HRWAI 用户启用/禁用状态
// @Description 管理员切换用户状态，返回切换后的新状态
// @Tags 管理端-用户
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} response.R{data=service.StatusResultDTO} "用户已启用/已禁用"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "用户不存在"
// @Router /admin/hrwai-users/{id}/status [put]
// ToggleHrwaiUserStatus 切换 HRWAI 用户启用/禁用状态 PUT /api/admin/hrwai-users/:id/status
func (h *AdminHandler) ToggleHrwaiUserStatus(c *gin.Context) {
	Endpoint[idParam, service.StatusResultDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "用户ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.StatusResultDTO, error) {
			next, err := h.adminSvc.ToggleHrwaiUserStatus(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			return &service.StatusResultDTO{Status: int(next)}, nil
		},
		// 判定不动（票8 逐端点判过）：AdminService.ToggleHrwaiUserStatus 的「用户不存在」是裸
		// errors.New、UPDATE 失败则原样上抛驱动错误 ⇒ api 侧无具名哨兵可分档，改判会把真 404
		// 也答成 500。正解在 service 侧升哨兵（admin_service.go 本批不在改动面）。
		// 已归位的一半：路径参数非数字今天回它自己的 400（票8 翻转前被这条表吞成 404）。
		ErrStatus: errStatusAll(http.StatusNotFound),
		Render: func(c *gin.Context, _ *idParam, resp *service.StatusResultDTO) {
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
	}.WithSuccess(okMsgNoData("用户删除成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 导师列表
// @Description 管理员分页查询导师（用户名/姓名模糊搜索）
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键字（用户名/姓名）"
// @Success 200 {object} response.R{data=service.TutorListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/tutors [get]
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
			return h.adminSvc.GetTutors(req.Page, req.PageSize, req.Keyword)
		},
	}.WithSuccess(okMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 添加导师
// @Description 管理员为导师建号（用户名/密码/姓名必填）
// @Tags 管理端-导师
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "建号请求 {username,password,name}"
// @Success 201 {object} response.R{data=service.TutorRegisterResultDTO} "讲师添加成功"
// @Failure 400 {object} response.R "参数错误/用户名已被注册"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/tutor [post]
// CreateTutor 添加导师 POST /api/admin/tutor
func (h *AdminHandler) CreateTutor(c *gin.Context) {
	Endpoint[createTutorReq, service.TutorRegisterResultDTO]{
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
		Invoke: func(ctx context.Context, req *createTutorReq) (*service.TutorRegisterResultDTO, error) {
			return h.authSvc.TutorRegister(req.Username, req.Password, req.Name)
		},
	}.WithSuccess(created("讲师添加成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 删除导师
// @Description 管理员删除导师账号，返回被删除的 tutor_id
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "导师 ID"
// @Success 200 {object} response.R{data=service.TutorDeletedDTO} "讲师删除成功"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id} [delete]
// DeleteTutor 删除导师 DELETE /api/admin/tutor/:tutor_id
func (h *AdminHandler) DeleteTutor(c *gin.Context) {
	Endpoint[idParam, service.TutorDeletedDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.TutorDeletedDTO, error) {
			return h.adminSvc.DeleteTutor(req.ID)
		},
	}.WithSuccess(okMsg("讲师删除成功"), http.StatusNotFound).Handle(c)
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
// @Router /admin/tutor/{tutor_id}/password [put]
// ResetTutorPassword 重置导师密码 PUT /api/admin/tutor/:tutor_id/password
func (h *AdminHandler) ResetTutorPassword(c *gin.Context) {
	Endpoint[resetPasswordReq, struct{}]{
		Parse: func(c *gin.Context) (*resetPasswordReq, error) {
			id, err := pathInt(c, "tutor_id", "讲师ID无效")
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
			if err := h.adminSvc.ResetTutorPassword(ctx, req.ID, req.Password); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
	}.WithSuccess(okMsgNoData("密码已重置"), http.StatusNotFound).Handle(c)
}

// @Summary 切换导师启用/禁用状态
// @Description 管理员切换导师状态，返回切换后的新状态
// @Tags 管理端-导师
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "导师 ID"
// @Success 200 {object} response.R{data=service.StatusResultDTO} "讲师已启用/已禁用"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师不存在"
// @Router /admin/tutor/{tutor_id}/status [put]
// ToggleTutorStatus 切换导师启用/禁用状态 PUT /api/admin/tutor/:tutor_id/status
func (h *AdminHandler) ToggleTutorStatus(c *gin.Context) {
	Endpoint[idParam, service.StatusResultDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*service.StatusResultDTO, error) {
			next, err := h.adminSvc.ToggleTutorStatus(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			return &service.StatusResultDTO{Status: next}, nil
		},
		// 与 ToggleHrwaiUserStatus 同一判定：admin_service 的「讲师不存在」是裸 errors.New，
		// 无哨兵可名 ⇒ 本批不动（参数错误那半边已随票8 归位 400）。
		ErrStatus: errStatusAll(http.StatusNotFound),
		Render: func(c *gin.Context, _ *idParam, resp *service.StatusResultDTO) {
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
// @Success 200 {object} response.R{data=service.AdminStatisticsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/statistics [get]
// GetStatistics 统计看板 GET /api/admin/statistics
func (h *AdminHandler) GetStatistics(c *gin.Context) {
	Endpoint[struct{}, service.AdminStatisticsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.AdminStatisticsDTO, error) {
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
	Filter       string
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
