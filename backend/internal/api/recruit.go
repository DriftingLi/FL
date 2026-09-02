// Package api 实现 HTTP handlers。
// 本文件：企业招聘端受保护接口（第四角色 recruiter，host-only cookie 隔离，脱敏简历列表与详情，审计留痕）。
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

// RecruitHandler 招聘端 handler（脱敏读）。
type RecruitHandler struct {
	svc *service.RecruitService
}

// NewRecruitHandler 创建 handler。
func NewRecruitHandler(svc *service.RecruitService) *RecruitHandler {
	return &RecruitHandler{svc: svc}
}

// RegisterRecruitRoutes 注册 /api/recruit 蓝图（企业招聘者工作区，角色守卫）。
func RegisterRecruitRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.RecruitService) {
	h := NewRecruitHandler(svc)
	g := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	g.GET("/resumes", h.ListResumes)
	g.GET("/resumes/:id", h.GetResume)
	g.GET("/me", recruitMe)
}

// ListResumes 招聘端脱敏简历列表 GET /api/recruit/resumes
// 过滤轴：region / position_id / credential_id / salary_min / salary_max / experience_years / available_in
// 默认排序 updated_at DESC（service 层保证）；读写最新，无缓存；读取后审计留痕。
func (h *RecruitHandler) ListResumes(c *gin.Context) {
	params := service.RecruitListParams{
		Page:        atoiDefault(c.Query("page"), 1),
		PageSize:    atoiDefault(c.Query("page_size"), 20),
		Region:      c.Query("region"),
		AvailableIn: c.Query("available_in"),
	}
	if v := queryIDPtr(c, "position_id"); v != nil {
		params.PositionID = v
	}
	if v := queryIDPtr(c, "credential_id"); v != nil {
		params.CredentialID = v
	}
	if v := queryIntPtr(c, "salary_min"); v != nil {
		params.SalaryMin = v
	}
	if v := queryIntPtr(c, "salary_max"); v != nil {
		params.SalaryMax = v
	}
	if v := queryIntPtr(c, "experience_years"); v != nil {
		params.ExperienceYears = v
	} else {
		if v := queryIntPtr(c, "experience_min"); v != nil {
			params.ExperienceMin = v
		}
		if v := queryIntPtr(c, "experience_max"); v != nil {
			params.ExperienceMax = v
		}
	}
	// #489：当前招聘者上下文用于 contact_state 回填
	params.RecruiterID = middleware.CurrentUserID(c)
	result, err := h.svc.List(params)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// 审计留痕：列表实际读取的每张卡各记一次（best-effort，不影响响应）
	if len(result.Items) > 0 {
		recruiterID := middleware.CurrentUserID(c)
		ids := make([]int, 0, len(result.Items))
		for _, it := range result.Items {
			ids = append(ids, it.UserID)
		}
		h.svc.LogViews(recruiterID, ids)
	}
	response.Success(c, result)
}

// GetResume 招聘端脱敏简历详情 GET /api/recruit/resumes/:id
// 与列表共用同一脱敏实现（service 层 desensitize），不存在两套逻辑；隐藏卡 404。
func (h *RecruitHandler) GetResume(c *gin.Context) {
	idStr := c.Param("id")
	uid, err := strconv.Atoi(idStr)
	if err != nil || uid <= 0 {
		response.BadRequest(c, "简历 ID 无效")
		return
	}
	card, err := h.svc.GetForRecruiter(uid, middleware.CurrentUserID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "简历不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	// 审计留痕：详情实际读取计一次
	h.svc.LogView(middleware.CurrentUserID(c), uid)
	response.Success(c, card)
}

// recruitMe 招聘者当前用户信息 GET /api/recruit/me（复用 /auth/me 的 ProfileDTO 形状，但仅 recruiter 可访问）
func recruitMe(c *gin.Context) {
	response.Success(c, map[string]any{
		"user_id": middleware.CurrentUserID(c),
		"account": middleware.CurrentAccount(c),
		"role":    middleware.CurrentRole(c),
	})
}
