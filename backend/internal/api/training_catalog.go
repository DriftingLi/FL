// Package api 实现 HTTP handlers。
// 本文件：培训目录蓝图（专业方向 / 课程等级 / 证书模板 / 题库标签 CRUD + 题目打标 + 学员端查询）。
package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// TrainingCatalogHandler 培训目录 handler。
type TrainingCatalogHandler struct {
	svc *service.TrainingCatalogService
}

// NewTrainingCatalogHandler 创建培训目录 handler。
func NewTrainingCatalogHandler(svc *service.TrainingCatalogService) *TrainingCatalogHandler {
	return &TrainingCatalogHandler{svc: svc}
}

// RegisterTrainingCatalogRoutes 注册培训目录蓝图：
//   - /api/admin/*：专业方向 / 课程等级 / 证书模板 / 题库标签 / 目标证件 CRUD 与题目打标（管理端）
//   - /api/catalog/*、/api/levels、/api/tags、/api/credentials：学员端查询
//   - /api/me/credential：当前证件读写（需登录）
func RegisterTrainingCatalogRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.TrainingCatalogService) {
	h := NewTrainingCatalogHandler(svc)

	// ===== 学员端查询（公开） =====
	rg.GET("/catalog/tree", h.GetCatalogTree)
	rg.GET("/levels", h.ListPublicLevels)
	rg.GET("/tags", h.ListPublicTags)
	rg.GET("/credentials", h.ListPublicCredentials)
	rg.GET("/credentials/grouped", h.ListGroupedCredentials)
	// 岗位字典公开读（学员端/招聘端共用，仅启用项）
	rg.GET("/positions", h.ListPublicPositions)
	// 当前证件（需登录，hrwai_user / admin / tutor 均可查询，切换仅 hrwai_user）
	rg.GET("/me/credential", middleware.JWTAuth(rd.Session), h.GetCurrentCredential)
	rg.PATCH("/me/credential", middleware.JWTAuth(rd.Session), h.SetCurrentCredential)

	// ===== 管理端 CRUD =====
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	g.GET("/catalog/tree", h.GetAdminCatalogTree)

	// ---- 岗位字典（问题4：与专业方向解绑） ----
	g.GET("/positions", h.ListPositions)
	g.POST("/position", h.CreatePosition)
	g.PUT("/position/:position_id", h.UpdatePosition)
	g.PUT("/position/:position_id/sort", h.SwapPositionSort)
	g.DELETE("/position/:position_id", h.DeletePosition)

	// ---- 专业方向 ----
	g.GET("/specialties", h.ListSpecialties)
	g.POST("/specialty", h.CreateSpecialty)
	g.PUT("/specialty/:specialty_id", h.UpdateSpecialty)
	g.PUT("/specialty/:specialty_id/sort", h.SwapSpecialtySort)
	g.DELETE("/specialty/:specialty_id", h.DeleteSpecialty)

	// ---- 课程等级 ----
	g.GET("/levels", h.ListLevels)
	g.POST("/level", h.CreateLevel)
	g.PUT("/level/:level_id", h.UpdateLevel)
	g.PUT("/level/:level_id/sort", h.SwapLevelSort)
	g.DELETE("/level/:level_id", h.DeleteLevel)

	// ---- 证书模板 ----
	g.GET("/certificate-templates", h.ListCertificateTemplates)
	g.POST("/certificate-template", h.CreateCertificateTemplate)
	g.PUT("/certificate-template/:id", h.UpdateCertificateTemplate)
	g.DELETE("/certificate-template/:id", h.DeleteCertificateTemplate)

	// ---- 目标证件 ----
	g.GET("/credentials", h.ListCredentials)
	g.POST("/credential", h.CreateCredential)
	g.PUT("/credential/:id", h.UpdateCredential)
	g.PUT("/credential/:id/sort", h.SwapCredentialSort)
	g.DELETE("/credential/:id", h.DeleteCredential)

	// ===== 题库标签与题目打标（admin + tutor，#问题2：导师端题库管理需要） =====
	tagG := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin", "tutor"))
	tagG.GET("/question-tags", h.ListQuestionTags)
	tagG.POST("/question-tag", h.CreateQuestionTag)
	tagG.PUT("/question-tag/:id", h.UpdateQuestionTag)
	tagG.DELETE("/question-tag/:id", h.DeleteQuestionTag)
	tagG.PUT("/question/:question_id/tags", h.SetQuestionTags)
}

