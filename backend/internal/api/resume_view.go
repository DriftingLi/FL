// Package api 学员侧简历查看留痕聚合（#374）。
package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterResumeViewRoutes 注册学员侧查看聚合 GET /api/resume/view-stats（#374）。
// 仅学员可访问：招聘方无法读取留痕数据（避免暴露浏览习惯）。
func RegisterResumeViewRoutes(rg *gin.RouterGroup, rd RouterDeps, recruitSvc *service.RecruitService) {
	h := NewResumeViewHandler(recruitSvc)
	g := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	g.GET("/view-stats", h.StudentViewStats)
}

// ResumeViewHandler 简历查看留痕聚合 handler。
type ResumeViewHandler struct {
	recruitSvc *service.RecruitService
}

// NewResumeViewHandler 创建简历查看留痕聚合 handler。
func NewResumeViewHandler(recruitSvc *service.RecruitService) *ResumeViewHandler {
	return &ResumeViewHandler{recruitSvc: recruitSvc}
}

// StudentViewStats 近 7 天查看过我的企业数 GET /api/resume/view-stats
// @Summary 简历查看留痕
// @Description 学员查看近 7 天查看过自己简历的企业数（按企业去重，仅聚合数，不含企业名）
// @Tags 学员端-简历卡
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "聚合数 {count}"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/view-stats [get]
func (h *ResumeViewHandler) StudentViewStats(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	cnt, err := h.recruitSvc.StudentViewStats(uid)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// 仅返回聚合数，不含企业名与任何身份信息
	response.Success(c, gin.H{"count": cnt})
}
