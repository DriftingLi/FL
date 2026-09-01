// Package api 实现 HTTP handlers。
// 本文件：招聘域职位端点（spec #449 T2 #451）。
//   - 企业侧 /api/recruit/jobs*（角色守卫 recruiter）：发布/编辑/上下架/我的职位列表/详情
//   - 学员侧 /api/jobs*（角色守卫 hrwai_user）：职位广场（open 且未强制下架）/详情
//
// L1 延伸：无 token 访问职位列表/详情一律被拒（回归断言在契约测试焊死）。
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// JobHandler 职位 handler。
type JobHandler struct {
	svc *service.JobPostingService
}

// NewJobHandler 创建职位 handler。
func NewJobHandler(svc *service.JobPostingService) *JobHandler {
	return &JobHandler{svc: svc}
}

// RegisterJobRoutes 注册职位相关路由：
//   - /api/recruit/jobs*（企业招聘者）
//   - /api/jobs*（学员）
func RegisterJobRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.JobPostingService) {
	h := NewJobHandler(svc)
	// 企业侧
	recruitG := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	recruitG.POST("/jobs", h.Create)
	recruitG.PUT("/jobs/:id", h.Update)
	recruitG.POST("/jobs/:id/toggle-status", h.ToggleStatus)
	recruitG.GET("/jobs", h.ListMine)
	recruitG.GET("/jobs/:id", h.GetMine)
	// 学员侧
	studentG := rg.Group("/jobs", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.GET("", h.ListPublic)
	studentG.GET("/:id", h.GetPublic)
}

// Create 企业发布职位 POST /api/recruit/jobs
// @Summary 发布职位
// @Description 企业发布职位（职位名 + 必填岗位 + 地区/薪资/经验要求/描述）
// @Tags 招聘域-职位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.JobPostingInput true "职位信息"
// @Success 201 {object} response.R "发布成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/jobs [post]
func (h *JobHandler) Create(c *gin.Context) {
	Endpoint[service.JobPostingInput, service.JobPostingDTO]{
		Parse: func(c *gin.Context) (*service.JobPostingInput, error) {
			return bindJSON[service.JobPostingInput](c)
		},
		Invoke: func(ctx context.Context, req *service.JobPostingInput) (*service.JobPostingDTO, error) {
			return h.svc.Create(middleware.CurrentUserID(c), req)
		},
		Render: func(c *gin.Context, _ *service.JobPostingInput, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderJobError(c, err)
				return
			}
			response.Created(c, "职位发布成功", *resp)
		},
	}.Handle(c)
}

// Update 企业编辑职位 PUT /api/recruit/jobs/:id
// @Summary 编辑职位
// @Description 企业编辑自己的职位（改别人的 403）
// @Tags 招聘域-职位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Param body body service.JobPostingInput true "职位信息"
// @Success 200 {object} response.R "更新成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权操作"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/jobs/{id} [put]
func (h *JobHandler) Update(c *gin.Context) {
	Endpoint[service.JobPostingInput, service.JobPostingDTO]{
		Parse: func(c *gin.Context) (*service.JobPostingInput, error) {
			req, err := bindJSON[service.JobPostingInput](c)
			if err != nil {
				return nil, err
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *service.JobPostingInput) (*service.JobPostingDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Update(middleware.CurrentUserID(c), id, req)
		},
		Render: func(c *gin.Context, _ *service.JobPostingInput, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderJobError(c, err)
				return
			}
			response.SuccessWithMsg(c, "职位已更新", *resp)
		},
	}.Handle(c)
}

// ToggleStatus 企业上架/下架职位 POST /api/recruit/jobs/:id/toggle-status
// @Summary 上架/下架职位
// @Description 企业上架/下架自己的职位（open↔closed；被强制下架的职位不能自行重新上架）
// @Tags 招聘域-职位
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Success 200 {object} response.R "操作成功"
// @Failure 400 {object} response.R "被强制下架或超上限"
// @Failure 403 {object} response.R "无权操作"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/jobs/{id}/toggle-status [post]
func (h *JobHandler) ToggleStatus(c *gin.Context) {
	Endpoint[struct{}, service.JobPostingDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobPostingDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.ToggleStatus(middleware.CurrentUserID(c), id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderJobError(c, err)
				return
			}
			msg := "职位已下架"
			if resp.Status == "open" {
				msg = "职位已上架"
			}
			response.SuccessWithMsg(c, msg, *resp)
		},
	}.Handle(c)
}

