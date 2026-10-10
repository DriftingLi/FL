// 本文件：管理角色与管理员账号的管理端点（#1621 段4）。
//
// 可达性：「授权界面仅超管可达」不靠特判 —— admin.role.manage / admin.account.manage 两个能力键
// **只挂在受保护角色**上（authz 的 protectedAdminCapabilities），普通角色的能力集里没有它们，
// 于是守卫自然拒绝、侧栏自然不显示。
package admin

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/core"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// roleIDParam 路径参数（改挂角色那条把路径参数与请求体合成 accountRolePayload，故无独立类型）。
type roleIDParam struct {
	RoleID int
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

// adminCreatePayload 新建管理员账号的入参（#1632）。role_id 缺省 0 = 先建号、后挂角色。
type adminCreatePayload struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"`
	RoleID   int    `json:"role_id"`
}

// adminIDParam 路径参数（删除账号；改挂角色那条把路径参数与请求体合成 accountRolePayload）。
type adminIDParam struct {
	AdminID int
}

// adminPasswordPayload 代重置口令的入参（路径参数 + 请求体在 Parse 期一次读全）。
type adminPasswordPayload struct {
	AdminID  int
	Password string `json:"password"`
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

// @Summary 新建管理员
// @Description 新建管理员账号并（可选）挂角色；口令走 6-20 位规则、bcrypt 落库，账号名唯一
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} response.R{data=admin.AdminAccountDTO} "管理员已创建"
// @Failure 400 {object} response.R "参数错误 / 口令不合规 / 角色不存在"
// @Failure 401 {object} response.R "未认证"
// @Failure 409 {object} response.R "账号名已存在"
// @Router /admin/accounts [post]
// CreateAdminAccount 新建管理员 POST /api/admin/accounts
func (h *adminHandler) CreateAdminAccount(c *gin.Context) {
	httpx.Endpoint[adminCreatePayload, AdminAccountDTO]{
		Parse: func(c *gin.Context) (*adminCreatePayload, error) {
			req, err := httpx.BindJSON[adminCreatePayload](c)
			if err != nil {
				return nil, err
			}
			if strings.TrimSpace(req.Username) == "" {
				return nil, httpx.BadRequest("账号不能为空")
			}
			if strings.TrimSpace(req.Name) == "" {
				return nil, httpx.BadRequest("姓名不能为空")
			}
			// 长度规则在动作里兜底（ADR-0064 决策 4），这里只是**在落到服务端之前**给出同一句文案
			if err := core.ValidatePasswordLength(req.Password); err != nil {
				return nil, httpx.BadRequest(err.Error())
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *adminCreatePayload) (*AdminAccountDTO, error) {
			return h.adminSvc.CreateAdminAccount(req.Username, req.Name, req.Password, req.RoleID)
		},
	}.WithSuccess(httpx.Created("管理员已创建"), http.StatusBadRequest).
		WithSentinel(ErrAdminUsernameTaken, http.StatusConflict).
		WithSentinel(ErrRoleNotFound, http.StatusBadRequest).Handle(c)
}

// @Summary 删除管理员
// @Description 删除管理员账号；**最后一个超管**与自己都不可删（防自锁第二层），先吊销其会话再落库删除
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Param admin_id path int true "管理员 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 400 {object} response.R "管理员ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "管理员不存在"
// @Failure 409 {object} response.R "必须保留至少一个超级管理员 / 不能删除自己的账号"
// @Router /admin/accounts/{admin_id} [delete]
// DeleteAdminAccount 删除管理员 DELETE /api/admin/accounts/:admin_id
func (h *adminHandler) DeleteAdminAccount(c *gin.Context) {
	httpx.Endpoint[adminIDParam, struct{}]{
		Parse: func(c *gin.Context) (*adminIDParam, error) {
			id, err := httpx.PathInt(c, "admin_id", "管理员ID无效")
			if err != nil {
				return nil, err
			}
			return &adminIDParam{AdminID: id}, nil
		},
		Invoke: func(ctx context.Context, req *adminIDParam) (*struct{}, error) {
			// 操作者身份取自令牌（自删判据要它），不从请求体收 —— 那会变成可伪造的入参
			actorID := middleware.CurrentUserID(c)
			return &struct{}{}, h.adminSvc.DeleteAdminAccount(ctx, actorID, req.AdminID)
		},
	}.WithSuccess(httpx.OkMsgNoData("已删除"), http.StatusBadRequest).
		WithSentinel(ErrInvalidAdminID, http.StatusBadRequest).
		WithSentinel(ErrAdminNotFound, http.StatusNotFound).
		WithSentinel(ErrLastSuperAdmin, http.StatusConflict).
		WithSentinel(ErrSelfDelete, http.StatusConflict).Handle(c)
}

// @Summary 重置管理员口令
// @Description 代重置某个管理员的口令：6-20 位规则 + bcrypt 落库 + 全会话吊销；响应不回显口令
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param admin_id path int true "管理员 ID"
// @Success 200 {object} response.R "已重置"
// @Failure 400 {object} response.R "管理员ID无效 / 口令不合规"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "管理员不存在"
// @Router /admin/accounts/{admin_id}/password [put]
// ResetAdminPassword 代重置管理员口令 PUT /api/admin/accounts/:admin_id/password
func (h *adminHandler) ResetAdminPassword(c *gin.Context) {
	httpx.Endpoint[adminPasswordPayload, struct{}]{
		Parse: func(c *gin.Context) (*adminPasswordPayload, error) {
			id, err := httpx.PathInt(c, "admin_id", "管理员ID无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSON[struct {
				Password string `json:"password"`
			}](c)
			if err != nil {
				return nil, err
			}
			// 长度规则在动作里兜底（ADR-0064 决策 4），这里只是同一句文案的前置拦截
			if err := core.ValidatePasswordLength(body.Password); err != nil {
				return nil, httpx.BadRequest(err.Error())
			}
			return &adminPasswordPayload{AdminID: id, Password: body.Password}, nil
		},
		Invoke: func(ctx context.Context, req *adminPasswordPayload) (*struct{}, error) {
			return &struct{}{}, h.adminSvc.ResetAdminPassword(ctx, req.AdminID, req.Password)
		},
	}.WithSuccess(httpx.OkMsgNoData("已重置"), http.StatusBadRequest).
		WithSentinel(ErrInvalidAdminID, http.StatusBadRequest).
		WithSentinel(ErrAdminNotFound, http.StatusNotFound).Handle(c)
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
	accounts.POST("", h.CreateAdminAccount)
	accounts.PUT("/:admin_id/role", h.AssignAdminRole)
	accounts.PUT("/:admin_id/password", h.ResetAdminPassword)
	accounts.DELETE("/:admin_id", h.DeleteAdminAccount)
}
