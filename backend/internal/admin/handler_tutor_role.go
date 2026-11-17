// 本文件：讲师角色链的管理端点（#2599 / 规格 E7）—— 角色 CRUD + 讲师挂角色。
//
// 全部挂在管理端的 `tutor.manage` 组里（与讲师账号管理同一组）：改讲师角色与改讲师账号
// 是同一件事的两个面，拆成两个键只会让「能建号不能授权」这种半截权限出现。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/pkg/httpx"
)

// tutorRoleIDParam 讲师角色路径参数。
type tutorRoleIDParam struct {
	RoleID int
}

// tutorRolePayload 建 / 改讲师角色的入参。
type tutorRolePayload struct {
	Name         string   `json:"name"`
	Remark       string   `json:"remark"`
	Capabilities []string `json:"capabilities"`
}

// tutorRoleUpdateParam 改角色：路径参数 + 请求体在 Parse 期一次读全（body 只能读一次）。
type tutorRoleUpdateParam struct {
	RoleID int
	tutorRolePayload
}

// tutorRoleAssignPayload 给讲师挂角色的入参。role_id = 0 表示摘掉角色（未授权）。
type tutorRoleAssignPayload struct {
	TutorID int `json:"tutor_id"`
	RoleID  int `json:"role_id"`
}

