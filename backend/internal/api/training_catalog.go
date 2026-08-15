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
//   - /api/admin/*：专业方向 / 课程等级 / 证书模板 / 题库标签 CRUD 与题目打标（管理端）
//   - /api/catalog/*、/api/specialties、/api/levels、/api/tags：学员端查询
func RegisterTrainingCatalogRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.TrainingCatalogService) {
	h := NewTrainingCatalogHandler(svc)

	// ===== 学员端查询（公开） =====
	rg.GET("/catalog/tree", h.GetCatalogTree)
	rg.GET("/levels", h.ListPublicLevels)
	rg.GET("/tags", h.ListPublicTags)

	// ===== 管理端 CRUD =====
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))
	g.GET("/catalog/tree", h.GetAdminCatalogTree)

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

	// ---- 题库标签 ----
	g.GET("/question-tags", h.ListQuestionTags)
	g.POST("/question-tag", h.CreateQuestionTag)
	g.PUT("/question-tag/:id", h.UpdateQuestionTag)
	g.DELETE("/question-tag/:id", h.DeleteQuestionTag)

	// ---- 题目打标 ----
	g.PUT("/question/:question_id/tags", h.SetQuestionTags)
}

// GetCatalogTree 课程目录树（学员端）GET /api/catalog/tree
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

// ListPublicLevels 课程等级列表（仅启用项）GET /api/levels
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

// ListPublicTags 题库标签列表（仅启用项）GET /api/tags
func (h *TrainingCatalogHandler) ListPublicTags(c *gin.Context) {
	Endpoint[struct{}, []service.QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.QuestionTagDict, error) {
			result := h.svc.ListQuestionTags(true)
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
			result := h.svc.ListQuestionTags(false)
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