// GetCatalogTree 培训目录树
// @Summary 培训目录树（公开）
// @Description 学员端课程目录树
// @Tags 学员端-培训目录
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /catalog/tree [get]
func (h *TrainingCatalogHandler) GetCatalogTree(c *gin.Context) {
	Endpoint[struct{}, service.CatalogTreeDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.CatalogTreeDTO, error) {
			return h.svc.GetCatalogTree(), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.CatalogTreeDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListPublicLevels 课程等级列表
// @Summary 课程等级（公开）
// @Description 仅启用项
// @Tags 学员端-培训目录
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /levels [get]
func (h *TrainingCatalogHandler) ListPublicLevels(c *gin.Context) {
	Endpoint[struct{}, []service.LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.LevelDict, error) {
			result := h.svc.ListLevels(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.LevelDict, _ error) {
			response.Success(c, gin.H{"levels": deref(resp)})
		},
	}.Handle(c)
}

// ListPublicTags 题库标签列表
// @Summary 题库标签（公开）
// @Description 仅启用项
// @Tags 学员端-培训目录
// @Produce json
// @Success 200 {object} response.R "success"
// @Router /tags [get]
func (h *TrainingCatalogHandler) ListPublicTags(c *gin.Context) {
	Endpoint[struct{}, []service.QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.QuestionTagDict, error) {
			result := h.svc.ListQuestionTags(true, false) // 学员端专项练习：隐藏来源标记标签
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.QuestionTagDict, _ error) {
			response.Success(c, gin.H{"tags": deref(resp)})
		},
	}.Handle(c)
}

// GetAdminCatalogTree 管理端目录树（含停用项与章节节点）GET /api/admin/catalog/tree
func (h *TrainingCatalogHandler) GetAdminCatalogTree(c *gin.Context) {
	Endpoint[struct{}, service.CatalogTreeDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.CatalogTreeDTO, error) {
			return h.svc.GetAdminCatalogTree(), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.CatalogTreeDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// ListSpecialties 专业方向列表（含停用项）GET /api/admin/specialties
func (h *TrainingCatalogHandler) ListSpecialties(c *gin.Context) {
	Endpoint[struct{}, []service.SpecialtyDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.SpecialtyDict, error) {
			result := h.svc.ListSpecialties(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.SpecialtyDict, _ error) {
			response.Success(c, gin.H{"specialties": deref(resp)})
		},
	}.Handle(c)
}

// ListLevels 课程等级列表（含停用项）GET /api/admin/levels
func (h *TrainingCatalogHandler) ListLevels(c *gin.Context) {
	Endpoint[struct{}, []service.LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.LevelDict, error) {
			result := h.svc.ListLevels(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.LevelDict, _ error) {
			response.Success(c, gin.H{"levels": deref(resp)})
		},
	}.Handle(c)
}

// ListCertificateTemplates 证书模板列表（含停用项）GET /api/admin/certificate-templates
func (h *TrainingCatalogHandler) ListCertificateTemplates(c *gin.Context) {
	Endpoint[struct{}, []service.CertificateTemplateDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CertificateTemplateDict, error) {
			result := h.svc.ListCertificateTemplates(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CertificateTemplateDict, _ error) {
			response.Success(c, gin.H{"certificate_templates": deref(resp)})
		},
	}.Handle(c)
}

// ListQuestionTags 题库标签列表（含停用项）GET /api/admin/question-tags
func (h *TrainingCatalogHandler) ListQuestionTags(c *gin.Context) {
	Endpoint[struct{}, []service.QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.QuestionTagDict, error) {
			result := h.svc.ListQuestionTags(false, true) // 管理端：全部可见
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.QuestionTagDict, _ error) {
			response.Success(c, gin.H{"tags": deref(resp)})
		},
	}.Handle(c)
}

// CreateSpecialty 创建专业方向 POST /api/admin/specialty
func (h *TrainingCatalogHandler) CreateSpecialty(c *gin.Context) {
	Endpoint[service.SpecialtyInput, service.SpecialtyDict]{
		Parse: func(c *gin.Context) (*service.SpecialtyInput, error) {
			var in service.SpecialtyInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.SpecialtyInput) (*service.SpecialtyDict, error) {
			result, err := h.svc.CreateSpecialty(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.SpecialtyInput, resp *service.SpecialtyDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "专业方向创建成功", deref(resp))
		},
	}.Handle(c)
}

// CreateLevel 创建课程等级 POST /api/admin/level
func (h *TrainingCatalogHandler) CreateLevel(c *gin.Context) {
	Endpoint[service.LevelInput, service.LevelDict]{
		Parse: func(c *gin.Context) (*service.LevelInput, error) {
			var in service.LevelInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.LevelInput) (*service.LevelDict, error) {
			result, err := h.svc.CreateLevel(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.LevelInput, resp *service.LevelDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "课程等级创建成功", deref(resp))
		},
	}.Handle(c)
}

// CreateCertificateTemplate 创建证书模板 POST /api/admin/certificate-template
func (h *TrainingCatalogHandler) CreateCertificateTemplate(c *gin.Context) {
	Endpoint[service.CertificateTemplateInput, service.CertificateTemplateDict]{
		Parse: func(c *gin.Context) (*service.CertificateTemplateInput, error) {
			var in service.CertificateTemplateInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.CertificateTemplateInput) (*service.CertificateTemplateDict, error) {
			result, err := h.svc.CreateCertificateTemplate(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.CertificateTemplateInput, resp *service.CertificateTemplateDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "证书模板创建成功", deref(resp))
		},
	}.Handle(c)
}

// CreateQuestionTag 创建题库标签 POST /api/admin/question-tag
func (h *TrainingCatalogHandler) CreateQuestionTag(c *gin.Context) {
	Endpoint[service.QuestionTagInput, service.QuestionTagDict]{
		Parse: func(c *gin.Context) (*service.QuestionTagInput, error) {
			var in service.QuestionTagInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.QuestionTagInput) (*service.QuestionTagDict, error) {
			result, err := h.svc.CreateQuestionTag(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.QuestionTagInput, resp *service.QuestionTagDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "题库标签创建成功", deref(resp))
		},
	}.Handle(c)
}

// specialtyUpdateReq 更新请求。
type specialtyUpdateReq struct {
	ID int
	In service.SpecialtyInput
}

// UpdateSpecialty 更新专业方向 PUT /api/admin/specialty/:specialty_id
func (h *TrainingCatalogHandler) UpdateSpecialty(c *gin.Context) {
	Endpoint[specialtyUpdateReq, service.SpecialtyDict]{
		Parse: func(c *gin.Context) (*specialtyUpdateReq, error) {
			id, err := strconv.Atoi(c.Param("specialty_id"))
			if err != nil {
				return nil, badRequest("专业方向ID无效")
			}
			var in service.SpecialtyInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &specialtyUpdateReq{ID: id, In: in}, nil
		},
		Invoke: func(ctx context.Context, req *specialtyUpdateReq) (*service.SpecialtyDict, error) {
			result, err := h.svc.UpdateSpecialty(req.ID, req.In)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *specialtyUpdateReq, resp *service.SpecialtyDict, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "专业方向更新成功", deref(resp))
		},
	}.Handle(c)
}

// levelUpdateReq 更新请求。
type levelUpdateReq struct {
	ID int
	In service.LevelInput
}

// UpdateLevel 更新课程等级 PUT /api/admin/level/:level_id
func (h *TrainingCatalogHandler) UpdateLevel(c *gin.Context) {
	Endpoint[levelUpdateReq, service.LevelDict]{
		Parse: func(c *gin.Context) (*levelUpdateReq, error) {
			id, err := strconv.Atoi(c.Param("level_id"))
			if err != nil {
				return nil, badRequest("课程等级ID无效")
			}
			var in service.LevelInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &levelUpdateReq{ID: id, In: in}, nil
		},
		Invoke: func(ctx context.Context, req *levelUpdateReq) (*service.LevelDict, error) {
			result, err := h.svc.UpdateLevel(req.ID, req.In)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *levelUpdateReq, resp *service.LevelDict, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "课程等级更新成功", deref(resp))
		},
	}.Handle(c)
}

