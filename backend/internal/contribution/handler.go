// Package contribution 资料投稿域的 HTTP 出口（公开 8 条 + 管理端 6 条，两蓝图分居 handler.go / handler_admin.go；
// #517 / ADR-0026；ADR-0070 域包形态）。本文件：/api/contributions 学员端蓝图 + 域哨兵状态码表。
package contribution

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// contributionReq 携带当前用户（session）。

// handler 投稿 handler。
type handler struct {
	svc *Service
}

// newHandler 构造投稿 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/contributions 蓝图（学员端 8 条）；管理端蓝图见 handler_admin.go 的 RegisterAdminRoutes。
// 投稿审核接口 V1 即挂 tutor+admin 双角色鉴权（#517：后端鉴权先行，前端仅管理端有 UI，讲师端二期）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, credRes middleware.CredentialResolver, svc *Service) {
	h := newHandler(svc)

	// ===== 学员端（hrwai_user）=====
	g := rg.Group("/contributions", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapContributionSubmit), middleware.CredentialScoped(credRes))
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

// ErrStatus 投稿域哨兵→状态码表（#611，ADR-0024）：不存在 → 404，
// 状态/校验/配额类 → 400；未命中（含解析错误已先行处理）走 500 默认信封。
//
// 暂存文件四校验（#1361 / ADR-0066 决策 5）的落档：
//   - 前缀不属本人 → **403**（那是别人的暂存位，属越权，不是「这个字段格式不对」）；
//   - 类型不在白名单 / 同一 URL 已被登记 → **400**（两件各有自己的哨兵与自己的句子，
//     压不成一句）。「已占用」本可表达为 409，但本仓从未使用 409、renderStatus 的单一咽喉里
//     没有那一档，域表放 409 会被静默渲染成 500 —— 同 faq.go「标识已占用」的同一处先例与同一理由；
//   - 文件在存储侧不存在 → **404**（引用的那个资源没有，语义就是 404）。
var ErrStatus = &httpx.ErrStatusTable{
	Entries: []httpx.ErrStatusEntry{
		{Sentinel: ErrContributionNotFound, Status: http.StatusNotFound},
		{Sentinel: ErrContributionFileMissing, Status: http.StatusNotFound},
		{Sentinel: ErrContributionStagedNotOwner, Status: http.StatusForbidden},
		{Sentinel: ErrContributionNotOwner, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionNotPending, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionNotApproved, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionQuotaDaily, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionQuotaPending, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionNoCredential, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionTitleRequired, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionIntroRequired, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFilesRequired, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFilesTooMany, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFileTooLarge, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionTotalTooLarge, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFileInvalid, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFileExtNotAllowed, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionFileAlreadyClaimed, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionRejectReason, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionArchiveReason, Status: http.StatusBadRequest},
		{Sentinel: ErrContributionInvalidReportReason, Status: http.StatusBadRequest},
	},
}