// ListMine 我的职位列表 GET /api/recruit/jobs（含 closed/强制下架历史）
// @Summary 我的职位列表
// @Description 企业查看自己的职位（含 closed/强制下架历史，按新鲜度排序）
// @Tags 招聘域-职位
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/jobs [get]
func (h *JobHandler) ListMine(c *gin.Context) {
	Endpoint[struct{}, service.JobListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobListResult, error) {
			params := service.JobListParams{
				Page:          atoiDefault(c.Query("page"), 1),
				PageSize:      atoiDefault(c.Query("page_size"), 20),
				MineOnly:      true,
				IncludeHidden: true,
			}
			if v := queryIDPtr(c, "position_id"); v != nil {
				params.PositionID = v
			}
			return h.svc.List(middleware.CurrentUserID(c), params)
		},
	}.Handle(c)
}

// GetMine 我的职位详情 GET /api/recruit/jobs/:id（企业侧可看历史）
// @Summary 我的职位详情
// @Description 企业查看自己的职位详情（closed/强制下架历史仍可见）
// @Tags 招聘域-职位
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Success 200 {object} response.R "详情"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权操作"
// @Router /recruit/jobs/{id} [get]
func (h *JobHandler) GetMine(c *gin.Context) {
	Endpoint[struct{}, service.JobPostingDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobPostingDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Get(middleware.CurrentUserID(c), id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderJobError(c, err)
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// ListPublic 学员职位广场 GET /api/jobs（只 open 且未强制下架，按新鲜度）
// @Summary 职位广场
// @Description 学员浏览职位广场（仅 open 且未强制下架，按新鲜度排序；支持岗位/地区/薪资/经验筛选）
// @Tags 招聘域-职位
// @Produce json
// @Security BearerAuth
// @Param position_id query int false "岗位 ID"
// @Param region query string false "地区"
// @Param salary_min query int false "最低薪资"
// @Param salary_max query int false "最高薪资"
// @Param experience query string false "经验要求"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证（L1 不公开）"
// @Router /jobs [get]
func (h *JobHandler) ListPublic(c *gin.Context) {
	Endpoint[struct{}, service.JobListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobListResult, error) {
			params := service.JobListParams{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
			}
			if v := queryIDPtr(c, "position_id"); v != nil {
				params.PositionID = v
			}
			if v := queryIDPtr(c, "salary_min"); v != nil {
				params.SalaryMin = v
			}
			if v := queryIDPtr(c, "salary_max"); v != nil {
				params.SalaryMax = v
			}
			params.Region = c.Query("region")
			params.Experience = c.Query("experience")
			return h.svc.List(0, params)
		},
	}.Handle(c)
}

// GetPublic 学员职位详情 GET /api/jobs/:id
// @Summary 职位详情
// @Description 学员查看职位详情（含企业名/主营/联系人；不含电话/邮箱/信用代码）
// @Tags 招聘域-职位
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Success 200 {object} response.R "详情"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "不存在或已下架"
// @Router /jobs/{id} [get]
func (h *JobHandler) GetPublic(c *gin.Context) {
	Endpoint[struct{}, service.JobPostingDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.JobPostingDTO, error) {
			id, err := pathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Get(0, id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.JobPostingDTO, err error) {
			if err != nil {
				renderJobError(c, err)
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// renderJobError 职位域错误映射（哨兵 → 状态码，禁止文案比对）。
func renderJobError(c *gin.Context, err error) {
	switch {
	case isParseError(err):
		renderStatus(c, http.StatusBadRequest, err.Error())
	case isJobNotFound(err):
		response.NotFound(c, err.Error())
	case isJobForbidden(err):
		response.Forbidden(c, err.Error())
	default:
		response.BadRequest(c, err.Error())
	}
}

func isParseError(err error) bool {
	var pe *ParseError
	return asParseError(err, &pe)
}
func isJobNotFound(err error) bool {
	return errors.Is(err, service.ErrJobNotFound)
}
func isJobForbidden(err error) bool {
	return errors.Is(err, service.ErrJobNotYours)
}
