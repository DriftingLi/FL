// Package handler 实现残值评估模块的 HTTP 处理器。
// 本文件：管理员对评估模块独立用户的管理接口（/api/valuation/admin/users）。
// 鉴权：走主体系 admin JWT（middleware.JWTAuth + RoleRequired("admin")），
// 与残值配置管理共用同一管理员鉴权链，不参与估值独立登录体系。
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	vservice "forklift-training/internal/valuation/service"
)

// ValuationAdminUserHandler 评估用户管理处理器
type ValuationAdminUserHandler struct {
	authSvc *vservice.ValuationAuthService
}

// NewValuationAdminUserHandler 构造评估用户管理处理器
func NewValuationAdminUserHandler(authSvc *vservice.ValuationAuthService) *ValuationAdminUserHandler {
	return &ValuationAdminUserHandler{authSvc: authSvc}
}

// List 处理 GET /api/valuation/admin/users?page=1&page_size=20&keyword=
// 分页查询评估用户，支持按用户名/姓名/手机号模糊搜索
func (h *ValuationAdminUserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	list, total, err := h.authSvc.ListValuationUsers(page, pageSize, keyword)
	if err != nil {
		Error(c, http.StatusInternalServerError, CodeDatabaseError, "查询评估用户列表失败")
		return
	}
	OK(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      list,
	})
}

// createValuationUserRequest 新增评估用户请求体
type createValuationUserRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Company  string `json:"company"`
}

// Create 处理 POST /api/valuation/admin/users
// 管理员新增评估用户（username 由手机号自动生成）
func (h *ValuationAdminUserHandler) Create(c *gin.Context) {
	var req createValuationUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}
	user, err := h.authSvc.AdminCreateValuationUser(req.Phone, req.Password, req.Name, req.Email, req.Company)
	if err != nil {
		Fail(c, CodeBadRequest, err.Error())
		return
	}
	OK(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"name":     user.Name,
		"phone":    user.Phone,
	})
}

// updateValuationUserRequest 更新评估用户请求体（不含密码）
type updateValuationUserRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Company string `json:"company"`
	Status  int16  `json:"status"`
}

// Update 处理 PUT /api/valuation/admin/users/:id
// 管理员更新评估用户资料（姓名/邮箱/公司/状态），不含密码
func (h *ValuationAdminUserHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var req updateValuationUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if req.Name == "" {
		Error(c, http.StatusBadRequest, CodeBadRequest, "姓名不能为空")
		return
	}
	if req.Status != 0 && req.Status != 1 {
		Error(c, http.StatusBadRequest, CodeBadRequest, "状态值非法（仅支持 0/1）")
		return
	}
	if err := h.authSvc.AdminUpdateValuationUser(id, req.Name, req.Email, req.Company, req.Status); err != nil {
		Fail(c, CodeDatabaseError, err.Error())
		return
	}
	OK(c, nil)
}

// resetValuationUserPasswordRequest 重置密码请求体
type resetValuationUserPasswordRequest struct {
	Password string `json:"password"`
}

// ResetPassword 处理 PUT /api/valuation/admin/users/:id/password
// 管理员重置评估用户密码
func (h *ValuationAdminUserHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	var req resetValuationUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if len(req.Password) < 6 || len(req.Password) > 20 {
		Error(c, http.StatusBadRequest, CodeInvalidParam, "密码长度需为 6-20 个字符")
		return
	}
	if err := h.authSvc.AdminResetValuationUserPassword(id, req.Password); err != nil {
		Fail(c, CodeDatabaseError, err.Error())
		return
	}
	OK(c, nil)
}

// Delete 处理 DELETE /api/valuation/admin/users/:id
// 管理员删除评估用户
func (h *ValuationAdminUserHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, CodeBadRequest, "id 必须为整数")
		return
	}
	if err := h.authSvc.AdminDeleteValuationUser(id); err != nil {
		Fail(c, CodeDatabaseError, err.Error())
		return
	}
	OK(c, nil)
}
