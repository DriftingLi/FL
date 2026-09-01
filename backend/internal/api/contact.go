// Package api 联系方式交换闭环（#375）。
package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterContactRoutes 注册联系方式交换相关路由（#375）。
func RegisterContactRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ContactService) {
	h := NewContactHandler(svc)
	// 招聘方：发起与查看我的申请 + 读取明文
	recruitG := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	recruitG.POST("/contact-requests", h.Create)
	recruitG.GET("/contact-requests", h.ListForRecruiter)
	recruitG.GET("/resumes/:id/contact", h.GetContact)

	// 学员侧：查看收到的申请 + 同意/拒绝/撤回
	studentG := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.GET("/contact-requests", h.ListForStudent)
	studentG.POST("/contact-requests/:id/approve", h.Approve)
	studentG.POST("/contact-requests/:id/reject", h.Reject)
	studentG.POST("/contact-requests/:id/revoke", h.Revoke)
}

// ContactHandler 联系方式交换 handler。
type ContactHandler struct {
	svc *service.ContactService
}

// NewContactHandler 创建联系方式交换 handler。
func NewContactHandler(svc *service.ContactService) *ContactHandler {
	return &ContactHandler{svc: svc}
}

// Create 企业发起交换申请 POST /api/recruit/contact-requests
// @Summary 发起交换申请
// @Description 企业招聘者带附言向学员发起联系方式交换申请（pending 唯一、30 天冷却、日限 20）
// @Tags 招聘域-联系方式交换
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "申请 {student_user_id, message(1-200)}"
// @Success 201 {object} response.R "申请已提交"
// @Failure 400 {object} response.R "参数错误/唯一/冷却/日限"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/contact-requests [post]
func (h *ContactHandler) Create(c *gin.Context) {
	var body struct {
		StudentUserID int    `json:"student_user_id"`
		Message       string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	recruiterID := middleware.CurrentUserID(c)
	dto, err := h.svc.Create(recruiterID, body.StudentUserID, body.Message)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "申请已提交", dto)
}

// ListForRecruiter 招聘方我的申请列表 GET /api/recruit/contact-requests
// @Summary 我的申请
// @Description 企业招聘者查看自己发出的交换申请（pending/approved/rejected/expired/revoked）
// @Tags 招聘域-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /recruit/contact-requests [get]
func (h *ContactHandler) ListForRecruiter(c *gin.Context) {
	recruiterID := middleware.CurrentUserID(c)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	items, total, err := h.svc.ListForRecruiter(recruiterID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// GetContact 明文联系方式 GET /api/recruit/resumes/:id/contact
// @Summary 明文联系方式
// @Description 企业读取学员明文联系方式与 PDF（仅 approved 授权有效时可用；投递产生的授权同样放行；实时校验无缓存）
// @Tags 招聘域-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "学员 ID"
// @Success 200 {object} response.R "明文 {real_name, contact_phone, wechat, resume_file_url}"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无有效授权"
// @Router /recruit/resumes/{id}/contact [get]
func (h *ContactHandler) GetContact(c *gin.Context) {
	idStr := c.Param("id")
	uid, err := strconv.Atoi(idStr)
	if err != nil || uid <= 0 {
		response.BadRequest(c, "学员 ID 无效")
		return
	}
	recruiterID := middleware.CurrentUserID(c)
	dto, err := h.svc.GetContact(recruiterID, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, service.ErrContactNoAuth) || errors.Is(err, service.ErrStudentGone) {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	// 仅返回明文字段（phone/wechat/real_name/pdf），不返回其他已脱敏字段（保持 L2 口径）
	response.Success(c, gin.H{
		"real_name":       dto.RealName,
		"contact_phone":   dto.ContactPhone,
		"wechat":          dto.Wechat,
		"resume_file_url": dto.ResumeFileURL,
	})
}

// ListForStudent 学员收到的申请列表 GET /api/resume/contact-requests
// @Summary 收到的申请
// @Description 学员查看收到的交换申请（含企业名/联系人/附言，不含企业电话）
// @Tags 学员端-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/contact-requests [get]
func (h *ContactHandler) ListForStudent(c *gin.Context) {
	studentID := middleware.CurrentUserID(c)
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	items, total, err := h.svc.ListForStudent(studentID, page, pageSize)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

// Approve 学员同意申请 POST /api/resume/contact-requests/:id/approve
// @Summary 同意申请
// @Description 学员同意交换申请（status → approved，招聘方收邮件通知）
// @Tags 学员端-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请 ID"
// @Success 200 {object} response.R "已同意"
// @Failure 400 {object} response.R "状态不允许/已过期"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/contact-requests/{id}/approve [post]
func (h *ContactHandler) Approve(c *gin.Context) {
	id, err := pathInt64(c, "id", "申请 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	studentID := middleware.CurrentUserID(c)
	dto, err := h.svc.Approve(studentID, id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, dto)
}

// Reject 学员拒绝申请 POST /api/resume/contact-requests/:id/reject
// @Summary 拒绝申请
// @Description 学员拒绝交换申请（status → rejected，30 天冷却）
// @Tags 学员端-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请 ID"
// @Success 200 {object} response.R "已拒绝"
// @Failure 400 {object} response.R "状态不允许"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/contact-requests/{id}/reject [post]
func (h *ContactHandler) Reject(c *gin.Context) {
	id, err := pathInt64(c, "id", "申请 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	studentID := middleware.CurrentUserID(c)
	dto, err := h.svc.Reject(studentID, id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, dto)
}

// Revoke 学员撤回授权 POST /api/resume/contact-requests/:id/revoke
// @Summary 撤回授权
// @Description 学员撤回已同意的授权（status → revoked，实时生效，明文端点随即 403）
// @Tags 学员端-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请 ID"
// @Success 200 {object} response.R "已撤回"
// @Failure 400 {object} response.R "状态不允许"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/contact-requests/{id}/revoke [post]
func (h *ContactHandler) Revoke(c *gin.Context) {
	id, err := pathInt64(c, "id", "申请 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	studentID := middleware.CurrentUserID(c)
	dto, err := h.svc.Revoke(studentID, id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, dto)
}