// certificateTemplateUpdateReq 更新请求。
type certificateTemplateUpdateReq struct {
	ID int
	In service.CertificateTemplateInput
}

// UpdateCertificateTemplate 更新证书模板 PUT /api/admin/certificate-template/:id
func (h *TrainingCatalogHandler) UpdateCertificateTemplate(c *gin.Context) {
	Endpoint[certificateTemplateUpdateReq, service.CertificateTemplateDict]{
		Parse: func(c *gin.Context) (*certificateTemplateUpdateReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证书模板ID无效")
			}
			var in service.CertificateTemplateInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &certificateTemplateUpdateReq{ID: id, In: in}, nil
		},
		Invoke: func(ctx context.Context, req *certificateTemplateUpdateReq) (*service.CertificateTemplateDict, error) {
			result, err := h.svc.UpdateCertificateTemplate(req.ID, req.In)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *certificateTemplateUpdateReq, resp *service.CertificateTemplateDict, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "证书模板更新成功", deref(resp))
		},
	}.Handle(c)
}

// questionTagUpdateReq 更新请求。
type questionTagUpdateReq struct {
	ID int
	In service.QuestionTagInput
}

// UpdateQuestionTag 更新题库标签 PUT /api/admin/question-tag/:id
func (h *TrainingCatalogHandler) UpdateQuestionTag(c *gin.Context) {
	Endpoint[questionTagUpdateReq, service.QuestionTagDict]{
		Parse: func(c *gin.Context) (*questionTagUpdateReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("题库标签ID无效")
			}
			var in service.QuestionTagInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &questionTagUpdateReq{ID: id, In: in}, nil
		},
		Invoke: func(ctx context.Context, req *questionTagUpdateReq) (*service.QuestionTagDict, error) {
			result, err := h.svc.UpdateQuestionTag(req.ID, req.In)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *questionTagUpdateReq, resp *service.QuestionTagDict, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题库标签更新成功", deref(resp))
		},
	}.Handle(c)
}

