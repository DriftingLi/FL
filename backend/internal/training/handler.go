// 本文件：培训域 HTTP 出口之一——学员端查询 handler 与 RegisterRoutes。
// 管理端见 handler_admin.go，目标证件见 handler_credential.go。
package training

import (
	"context"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// handler 培训域 handler（培训目录 / 岗位字典 / 目标证件三面共用同一私有类型）。
type handler struct {
	svc *Service
}

// newHandler 创建培训域 handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册培训域学员端路由（公开读面）：
//   - /api/catalog/tree、/api/levels、/api/tags：培训目录查询
//   - /api/positions：岗位字典公开读（学员端/招聘端共用，仅启用项）
//
// 证件面（/api/credentials、/api/me/credential）与 /api/admin/* 见另外两个注册函数。
func RegisterRoutes(rg *gin.RouterGroup, _ *security.Session, svc *Service) {
	h := newHandler(svc)

	// ===== 学员端查询（公开） =====
	rg.GET("/catalog/tree", h.GetCatalogTree)
	rg.GET("/levels", h.ListPublicLevels)
	rg.GET("/tags", h.ListPublicTags)
	// 岗位字典公开读（学员端/招聘端共用，仅启用项）
	rg.GET("/positions", h.ListPublicPositions)
}

// GetCatalogTree 培训目录树
// @Summary 培训目录树（公开）
// @Description 学员端课程目录树（credential_id 可选：传了按目标证件分区，与课程列表同口径；不传不分区）
// @Tags 学员端-培训目录
// @Produce json
// @Param credential_id query int false "目标证件ID"
// @Success 200 {object} response.R{data=training.CatalogTreeDTO} "success"
// @Router /catalog/tree [get]
func (h *handler) GetCatalogTree(c *gin.Context) {
	httpx.Endpoint[struct{}, CatalogTreeDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*CatalogTreeDTO, error) {
			return h.svc.GetCatalogTree(httpx.QueryIDPtr(c, "credential_id")), nil
		},
	}.Handle(c)
}

// ListPublicLevels 课程等级列表
// @Summary 课程等级（公开）
// @Description 仅启用项
// @Tags 学员端-培训目录
// @Produce json
// @Success 200 {object} response.R{data=training.LevelListDTO} "success"
// @Router /levels [get]
func (h *handler) ListPublicLevels(c *gin.Context) {
	httpx.Endpoint[struct{}, []LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]LevelDict, error) {
			result := h.svc.ListLevels(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]LevelDict) {
			response.Success(c, LevelListDTO{Levels: *resp})
		},
	}.Handle(c)
}

// ListPublicTags 题库标签列表
// @Summary 题库标签（公开）
// @Description 仅启用项（credential_id 可选：传了按目标证件分区，与抽题池同口径；不传不分区）
// @Tags 学员端-培训目录
// @Produce json
// @Param credential_id query int false "目标证件ID"
// @Success 200 {object} response.R{data=training.QuestionTagListDTO} "success"
// @Router /tags [get]
func (h *handler) ListPublicTags(c *gin.Context) {
	httpx.Endpoint[struct{}, []QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]QuestionTagDict, error) {
			result, err := h.svc.ListQuestionTags(true, false, httpx.QueryIDPtr(c, "credential_id")) // 学员端专项练习：隐藏来源标记标签
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]QuestionTagDict) {
			response.Success(c, QuestionTagListDTO{Tags: *resp})
		},
	}.Handle(c)
}

// ListPublicPositions 岗位字典公开列表
// @Summary 岗位字典
// @Description 学员端/招聘端可用的岗位字典（仅启用项）
// @Tags 招聘域-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.PositionListDTO} "岗位列表"
// @Router /positions [get]
func (h *handler) ListPublicPositions(c *gin.Context) {
	items := h.svc.ListPositions(true)
	response.Success(c, PositionListDTO{Positions: items})
}
