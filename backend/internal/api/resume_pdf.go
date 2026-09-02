// Package api 在线简历 PDF 端点（spec #484 / 子票 #485）。
//   - GET /api/recruit/resumes/:id/pdf：招聘者未授权即可内嵌预览（recruiter 鉴权；隐藏卡 404）
//   - GET /api/resume/pdf：学员预览自己的打码在线简历（本人 JWT；所见即招聘者所见）
//
// 响应 inline PDF（Content-Type: application/pdf），前端以带鉴权的 blob 取流内嵌。
package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ResumePDFHandler 在线简历 PDF handler。
type ResumePDFHandler struct {
	recruitSvc *service.RecruitService
	renderer   *service.ResumePDFRenderer
}

// NewResumePDFHandler 创建在线简历 PDF handler。
func NewResumePDFHandler(recruitSvc *service.RecruitService, renderer *service.ResumePDFRenderer) *ResumePDFHandler {
	return &ResumePDFHandler{recruitSvc: recruitSvc, renderer: renderer}
}

// RegisterResumePDFRoutes 注册在线简历 PDF 路由：
//   - 招聘者侧 /api/recruit/resumes/:id/pdf（未授权即可预览打码版）
//   - 学员侧 /api/resume/pdf（本人预览）
func RegisterResumePDFRoutes(rg *gin.RouterGroup, rd RouterDeps, recruitSvc *service.RecruitService, renderer *service.ResumePDFRenderer) {
	h := NewResumePDFHandler(recruitSvc, renderer)
	recruitG := rg.Group("/recruit", middleware.JWTAuth(rd.Session), middleware.RoleRequired("recruiter"))
	recruitG.GET("/resumes/:id/pdf", h.RecruiterResumePDF)
	studentG := rg.Group("/resume", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	studentG.GET("/pdf", h.MyResumePDF)
}

// serveResumePDF 公共 PDF 响应出口：渲染 → inline 字节流。
func (h *ResumePDFHandler) serveResumePDF(c *gin.Context, card *model.JobCard, compress bool) {
	content, err := h.renderer.RenderResumePDF(card, compress)
	if err != nil {
		response.ServerError(c, "简历 PDF 生成失败")
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "inline; filename=resume.pdf")
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/pdf", content)
}

// RecruiterResumePDF 招聘者预览在线简历 PDF GET /api/recruit/resumes/:id/pdf
// @Summary 在线简历 PDF（打码版）
// @Description 招聘者未授权即可内嵌预览学员的在线简历（姓名打码、无电话/微信、地址到市、无工作照/证书原图）；隐藏卡 404
// @Tags 招聘域-简历
// @Produce application/pdf
// @Security BearerAuth
// @Param id path int true "学员 ID"
// @Success 200 {string} binary "PDF 字节流"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "简历不存在"
// @Router /recruit/resumes/{id}/pdf [get]
func (h *ResumePDFHandler) RecruiterResumePDF(c *gin.Context) {
	idStr := c.Param("id")
	uid, err := strconv.Atoi(idStr)
	if err != nil || uid <= 0 {
		response.BadRequest(c, "学员 ID 无效")
		return
	}
	card, err := h.recruitSvc.GetRaw(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "简历不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	// 审计留痕：预览即一次查看
	h.recruitSvc.LogView(middleware.CurrentUserID(c), uid)
	h.serveResumePDF(c, card, c.Query("test_compress") != "false")
}

// MyResumePDF 学员预览自己的在线简历 GET /api/resume/pdf
// @Summary 我的在线简历 PDF
// @Description 学员查看自己的打码在线简历（与招聘者所见同一份；本人鉴权；未建简历 404）
// @Tags 学员端-简历卡
// @Produce application/pdf
// @Security BearerAuth
// @Success 200 {string} binary "PDF 字节流"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "简历不存在"
// @Router /resume/pdf [get]
func (h *ResumePDFHandler) MyResumePDF(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	card, err := h.recruitSvc.GetRawAny(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "简历不存在")
			return
		}
		response.ServerError(c, err.Error())
		return
	}
	h.serveResumePDF(c, card, c.Query("test_compress") != "false")
}
