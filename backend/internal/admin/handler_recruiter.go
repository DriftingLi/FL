// 本文件：管理域 HTTP 出口之二 —— 企业招聘者管理（邀约制，管理员创建，Host-only 隔离）。
// 本域两条蓝图分居两文件，故 handler 类型带 Recruiter 后缀（同包不能有两个 RegisterRoutes；装配点见 handler.go 头）。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/auth"
	"forklift-training/internal/authz"
	"forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// RegisterAdminRecruiterRoutes 注册 /api/admin/recruiters 蓝图（CapRecruiterManage，ADR-0064）。
func RegisterAdminRecruiterRoutes(rg *gin.RouterGroup, session *security.Session, authSvc *auth.Service) {
	g := rg.Group("/admin/recruiters", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapRecruiterManage))
	h := newRecruiterHandler(authSvc)

	g.POST("", h.Create)
	g.PUT("/:id/status", h.ToggleStatus)
	g.PUT("/:id", h.Edit)
	g.PUT("/:id/password", h.ResetPassword)
	g.GET("", h.List)
}

// recruiterHandler 企业招聘者管理 handler（邀约制）。
type recruiterHandler struct {
	authSvc *auth.Service
}

// newRecruiterHandler 创建 handler。
func newRecruiterHandler(authSvc *auth.Service) *recruiterHandler {
	return &recruiterHandler{authSvc: authSvc}
}

// @Summary 创建招聘者账号
// @Description 管理员邀约制创建企业招聘者（企业信息必填，响应不回显口令）
// @Tags 管理端-招聘者
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object false "创建请求 {username,password,company_name,credit_code,business_scope,contact_name,contact_phone,contact_email,wechat}"
// @Success 201 {object} response.R{data=auth.RecruiterCreatedDTO} "招聘者账号创建成功"
// @Failure 400 {object} response.R "参数校验失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/recruiters [post]
// Create 创建招聘者账号 POST /api/admin/recruiters
func (h *recruiterHandler) Create(c *gin.Context) {
	httpx.Endpoint[auth.RecruiterCreateInput, auth.RecruiterCreatedDTO]{
		Parse: func(c *gin.Context) (*auth.RecruiterCreateInput, error) {
			req, err := httpx.BindJSON[auth.RecruiterCreateInput](c)
			if err != nil {
				return nil, err
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *auth.RecruiterCreateInput) (*auth.RecruiterCreatedDTO, error) {
			rec, err := h.authSvc.CreateRecruiter(*req)
			if err != nil {
				return nil, err
			}
			dto := auth.NewRecruiterCreatedDTO(rec)
			return &dto, nil
		},
	}.WithSuccess(httpx.Created("招聘者账号创建成功"), http.StatusBadRequest).Handle(c)
}

// @Summary 切换招聘者启用/禁用状态
// @Description 管理员切换招聘者状态，返回切换后的新状态
// @Tags 管理端-招聘者
// @Produce json
// @Security BearerAuth
// @Param id path int true "招聘者 ID"
// @Success 200 {object} response.R{data=admin.StatusResultDTO} "招聘者已启用/已禁用"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "招聘者不存在"
// @Router /admin/recruiters/{id}/status [put]
// ToggleStatus 切换招聘者启用/禁用 PUT /api/admin/recruiters/:id/status
func (h *recruiterHandler) ToggleStatus(c *gin.Context) {
	httpx.Endpoint[idParam, StatusResultDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*StatusResultDTO, error) {
			next, err := h.authSvc.ToggleRecruiterStatus(ctx, req.ID)
			if err != nil {
				return nil, err
			}
			return &StatusResultDTO{Status: int(next)}, nil
		},
		// 判定不动（票8 逐端点判过，同 handler.go 的两处 Toggle）：AuthService.ToggleRecruiterStatus
		// 的「招聘者不存在」是裸 errors.New、后面的 UPDATE/回写错误原样上抛 ⇒ 无哨兵可分档。
		ErrStatus: &httpx.ErrStatusTable{Entries: []httpx.ErrStatusEntry{
			{Sentinel: core.ErrRecruiterNotFound, Status: http.StatusNotFound},
			{Sentinel: nil, Status: http.StatusInternalServerError},
		}},
		Render: func(c *gin.Context, _ *idParam, resp *StatusResultDTO) {
			msg := "招聘者已启用"
			if resp.Status == 0 {
				msg = "招聘者已禁用"
			}
			response.SuccessWithMsg(c, msg, resp)
		},
	}.Handle(c)
}

