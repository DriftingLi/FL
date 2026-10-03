// Package api 实现 HTTP handlers。
// 本文件：招聘域投递端点（spec #449 T3 #452）。
//   - 学员侧 /api/jobs/:id/apply：投递即授权
//   - 学员侧 /api/resume/applications*：我的投递（列表/撤回），对齐既有「学员侧招聘数据挂在简历前缀下」
package job

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// ApplicationErrStatus 投递域哨兵→状态码表（#611）：职位不可投/不存在 → 404，非本人 → 403，
// 其余（重复投递/冷却/日限/简历不完整等业务校验）兜底 400。
var ApplicationErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrApplyJobInactive, Status: http.StatusNotFound},
		{Sentinel: ErrJobNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrApplyNotYours, Status: http.StatusForbidden},
	},
	Fallback: http.StatusBadRequest,
}

// RegisterApplicationRoutes 注册投递相关路由。
func RegisterApplicationRoutes(rg *gin.RouterGroup, session *security.Session, svc *ApplicationService) {
	h := newApplicationHandler(svc)
	// 学员侧职位投递动作挂在 /api/jobs 下
	studentJobG := rg.Group("/jobs", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapJobApply))
	studentJobG.POST("/:id/apply", h.Apply)
	// 我的投递挂在 /api/resume 前缀下（对齐既有「学员侧招聘数据挂在简历前缀下」的写法）
	studentG := rg.Group("/resume", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapResumeManage))
	studentG.GET("/applications", h.ListMine)
	studentG.POST("/applications/:id/withdraw", h.Withdraw)
}

// ApplicationHandler 投递 handler。
type applicationHandler struct {
	svc *ApplicationService
}

// NewApplicationHandler 创建投递 handler。
func newApplicationHandler(svc *ApplicationService) *applicationHandler {
	return &applicationHandler{svc: svc}
}

// Apply 学员投递职位 POST /api/jobs/:id/apply
// @Summary 投递职位（投递即授权）
// @Description 投递即授权：同事务写投递记录 + 写/复活 approved 联系方式授权（source=application），企业当场可取得明文联系方式。hidden 简历可投递；缺真实姓名/电话 400；applied 唯一；30 天冷却；日限 10。
// @Tags 招聘域-投递
// @Produce json
// @Security BearerAuth
// @Param id path int true "职位 ID"
// @Success 201 {object} response.R{data=job.ApplicationDTO} "投递成功"
// @Failure 400 {object} response.R "重复投递/冷却/日限/简历不完整"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "职位不可投递"
// @Router /jobs/{id}/apply [post]
func (h *applicationHandler) Apply(c *gin.Context) {
	httpx.Endpoint[struct{}, ApplicationDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ApplicationDTO, error) {
			id, err := httpx.PathInt(c, "id", "职位 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.Apply(middleware.CurrentUserID(c), id)
		},
		ErrStatus: ApplicationErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *ApplicationDTO) {
			response.Created(c, "投递成功，企业已可查看你的联系方式", *resp)
		},
	}.Handle(c)
}

// ListMine 我的投递列表 GET /api/resume/applications
// @Summary 我的投递
// @Description 学员查看自己的投递（applied/rejected/withdrawn 三态 + 企业是否已查看）
// @Tags 招聘域-投递
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R{data=job.ApplicationListResult} "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/applications [get]
func (h *applicationHandler) ListMine(c *gin.Context) {
	httpx.Endpoint[struct{}, ApplicationListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ApplicationListResult, error) {
			page := httpx.QueryIntDefault(c, "page", 1)
			pageSize := httpx.QueryIntDefault(c, "page_size", 20)
			items, total, err := h.svc.ListForStudent(middleware.CurrentUserID(c), page, pageSize)
			if err != nil {
				return nil, err
			}
			return &ApplicationListResult{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
		},
	}.Handle(c)
}

// Withdraw 撤回投递 POST /api/resume/applications/:id/withdraw
// @Summary 撤回投递
// @Description 撤回投递；revoke_contact 默认 false 不连带收回联系方式授权，true 则授权置 revoked（明文端点 403）。撤回后可立即重新投递。
// @Tags 招聘域-投递
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投递 ID"
// @Param body body object false "撤回选项 {revoke_contact?: boolean}"
// @Success 200 {object} response.R{data=job.ApplicationDTO} "已撤回"
// @Failure 400 {object} response.R "状态不允许"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权操作"
// @Router /resume/applications/{id}/withdraw [post]
// body: { revoke_contact?: boolean } 默认 false——撤回投递默认不连带收回联系方式授权。
func (h *applicationHandler) Withdraw(c *gin.Context) {
	httpx.Endpoint[struct{}, ApplicationDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*ApplicationDTO, error) {
			id, err := httpx.PathInt64(c, "id", "投递 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				RevokeContact bool `json:"revoke_contact"`
			}
			_ = c.ShouldBindJSON(&body)
			return h.svc.Withdraw(middleware.CurrentUserID(c), id, body.RevokeContact)
		},
		ErrStatus: ApplicationErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *ApplicationDTO) {
			response.SuccessWithMsg(c, "投递已撤回", *resp)
		},
	}.Handle(c)
}
