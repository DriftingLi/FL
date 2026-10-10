// 本文件：管理角色与管理员账号的管理端点（#1621 段4）。
//
// 可达性：「授权界面仅超管可达」不靠特判 —— admin.role.manage / admin.account.manage 两个能力键
// **只挂在受保护角色**上（authz 的 protectedAdminCapabilities），普通角色的能力集里没有它们，
// 于是守卫自然拒绝、侧栏自然不显示。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// roleIDParam / adminIDParam 路径参数。
type roleIDParam struct {
	RoleID int
}

type adminIDParam struct {
	AdminID int
}

// rolePayload 建角色 / 改角色的入参。
type rolePayload struct {
	Name         string   `json:"name"`
	Remark       string   `json:"remark"`
	Capabilities []string `json:"capabilities"`
}

// roleUpdateParam 改角色：路径参数 + 请求体**在 Parse 期一次读全**（body 只能读一次，
// 在 Invoke 里再 BindJSON 只会拿到空值）。
type roleUpdateParam struct {
	RoleID int
	rolePayload
}

// accountRolePayload 改挂角色的入参。
type accountRolePayload struct {
	AdminID int `json:"admin_id"`
	RoleID  int `json:"role_id"`
}

// @Summary 管理角色列表
// @Description 列出全部管理角色及其能力键（受保护角色返回受保护能力全集）
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=admin.AdminRoleListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/roles [get]
// ListAdminRoles 角色列表 GET /api/admin/roles
func (h *adminHandler) ListAdminRoles(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminRoleListDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminRoleListDTO, error) {
			return h.adminSvc.ListAdminRoles()
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 新建管理角色
// @Description 新建角色并落能力键（能力键必须是 authz 能力表里的键）
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} response.R{data=admin.AdminRoleDTO} "角色已创建"
// @Failure 400 {object} response.R "参数错误 / 未知能力键"
// @Failure 401 {object} response.R "未认证"
// @Failure 409 {object} response.R "角色名已存在"
// @Router /admin/roles [post]
// CreateAdminRole 新建角色 POST /api/admin/roles
func (h *adminHandler) CreateAdminRole(c *gin.Context) {
	httpx.Endpoint[rolePayload, AdminRoleDTO]{
		Parse: func(c *gin.Context) (*rolePayload, error) {
			req, err := httpx.BindJSON[rolePayload](c)
			if err != nil {
				return nil, err
			}
			if req.Name == "" {
				return nil, httpx.BadRequest("角色名不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *rolePayload) (*AdminRoleDTO, error) {
			return h.adminSvc.CreateAdminRole(req.Name, req.Remark, req.Capabilities)
		},
	}.WithSuccess(httpx.Created("角色已创建"), http.StatusBadRequest).
		WithSentinel(ErrUnknownCapability, http.StatusBadRequest).
		WithSentinel(ErrRoleNameTaken, http.StatusConflict).Handle(c)
}

// @Summary 修改管理角色
// @Description 改角色名 / 备注 / 能力集；受保护角色（超级管理员）一律拒绝
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param role_id path int true "角色 ID"
// @Success 200 {object} response.R{data=admin.AdminRoleDTO} "已保存"
// @Failure 400 {object} response.R "参数错误 / 未知能力键"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "角色不存在"
// @Failure 409 {object} response.R "受保护角色不可修改 / 角色名已存在"
// @Router /admin/roles/{role_id} [put]
// UpdateAdminRole 修改角色 PUT /api/admin/roles/:role_id
func (h *adminHandler) UpdateAdminRole(c *gin.Context) {
	httpx.Endpoint[roleUpdateParam, AdminRoleDTO]{
		Parse: func(c *gin.Context) (*roleUpdateParam, error) {
			id, err := httpx.PathInt(c, "role_id", "角色ID无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSON[rolePayload](c)
			if err != nil {
				return nil, err
			}
			if body.Name == "" {
				return nil, httpx.BadRequest("角色名不能为空")
			}
			return &roleUpdateParam{RoleID: id, rolePayload: *body}, nil
		},
		Invoke: func(ctx context.Context, req *roleUpdateParam) (*AdminRoleDTO, error) {
			return h.adminSvc.UpdateAdminRole(req.RoleID, req.Name, req.Remark, req.Capabilities)
		},
	}.WithSuccess(httpx.OkMsg("已保存"), http.StatusBadRequest).
		WithSentinel(ErrUnknownCapability, http.StatusBadRequest).
		WithSentinel(ErrRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrProtectedRole, http.StatusConflict).
		WithSentinel(ErrRoleNameTaken, http.StatusConflict).Handle(c)
}

// @Summary 删除管理角色
// @Description 删除角色；受保护角色或被管理员挂着的角色一律拒绝
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Param role_id path int true "角色 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 400 {object} response.R "角色ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "角色不存在"
// @Failure 409 {object} response.R "受保护角色不可删除 / 角色仍被使用"
// @Router /admin/roles/{role_id} [delete]
// DeleteAdminRole 删除角色 DELETE /api/admin/roles/:role_id
func (h *adminHandler) DeleteAdminRole(c *gin.Context) {
	httpx.Endpoint[roleIDParam, struct{}]{
		Parse: func(c *gin.Context) (*roleIDParam, error) {
			id, err := httpx.PathInt(c, "role_id", "角色ID无效")
			if err != nil {
				return nil, err
			}
			return &roleIDParam{RoleID: id}, nil
		},
		Invoke: func(ctx context.Context, req *roleIDParam) (*struct{}, error) {
			return &struct{}{}, h.adminSvc.DeleteAdminRole(req.RoleID)
		},
	}.WithSuccess(httpx.OkMsgNoData("已删除"), http.StatusBadRequest).
		WithSentinel(ErrRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrProtectedRole, http.StatusConflict).
		WithSentinel(ErrRoleInUse, http.StatusConflict).Handle(c)
}

// @Summary 管理员账号列表
// @Description 列出全部管理员及其所挂角色（role_id 为 0 表示尚未挂角色）
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=admin.AdminAccountListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/accounts [get]
// ListAdminAccounts 管理员列表 GET /api/admin/accounts
func (h *adminHandler) ListAdminAccounts(c *gin.Context) {
	httpx.Endpoint[struct{}, AdminAccountListDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*AdminAccountListDTO, error) {
			return h.adminSvc.ListAdminAccounts()
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 管理员改挂角色
// @Description 给管理员改挂角色；**最后一个超管不可降级**（防自锁第二层）
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param admin_id path int true "管理员 ID"
// @Success 200 {object} response.R{data=admin.AdminAccountDTO} "已保存"
// @Failure 400 {object} response.R "管理员ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "管理员或角色不存在"
// @Failure 409 {object} response.R "必须保留至少一个超级管理员"
// @Router /admin/accounts/{admin_id}/role [put]
// AssignAdminRole 改挂角色 PUT /api/admin/accounts/:admin_id/role
func (h *adminHandler) AssignAdminRole(c *gin.Context) {
	httpx.Endpoint[accountRolePayload, AdminAccountDTO]{
		Parse: func(c *gin.Context) (*accountRolePayload, error) {
			id, err := httpx.PathInt(c, "admin_id", "管理员ID无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSON[struct {
				RoleID int `json:"role_id"`
			}](c)
			if err != nil {
				return nil, err
			}
			return &accountRolePayload{AdminID: id, RoleID: body.RoleID}, nil
		},
		Invoke: func(ctx context.Context, req *accountRolePayload) (*AdminAccountDTO, error) {
			return h.adminSvc.AssignAdminRole(req.AdminID, req.RoleID)
		},
	}.WithSuccess(httpx.OkMsg("已保存"), http.StatusBadRequest).
		WithSentinel(ErrInvalidAdminID, http.StatusBadRequest).
		WithSentinel(ErrAdminNotFound, http.StatusNotFound).
		WithSentinel(ErrRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrLastSuperAdmin, http.StatusConflict).Handle(c)
}

// RegisterAdminAuthzRoutes 注册授权管理蓝图（角色 CRUD + 管理员挂角色，#1621 段4）。
//
// 两个分组各挂一个能力键，键只存在于受保护角色 ⇒ 非超管一律 403。
func RegisterAdminAuthzRoutes(rg *gin.RouterGroup, session *security.Session, adminSvc *Service) {
	h := newHandler(adminSvc, nil, nil, nil)

	roles := rg.Group("/admin/roles", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAdminRoleManage))
	roles.GET("", h.ListAdminRoles)
	roles.POST("", h.CreateAdminRole)
	roles.PUT("/:role_id", h.UpdateAdminRole)
	roles.DELETE("/:role_id", h.DeleteAdminRole)

	accounts := rg.Group("/admin/accounts", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapAdminAccountManage))
	accounts.GET("", h.ListAdminAccounts)
	accounts.PUT("/:admin_id/role", h.AssignAdminRole)
}
