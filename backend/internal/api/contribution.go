// Package api 实现 HTTP handlers。
// 本文件：资料投稿（contribution）蓝图（#517 / ADR-0026）。
package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// contributionReq 携带当前用户（session）。

// ContributionHandler 投稿 handler。
type ContributionHandler struct {
	svc *service.ContributionService
}

// NewContributionHandler 构造投稿 handler。
func NewContributionHandler(svc *service.ContributionService) *ContributionHandler {
	return &ContributionHandler{svc: svc}
}

// RegisterContributionRoutes 注册 /api/contributions 蓝图（学员）+ /api/admin/contributions（审核）。
// 投稿审核接口 V1 即挂 tutor+admin 双角色鉴权（#517：后端鉴权先行，前端仅管理端有 UI，讲师端二期）。
func RegisterContributionRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ContributionService) {
	h := NewContributionHandler(svc)

	// ===== 学员端（hrwai_user）=====
	g := rg.Group("/contributions", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))
	// POST /api/contributions/upload-file 先传文件（暂存位）拿 URL
	g.POST("/upload-file", h.UploadFile)
	// POST /api/contributions 创建投稿
	g.POST("", h.Create)
	// GET /api/contributions?credential_id=&sort=latest|hot&page=&page_size= 公开广场（仅 approved）
	g.GET("", h.ListPublic)
	// GET /api/contributions/mine 我的投稿（全部状态）
	g.GET("/mine", h.ListMine)
	// GET /api/contributions/:id 详情（公开 approved 或作者本人）
	g.GET("/:id", h.GetDetail)
	// POST /api/contributions/:id/download 下载（计数幂等，作者不计）
	g.POST("/:id/download", h.Download)
	// DELETE /api/contributions/:id 撤回 pending
	g.DELETE("/:id", h.Withdraw)
	// POST /api/contributions/:id/report 举报（已上架）
	g.POST("/:id/report", h.Report)

	// ===== 管理端审核队列（admin + tutor；讲师前端二期）=====
	adminG := rg.Group("/admin/contributions", middleware.JWTAuth(rd.Session), middleware.RoleRequired("tutor", "admin"))
	// GET /api/admin/contributions/pending 待审核队列
	adminG.GET("/pending", h.ListPending)
	// POST /api/admin/contributions/:id/approve 通过（发分）
	adminG.POST("/:id/approve", h.Approve)
	// POST /api/admin/contributions/:id/reject 驳回（必填原因）
	adminG.POST("/:id/reject", h.Reject)
	// POST /api/admin/contributions/:id/archive 下架（追回积分）
	adminG.POST("/:id/archive", h.Archive)
	// GET /api/admin/contributions/reports 举报队列
	adminG.GET("/reports", h.ListReports)
	// POST /api/admin/contributions/reports/:id/handle 处置举报
	adminG.POST("/reports/:id/handle", h.HandleReport)
}

// currentUserID 从上下文取当前学员 id。
func currentUserID(c *gin.Context) (int, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	userID, _ := uid.(int)
	if userID <= 0 {
		return 0, errors.New("未认证")
	}
	return userID, nil
}