// @Summary 讲师角色列表
// @Description 列出全部讲师角色及其能力键（受保护的全权讲师返回讲师侧能力全集）
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=admin.TutorRoleListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "权限不足"
// @Router /admin/tutor-roles [get]
// ListTutorRoles 讲师角色列表 GET /api/admin/tutor-roles
func (h *adminHandler) ListTutorRoles(c *gin.Context) {
	httpx.Endpoint[struct{}, TutorRoleListDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*TutorRoleListDTO, error) {
			return h.adminSvc.ListTutorRoles()
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// @Summary 新建讲师角色
// @Description 新建讲师角色并落能力键（能力键必须是讲师侧 7 键之一，管理员侧键一律拒）
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} response.R{data=admin.TutorRoleDTO} "角色已创建"
// @Failure 400 {object} response.R "参数错误 / 未知能力键"
// @Failure 401 {object} response.R "未认证"
// @Failure 409 {object} response.R "角色名已存在"
// @Router /admin/tutor-roles [post]
// CreateTutorRole 新建讲师角色 POST /api/admin/tutor-roles
func (h *adminHandler) CreateTutorRole(c *gin.Context) {
	httpx.Endpoint[tutorRolePayload, TutorRoleDTO]{
		Parse: func(c *gin.Context) (*tutorRolePayload, error) {
			req, err := httpx.BindJSON[tutorRolePayload](c)
			if err != nil {
				return nil, err
			}
			if req.Name == "" {
				return nil, httpx.BadRequest("角色名不能为空")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *tutorRolePayload) (*TutorRoleDTO, error) {
			return h.adminSvc.CreateTutorRole(req.Name, req.Remark, req.Capabilities)
		},
	}.WithSuccess(httpx.Created("角色已创建"), http.StatusBadRequest).
		WithSentinel(ErrUnknownCapability, http.StatusBadRequest).
		WithSentinel(ErrRoleNameTaken, http.StatusConflict).Handle(c)
}

// @Summary 修改讲师角色
// @Description 改角色名 / 备注 / 能力集；受保护的全权讲师角色一律拒绝
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param role_id path int true "角色 ID"
// @Success 200 {object} response.R{data=admin.TutorRoleDTO} "已保存"
// @Failure 400 {object} response.R "参数错误 / 未知能力键"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "角色不存在"
// @Failure 409 {object} response.R "受保护角色不可改 / 角色名已存在"
// @Router /admin/tutor-roles/{role_id} [put]
// UpdateTutorRole 修改讲师角色 PUT /api/admin/tutor-roles/:role_id
func (h *adminHandler) UpdateTutorRole(c *gin.Context) {
	httpx.Endpoint[tutorRoleUpdateParam, TutorRoleDTO]{
		Parse: func(c *gin.Context) (*tutorRoleUpdateParam, error) {
			id, err := httpx.PathInt(c, "role_id", "角色ID无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSON[tutorRolePayload](c)
			if err != nil {
				return nil, err
			}
			return &tutorRoleUpdateParam{RoleID: id, tutorRolePayload: *body}, nil
		},
		Invoke: func(ctx context.Context, req *tutorRoleUpdateParam) (*TutorRoleDTO, error) {
			return h.adminSvc.UpdateTutorRole(req.RoleID, req.Name, req.Remark, req.Capabilities)
		},
	}.WithSuccess(httpx.OkMsg("已保存"), http.StatusBadRequest).
		WithSentinel(ErrProtectedRole, http.StatusConflict).
		WithSentinel(ErrTutorRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrRoleNameTaken, http.StatusConflict).
		WithSentinel(ErrUnknownCapability, http.StatusBadRequest).Handle(c)
}

// @Summary 删除讲师角色
// @Description 删除讲师角色；受保护角色与仍被讲师挂着的角色一律拒绝
// @Tags 管理端-权限
// @Produce json
// @Security BearerAuth
// @Param role_id path int true "角色 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "角色不存在"
// @Failure 409 {object} response.R "受保护角色不可删 / 角色仍在使用"
// @Router /admin/tutor-roles/{role_id} [delete]
// DeleteTutorRole 删除讲师角色 DELETE /api/admin/tutor-roles/:role_id
func (h *adminHandler) DeleteTutorRole(c *gin.Context) {
	httpx.Endpoint[tutorRoleIDParam, struct{}]{
		Parse: func(c *gin.Context) (*tutorRoleIDParam, error) {
			id, err := httpx.PathInt(c, "role_id", "角色ID无效")
			if err != nil {
				return nil, err
			}
			return &tutorRoleIDParam{RoleID: id}, nil
		},
		Invoke: func(ctx context.Context, req *tutorRoleIDParam) (*struct{}, error) {
			return &struct{}{}, h.adminSvc.DeleteTutorRole(req.RoleID)
		},
	}.WithSuccess(httpx.OkMsgNoData("已删除"), http.StatusBadRequest).
		WithSentinel(ErrProtectedRole, http.StatusConflict).
		WithSentinel(ErrTutorRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrTutorRoleInUse, http.StatusConflict).Handle(c)
}

// @Summary 给讲师挂角色
// @Description 挂 / 换 / 摘讲师角色；role_id=0 表示摘掉（未授权）。摘掉最后一个全权讲师会被拒
// @Tags 管理端-权限
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tutor_id path int true "讲师 ID"
// @Success 200 {object} response.R{data=admin.TutorDTO} "已保存"
// @Failure 400 {object} response.R "讲师ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "讲师或角色不存在"
// @Failure 409 {object} response.R "必须保留至少一个全权讲师"
// @Router /admin/tutor/{tutor_id}/role [put]
// AssignTutorRole 给讲师挂角色 PUT /api/admin/tutor/:tutor_id/role
func (h *adminHandler) AssignTutorRole(c *gin.Context) {
	httpx.Endpoint[tutorRoleAssignPayload, TutorDTO]{
		Parse: func(c *gin.Context) (*tutorRoleAssignPayload, error) {
			id, err := httpx.PathInt(c, "tutor_id", "讲师ID无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSON[struct {
				RoleID int `json:"role_id"`
			}](c)
			if err != nil {
				return nil, err
			}
			return &tutorRoleAssignPayload{TutorID: id, RoleID: body.RoleID}, nil
		},
		Invoke: func(ctx context.Context, req *tutorRoleAssignPayload) (*TutorDTO, error) {
			return h.adminSvc.AssignTutorRole(req.TutorID, req.RoleID)
		},
	}.WithSuccess(httpx.OkMsg("已保存"), http.StatusBadRequest).
		WithSentinel(ErrInvalidTutorID, http.StatusBadRequest).
		WithSentinel(ErrTutorRoleNotFound, http.StatusNotFound).
		WithSentinel(ErrLastFullTutor, http.StatusConflict).Handle(c)
}

// 路由注册不另开 RegisterRoutes：本组四条端点挂进 handler.go 的 `tutors` 组
// （组级 `CapabilityRequired(authz.CapTutorManage)`），与讲师账号管理共用一道守卫。
// 另开一个注册函数会让「同一能力键两道组」出现，也躲不过 TestBlueprintCapabilityCoverage 的
// 「每个 RegisterRoutes 必须自挂守卫」判据。