// specialtyIDReq ID 路径参数请求。
type specialtyIDReq struct {
	ID int
}

// DeleteSpecialty 删除专业方向 DELETE /api/admin/specialty/:specialty_id
func (h *TrainingCatalogHandler) DeleteSpecialty(c *gin.Context) {
	Endpoint[specialtyIDReq, struct{}]{
		Parse: func(c *gin.Context) (*specialtyIDReq, error) {
			id, err := strconv.Atoi(c.Param("specialty_id"))
			if err != nil {
				return nil, badRequest("专业方向ID无效")
			}
			return &specialtyIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *specialtyIDReq) (*struct{}, error) {
			if err := h.svc.DeleteSpecialty(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *specialtyIDReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "专业方向删除成功", nil)
		},
	}.Handle(c)
}

// levelIDReq ID 路径参数请求。
type levelIDReq struct {
	ID int
}

// DeleteLevel 删除课程等级 DELETE /api/admin/level/:level_id
func (h *TrainingCatalogHandler) DeleteLevel(c *gin.Context) {
	Endpoint[levelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*levelIDReq, error) {
			id, err := strconv.Atoi(c.Param("level_id"))
			if err != nil {
				return nil, badRequest("课程等级ID无效")
			}
			return &levelIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *levelIDReq) (*struct{}, error) {
			if err := h.svc.DeleteLevel(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *levelIDReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "课程等级删除成功", nil)
		},
	}.Handle(c)
}

// certificateTemplateIDReq ID 路径参数请求。
type certificateTemplateIDReq struct {
	ID int
}

// DeleteCertificateTemplate 删除证书模板 DELETE /api/admin/certificate-template/:id
func (h *TrainingCatalogHandler) DeleteCertificateTemplate(c *gin.Context) {
	Endpoint[certificateTemplateIDReq, struct{}]{
		Parse: func(c *gin.Context) (*certificateTemplateIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证书模板ID无效")
			}
			return &certificateTemplateIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *certificateTemplateIDReq) (*struct{}, error) {
			if err := h.svc.DeleteCertificateTemplate(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *certificateTemplateIDReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "证书模板删除成功", nil)
		},
	}.Handle(c)
}

// questionTagIDReq ID 路径参数请求。
type questionTagIDReq struct {
	ID int
}

