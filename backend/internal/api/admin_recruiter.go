// Package api 实现 HTTP handlers。
// 本文件：企业招聘者管理（邀约制，管理员创建，Host-only 隔离）。
package api

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterAdminRecruiterRoutes 注册 /api/admin/recruiters 蓝图（管理员邀约制创建招聘者）。
func RegisterAdminRecruiterRoutes(rg *gin.RouterGroup, rd RouterDeps, authSvc *service.AuthService) {
	g := rg.Group("/admin/recruiters", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	g.POST("", NewAdminRecruiterHandler(authSvc).Create)
	g.PUT("/:id/status", NewAdminRecruiterHandler(authSvc).ToggleStatus)
	g.PUT("/:id", NewAdminRecruiterHandler(authSvc).Edit)
	g.PUT("/:id/password", NewAdminRecruiterHandler(authSvc).ResetPassword)
	g.GET("", NewAdminRecruiterHandler(authSvc).List)
}

// AdminRecruiterHandler 企业招聘者管理 handler（邀约制）。
type AdminRecruiterHandler struct {
	authSvc *service.AuthService
}

// NewAdminRecruiterHandler 创建 handler。
func NewAdminRecruiterHandler(authSvc *service.AuthService) *AdminRecruiterHandler {
	return &AdminRecruiterHandler{authSvc: authSvc}
}

// Create 创建招聘者账号 POST /api/admin/recruiters
func (h *AdminRecruiterHandler) Create(c *gin.Context) {
	Endpoint[service.RecruiterCreateInput, map[string]any]{
		Parse: func(c *gin.Context) (*service.RecruiterCreateInput, error) {
			req, err := bindJSON[service.RecruiterCreateInput](c)
			if err != nil {
				return nil, err
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *service.RecruiterCreateInput) (*map[string]any, error) {
			rec, err := h.authSvc.CreateRecruiter(*req)
			if err != nil {
				return nil, err
			}
			m := map[string]any{
				"id":             rec.ID,
				"username":       rec.Username,
				"company_name":   rec.CompanyName,
				"credit_code":    rec.CreditCode,
				"business_scope": rec.BusinessScope,
				"contact_name":   rec.ContactName,
				"contact_phone":  rec.ContactPhone,
				"contact_email":  rec.ContactEmail,
				"wechat":         rec.Wechat,
				"status":         rec.Status,
			}
			return &m, nil
		},
		Render: func(c *gin.Context, _ *service.RecruiterCreateInput, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "招聘者账号创建成功", *resp)
		},
	}.Handle(c)
}

// ToggleStatus 切换招聘者启用/禁用 PUT /api/admin/recruiters/:id/status
func (h *AdminRecruiterHandler) ToggleStatus(c *gin.Context) {
	Endpoint[idParam, map[string]any]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*map[string]any, error) {
			next, err := h.authSvc.ToggleRecruiterStatus(req.ID)
			if err != nil {
				return nil, err
			}
			m := map[string]any{"status": next}
			return &m, nil
		},
		Render: func(c *gin.Context, _ *idParam, resp *map[string]any, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			msg := "招聘者已启用"
			if (*resp)["status"] == int16(0) {
				msg = "招聘者已禁用"
			}
			response.SuccessWithMsg(c, msg, *resp)
		},
	}.Handle(c)
}

// Edit 编辑招聘者企业信息 PUT /api/admin/recruiters/:id（#417）。
func (h *AdminRecruiterHandler) Edit(c *gin.Context) {
	Endpoint[idParam, map[string]any]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*map[string]any, error) {
			var in service.RecruiterEditInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			rec, err := h.authSvc.EditRecruiter(req.ID, in)
			if err != nil {
				return nil, err
			}
			m := map[string]any{
				"id":             rec.ID,
				"username":       rec.Username,
				"company_name":   rec.CompanyName,
				"credit_code":    rec.CreditCode,
				"business_scope": rec.BusinessScope,
				"contact_name":   rec.ContactName,
				"contact_phone":  rec.ContactPhone,
				"contact_email":  rec.ContactEmail,
				"wechat":         rec.Wechat,
			}
			return &m, nil
		},
		Render: func(c *gin.Context, _ *idParam, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "招聘者信息已更新", *resp)
		},
	}.Handle(c)
}

// ResetPassword 重置招聘者密码 PUT /api/admin/recruiters/:id/password（#417）。
func (h *AdminRecruiterHandler) ResetPassword(c *gin.Context) {
	Endpoint[idParam, struct{}]{
		Parse: func(c *gin.Context) (*idParam, error) {
			id, err := pathInt(c, "id", "招聘者ID无效")
			if err != nil {
				return nil, err
			}
			return &idParam{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *idParam) (*struct{}, error) {
			var body struct {
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.Password == "" {
				return nil, badRequest("新密码不能为空")
			}
			if err := h.authSvc.ResetRecruiterPassword(req.ID, body.Password); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, _ *idParam, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "密码已重置", map[string]any{})
		},
	}.Handle(c)
}

// List 招聘者列表 GET /api/admin/recruiters（#416：分页 + 关键字过滤，字段白名单无凭据）。
func (h *AdminRecruiterHandler) List(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	keyword := c.Query("keyword")
	resp, err := h.authSvc.ListRecruiters(page, pageSize, keyword)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, resp)
}
