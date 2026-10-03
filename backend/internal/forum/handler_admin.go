// Package forum 论坛域的管理端 HTTP 出口（/api/admin/forum 蓝图，10 条；ADR-0050 决策 3、ADR-0070 域包形态）。
//
// 本域两条蓝图分居两文件，故注册函数带 Admin 后缀；handler 类型与 ErrStatus 表（handler.go）共用。
package forum

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// RegisterAdminRoutes 注册 /api/admin/forum 蓝图（需登录 + 论坛治理能力 CapForumModerate；10 条）。
//
// svc 不可省：AdminListTopics / AdminGetTopic 是纯委托（h.ListTopics / h.GetTopic），
// 真正读列表与详情的是 svc —— 只传 modSvc 会让这两条管理端路由 panic 成 500，而公开端点照样绿。
// imageSvc 传 nil 是安全的：管理端 10 条路由里没有上传面（唯一用 imageSvc 的是公开的 Upload）。
func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service, modSvc *ModerationService) {
	h := newHandler(svc, modSvc, nil)

	adminG := rg.Group("/admin/forum", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapForumModerate))
	adminG.GET("/topics", h.AdminListTopics)
	adminG.GET("/topics/:id", h.AdminGetTopic)
	adminG.DELETE("/topics/:id", h.AdminDeleteTopic)
	adminG.DELETE("/replies/:id", h.AdminDeleteReply)
	// 精选位（#742）：全类别可精/可撤；首次加精同事务给帖主 featured_bonus +30（幂等）
	adminG.POST("/topics/:id/featured", h.AdminFeatureTopic)
	adminG.DELETE("/topics/:id/featured", h.AdminUnfeatureTopic)
	adminG.POST("/topics/:id/experience", h.AdminDesignateExperience)
	adminG.DELETE("/topics/:id/experience", h.AdminRevokeExperience)
	// 举报管理（ADR-0018）：status query 0 待处理 / 1 已处理，缺省全部
	adminG.GET("/reports", h.ListReports)
	adminG.PUT("/reports/:id", h.HandleReport)
}

// AdminListTopics 管理员帖子列表 GET /api/admin/forum/topics?scope=&category=&page=
//
// 与 ListTopics 同一读面（管理端沿用学员端实现，ADR-0047 §3）：注解必须留在本函数上，
// 否则 /admin/forum/topics 会从 swagger 里消失（文档面缩水，spec #962 片四）。
//
// @Summary 管理员查看帖子列表
// @Description 与学员端列表同一读面（scope/category/featured/chapter_id/keyword/sort/order 同参数）
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param scope query string false "范围 all|general|chapter"
// @Param category query string false "类别 discussion|question|experience，省略表示不过滤"
// @Param featured query string false "精选过滤 true|false，省略表示不过滤"
// @Param chapter_id query int false "章节ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(10)
// @Param keyword query string false "关键词"
// @Param sort query string false "排序 latest|hot|created"
// @Param order query string false "排序方向 asc|desc"
// @Success 200 {object} response.R{data=forum.ForumTopicPageResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/forum/topics [get]
func (h *handler) AdminListTopics(c *gin.Context) {
	h.ListTopics(c)
}

// AdminGetTopic 管理员查看帖子详情（含回复）GET /api/admin/forum/topics/:id?sort=time|hot
// AdminGetTopic 管理员查看帖子详情 GET /api/admin/forum/topics/:id
// @Summary 管理员查看帖子
// @Description 管理端只读查看帖子详情（含回复）
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDetailDTO} "详情"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在"
// @Router /admin/forum/topics/{id} [get]
func (h *handler) AdminGetTopic(c *gin.Context) {
	// 与 GetTopic 逐字重复的 Endpoint 装配已删除（ADR-0047 §3 / spec #940 片二）：
	// 两处的请求形状、404 文案与错误分支完全同源，管理端沿用同一实现即可。
	// 注解必须留在本函数上 —— swaggo 从函数注释生成 /admin/forum/topics/{id}，
	// 把函数整个删掉会让该端点从 swagger 里消失（文档面缩水）。
	h.GetTopic(c)
}

