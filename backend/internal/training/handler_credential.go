// 本文件：培训域 HTTP 出口之三——目标证件 handler 与 RegisterCredentialRoutes。
// 学员端查询见 handler.go，管理端目录面见 handler_admin.go。
package training

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

// RegisterCredentialRoutes 注册培训域证件路由：
//   - /api/credentials、/api/credentials/grouped：学员端公开查询
//   - /api/me/credential：当前证件读写（需登录；hrwai_user / admin / tutor 均可查询，切换仅 hrwai_user）
//   - /api/admin/credential*：管理端 CRUD 与排序（CapCatalogManage）
func RegisterCredentialRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	// ===== 学员端查询（公开） =====
	rg.GET("/credentials", h.ListPublicCredentials)
	rg.GET("/credentials/grouped", h.ListGroupedCredentials)
	// 当前证件（需登录）
	rg.GET("/me/credential", middleware.JWTAuth(session), h.GetCurrentCredential)
	rg.PATCH("/me/credential", middleware.JWTAuth(session), h.SetCurrentCredential)

	// ===== 管理端 CRUD =====
	g := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapCatalogManage))
	g.GET("/credentials", h.ListCredentials)
	g.POST("/credential", h.CreateCredential)
	g.PUT("/credential/:id", h.UpdateCredential)
	g.PUT("/credential/:id/sort", h.SwapCredentialSort)
	g.DELETE("/credential/:id", h.DeleteCredential)
}

// ===== 目标证件 =====

// ListPublicCredentials 目标证件列表（公开，仅启用项）GET /api/credentials
// ListPublicCredentials 证件列表 GET /api/credentials
// @Summary 证件列表
// @Description 学员端公开证件列表（目标证件，仅启用项）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.CredentialListDTO} "success"
// @Router /credentials [get]
func (h *handler) ListPublicCredentials(c *gin.Context) {
	httpx.Endpoint[struct{}, []CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]CredentialDict, error) {
			result := h.svc.ListCredentials(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]CredentialDict) {
			response.Success(c, CredentialListDTO{Credentials: *resp})
		},
	}.Handle(c)
}

// ListGroupedCredentials 分组目标证件（公开）GET /api/credentials/grouped
// ListGroupedCredentials 证件分组列表 GET /api/credentials/grouped
// @Summary 证件分组列表
// @Description 学员端公开证件列表（按类别分组）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.GroupedCredentialsDTO} "success"
// @Router /credentials/grouped [get]
func (h *handler) ListGroupedCredentials(c *gin.Context) {
	httpx.Endpoint[struct{}, GroupedCredentialsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*GroupedCredentialsDTO, error) {
			result := h.svc.ListGroupedCredentials()
			return &result, nil
		},
	}.Handle(c)
}

// ListCredentials 目标证件列表（管理端，含停用）
// @Summary 证件列表（管理端）
// @Description 管理端目标证件列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.CredentialListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/credentials [get]
func (h *handler) ListCredentials(c *gin.Context) {
	httpx.Endpoint[struct{}, []CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]CredentialDict, error) {
			result := h.svc.ListCredentials(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]CredentialDict) {
			response.Success(c, CredentialListDTO{Credentials: *resp})
		},
	}.Handle(c)
}

// CreateCredential 创建目标证件
// @Summary 创建证件
// @Description 管理员创建目标证件字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.CredentialInput true "证件"
// @Success 201 {object} response.R{data=training.CredentialDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/credential [post]
func (h *handler) CreateCredential(c *gin.Context) {
	httpx.Endpoint[CredentialInput, CredentialDict]{
		Parse:  httpx.BindJSONMsgFunc[CredentialInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreateCredential),
	}.WithSuccess(httpx.Created("证件创建成功"), http.StatusBadRequest).Handle(c)
}