// UploadFile 上传投稿暂存文件 POST /api/contributions/upload-file
func (h *ContributionHandler) UploadFile(c *gin.Context) {
	Endpoint[struct{}, service.ContributionFileDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionFileDTO, error) {
			file, err := c.FormFile("file")
			if err != nil {
				return nil, badRequest("未找到上传文件")
			}
			return h.svc.UploadFile(ctx, file)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionFileDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// createContributionReq 创建投稿请求体。
type createContributionReq struct {
	CredentialID int    `json:"credential_id"`
	Title        string `json:"title"`
	Intro        string `json:"intro"`
	IsAnonymous  bool   `json:"is_anonymous"`
	Files        []struct {
		FileURL     string `json:"file_url"`
		FileName    string `json:"file_name"`
		FileSize    int64  `json:"file_size"`
		ContentType string `json:"content_type"`
	} `json:"files"`
}

// Create 创建投稿 POST /api/contributions
func (h *ContributionHandler) Create(c *gin.Context) {
	Endpoint[createContributionReq, service.ContributionItemDTO]{
		Parse: func(c *gin.Context) (*createContributionReq, error) {
			return bindJSON[createContributionReq](c)
		},
		Invoke: func(ctx context.Context, req *createContributionReq) (*service.ContributionItemDTO, error) {
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			files := make([]service.ContributionFileDTO, 0, len(req.Files))
			for _, f := range req.Files {
				files = append(files, service.ContributionFileDTO{
					FileURL: f.FileURL, FileName: f.FileName, FileSize: f.FileSize, ContentType: f.ContentType,
				})
			}
			return h.svc.Create(service.CreateContributionInput{
				UserID: userID, CredentialID: req.CredentialID, Title: req.Title, Intro: req.Intro,
				IsAnonymous: req.IsAnonymous, Files: files,
			})
		},
		Render: func(c *gin.Context, _ *createContributionReq, resp *service.ContributionItemDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// listPublicReq 公开广场列表查询。
type listPublicReq struct {
	CredentialID int    `json:"credential_id"`
	Sort         string `json:"sort"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

// ListPublic 公开广场 GET /api/contributions?credential_id=&sort=&page=&page_size=
func (h *ContributionHandler) ListPublic(c *gin.Context) {
	Endpoint[listPublicReq, service.ContributionPageResult]{
		Parse: func(c *gin.Context) (*listPublicReq, error) {
			credID, err := strconv.Atoi(c.Query("credential_id"))
			if err != nil || credID <= 0 {
				return nil, badRequest("credential_id 必填")
			}
			return &listPublicReq{
				CredentialID: credID,
				Sort:         c.Query("sort"),
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 20),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listPublicReq) (*service.ContributionPageResult, error) {
			return h.svc.ListPublic(service.ListPublicInput{
				CredentialID: req.CredentialID, Sort: req.Sort, Page: req.Page, PageSize: req.PageSize,
			})
		},
		Render: func(c *gin.Context, _ *listPublicReq, resp *service.ContributionPageResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListMine 我的投稿 GET /api/contributions/mine
func (h *ContributionHandler) ListMine(c *gin.Context) {
	Endpoint[struct{}, service.ContributionPageResult]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionPageResult, error) {
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.ListMine(userID, atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20))
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionPageResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetDetail 投稿详情 GET /api/contributions/:id
func (h *ContributionHandler) GetDetail(c *gin.Context) {
	Endpoint[struct{}, service.ContributionItemDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionItemDTO, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, _ := currentUserID(c)
			return h.svc.GetDetail(id, userID)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionItemDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Download 下载投稿 POST /api/contributions/:id/download
func (h *ContributionHandler) Download(c *gin.Context) {
	Endpoint[struct{}, service.DownloadResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.DownloadResult, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Download(userID, id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.DownloadResult, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// Withdraw 撤回投稿 DELETE /api/contributions/:id
func (h *ContributionHandler) Withdraw(c *gin.Context) {
	Endpoint[struct{}, struct{}]{
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			if err := h.svc.Withdraw(userID, id); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *struct{}, _ *struct{}, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.SuccessWithMsg(c, "已撤回", nil)
		},
	}.Handle(c)
}

// reportContributionReq 举报请求体。
type reportContributionReq struct {
	Reason string `json:"reason"`
}

// Report 举报 POST /api/contributions/:id/report
func (h *ContributionHandler) Report(c *gin.Context) {
	Endpoint[reportContributionReq, struct{}]{
		Parse: func(c *gin.Context) (*reportContributionReq, error) {
			return bindJSON[reportContributionReq](c)
		},
		Invoke: func(ctx context.Context, req *reportContributionReq) (*struct{}, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			if err := h.svc.Report(userID, id, req.Reason); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *reportContributionReq, _ *struct{}, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.SuccessWithMsg(c, "举报已提交", nil)
		},
	}.Handle(c)
}

// ListPending 审核队列 GET /api/admin/contributions/pending
func (h *ContributionHandler) ListPending(c *gin.Context) {
	Endpoint[struct{}, service.ContributionPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionPageResult, error) {
			return h.svc.ListPending(atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20))
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionPageResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// reviewerID 从上下文取审核者 id（admin.id / tutor.tutor_id）。
func reviewerID(c *gin.Context) (int, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	userID, _ := uid.(int)
	if userID <= 0 {
		return 0, errors.New("未认证")
	}
	return userID, nil
}

// Approve 通过投稿 POST /api/admin/contributions/:id/approve
func (h *ContributionHandler) Approve(c *gin.Context) {
	Endpoint[struct{}, service.ContributionItemDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionItemDTO, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := reviewerID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Approve(rid, id)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionItemDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.SuccessWithMsg(c, "已通过", resp)
		},
	}.Handle(c)
}

// contributionRejectReq 驳回请求体（必填原因）。
type contributionRejectReq struct {
	Reason string `json:"reason"`
}

// Reject 驳回 POST /api/admin/contributions/:id/reject
func (h *ContributionHandler) Reject(c *gin.Context) {
	Endpoint[contributionRejectReq, service.ContributionItemDTO]{
		Parse: func(c *gin.Context) (*contributionRejectReq, error) { return bindJSON[contributionRejectReq](c) },
		Invoke: func(ctx context.Context, req *contributionRejectReq) (*service.ContributionItemDTO, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := reviewerID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Reject(rid, id, req.Reason)
		},
		Render: func(c *gin.Context, _ *contributionRejectReq, resp *service.ContributionItemDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.SuccessWithMsg(c, "已驳回", resp)
		},
	}.Handle(c)
}

// Archive 下架投稿 POST /api/admin/contributions/:id/archive
func (h *ContributionHandler) Archive(c *gin.Context) {
	Endpoint[contributionRejectReq, service.ContributionItemDTO]{
		Parse: func(c *gin.Context) (*contributionRejectReq, error) { return bindJSON[contributionRejectReq](c) },
		Invoke: func(ctx context.Context, req *contributionRejectReq) (*service.ContributionItemDTO, error) {
			id, err := pathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := reviewerID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Archive(rid, id, req.Reason)
		},
		Render: func(c *gin.Context, _ *contributionRejectReq, resp *service.ContributionItemDTO, err error) {
			if err != nil {
				renderContributionError(c, err)
				return
			}
			response.SuccessWithMsg(c, "已下架", resp)
		},
	}.Handle(c)
}

// ListReports 举报队列 GET /api/admin/contributions/reports?status=
func (h *ContributionHandler) ListReports(c *gin.Context) {
	Endpoint[struct{}, service.ContributionReportPageResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.ContributionReportPageResult, error) {
			var status *int
			if s := c.Query("status"); s != "" {
				// status 0 待处理 / 1 已处理；queryIntPtr 对缺失返回 nil（全部）
				if v, ok := requiredPositiveID(s); ok {
					status = &v
				} else if s == "0" {
					z := 0
					status = &z
				}
			}
			return h.svc.ListReports(atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("page_size"), 20), status)
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.ContributionReportPageResult, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// handleReportReq 处置举报请求体。
type handleReportReq struct {
	Action string `json:"action"` // archive=下架被举报投稿 / dismiss=驳回举报
}

// HandleReport 处置举报 POST /api/admin/contributions/reports/:id/handle
func (h *ContributionHandler) HandleReport(c *gin.Context) {
	Endpoint[handleReportReq, struct{}]{
		Parse: func(c *gin.Context) (*handleReportReq, error) { return bindJSON[handleReportReq](c) },
		Invoke: func(ctx context.Context, req *handleReportReq) (*struct{}, error) {
			id, err := pathInt64(c, "id", "举报ID无效")
			if err != nil {
				return nil, err
			}
			rid, err := reviewerID(c)
			if err != nil {
				return nil, err
			}
			if err := h.svc.HandleReport(rid, id, req.Action); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *handleReportReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已处理", nil)
		},
	}.Handle(c)
}

// renderContributionError 统一投稿错误 → HTTP 状态码映射（ADR-0024 哨兵 → 状态码）。
func renderContributionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrContributionNotFound):
		response.NotFound(c, err.Error())
	case errors.Is(err, service.ErrContributionNotOwner),
		errors.Is(err, service.ErrContributionNotPending),
		errors.Is(err, service.ErrContributionNotApproved),
		errors.Is(err, service.ErrContributionQuotaDaily),
		errors.Is(err, service.ErrContributionQuotaPending),
		errors.Is(err, service.ErrContributionNoCredential),
		errors.Is(err, service.ErrContributionTitleRequired),
		errors.Is(err, service.ErrContributionIntroRequired),
		errors.Is(err, service.ErrContributionFilesRequired),
		errors.Is(err, service.ErrContributionFilesTooMany),
		errors.Is(err, service.ErrContributionFileTooLarge),
		errors.Is(err, service.ErrContributionTotalTooLarge),
		errors.Is(err, service.ErrContributionFileInvalid),
		errors.Is(err, service.ErrContributionRejectReason),
		errors.Is(err, service.ErrContributionArchiveReason),
		errors.Is(err, service.ErrContributionInvalidReportReason):
		response.BadRequest(c, err.Error())
	default:
		var pe *ParseError
		if asParseError(err, &pe) {
			renderStatus(c, pe.Status, pe.Message)
			return
		}
		response.ServerError(c, err.Error())
	}
}
