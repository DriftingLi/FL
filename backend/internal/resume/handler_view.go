// 本文件：简历域 HTTP 出口之二 —— 学员侧简历查看留痕聚合（#374，GET /api/resume/view-stats）。
// 为什么归简历域：这条出口读的是「谁看过我的简历」，主语是学员自己的简历；写入侧（招聘者预览留痕）
// 在 internal/recruit。表 recruit_resume_views 由此被两侧各持一半读写面。
package resume

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/response"
)

// RegisterViewRoutes 注册学员侧查看聚合 GET /api/resume/view-stats（#374）。
// 仅学员可访问：招聘方无法读取留痕数据（避免暴露浏览习惯）。
func RegisterViewRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newViewHandler(svc)
	g := rg.Group("/resume", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapResumeManage))
	g.GET("/view-stats", h.StudentViewStats)
}

// viewHandler 简历查看留痕聚合 handler。
type viewHandler struct {
	svc *Service
}

// newViewHandler 创建简历查看留痕聚合 handler。
func newViewHandler(svc *Service) *viewHandler {
	return &viewHandler{svc: svc}
}

// StudentViewStats 近 7 天查看过我的企业数 GET /api/resume/view-stats
// @Summary 简历查看留痕
// @Description 学员查看近 7 天查看过自己简历的企业数（按企业去重，仅聚合数，不含企业名）
// @Tags 学员端-简历卡
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=object{count=integer}} "聚合数 {count}"
// @Failure 401 {object} response.R "未认证"
// @Router /resume/view-stats [get]
func (h *viewHandler) StudentViewStats(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	cnt, err := h.svc.StudentViewStats(uid)
	if err != nil {
		response.ServerErrorCause(c, "", err)
		return
	}
	// 仅返回聚合数，不含企业名与任何身份信息
	// 注解层声明的 data 形状：object{count=integer}（见上方 @Success；本片只补注解，不动构造）。
	response.Success(c, gin.H{"count": cnt})
}
