// 本文件：反馈小域的 HTTP 出口（ADR-0089 / ADR-0070）——两条蓝图四条路由。
//   - 学员面 /api/feedback：提交 + 本人历史（能力点 feedback.submit）
//   - 管理面 /api/admin/feedback：列表（可按状态筛）+ 标记已处理（能力点 feedback.manage）
package feedback

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// ErrStatus 反馈域哨兵→状态码表（ADR-0024）：不存在 → 404，校验类 → 400（fallback），
// 存储未配置 → 500（它是装配故障，不是用户输入问题）。
var ErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrFeedbackNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrFeedbackStorageUnconfigured, Status: http.StatusInternalServerError},
	},
	Fallback: http.StatusBadRequest,
}

// handler 反馈 handler。
type handler struct {
	svc *Service
}

// newHandler 创建反馈 handler。
func newHandler(svc *Service) *handler { return &handler{svc: svc} }

// RegisterRoutes 注册反馈两条蓝图。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/feedback", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapFeedbackSubmit))
	g.POST("", h.Submit)
	g.GET("", h.ListMine)

	ag := rg.Group("/admin/feedback", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapFeedbackManage))
	ag.GET("", h.AdminList)
	ag.POST("/:id/handle", h.MarkHandled)
}

// Submit 提交反馈 POST /api/feedback
// @Summary 提交反馈
// @Description 学员向平台提交一条反馈（ADR-0089）：文字 1–1000 字必填；图片 0–3 张（仅本站 images/forum/ 前缀，复用论坛图片上传通道）；联系方式可空且不校验格式
// @Tags 学员端-反馈
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body feedback.FeedbackInput true "反馈内容"
// @Success 201 {object} response.R{data=feedback.FeedbackDTO} "提交成功"
// @Failure 400 {object} response.R "内容为空/超长/图片张数或地址不合法"
// @Failure 401 {object} response.R "未认证"
// @Router /feedback [post]
func (h *handler) Submit(c *gin.Context) {
	httpx.Endpoint[FeedbackInput, FeedbackDTO]{
		Parse: func(c *gin.Context) (*FeedbackInput, error) {
			return httpx.BindJSON[FeedbackInput](c)
		},
		Invoke: func(ctx context.Context, req *FeedbackInput) (*FeedbackDTO, error) {
			return h.svc.Submit(middleware.CurrentUserID(c), req)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *FeedbackInput, resp *FeedbackDTO) {
			response.Created(c, "反馈已提交，感谢你帮助平台改进", *resp)
		},
	}.Handle(c)
}

// ListMine 我的反馈历史 GET /api/feedback
// @Summary 我的反馈历史
// @Description 学员查看本人提交过的反馈（按时间倒序，分页）；他人反馈一律不可见
// @Tags 学员端-反馈
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R{data=feedback.FeedbackListResult} "列表"
// @Failure 401 {object} response.R "未认证"
// @Router /feedback [get]
func (h *handler) ListMine(c *gin.Context) {
	httpx.Endpoint[struct{}, FeedbackListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*FeedbackListResult, error) {
			return h.svc.ListMine(middleware.CurrentUserID(c),
				httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20))
		},
		ErrStatus: ErrStatus,
	}.Handle(c)
}

// AdminList 反馈列表（管理端）GET /api/admin/feedback
// @Summary 管理端反馈列表
// @Description 管理端查看全部反馈（分页，可按状态筛：pending / handled / 空=全部）；每行带提交人 user_id 与昵称
// @Tags 管理端-反馈
// @Produce json
// @Security BearerAuth
// @Param status query string false "状态 pending|handled（省略或 all = 全部）"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {object} response.R{data=feedback.FeedbackListResult} "列表"
// @Failure 400 {object} response.R "状态筛选值无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Router /admin/feedback [get]
func (h *handler) AdminList(c *gin.Context) {
	httpx.Endpoint[struct{}, FeedbackListResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*FeedbackListResult, error) {
			return h.svc.ListAdmin(c.Query("status"),
				httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20))
		},
		ErrStatus: ErrStatus,
	}.Handle(c)
}

// MarkHandled 标记已处理（管理端）POST /api/admin/feedback/:id/handle
// @Summary 标记反馈已处理
// @Description 把一条反馈标记为已处理（**幂等**：已处理的行再点一次返回同一行、200、不改处理时间）；标记不向学员发任何消息
// @Tags 管理端-反馈
// @Produce json
// @Security BearerAuth
// @Param id path int true "反馈 ID"
// @Success 200 {object} response.R{data=feedback.FeedbackDTO} "已处理"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "反馈不存在"
// @Router /admin/feedback/{id}/handle [post]
func (h *handler) MarkHandled(c *gin.Context) {
	httpx.Endpoint[struct{}, FeedbackDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*FeedbackDTO, error) {
			id, err := httpx.PathInt64(c, "id", "反馈 ID 无效")
			if err != nil {
				return nil, err
			}
			return h.svc.MarkHandled(middleware.CurrentUserID(c), id)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *struct{}, resp *FeedbackDTO) {
			response.SuccessWithMsg(c, "已标记为已处理", *resp)
		},
	}.Handle(c)
}
