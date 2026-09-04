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
// @Summary 上传投稿文件（暂存）
// @Description 先传后交：逐个文件上传到暂存位，返回 URL 与元数据，随投稿表单提交时引用。扩展名白名单 pdf/doc/docx/ppt/pptx/xls/xlsx/zip/mp4，单文件 ≤20MB
// @Tags 学员端-投稿
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "投稿文件"
// @Success 200 {object} response.R{data=service.ContributionFileDTO} "success"
// @Failure 400 {object} response.R "格式/大小不合规"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions/upload-file [post]
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
// @Summary 创建投稿（pending）
// @Description 学员提交资料投稿（1–5 个文件，合计 ≤50MB，必挂当前证件）。资格：仅学员且已选证件；配额：日 ≤3 份、pending 积压 ≤5 份。未过审不产生积分
// @Tags 学员端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createContributionReq true "投稿表单"
// @Success 200 {object} response.R{data=service.ContributionItemDTO} "success"
// @Failure 400 {object} response.R "校验失败/配额已满/未选证件"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions [post]
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

// ListPublic 公开广场 GET /api/contributions
// @Summary 投稿公开广场列表
// @Description 仅 approved 投稿，按目标证件过滤，sort=latest|hot（hot 走下载量降序）。分页
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param credential_id query int true "目标证件ID"
// @Param sort query string false "排序 latest|hot" default(latest)
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.ContributionPageResult} "success"
// @Failure 400 {object} response.R "credential_id 必填"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions [get]
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
// @Summary 我的投稿列表（全部状态）
// @Description 作者本人视角，含 pending/rejected/archived/withdrawn 及驳回/下架原因
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.ContributionPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions/mine [get]
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
// @Summary 投稿详情（含文件清单）
// @Description 公开仅 approved 可看；作者本人可看全部状态（含驳回原因）。匿名投稿作者显示「匿名学员」
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=service.ContributionItemDTO} "success"
// @Failure 404 {object} response.R "不存在或非公开"
// @Router /contributions/{id} [get]
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
// @Summary 下载投稿（计数）
// @Description 每人每稿终身计一次（唯一约束幂等），作者本人不计；跨 10/50/200 档当场直记达阶奖励。返回是否新增计数与本次达阶分值
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=service.DownloadResult} "success"
// @Failure 400 {object} response.R "非已上架状态"
// @Router /contributions/{id}/download [post]
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
// @Summary 撤回待审投稿
// @Description 仅作者本人、仅 pending 可撤回（withdrawn，未发分故无需回滚）
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R "已撤回"
// @Failure 400 {object} response.R "非本人或非 pending"
// @Router /contributions/{id} [delete]
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

// Report 举报投稿 POST /api/contributions/:id/report
// @Summary 举报已上架投稿
// @Description 四理由：piracy/content_error/violation/stale；同一学员对同一投稿唯一，重复举报合并
// @Tags 学员端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Param body body reportContributionReq true "举报理由"
// @Success 200 {object} response.R "举报已提交"
// @Failure 400 {object} response.R "理由非法或非已上架"
// @Router /contributions/{id}/report [post]
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

// ListPending 待审核队列 GET /api/admin/contributions/pending
// @Summary 待审核投稿队列
// @Description 管理端/讲师端（tutor+admin 鉴权）；V1 仅管理端有 UI
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.ContributionPageResult} "success"
// @Router /admin/contributions/pending [get]
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
// @Summary 审核通过（发分 +50）
// @Description pending → approved，直记 +50 积分（幂等占坑防双发）+ 站内信，同事务
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=service.ContributionItemDTO} "已通过"
// @Failure 400 {object} response.R "非 pending"
// @Router /admin/contributions/{id}/approve [post]
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

// Reject 驳回投稿 POST /api/admin/contributions/:id/reject
// @Summary 驳回投稿
// @Description pending → rejected，原因必填（送达作者站内信）。不发分。重提=新建投稿
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Param body body contributionRejectReq true "驳回原因"
// @Success 200 {object} response.R{data=service.ContributionItemDTO} "已驳回"
// @Failure 400 {object} response.R "原因必填或非 pending"
// @Router /admin/contributions/{id}/reject [post]
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
// @Summary 下架投稿（追回积分）
// @Description approved → archived，必填原因；追回该稿累计投稿分（过审+达阶，rollback 对冲封底 0）+ 站内信
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Param body body contributionRejectReq true "下架原因"
// @Success 200 {object} response.R{data=service.ContributionItemDTO} "已下架"
// @Failure 400 {object} response.R "原因必填或非 approved"
// @Router /admin/contributions/{id}/archive [post]
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

// ListReports 举报队列 GET /api/admin/contributions/reports
// @Summary 投稿举报队列
// @Description status 0 待处理 / 1 已处理，缺省全部
// @Tags 管理端-投稿
// @Produce json
// @Security BearerAuth
// @Param status query int false "状态 0|1"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=service.ContributionReportPageResult} "success"
// @Router /admin/contributions/reports [get]
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
// @Summary 处置投稿举报
// @Description action=archive 下架被举报投稿（追回积分）并标记处理；action=dismiss 驳回举报
// @Tags 管理端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "举报ID"
// @Param body body handleReportReq true "处置动作"
// @Success 200 {object} response.R "已处理"
// @Router /admin/contributions/reports/{id}/handle [post]
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
