// 本文件：招聘域 HTTP 出口之二 —— 招聘者预览学员的打码在线简历 PDF
// （GET /api/recruit/resumes/:id/pdf，spec #484 / 子票 #485：未授权即可内嵌预览，隐藏卡 404）。
//
// 为什么这条出口在招聘域而不是简历域：它要 RecruitService（GetRaw 取卡 + LogView 审计留痕），
// 而简历域又要被招聘域单向依赖（脱敏投影 resume.Desensitize）—— 出口留在简历域会把那条边变成双向。
// 渲染与 inline 响应写出的单点仍在 internal/resume（resume.PDFRenderer / ServePDF / PDFCompress）。
package recruit

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/resume"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// pdfHandler 招聘者侧在线简历 PDF handler。
type pdfHandler struct {
	svc      *Service
	renderer *resume.PDFRenderer
}

// newPDFHandler 创建招聘者侧在线简历 PDF handler。
func newPDFHandler(svc *Service, renderer *resume.PDFRenderer) *pdfHandler {
	return &pdfHandler{svc: svc, renderer: renderer}
}

// RegisterPDFRoutes 注册招聘者侧在线简历 PDF 路由 GET /api/recruit/resumes/:id/pdf。
func RegisterPDFRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, renderer *resume.PDFRenderer) {
	h := newPDFHandler(svc, renderer)
	g := rg.Group("/recruit", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapResumePDFView))
	g.GET("/resumes/:id/pdf", h.RecruiterResumePDF)
}

// RecruiterResumePDF 招聘者预览在线简历 PDF GET /api/recruit/resumes/:id/pdf
// @Summary 在线简历 PDF（打码版）
// @Description 招聘者未授权即可内嵌预览学员的在线简历（姓名打码、无电话/微信、地址到市、无工作照/证书原图）；隐藏卡 404
// @Tags 招聘域-简历
// @Produce application/pdf
// @Security BearerAuth
// @Param id path int true "学员 ID"
// @Success 200 {string} binary "PDF 字节流（非统一信封：二进制流无 data）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "简历不存在"
// @Router /recruit/resumes/{id}/pdf [get]
func (h *pdfHandler) RecruiterResumePDF(c *gin.Context) {
	uid, err := httpx.PathInt(c, "id", "学员 ID 无效")
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	card, err := h.svc.GetRaw(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "简历不存在")
			return
		}
		response.ServerErrorCause(c, "", err)
		return
	}
	// 审计留痕：预览即一次查看
	h.svc.LogView(middleware.CurrentUserID(c), uid)
	resume.ServePDF(c, h.renderer, card, resume.PDFCompress(c))
}