// @Summary 编辑招聘者企业信息
// @Description 管理员编辑企业信息与联系人（不改账号归属与角色；响应比创建少一个 status）
// @Tags 管理端-招聘者
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "招聘者 ID"
// @Param body body object false "编辑请求 {username,company_name,credit_code,business_scope,contact_name,contact_phone,contact_email,wechat}"
// @Success 200 {object} response.R{data=auth.RecruiterUpdatedDTO} "招聘者信息已更新"
// @Failure 400 {object} response.R "参数校验失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/recruiters/{id} [put]
// Edit 编辑招聘者企业信息 PUT /api/admin/recruiters/:id（#417）。
func (h *recruiterHandler) Edit(c *gin.Context) {
	httpx.Endpoint[idParam, auth.RecruiterUpdatedDTO]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*auth.RecruiterUpdatedDTO, error) {
			var in auth.RecruiterEditInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			rec, err := h.authSvc.EditRecruiter(req.ID, in)
			if err != nil {
				return nil, err
			}
			dto := auth.NewRecruiterUpdatedDTO(rec)
			return &dto, nil
		},
	}.WithSuccess(httpx.OkMsg("招聘者信息已更新"), http.StatusBadRequest).Handle(c)
}

// @Summary 重置招聘者密码
// @Description 管理员强制重置招聘者口令（吊销其 refresh），响应 data 为 {}（空对象，不回显口令）
// @Tags 管理端-招聘者
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "招聘者 ID"
// @Param body body object false "重置请求 {password}"
// @Success 200 {object} response.R{data=auth.RecruiterPasswordResetResult} "密码已重置"
// @Failure 400 {object} response.R "新密码不能为空/长度非法"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/recruiters/{id}/password [put]
// ResetPassword 重置招聘者密码 PUT /api/admin/recruiters/:id/password（#417）。
func (h *recruiterHandler) ResetPassword(c *gin.Context) {
	httpx.Endpoint[idParam, auth.RecruiterPasswordResetResult]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := httpx.PathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*auth.RecruiterPasswordResetResult, error) {
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.Password == "" {
				return nil, httpx.BadRequest("新密码不能为空")
			}
			if err := h.authSvc.ResetRecruiterPassword(ctx, req.ID, body.Password); err != nil {
				return nil, err
			}
			return &auth.RecruiterPasswordResetResult{}, nil
		},
	}.WithSuccess(httpx.OkMsg("密码已重置"), http.StatusBadRequest).Handle(c)
}

// @Summary 招聘者列表
// @Description 管理员分页查询招聘者（企业名/账号模糊搜索，字段白名单无凭据）
// @Tags 管理端-招聘者
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（企业名/账号）"
// @Success 200 {object} response.R{data=auth.RecruiterListResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "查询失败"
// @Router /admin/recruiters [get]
// List 招聘者列表 GET /api/admin/recruiters（#416：分页 + 关键字过滤，字段白名单无凭据）。
func (h *recruiterHandler) List(c *gin.Context) {
	page := httpx.QueryIntDefault(c, "page", 1)
	pageSize := httpx.QueryIntDefault(c, "page_size", 20)
	keyword := c.Query("keyword")
	resp, err := h.authSvc.ListRecruiters(page, pageSize, keyword)
	if err != nil {
		response.ServerErrorCause(c, "", err)
		return
	}
	response.Success(c, resp)
}