// AdminDeleteTopic 管理员删除任意主题 DELETE /api/admin/forum/topics/:id
// AdminDeleteTopic 管理员删除帖子 DELETE /api/admin/forum/topics/:id
// @Summary 管理员删除帖子
// @Description 管理端删除帖子（若曾发分则按 rollback 对冲扣减，封底 0，幂等）
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 400 {object} response.R "主题ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Router /admin/forum/topics/{id} [delete]
func (h *handler) AdminDeleteTopic(c *gin.Context) {
	httpx.Endpoint[topicIDReq, struct{}]{
		Parse: func(c *gin.Context) (*topicIDReq, error) {
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicIDReq{TopicID: topicID}, nil
		},
		Invoke: func(ctx context.Context, req *topicIDReq) (*struct{}, error) {
			if err := h.modSvc.AdminDeleteTopic(req.TopicID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *topicIDReq, _ *struct{}) {
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// AdminFeatureTopic 管理员加精帖子 POST /api/admin/forum/topics/:id/featured
// @Summary 管理员加精帖子
// @Description 全类别可精；首次加精同事务给帖主 featured_bonus +30（每帖幂等一次，取消重精不重复发分）；状态已一致时幂等短路
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "加精成功"
// @Failure 400 {object} response.R "主题ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Failure 403 {object} response.R "需要管理员角色"
// @Router /admin/forum/topics/{id}/featured [post]
func (h *handler) AdminFeatureTopic(c *gin.Context) {
	h.handleSetFeatured(c, true)
}

// AdminUnfeatureTopic 管理员取消精选 DELETE /api/admin/forum/topics/:id/featured
// @Summary 管理员取消精选
// @Description 只改状态，已发放的加精奖励不回滚；状态已一致时幂等短路
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "已取消精选"
// @Failure 400 {object} response.R "主题ID无效 / 经验帖须先取消认定"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Failure 403 {object} response.R "需要管理员角色"
// @Router /admin/forum/topics/{id}/featured [delete]
func (h *handler) AdminUnfeatureTopic(c *gin.Context) {
	h.handleSetFeatured(c, false)
}

// AdminDesignateExperience 管理员认定备考经验 POST /api/admin/forum/topics/:id/experience
// @Summary 管理员认定备考经验
// @Description 一个认定动作同时置 is_experience 与 is_featured（经验蕴含精选）；首次认定同事务给帖主 +30（与加精共用一笔，每帖幂等一次）；状态已一致时幂等短路
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "认定成功"
// @Failure 400 {object} response.R "主题ID无效 / 已采纳帖须先取消采纳"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Failure 403 {object} response.R "需要管理员角色"
// @Router /admin/forum/topics/{id}/experience [post]
func (h *handler) AdminDesignateExperience(c *gin.Context) {
	h.handleExperience(c, true)
}

// AdminRevokeExperience 管理员取消经验认定 DELETE /api/admin/forum/topics/:id/experience
// @Summary 管理员取消经验认定
// @Description 只撤经验归类，保留精选位；已发放的认定奖励不回滚；状态已一致时幂等短路
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "主题 ID"
// @Success 200 {object} response.R{data=forum.ForumTopicDTO} "已取消经验认定"
// @Failure 400 {object} response.R "主题ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "主题不存在（票5 存在性档）"
// @Failure 403 {object} response.R "需要管理员角色"
// @Router /admin/forum/topics/{id}/experience [delete]
func (h *handler) AdminRevokeExperience(c *gin.Context) {
	h.handleExperience(c, false)
}

// handleExperience 认定/取消经验共用管线（ADR-0040）：权限由路由组的 CapabilityRequired(authz.CapForumModerate) 收口。
func (h *handler) handleExperience(c *gin.Context, designate bool) {
	httpx.Endpoint[topicIDReq, ForumTopicDTO]{
		Parse: func(c *gin.Context) (*topicIDReq, error) {
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicIDReq{TopicID: topicID}, nil
		},
		Invoke: func(ctx context.Context, req *topicIDReq) (*ForumTopicDTO, error) {
			if designate {
				return h.modSvc.DesignateExperience(req.TopicID)
			}
			return h.modSvc.RevokeExperience(req.TopicID)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *topicIDReq, resp *ForumTopicDTO) {
			if designate {
				response.SuccessWithMsg(c, "已认定为备考经验", resp)
			} else {
				response.SuccessWithMsg(c, "已取消经验认定", resp)
			}
		},
	}.Handle(c)
}

// handleSetFeatured 加精/取消精选共用管线（#742）：状态迁移 + 首次加精发分在同一服务方法内。
func (h *handler) handleSetFeatured(c *gin.Context, featured bool) {
	httpx.Endpoint[topicIDReq, ForumTopicDTO]{
		Parse: func(c *gin.Context) (*topicIDReq, error) {
			topicID, err := httpx.PathInt64(c, "id", "主题ID无效")
			if err != nil {
				return nil, err
			}
			return &topicIDReq{TopicID: topicID}, nil
		},
		Invoke: func(ctx context.Context, req *topicIDReq) (*ForumTopicDTO, error) {
			return h.modSvc.SetFeatured(req.TopicID, featured)
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *topicIDReq, resp *ForumTopicDTO) {
			if featured {
				response.SuccessWithMsg(c, "加精成功", resp)
			} else {
				response.SuccessWithMsg(c, "已取消精选", resp)
			}
		},
	}.Handle(c)
}

// AdminDeleteReply 管理员删除任意回复 DELETE /api/admin/forum/replies/:id
// AdminDeleteReply 管理员删除回复 DELETE /api/admin/forum/replies/:id
// @Summary 管理员删除回复
// @Description 管理端删除任意回复（若为被采纳回答则按 rollback 对冲回收，封底 0，幂等）
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param id path int true "回复 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 400 {object} response.R "回复ID无效"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "回复不存在（票5 存在性档）"
// @Router /admin/forum/replies/{id} [delete]
func (h *handler) AdminDeleteReply(c *gin.Context) {
	httpx.Endpoint[replyIDReq, struct{}]{
		Parse: func(c *gin.Context) (*replyIDReq, error) {
			replyID, err := httpx.PathInt64(c, "id", "回复ID无效")
			if err != nil {
				return nil, err
			}
			return &replyIDReq{ReplyID: replyID}, nil
		},
		Invoke: func(ctx context.Context, req *replyIDReq) (*struct{}, error) {
			if err := h.modSvc.AdminDeleteReply(req.ReplyID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *replyIDReq, _ *struct{}) {
			response.SuccessWithMsg(c, "已删除", nil)
		},
	}.Handle(c)
}

// ListReports 管理端举报列表 GET /api/admin/forum/reports?status=&page=&page_size=
// ListReports 管理端举报列表（GET /api/admin/forum/reports?status=&page=&page_size=）
// @Summary 管理端举报列表
// @Description 分页查询论坛举报（status 0 待处理 / 1 已处理，省略表示全部）
// @Tags 管理端-论坛
// @Produce json
// @Security BearerAuth
// @Param status query int false "状态 0 待处理 / 1 已处理，省略表示全部"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=forum.ForumReportPageResult} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/forum/reports [get]
func (h *handler) ListReports(c *gin.Context) {
	var status *int16
	if c.Query("status") != "" {
		v := int16(httpx.QueryIntDefault(c, "status", -1))
		if v != 0 && v != 1 {
			response.BadRequest(c, "status 仅支持 0（待处理）/ 1（已处理）")
			return
		}
		status = &v
	}
	resp, err := h.modSvc.ListReports(httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20), status)
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.Success(c, resp)
}

// HandleReport 处理举报 PUT /api/admin/forum/reports/:id
// @Summary 处理论坛举报
// @Description 管理端处理论坛举报（status: 1 标记已处理）
// @Tags 管理端-论坛
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "举报 ID"
// @Param body body object true "处理状态 {status: int16}"
// @Success 200 {object} response.R "已更新"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "举报不存在（票5 存在性档）"
// @Router /admin/forum/reports/{id} [put]
func (h *handler) HandleReport(c *gin.Context) {
	id, err := httpx.PathInt64(c, "id", "举报 ID 无效")
	if err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	var body struct {
		Status int16 `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.modSvc.HandleReport(id, body.Status); err != nil {
		ErrStatus.RenderError(c, err)
		return
	}
	response.SuccessWithMsg(c, "举报状态已更新", nil)
}
