// 本文件：站内信通知域的 HTTP 出口（ADR-0070）——/api/notifications 蓝图。
// 装配点：internal/api/routes_registry.go 调 RegisterRoutes(api, rd.Session, deps.NotificationSvc)。
package notification

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 站内信通知 handler。
type handler struct {
	svc *Service
}

// newHandler 创建站内信通知 handler。
func newHandler(svc *Service) *handler { return &handler{svc: svc} }

// RegisterRoutes 注册 /api/notifications 蓝图（登录用户站内信）。
// 四条端点都只要求登录、不挂能力守卫：站内信按收件人鉴权（任何已登录角色都可能收到），
// 不是资源域能力——豁免理由登记在 internal/api/authz_coverage_lock_test.go。
//
// 收件人身份按 JWT role 分派（ADR-0095）：学员查 user_id、讲师查 tutor_id；
// admin / recruiter 没有信箱，得 403（不是空列表）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/notifications", middleware.JWTAuth(session))

	// GET /api/notifications?page=&page_size= 分页查询通知（含未读数）
	g.GET("", h.List)
	// GET /api/notifications/unread-count 未读数
	g.GET("/unread-count", h.UnreadCount)
	// POST /api/notifications/:id/read 单条标记已读
	g.POST("/:id/read", h.MarkRead)
	// POST /api/notifications/read-all 全部标记已读
	g.POST("/read-all", h.MarkAllRead)
}

// currentRecipient 由当前登录态构造收件人（role→列的唯一入口在 RecipientForRole）。
// admin / recruiter 返回 ErrNoMailbox ⇒ handler 映射 403。
func currentRecipient(c *gin.Context) (Recipient, error) {
	return RecipientForRole(middleware.CurrentRole(c), middleware.CurrentUserID(c))
}

// notificationListReq 通知列表请求（收件人 + 分页）。
type notificationListReq struct {
	Recipient Recipient
	Page      int
	PageSize  int
}

// List 通知列表
// @Summary 通知列表
// @Description 分页查询通知（含未读数）。收件人按登录角色分派：学员查学员信箱、讲师查讲师信箱；admin/recruiter 无信箱返回 403。
// @Tags 通知
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Success 200 {object} response.R{data=NotificationListPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "该角色没有站内信箱"
// @Router /notifications [get]
func (h *handler) List(c *gin.Context) {
	httpx.Endpoint[notificationListReq, NotificationListPageResult]{
		Parse: func(c *gin.Context) (*notificationListReq, error) {
			r, err := currentRecipient(c)
			if err != nil {
				return nil, err
			}
			return &notificationListReq{
				Recipient: r,
				Page:      httpx.QueryIntDefault(c, "page", 1),
				PageSize:  httpx.QueryIntDefault(c, "page_size", 10),
			}, nil
		},
		Invoke: func(ctx context.Context, req *notificationListReq) (*NotificationListPageResult, error) {
			return h.svc.List(ctx, req.Recipient, req.Page, req.PageSize)
		},
		ErrStatus: httpx.ErrStatusAllPrefix(http.StatusInternalServerError, "查询失败: "),
	}.WithSentinel(ErrNoMailbox, http.StatusForbidden).Handle(c)
}

// UnreadCount 未读通知数
// @Summary 未读通知数
// @Description 查询未读通知数量。收件人按登录角色分派；admin/recruiter 无信箱返回 403。
// @Tags 通知
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=NotificationUnreadCountDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "该角色没有站内信箱"
// @Router /notifications/unread-count [get]
func (h *handler) UnreadCount(c *gin.Context) {
	httpx.Endpoint[notificationRecipientReq, int64]{
		Parse: func(c *gin.Context) (*notificationRecipientReq, error) {
			r, err := currentRecipient(c)
			if err != nil {
				return nil, err
			}
			return &notificationRecipientReq{Recipient: r}, nil
		},
		Invoke: func(ctx context.Context, req *notificationRecipientReq) (*int64, error) {
			count, err := h.svc.UnreadCount(ctx, req.Recipient)
			if err != nil {
				return nil, err
			}
			return &count, nil
		},
		ErrStatus: httpx.ErrStatusAllPrefix(http.StatusInternalServerError, "查询失败: "),
		Render: func(c *gin.Context, _ *notificationRecipientReq, resp *int64) {
			response.Success(c, NotificationUnreadCountDTO{Count: *resp})
		},
	}.WithSentinel(ErrNoMailbox, http.StatusForbidden).Handle(c)
}

// markReadReq 单条标记已读请求（路径 ID + 收件人）。
type markReadReq struct {
	Recipient Recipient
	ID        int64
}

// notificationRecipientReq 仅带收件人的请求（未读数 / 全部已读）。
type notificationRecipientReq struct {
	Recipient Recipient
}

// MarkRead 标记单条已读
// @Summary 标记单条已读
// @Description 标记指定通知为已读。admin/recruiter 无信箱返回 403。
// @Tags 通知
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "该角色没有站内信箱"
// @Router /notifications/{id}/read [post]
func (h *handler) MarkRead(c *gin.Context) {
	httpx.Endpoint[markReadReq, struct{}]{
		Parse: func(c *gin.Context) (*markReadReq, error) {
			id, err := httpx.PathInt64(c, "id", "通知ID无效")
			if err != nil {
				return nil, err
			}
			r, err := currentRecipient(c)
			if err != nil {
				return nil, err
			}
			return &markReadReq{Recipient: r, ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *markReadReq) (*struct{}, error) {
			if err := h.svc.MarkRead(ctx, req.Recipient, req.ID); err != nil {
				return nil, err
			}
			return nil, nil
		},
	}.WithSuccess(httpx.OkMsgNoData("已标记为已读"), http.StatusBadRequest).
		WithSentinel(ErrNoMailbox, http.StatusForbidden).Handle(c)
}

// MarkAllRead 全部标记已读
// @Summary 全部标记已读
// @Description 全部通知标记为已读。admin/recruiter 无信箱返回 403。
// @Tags 通知
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "该角色没有站内信箱"
// @Router /notifications/read-all [post]
func (h *handler) MarkAllRead(c *gin.Context) {
	httpx.Endpoint[notificationRecipientReq, struct{}]{
		Parse: func(c *gin.Context) (*notificationRecipientReq, error) {
			r, err := currentRecipient(c)
			if err != nil {
				return nil, err
			}
			return &notificationRecipientReq{Recipient: r}, nil
		},
		Invoke: func(ctx context.Context, req *notificationRecipientReq) (*struct{}, error) {
			if err := h.svc.MarkAllRead(ctx, req.Recipient); err != nil {
				return nil, err
			}
			return nil, nil
		},
		ErrStatus: httpx.ErrStatusAllPrefix(http.StatusInternalServerError, "操作失败: "),
		Render: func(c *gin.Context, _ *notificationRecipientReq, _ *struct{}) {
			response.SuccessWithMsg(c, "已全部标记为已读", nil)
		},
	}.WithSentinel(ErrNoMailbox, http.StatusForbidden).Handle(c)
}
