// 本文件：当前管理员的**有效能力集**出口（#1618 段1）。
//
// 为什么单独一个端点、而不并进 /auth/me：/auth/me 的响应形状被契约测试锁住且服务四种角色，
// 而「管理端能力集」只对 admin 有意义 —— 塞进去会让四端共用的形状多出一个恒空的字段。
package admin

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/pkg/httpx"
)

// AdminCapabilitiesDTO 当前管理员的有效能力集（能力键按字典序稳定输出）。
type AdminCapabilitiesDTO struct {
	// Capabilities 有效能力键；未挂角色时为空数组（**不是 null** —— 契约由
	// apitypes 的可空性锁按 nullability 声明核对）。
	Capabilities []string `json:"capabilities" nullability:"nonnil"`
	// Granted 是否已挂角色。false = 未授权（role_id 为 NULL）或账号不存在。
	// 前端据此区分「还没分配角色」与「角色里什么都没勾」：两者都是空集，但话术不同。
	Granted bool `json:"granted"`
}

// myCapabilitiesReq 本端点的输入（端点无参数，只带出身份）。
type myCapabilitiesReq struct {
	AdminID int
}

// @Summary 当前管理员的有效能力集
// @Description 登录后前端拉取「我的管理端能力」，用于路由守卫与侧栏过滤（#1618 段1）
// @Tags 管理端-账号
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=admin.AdminCapabilitiesDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "非管理员身份"
// @Router /admin/me/capabilities [get]
// MyCapabilities 当前管理员的有效能力集 GET /api/admin/me/capabilities
func (h *adminHandler) MyCapabilities(c *gin.Context) {
	httpx.Endpoint[myCapabilitiesReq, AdminCapabilitiesDTO]{
		Parse: func(c *gin.Context) (*myCapabilitiesReq, error) {
			if middleware.CurrentRole(c) != string(authz.RoleAdmin) {
				return nil, &httpx.ParseError{Status: http.StatusForbidden, Message: "权限不足"}
			}
			return &myCapabilitiesReq{AdminID: middleware.CurrentUserID(c)}, nil
		},
		Invoke: func(ctx context.Context, req *myCapabilitiesReq) (*AdminCapabilitiesDTO, error) {
			return h.adminSvc.AdminCapabilitySet(req.AdminID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}