// DeleteQuestionTag 删除题库标签 DELETE /api/admin/question-tag/:id
func (h *TrainingCatalogHandler) DeleteQuestionTag(c *gin.Context) {
	Endpoint[questionTagIDReq, struct{}]{
		Parse: func(c *gin.Context) (*questionTagIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("题库标签ID无效")
			}
			return &questionTagIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *questionTagIDReq) (*struct{}, error) {
			if err := h.svc.DeleteQuestionTag(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *questionTagIDReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题库标签删除成功", nil)
		},
	}.Handle(c)
}

// swapSpecialtySortReq 交换排序请求。
type swapSpecialtySortReq struct {
	ID       int
	SwapWith int
}

// SwapSpecialtySort 交换专业方向排序 PUT /api/admin/specialty/:specialty_id/sort（body: {"swap_with": <id>}）
func (h *TrainingCatalogHandler) SwapSpecialtySort(c *gin.Context) {
	Endpoint[swapSpecialtySortReq, struct{}]{
		Parse: func(c *gin.Context) (*swapSpecialtySortReq, error) {
			id, err := strconv.Atoi(c.Param("specialty_id"))
			if err != nil {
				return nil, badRequest("专业方向ID无效")
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
				return nil, badRequest("swap_with 参数无效")
			}
			return &swapSpecialtySortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: func(ctx context.Context, req *swapSpecialtySortReq) (*struct{}, error) {
			if err := h.svc.SwapSpecialtySort(req.ID, req.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *swapSpecialtySortReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "排序已交换", nil)
		},
	}.Handle(c)
}

// swapLevelSortReq 交换排序请求。
type swapLevelSortReq struct {
	ID       int
	SwapWith int
}

// SwapLevelSort 交换课程等级排序 PUT /api/admin/level/:level_id/sort（body: {"swap_with": <id>}）
func (h *TrainingCatalogHandler) SwapLevelSort(c *gin.Context) {
	Endpoint[swapLevelSortReq, struct{}]{
		Parse: func(c *gin.Context) (*swapLevelSortReq, error) {
			id, err := strconv.Atoi(c.Param("level_id"))
			if err != nil {
				return nil, badRequest("课程等级ID无效")
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
				return nil, badRequest("swap_with 参数无效")
			}
			return &swapLevelSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: func(ctx context.Context, req *swapLevelSortReq) (*struct{}, error) {
			if err := h.svc.SwapLevelSort(req.ID, req.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *swapLevelSortReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "排序已交换", nil)
		},
	}.Handle(c)
}

// setQuestionTagsReq 全量替换题目标签请求。
type setQuestionTagsReq struct {
	QuestionID int
	TagIDs     []int
}

// SetQuestionTags 全量替换题目标签 PUT /api/admin/question/:question_id/tags
func (h *TrainingCatalogHandler) SetQuestionTags(c *gin.Context) {
	Endpoint[setQuestionTagsReq, struct{}]{
		Parse: func(c *gin.Context) (*setQuestionTagsReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			var req struct {
				TagIDs []int `json:"tag_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &setQuestionTagsReq{QuestionID: id, TagIDs: req.TagIDs}, nil
		},
		Invoke: func(ctx context.Context, req *setQuestionTagsReq) (*struct{}, error) {
			if err := h.svc.SetQuestionTags(req.QuestionID, req.TagIDs); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, req *setQuestionTagsReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题目标签已更新", map[string]any{"tag_ids": req.TagIDs})
		},
	}.Handle(c)
}

// ===== 目标证件 =====

// ListPublicCredentials 目标证件列表（公开，仅启用项）GET /api/credentials
// ListPublicCredentials 证件列表 GET /api/credentials
// @Summary 证件列表
// @Description 学员端公开证件列表（目标证件，仅启用项）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "证件列表"
// @Router /credentials [get]
func (h *TrainingCatalogHandler) ListPublicCredentials(c *gin.Context) {
	Endpoint[struct{}, []service.CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CredentialDict, error) {
			result := h.svc.ListCredentials(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CredentialDict, _ error) {
			response.Success(c, gin.H{"credentials": deref(resp)})
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
// @Success 200 {object} response.R "分组列表"
// @Router /credentials/grouped [get]
func (h *TrainingCatalogHandler) ListGroupedCredentials(c *gin.Context) {
	Endpoint[struct{}, map[string][]service.CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*map[string][]service.CredentialDict, error) {
			result := h.svc.ListGroupedCredentials()
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *map[string][]service.CredentialDict, _ error) {
			response.Success(c, deref(resp))
		},
	}.Handle(c)
}

// ListCredentials 目标证件列表（管理端，含停用）GET /api/admin/credentials
func (h *TrainingCatalogHandler) ListCredentials(c *gin.Context) {
	Endpoint[struct{}, []service.CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CredentialDict, error) {
			result := h.svc.ListCredentials(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CredentialDict, _ error) {
			response.Success(c, gin.H{"credentials": deref(resp)})
		},
	}.Handle(c)
}

// CreateCredential 创建目标证件 POST /api/admin/credential
func (h *TrainingCatalogHandler) CreateCredential(c *gin.Context) {
	Endpoint[service.CredentialInput, service.CredentialDict]{
		Parse: func(c *gin.Context) (*service.CredentialInput, error) {
			var in service.CredentialInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.CredentialInput) (*service.CredentialDict, error) {
			result, err := h.svc.CreateCredential(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.CredentialInput, resp *service.CredentialDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "证件创建成功", deref(resp))
		},
	}.Handle(c)
}

// credentialUpdateReq 更新请求
type credentialUpdateReq struct {
	ID int
	In service.CredentialInput
}

// UpdateCredential 更新目标证件 PUT /api/admin/credential/:id
func (h *TrainingCatalogHandler) UpdateCredential(c *gin.Context) {
	Endpoint[credentialUpdateReq, service.CredentialDict]{
		Parse: func(c *gin.Context) (*credentialUpdateReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证件ID无效")
			}
			var in service.CredentialInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &credentialUpdateReq{ID: id, In: in}, nil
		},
		Invoke: func(ctx context.Context, req *credentialUpdateReq) (*service.CredentialDict, error) {
			result, err := h.svc.UpdateCredential(req.ID, req.In)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *credentialUpdateReq, resp *service.CredentialDict, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "证件更新成功", deref(resp))
		},
	}.Handle(c)
}

// credentialIDReq ID 路径参数
type credentialIDReq struct {
	ID int
}

// DeleteCredential 删除目标证件 DELETE /api/admin/credential/:id
func (h *TrainingCatalogHandler) DeleteCredential(c *gin.Context) {
	Endpoint[credentialIDReq, struct{}]{
		Parse: func(c *gin.Context) (*credentialIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证件ID无效")
			}
			return &credentialIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *credentialIDReq) (*struct{}, error) {
			if err := h.svc.DeleteCredential(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *credentialIDReq, _ *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "证件删除成功", nil)
		},
	}.Handle(c)
}

// swapCredentialSortReq 交换排序请求
type swapCredentialSortReq struct {
	ID       int
	SwapWith int
}

// SwapCredentialSort 交换目标证件排序 PUT /api/admin/credential/:id/sort
func (h *TrainingCatalogHandler) SwapCredentialSort(c *gin.Context) {
	Endpoint[swapCredentialSortReq, struct{}]{
		Parse: func(c *gin.Context) (*swapCredentialSortReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证件ID无效")
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
				return nil, badRequest("swap_with 参数无效")
			}
			return &swapCredentialSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: func(ctx context.Context, req *swapCredentialSortReq) (*struct{}, error) {
			if err := h.svc.SwapCredentialSort(req.ID, req.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *swapCredentialSortReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "排序已交换", nil)
		},
	}.Handle(c)
}

// GetCurrentCredential 获取当前证件 GET /api/me/credential
// GetCurrentCredential 当前证件 GET /api/me/credential
// @Summary 当前证件
// @Description 查询当前目标证件（hrwai_user/admin/tutor 均可查询）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "当前证件"
// @Failure 401 {object} response.R "未认证"
// @Router /me/credential [get]
func (h *TrainingCatalogHandler) GetCurrentCredential(c *gin.Context) {
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
		response.Success(c, gin.H{"credential": nil})
		return
	}
	response.Success(c, gin.H{"credential": dict})
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
// @Success 200 {object} response.R "已切换"
// @Failure 400 {object} response.R "证件不存在"
// @Failure 401 {object} response.R "未认证"
// @Router /me/credential [patch]
func (h *TrainingCatalogHandler) SetCurrentCredential(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	if uid <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	if role := middleware.CurrentRole(c); role != "" && role != service.HrwaiRole {
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
	response.SuccessWithMsg(c, "当前证件已切换", gin.H{"credential": dict})
}

// ListPositions 岗位列表（管理端含停用项）GET /api/admin/positions
// @Summary 岗位列表
// @Description 管理端岗位字典列表（含停用项）
// @Tags 管理端-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "岗位列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/positions [get]
func (h *TrainingCatalogHandler) ListPositions(c *gin.Context) {
	items := h.svc.ListPositions(false)
	response.Success(c, gin.H{"positions": items})
}

// CreatePosition 创建岗位 POST /api/admin/position
// @Summary 创建岗位
// @Description 管理员创建岗位字典项
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.PositionInput true "岗位信息"
// @Success 201 {object} response.R "创建成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position [post]
func (h *TrainingCatalogHandler) CreatePosition(c *gin.Context) {
	Endpoint[service.PositionInput, service.PositionDict]{
		Parse: func(c *gin.Context) (*service.PositionInput, error) {
			var in service.PositionInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.PositionInput) (*service.PositionDict, error) {
			result, err := h.svc.CreatePosition(*in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.PositionInput, resp *service.PositionDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "岗位创建成功", deref(resp))
		},
	}.Handle(c)
}

// UpdatePosition 更新岗位 PUT /api/admin/position/:position_id
// @Summary 更新岗位
// @Description 管理员更新岗位字典项
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param position_id path int true "岗位 ID"
// @Param body body service.PositionInput true "岗位信息"
// @Success 200 {object} response.R "更新成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position/{position_id} [put]
func (h *TrainingCatalogHandler) UpdatePosition(c *gin.Context) {
	Endpoint[service.PositionInput, service.PositionDict]{
		Parse: func(c *gin.Context) (*service.PositionInput, error) {
			var in service.PositionInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &in, nil
		},
		Invoke: func(ctx context.Context, in *service.PositionInput) (*service.PositionDict, error) {
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			result, err := h.svc.UpdatePosition(id, *in)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *service.PositionInput, resp *service.PositionDict, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "岗位已更新", deref(resp))
		},
	}.Handle(c)
}

// SwapPositionSort 交换岗位排序 PUT /api/admin/position/:position_id/sort
// @Summary 交换岗位排序
// @Description 管理员交换两个岗位的排序位置
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param position_id path int true "岗位 ID"
// @Param body body object true "交换目标 {swap_with: int}"
// @Success 200 {object} response.R "已交换"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position/{position_id}/sort [put]
func (h *TrainingCatalogHandler) SwapPositionSort(c *gin.Context) {
	Endpoint[struct{}, struct{}]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			var body struct {
				SwapWith int `json:"swap_with"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				return nil, badRequest("请求数据无效")
			}
			if err := h.svc.SwapPositionSort(id, body.SwapWith); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *struct{}, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "排序已更新", nil)
		},
	}.Handle(c)
}

// DeletePosition 删除岗位 DELETE /api/admin/position/:position_id
// @Summary 删除岗位
// @Description 管理员删除岗位字典项（已关联职位/简历置空 position_id，不级联删除）
// @Tags 管理端-岗位字典
// @Produce json
// @Security BearerAuth
// @Param position_id path int true "岗位 ID"
// @Success 200 {object} response.R "已删除"
// @Failure 400 {object} response.R "删除失败"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position/{position_id} [delete]
func (h *TrainingCatalogHandler) DeletePosition(c *gin.Context) {
	Endpoint[struct{}, struct{}]{
		Parse: func(c *gin.Context) (*struct{}, error) {
			return &struct{}{}, nil
		},
		Invoke: func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			if err := h.svc.DeletePosition(id); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *struct{}, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "岗位已删除", nil)
		},
	}.Handle(c)
}

// ListPublicPositions 岗位字典公开列表 GET /api/positions
// @Summary 岗位字典
// @Description 学员端/招聘端可用的岗位字典（仅启用项）
// @Tags 招聘域-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R "岗位列表"
// @Router /positions [get]
func (h *TrainingCatalogHandler) ListPublicPositions(c *gin.Context) {
	items := h.svc.ListPositions(true)
	response.Success(c, gin.H{"positions": items})
}