// UpdateCredential 更新目标证件
// @Summary 更新证件
// @Description 管理员更新目标证件字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "证件ID"
// @Param body body training.CredentialInput true "证件"
// @Success 200 {object} response.R{data=training.CredentialDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证件不存在"
// @Router /admin/credential/{id} [put]
func (h *handler) UpdateCredential(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[CredentialInput], CredentialDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[CredentialInput], error) {
			id, err := httpx.PathInt(c, "id", "证件ID无效")
			if err != nil {
				return nil, err
			}
			var in CredentialInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &catalogUpdateReq[CredentialInput]{ID: id, In: in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[CredentialInput]) (CredentialDict, error) {
			return h.svc.UpdateCredential(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("证件更新成功"), http.StatusInternalServerError).
		WithSentinel(ErrCredentialNotFound, http.StatusNotFound).Handle(c)
}

// credentialIDReq ID 路径参数
type credentialIDReq struct {
	ID int
}

// DeleteCredential 删除目标证件
// @Summary 删除证件
// @Description 管理员删除目标证件字典项；无返回载荷。练习进度分区随证件删除（迁移 000040 的 ON DELETE CASCADE），课程/题目置空归属；证件下仍有投稿时**阻塞删除**并回该句事实（含条数），不做静默级联删投稿（CONTEXT.md「证件删除的阻塞项」/ #1360）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Param id path int true "证件ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "该证件下仍有 N 篇投稿（文案带条数）"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证件不存在"
// @Router /admin/credential/{id} [delete]
func (h *handler) DeleteCredential(c *gin.Context) {
	httpx.Endpoint[credentialIDReq, struct{}]{
		Parse: func(c *gin.Context) (*credentialIDReq, error) {
			id, err := httpx.PathInt(c, "id", "证件ID无效")
			if err != nil {
				return nil, err
			}
			return &credentialIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req credentialIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteCredential(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("证件删除成功"), http.StatusInternalServerError).
		// #1360：投稿阻塞走 400（ADR-0064 决策 9 让 4xx 原样发出那句话，条数因此在文案里）。
		// 不用 409：本仓从未使用 409，renderStatus 的单一咽喉里没有 409 分支，域表放 409 会被
		// 静默渲染成 500（同 faq.go 里「标识已占用」的同一处先例与同一理由）。
		WithSentinel(ErrCredentialHasContributions, http.StatusBadRequest).
		WithSentinel(ErrCredentialNotFound, http.StatusNotFound).Handle(c)
}

// SwapCredentialSort 交换目标证件排序
// @Summary 交换证件排序
// @Description 管理员交换两个目标证件的排序位置（body: {"swap_with": <id>}）；无返回载荷
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "证件ID"
// @Param body body object true "交换目标 {swap_with: int}"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/credential/{id}/sort [put]
func (h *handler) SwapCredentialSort(c *gin.Context) {
	httpx.Endpoint[catalogSwapSortReq, struct{}]{
		Parse: catalogSwapSortParse("id", "证件ID无效"),
		Invoke: httpx.Invoke(func(req catalogSwapSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapCredentialSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(httpx.OkMsgNoData("排序已交换"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, SortFacts400...).Handle(c)
}

// GetCurrentCredential 获取当前证件 GET /api/me/credential
// GetCurrentCredential 当前证件 GET /api/me/credential
// @Summary 当前证件
// @Description 查询当前目标证件（hrwai_user/admin/tutor 均可查询）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.CurrentCredentialDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /me/credential [get]
func (h *handler) GetCurrentCredential(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	dict, err := h.svc.GetCurrentCredential(uid)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if dict == nil {
		response.Success(c, CurrentCredentialDTO{Credential: nil})
		return
	}
	response.Success(c, CurrentCredentialDTO{Credential: dict})
}

// SetCurrentCredential 设置当前证件 PATCH /api/me/credential
// SetCurrentCredential 切换当前证件 PATCH /api/me/credential
// @Summary 切换当前证件
// @Description 学员切换当前目标证件（仅 hrwai_user；切换即全局过滤器）
// @Tags 学员端-目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "证件 ID {credential_id: int}"
// @Success 200 {object} response.R{data=training.CurrentCredentialDTO} "success"
// @Failure 400 {object} response.R "证件不存在"
// @Failure 401 {object} response.R "未认证"
// @Router /me/credential [patch]
func (h *handler) SetCurrentCredential(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	if role := middleware.CurrentRole(c); role != "" && role != string(authz.RoleStudent) {
		response.Forbidden(c, "仅学员可切换证件")
		return
	}
	var req struct {
		CredentialID int `json:"credential_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CredentialID <= 0 {
		response.BadRequest(c, "证件ID无效")
		return
	}
	dict, err := h.svc.SetCurrentCredential(uid, req.CredentialID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "当前证件已切换", CurrentCredentialDTO{Credential: dict})
}
