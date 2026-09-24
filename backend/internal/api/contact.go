// Package api 联系方式交换闭环（#375）。
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterContactRoutes 注册联系方式交换相关路由（#375）。
func RegisterContactRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ContactService) {
	h := NewContactHandler(svc)
	// 招聘方：发起与查看我的申请 + 读取明文
	recruitG := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapContactRequest))
	recruitG.POST("/contact-requests", h.Create)
	recruitG.GET("/contact-requests", h.ListForRecruiter)
	recruitG.GET("/resumes/:id/contact", h.GetContact)

	// 学员侧：查看收到的申请 + 同意/拒绝/撤回
	studentG := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapContactRespond))
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

// contactCreateBody 请求体的两段（键名即 wire 契约，不并入 Req：Req 还带会话身份，那不是本请求的 body）。
type contactCreateBody struct {
	StudentUserID int    `json:"student_user_id"`
	Message       string `json:"message"`
}

// contactCreateReq 发起交换申请的端点内请求：body 两段 + 从会话取出的招聘者身份。
// 身份在 Parse 段读——Invoke 只拿得到 ctx，拿不到 gin.Context。
type contactCreateReq struct {
	RecruiterID   int
	StudentUserID int
	Message       string
}

// contactCreateFacts400 「发起交换申请」这一面的**输入不合法与业务事实**全集（ADR-0065 决策 3·4）。
//
// 档位口径（本波决策 3 已落码的那条，这里复述以免被下一波「顺手统一」）：
//   - body 里的引用指向不存在的行 ⇒ 400。`ErrStudentNotFound` / `ErrRecruiterNotFound` 在本端点
//     落 400，而在 `GET /student/profile` 落 404 —— **不是同一件事实的两种码，是两件事实**：
//     404 说的是「被请求的那个资源没有」，而本端点被请求的资源（申请集合）在，坏的是 body 的引用。
//     载体合一（一个事实一个哨兵）从来不要求档位合一（决策 4 原文的「400→404」由此更正）。
//   - 其余 400 条各说一件事实，压成一句会丢掉「是哪一件」。
//   - 表里没有的（`expireClosed` / 计数 / 写入 等 DB 故障）走端点默认面 500，不再冒充上面任何一句。
var contactCreateFacts400 = []error{
	service.ErrContactMessageEmpty,
	service.ErrContactMessageTooLong,
	service.ErrContactReqInvalid,
	service.ErrStudentNotFound,
	service.ErrRecruiterNotFound,
	service.ErrRecruiterDisabled,
	service.ErrContactPendingExists,
	service.ErrContactInCooldown,
	service.ErrContactDailyLimit,
}

// Create 企业发起交换申请 POST /api/recruit/contact-requests
// @Summary 发起交换申请
// @Description 企业招聘者带附言向学员发起联系方式交换申请（pending 唯一、30 天冷却、日限 20）
// @Tags 招聘域-联系方式交换
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "申请 {student_user_id, message(1-200)}"
// @Success 201 {object} response.R{data=service.ContactRequestDTO} "申请已提交"
// @Failure 400 {object} response.R "附言空/超长、参数错、引用的学员或招聘者不存在或已禁用、pending 唯一、冷却期、日限"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "服务端内部错误（DB 故障；不外发驱动原文）"
// @Router /recruit/contact-requests [post]
func (h *ContactHandler) Create(c *gin.Context) {
	Endpoint[contactCreateReq, service.ContactRequestDTO]{
		Parse: func(c *gin.Context) (*contactCreateReq, error) {
			body, err := bindJSON[contactCreateBody](c)
			if err != nil {
				return nil, err
			}
			return &contactCreateReq{
				RecruiterID:   middleware.CurrentUserID(c),
				StudentUserID: body.StudentUserID,
				Message:       body.Message,
			}, nil
		},
		Invoke: func(ctx context.Context, req *contactCreateReq) (*service.ContactRequestDTO, error) {
			return h.svc.Create(req.RecruiterID, req.StudentUserID, req.Message)
		},
	}.WithSuccess(created("申请已提交"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, contactCreateFacts400...).Handle(c)
}

// ListForRecruiter 招聘方我的申请列表 GET /api/recruit/contact-requests
// @Summary 我的申请
// @Description 企业招聘者查看自己发出的交换申请（pending/approved/rejected/expired/revoked）
// @Tags 招聘域-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R{data=service.ContactRequestListResult} "列表"
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
	// typed page（#1095）：键序 = 旧 gin.H 的 map 键序，响应字节不变（登记表 service.ContactRequestListResult）。
	response.Success(c, service.ContactRequestListResult{Items: items, Page: page, PageSize: pageSize, Total: total})
}

// GetContact 明文联系方式 GET /api/recruit/resumes/:id/contact
// @Summary 明文联系方式
// @Description 企业读取学员明文联系方式与 PDF（仅 approved 授权有效时可用；投递产生的授权同样放行；实时校验无缓存）
// @Tags 招聘域-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "学员 ID"
// @Success 200 {object} response.R{data=service.ContactPlainDTO} "明文 {real_name, contact_phone, wechat, resume_file_url}"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无有效授权"
// @Router /recruit/resumes/{id}/contact [get]
func (h *ContactHandler) GetContact(c *gin.Context) {
	uid, err := pathInt(c, "id", "学员 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	recruiterID := middleware.CurrentUserID(c)
	dto, err := h.svc.GetContact(recruiterID, uid)
	if err != nil {
		// 三个失败原因一律答 403，但各按自己的具名哨兵给文案（ADR-0062 决策 9 对称化后新增第三态）：
		// ErrContactNoAuth 从未授权 / ErrStudentGone 学员已注销 / ErrCompanyUnavailable 企业自身被停用。
		// 「授权存在」与「授权可用」不混为一谈——被禁用的企业手上确有 approved，报的却是企业已停用。
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, service.ErrContactNoAuth) ||
			errors.Is(err, service.ErrStudentGone) || errors.Is(err, service.ErrCompanyUnavailable) {
			response.Forbidden(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	// 授权后补齐面（#489）：明文核心字段 + 上传附件 + 工作照 + 证书原图（含 image_urls）。
	// 仅 approved 授权可到达本响应（service GetContact 实时校验），未授权 403。
	response.Success(c, gin.H{
		"real_name":             dto.RealName,
		"contact_phone":         dto.ContactPhone,
		"wechat":                dto.Wechat,
		"resume_file_url":       dto.ResumeFileURL,
		"photos":                dto.Photos,
		"resume_certifications": dto.ResumeCertifications,
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
// @Success 200 {object} response.R{data=service.ContactRequestListResult} "列表"
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
	// typed page（#1095）：同上，学员侧共用同一 DTO。
	response.Success(c, service.ContactRequestListResult{Items: items, Page: page, PageSize: pageSize, Total: total})
}

// Approve 学员同意申请 POST /api/resume/contact-requests/:id/approve
// @Summary 同意申请
// @Description 学员同意交换申请（status → approved，招聘方收邮件通知）
// @Tags 学员端-联系方式交换
// @Produce json
// @Security BearerAuth
// @Param id path int true "申请 ID"
// @Success 200 {object} response.R{data=service.ContactRequestDTO} "已同意"
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
// @Success 200 {object} response.R{data=service.ContactRequestDTO} "已拒绝"
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
// @Success 200 {object} response.R{data=service.ContactRequestDTO} "已撤回"
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
