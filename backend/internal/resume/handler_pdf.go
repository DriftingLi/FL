// 本文件：简历域 HTTP 出口之三 —— 在线简历 PDF（spec #484 / 子票 #485）的**学员侧与渲染出口单点**。
//
//   - GET /api/resume/pdf：学员预览自己的打码在线简历（本人 JWT；所见即招聘者所见）—— 本文件注册。
//   - GET /api/recruit/resumes/:id/pdf：招聘者未授权即可内嵌预览（recruiter 鉴权；隐藏卡 404）——
//     路由在 internal/recruit/handler_pdf.go（那条出口要 RecruitService），但**渲染与响应写出**仍走本文件：
//     招聘者侧只注入自己的服务 + 本包的 PDFRenderer，调 ServePDF / PDFCompress。
//
// 响应 inline PDF（Content-Type: application/pdf），前端以带鉴权的 blob 取流内嵌。
package resume

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// pdfHandler 在线简历 PDF handler（学员侧）。
type pdfHandler struct {
	svc      *Service
	renderer *PDFRenderer
}

// newPDFHandler 创建在线简历 PDF handler。
func newPDFHandler(svc *Service, renderer *PDFRenderer) *pdfHandler {
	return &pdfHandler{svc: svc, renderer: renderer}
}

// RegisterPDFRoutes 注册学员侧在线简历 PDF 路由 GET /api/resume/pdf（本人预览）。
// 招聘者侧那条（/api/recruit/resumes/:id/pdf）由 internal/recruit.RegisterPDFRoutes 注册。
func RegisterPDFRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, renderer *PDFRenderer) {
	h := newPDFHandler(svc, renderer)
	g := rg.Group("/resume", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapResumePDF))
	g.GET("/pdf", h.MyResumePDF)
}

// PDFCompress 决定是否压缩：测试模式（gin.TestMode）关闭压缩便于契约测试做字节级文本断言；
// 生产/开发默认压缩（体积小）。不暴露公共 query 钩子（Standards 审查 #485）。
//
// 导出理由：两个域的 PDF 出口共用同一份压缩判据与同一段 inline 响应写出（ServePDF），
// 就地各抄一份就是第二真源。gin 只允许出现在 handler*.go（internal/layers 判据），故它住这里。
func PDFCompress(c *gin.Context) bool {
	return gin.Mode() != gin.TestMode
}

// ServePDF 简历 PDF 的公共响应出口：渲染 → inline 字节流。
// 调用方：本包 MyResumePDF（学员侧）与 internal/recruit 的 RecruiterResumePDF（招聘者侧）。
func ServePDF(c *gin.Context, renderer *PDFRenderer, card *model.JobCard, compress bool) {
	content, err := renderer.RenderResumePDF(card, compress)
	if err != nil {
		response.ServerError(c, "简历 PDF 生成失败")
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "inline; filename=resume.pdf")
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/pdf", content)
}

// MyResumePDF 学员预览自己的在线简历 GET /api/resume/pdf
// @Summary 我的在线简历 PDF
// @Description 学员查看自己的打码在线简历（与招聘者所见同一份；本人鉴权；未建简历 404）
// @Tags 学员端-简历卡
// @Produce application/pdf
// @Security BearerAuth
// @Success 200 {string} binary "PDF 字节流（非统一信封：二进制流无 data）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "简历不存在"
// @Router /resume/pdf [get]
func (h *pdfHandler) MyResumePDF(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	card, err := h.svc.GetRawAny(uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "简历不存在")
			return
		}
		response.ServerErrorCause(c, "", err)
		return
	}
	ServePDF(c, h.renderer, card, PDFCompress(c))
}