// UploadFile 上传投稿暂存文件 POST /api/contributions/upload-file
// @Summary 上传投稿文件（暂存）
// @Description 先传后交：逐个文件上传到**本人**暂存位 contributions/<当前用户id>/，返回 URL 与元数据，随投稿表单提交时引用。扩展名白名单 pdf/doc/docx/ppt/pptx/xls/xlsx/zip/mp4，单文件 ≤20MB（ADR-0066 决策 5：归属由路径承载）
// @Tags 学员端-投稿
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "投稿文件"
// @Success 200 {object} response.R{data=contribution.ContributionFileDTO} "success"
// @Failure 400 {object} response.R "格式/大小不合规"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions/upload-file [post]
func (h *handler) UploadFile(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionFileDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionFileDTO, error) {
			// 暂存位按用户分区 ⇒ 上传这一刻就知道「这是谁的」，Create 的归属校验才读得判据。
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			file, err := c.FormFile("file")
			if err != nil {
				return nil, httpx.BadRequest("未找到上传文件")
			}
			return h.svc.UploadFile(ctx, userID, file)
		},
		// #611：错误映射收编至 ErrStatus
		ErrStatus: ErrStatus,
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
// @Description 学员提交资料投稿（1–5 个文件，合计 ≤50MB，目标证件可选、默认当前证件；投给非当前证件的稿需切过去可见）。资格：仅学员且已选证件；配额：日 ≤3 份、pending 积压 ≤5 份。提交的每个 file_url 过四校验：前缀属本人 contributions/<id>/、扩展名在白名单内、未被任何投稿登记过、文件真实存在（ADR-0066 决策 5 / #1361）。未过审不产生积分
// @Tags 学员端-投稿
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createContributionReq true "投稿表单"
// @Success 200 {object} response.R{data=contribution.ContributionItemDTO} "success"
// @Failure 400 {object} response.R "校验失败/配额已满/未选证件/类型不在白名单/文件已被登记"
// @Failure 401 {object} response.R "未认证"
// @Failure 403 {object} response.R "暂存文件不属于本人目录"
// @Failure 404 {object} response.R "暂存文件不存在"
// @Router /contributions [post]
func (h *handler) Create(c *gin.Context) {
	httpx.Endpoint[createContributionReq, ContributionItemDTO]{
		Parse: func(c *gin.Context) (*createContributionReq, error) {
			return httpx.BindJSON[createContributionReq](c)
		},
		Invoke: func(ctx context.Context, req *createContributionReq) (*ContributionItemDTO, error) {
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			files := make([]ContributionFileDTO, 0, len(req.Files))
			for _, f := range req.Files {
				files = append(files, ContributionFileDTO{
					FileURL: f.FileURL, FileName: f.FileName, FileSize: f.FileSize, ContentType: f.ContentType,
				})
			}
			return h.svc.Create(CreateContributionInput{
				UserID: userID, CredentialID: req.CredentialID, Title: req.Title, Intro: req.Intro,
				IsAnonymous: req.IsAnonymous, Files: files,
			})
		},
		// #611：错误映射收编至 ErrStatus
		ErrStatus: ErrStatus,
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
// @Success 200 {object} response.R{data=contribution.ContributionPageResult} "success"
// @Failure 400 {object} response.R "credential_id 必填"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions [get]
func (h *handler) ListPublic(c *gin.Context) {
	httpx.Endpoint[listPublicReq, ContributionPageResult]{
		Parse: func(c *gin.Context) (*listPublicReq, error) {
			cred := middleware.CredentialIDPtr(c)
			if cred == nil {
				return nil, httpx.BadRequest("credential_id 必填（未选择当前证件）")
			}
			credID := *cred
			return &listPublicReq{
				CredentialID: credID,
				Sort:         c.Query("sort"),
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 20),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listPublicReq) (*ContributionPageResult, error) {
			return h.svc.ListPublic(ListPublicInput{
				CredentialID: req.CredentialID, Sort: req.Sort, Page: req.Page, PageSize: req.PageSize,
			})
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// ListMine 我的投稿 GET /api/contributions/mine
// @Summary 我的投稿列表（全部状态）
// @Description 作者本人视角，含 pending/rejected/archived/withdrawn 及驳回/下架原因
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Success 200 {object} response.R{data=contribution.ContributionPageResult} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /contributions/mine [get]
func (h *handler) ListMine(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionPageResult]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionPageResult, error) {
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.ListMine(userID, httpx.QueryIntDefault(c, "page", 1), httpx.QueryIntDefault(c, "page_size", 20))
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// GetDetail 投稿详情 GET /api/contributions/:id
// @Summary 投稿详情（含文件清单）
// @Description 公开仅 approved 可看；作者本人可看全部状态（含驳回原因）。匿名投稿作者显示「匿名学员」
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=contribution.ContributionItemDTO} "success"
// @Failure 404 {object} response.R "不存在或非公开"
// @Router /contributions/{id} [get]
func (h *handler) GetDetail(c *gin.Context) {
	httpx.Endpoint[struct{}, ContributionItemDTO]{
		Parse: func(c *gin.Context) (*struct{}, error) { return &struct{}{}, nil },
		Invoke: func(ctx context.Context, _ *struct{}) (*ContributionItemDTO, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, _ := currentUserID(c)
			return h.svc.GetDetail(id, userID)
		},
		// #611：错误映射收编至 ErrStatus
		ErrStatus: ErrStatus,
	}.Handle(c)
}

// Download 下载投稿 POST /api/contributions/:id/download
// @Summary 下载投稿（计数）
// @Description 每人每稿终身计一次（唯一约束幂等），作者本人不计；跨 10/50/200 档当场直记达阶奖励。返回是否新增计数与本次达阶分值
// @Tags 学员端-投稿
// @Produce json
// @Security BearerAuth
// @Param id path int true "投稿ID"
// @Success 200 {object} response.R{data=contribution.DownloadResult} "success"
// @Failure 400 {object} response.R "非已上架状态"
// @Router /contributions/{id}/download [post]
func (h *handler) Download(c *gin.Context) {
	httpx.Endpoint[struct{}, DownloadResult]{
		Invoke: func(ctx context.Context, _ *struct{}) (*DownloadResult, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
			if err != nil {
				return nil, err
			}
			userID, err := currentUserID(c)
			if err != nil {
				return nil, err
			}
			return h.svc.Download(userID, id)
		},
		// #611：错误映射收编至 ErrStatus
		ErrStatus: ErrStatus,
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
func (h *handler) Withdraw(c *gin.Context) {
	httpx.Endpoint[struct{}, struct{}]{
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
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
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *struct{}, _ *struct{}) {
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
func (h *handler) Report(c *gin.Context) {
	httpx.Endpoint[reportContributionReq, struct{}]{
		Parse: func(c *gin.Context) (*reportContributionReq, error) {
			return httpx.BindJSON[reportContributionReq](c)
		},
		Invoke: func(ctx context.Context, req *reportContributionReq) (*struct{}, error) {
			id, err := httpx.PathInt64(c, "id", "投稿ID无效")
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
		ErrStatus: ErrStatus,
		Render: func(c *gin.Context, _ *reportContributionReq, _ *struct{}) {
			response.SuccessWithMsg(c, "举报已提交", nil)
		},
	}.Handle(c)
}
