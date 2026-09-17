// Package api 实现 HTTP handlers。
// 本文件：培训目录蓝图（专业方向 / 课程等级 / 证书模板 / 题库标签 CRUD + 题目打标 + 学员端查询）。
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
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
	g := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapCatalogManage))
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
	tagG := rg.Group("/admin", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapCatalogAuthor))
	tagG.GET("/question-tags", h.ListQuestionTags)
	tagG.POST("/question-tag", h.CreateQuestionTag)
	tagG.PUT("/question-tag/:id", h.UpdateQuestionTag)
	tagG.DELETE("/question-tag/:id", h.DeleteQuestionTag)
	tagG.PUT("/question/:question_id/tags", h.SetQuestionTags)
}

// GetCatalogTree 培训目录树
// @Summary 培训目录树（公开）
// @Description 学员端课程目录树（credential_id 可选：传了按目标证件分区，与课程列表同口径；不传不分区）
// @Tags 学员端-培训目录
// @Produce json
// @Param credential_id query int false "目标证件ID"
// @Success 200 {object} response.R{data=service.CatalogTreeDTO} "success"
// @Router /catalog/tree [get]
func (h *TrainingCatalogHandler) GetCatalogTree(c *gin.Context) {
	Endpoint[struct{}, service.CatalogTreeDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.CatalogTreeDTO, error) {
			return h.svc.GetCatalogTree(queryIDPtr(c, "credential_id")), nil
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
// @Success 200 {object} response.R{data=service.LevelListDTO} "success"
// @Router /levels [get]
func (h *TrainingCatalogHandler) ListPublicLevels(c *gin.Context) {
	Endpoint[struct{}, []service.LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.LevelDict, error) {
			result := h.svc.ListLevels(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.LevelDict, _ error) {
			response.Success(c, service.LevelListDTO{Levels: *resp})
		},
	}.Handle(c)
}

// ListPublicTags 题库标签列表
// @Summary 题库标签（公开）
// @Description 仅启用项（credential_id 可选：传了按目标证件分区，与抽题池同口径；不传不分区）
// @Tags 学员端-培训目录
// @Produce json
// @Param credential_id query int false "目标证件ID"
// @Success 200 {object} response.R{data=service.QuestionTagListDTO} "success"
// @Router /tags [get]
func (h *TrainingCatalogHandler) ListPublicTags(c *gin.Context) {
	Endpoint[struct{}, []service.QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.QuestionTagDict, error) {
			result := h.svc.ListQuestionTags(true, false, queryIDPtr(c, "credential_id")) // 学员端专项练习：隐藏来源标记标签
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.QuestionTagDict, _ error) {
			response.Success(c, service.QuestionTagListDTO{Tags: *resp})
		},
	}.Handle(c)
}

// GetAdminCatalogTree 管理端目录树（含停用项与章节节点）
// @Summary 管理端目录树
// @Description 含停用项与章节节点的完整目录树（需 CapCatalogManage）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.CatalogTreeDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/catalog/tree [get]
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

// ListSpecialties 专业方向列表（含停用项）
// @Summary 专业方向列表
// @Description 管理端专业方向字典列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.SpecialtyListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/specialties [get]
func (h *TrainingCatalogHandler) ListSpecialties(c *gin.Context) {
	Endpoint[struct{}, []service.SpecialtyDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.SpecialtyDict, error) {
			result := h.svc.ListSpecialties(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.SpecialtyDict, _ error) {
			// 字节形状不变：{"specialties": [...]}，只是从内联 gin.H 换成具名 DTO（注解才能指认 data）
			response.Success(c, service.SpecialtyListDTO{Specialties: *resp})
		},
	}.Handle(c)
}

// ListLevels 课程等级列表（含停用项）
// @Summary 课程等级列表
// @Description 管理端课程等级字典列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.LevelListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/levels [get]
func (h *TrainingCatalogHandler) ListLevels(c *gin.Context) {
	Endpoint[struct{}, []service.LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.LevelDict, error) {
			result := h.svc.ListLevels(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.LevelDict, _ error) {
			response.Success(c, service.LevelListDTO{Levels: *resp})
		},
	}.Handle(c)
}

// ListCertificateTemplates 证书模板列表（含停用项）
// @Summary 证书模板列表
// @Description 管理端证书模板列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.CertificateTemplateListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/certificate-templates [get]
func (h *TrainingCatalogHandler) ListCertificateTemplates(c *gin.Context) {
	Endpoint[struct{}, []service.CertificateTemplateDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CertificateTemplateDict, error) {
			result := h.svc.ListCertificateTemplates(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CertificateTemplateDict, _ error) {
			response.Success(c, service.CertificateTemplateListDTO{CertificateTemplates: *resp})
		},
	}.Handle(c)
}

// ListQuestionTags 题库标签列表（含停用项）
// @Summary 题库标签列表
// @Description 管理端题库标签列表（含停用项与题目计数）
// @Tags 题库管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.QuestionTagListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question-tags [get]
func (h *TrainingCatalogHandler) ListQuestionTags(c *gin.Context) {
	Endpoint[struct{}, []service.QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.QuestionTagDict, error) {
			result := h.svc.ListQuestionTags(false, true, nil) // 管理端：全部可见、不分区
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.QuestionTagDict, _ error) {
			response.Success(c, service.QuestionTagListDTO{Tags: *resp})
		},
	}.Handle(c)
}

// CreateSpecialty 创建专业方向
// @Summary 创建专业方向
// @Description 管理员创建专业方向字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.SpecialtyInput true "专业方向"
// @Success 201 {object} response.R{data=service.SpecialtyDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/specialty [post]
func (h *TrainingCatalogHandler) CreateSpecialty(c *gin.Context) {
	Endpoint[service.SpecialtyInput, service.SpecialtyDict]{
		Parse:  bindJSONMsgFunc[service.SpecialtyInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreateSpecialty),
	}.WithSuccess(created("专业方向创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateLevel 创建课程等级
// @Summary 创建课程等级
// @Description 管理员创建全局课程等级字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.LevelInput true "课程等级"
// @Success 201 {object} response.R{data=service.LevelDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/level [post]
func (h *TrainingCatalogHandler) CreateLevel(c *gin.Context) {
	Endpoint[service.LevelInput, service.LevelDict]{
		Parse:  bindJSONMsgFunc[service.LevelInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreateLevel),
	}.WithSuccess(created("课程等级创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateCertificateTemplate 创建证书模板
// @Summary 创建证书模板
// @Description 管理员创建证书模板（有效期单位天）
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.CertificateTemplateInput true "证书模板"
// @Success 201 {object} response.R{data=service.CertificateTemplateDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/certificate-template [post]
func (h *TrainingCatalogHandler) CreateCertificateTemplate(c *gin.Context) {
	Endpoint[service.CertificateTemplateInput, service.CertificateTemplateDict]{
		Parse:  bindJSONMsgFunc[service.CertificateTemplateInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreateCertificateTemplate),
	}.WithSuccess(created("证书模板创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateQuestionTag 创建题库标签
// @Summary 创建题库标签
// @Description 管理员/讲师创建题库标签
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.QuestionTagInput true "题库标签"
// @Success 201 {object} response.R{data=service.QuestionTagDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question-tag [post]
func (h *TrainingCatalogHandler) CreateQuestionTag(c *gin.Context) {
	Endpoint[service.QuestionTagInput, service.QuestionTagDict]{
		Parse:  bindJSONMsgFunc[service.QuestionTagInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreateQuestionTag),
	}.WithSuccess(created("题库标签创建成功"), http.StatusBadRequest).Handle(c)
}

// specialtyUpdateReq 更新请求。
type specialtyUpdateReq struct {
	ID int
	In service.SpecialtyInput
}

// UpdateSpecialty 更新专业方向
// @Summary 更新专业方向
// @Description 管理员更新专业方向字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param specialty_id path int true "专业方向ID"
// @Param body body service.SpecialtyInput true "专业方向"
// @Success 200 {object} response.R{data=service.SpecialtyDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "专业方向不存在"
// @Router /admin/specialty/{specialty_id} [put]
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
		Invoke: invoke(func(req specialtyUpdateReq) (service.SpecialtyDict, error) {
			return h.svc.UpdateSpecialty(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("专业方向更新成功"), http.StatusNotFound).Handle(c)
}

// levelUpdateReq 更新请求。
type levelUpdateReq struct {
	ID int
	In service.LevelInput
}

// UpdateLevel 更新课程等级
// @Summary 更新课程等级
// @Description 管理员更新课程等级字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param level_id path int true "等级ID"
// @Param body body service.LevelInput true "课程等级"
// @Success 200 {object} response.R{data=service.LevelDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程等级不存在"
// @Router /admin/level/{level_id} [put]
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
		Invoke: invoke(func(req levelUpdateReq) (service.LevelDict, error) {
			return h.svc.UpdateLevel(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("课程等级更新成功"), http.StatusNotFound).Handle(c)
}

// certificateTemplateUpdateReq 更新请求。
type certificateTemplateUpdateReq struct {
	ID int
	In service.CertificateTemplateInput
}

// UpdateCertificateTemplate 更新证书模板
// @Summary 更新证书模板
// @Description 管理员更新证书模板
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模板ID"
// @Param body body service.CertificateTemplateInput true "证书模板"
// @Success 200 {object} response.R{data=service.CertificateTemplateDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证书模板不存在"
// @Router /admin/certificate-template/{id} [put]
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
		Invoke: invoke(func(req certificateTemplateUpdateReq) (service.CertificateTemplateDict, error) {
			return h.svc.UpdateCertificateTemplate(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("证书模板更新成功"), http.StatusNotFound).Handle(c)
}

// questionTagUpdateReq 更新请求。
type questionTagUpdateReq struct {
	ID int
	In service.QuestionTagInput
}

// UpdateQuestionTag 更新题库标签
// @Summary 更新题库标签
// @Description 管理员/讲师更新题库标签
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "标签ID"
// @Param body body service.QuestionTagInput true "题库标签"
// @Success 200 {object} response.R{data=service.QuestionTagDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "题库标签不存在"
// @Router /admin/question-tag/{id} [put]
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
		Invoke: invoke(func(req questionTagUpdateReq) (service.QuestionTagDict, error) {
			return h.svc.UpdateQuestionTag(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("题库标签更新成功"), http.StatusNotFound).Handle(c)
}

// specialtyIDReq ID 路径参数请求。
type specialtyIDReq struct {
	ID int
}

// DeleteSpecialty 删除专业方向
// @Summary 删除专业方向
// @Description 管理员删除专业方向字典项；无返回载荷
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Param specialty_id path int true "专业方向ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "专业方向不存在"
// @Router /admin/specialty/{specialty_id} [delete]
func (h *TrainingCatalogHandler) DeleteSpecialty(c *gin.Context) {
	Endpoint[specialtyIDReq, struct{}]{
		Parse: func(c *gin.Context) (*specialtyIDReq, error) {
			id, err := strconv.Atoi(c.Param("specialty_id"))
			if err != nil {
				return nil, badRequest("专业方向ID无效")
			}
			return &specialtyIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req specialtyIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteSpecialty(req.ID)
		}),
	}.WithSuccess(okMsgNoData("专业方向删除成功"), http.StatusNotFound).Handle(c)
}

// levelIDReq ID 路径参数请求。
type levelIDReq struct {
	ID int
}

// DeleteLevel 删除课程等级
// @Summary 删除课程等级
// @Description 管理员删除课程等级字典项；无返回载荷
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Param level_id path int true "等级ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程等级不存在"
// @Router /admin/level/{level_id} [delete]
func (h *TrainingCatalogHandler) DeleteLevel(c *gin.Context) {
	Endpoint[levelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*levelIDReq, error) {
			id, err := strconv.Atoi(c.Param("level_id"))
			if err != nil {
				return nil, badRequest("课程等级ID无效")
			}
			return &levelIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req levelIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteLevel(req.ID)
		}),
	}.WithSuccess(okMsgNoData("课程等级删除成功"), http.StatusNotFound).Handle(c)
}

// certificateTemplateIDReq ID 路径参数请求。
type certificateTemplateIDReq struct {
	ID int
}

// DeleteCertificateTemplate 删除证书模板
// @Summary 删除证书模板
// @Description 管理员删除证书模板；无返回载荷
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Param id path int true "模板ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证书模板不存在"
// @Router /admin/certificate-template/{id} [delete]
func (h *TrainingCatalogHandler) DeleteCertificateTemplate(c *gin.Context) {
	Endpoint[certificateTemplateIDReq, struct{}]{
		Parse: func(c *gin.Context) (*certificateTemplateIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证书模板ID无效")
			}
			return &certificateTemplateIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req certificateTemplateIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteCertificateTemplate(req.ID)
		}),
	}.WithSuccess(okMsgNoData("证书模板删除成功"), http.StatusNotFound).Handle(c)
}

// questionTagIDReq ID 路径参数请求。
type questionTagIDReq struct {
	ID int
}

// DeleteQuestionTag 删除题库标签
// @Summary 删除题库标签
// @Description 管理员/讲师删除题库标签；无返回载荷
// @Tags 题库管理
// @Produce json
// @Security BearerAuth
// @Param id path int true "标签ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "题库标签不存在"
// @Router /admin/question-tag/{id} [delete]
func (h *TrainingCatalogHandler) DeleteQuestionTag(c *gin.Context) {
	Endpoint[questionTagIDReq, struct{}]{
		Parse: func(c *gin.Context) (*questionTagIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("题库标签ID无效")
			}
			return &questionTagIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req questionTagIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteQuestionTag(req.ID)
		}),
	}.WithSuccess(okMsgNoData("题库标签删除成功"), http.StatusNotFound).Handle(c)
}

// swapSpecialtySortReq 交换排序请求。
type swapSpecialtySortReq struct {
	ID       int
	SwapWith int
}

// SwapSpecialtySort 交换专业方向排序
// @Summary 交换专业方向排序
// @Description 管理员交换两个专业方向的排序位置（body: {"swap_with": <id>}）；无返回载荷
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param specialty_id path int true "专业方向ID"
// @Param body body object true "交换目标 {swap_with: int}"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/specialty/{specialty_id}/sort [put]
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
		Invoke: invoke(func(req swapSpecialtySortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapSpecialtySort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(okMsgNoData("排序已交换"), http.StatusBadRequest).Handle(c)
}

// swapLevelSortReq 交换排序请求。
type swapLevelSortReq struct {
	ID       int
	SwapWith int
}

// SwapLevelSort 交换课程等级排序
// @Summary 交换课程等级排序
// @Description 管理员交换两个课程等级的排序位置（body: {"swap_with": <id>}）；无返回载荷
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param level_id path int true "等级ID"
// @Param body body object true "交换目标 {swap_with: int}"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/level/{level_id}/sort [put]
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
		Invoke: invoke(func(req swapLevelSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapLevelSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(okMsgNoData("排序已交换"), http.StatusBadRequest).Handle(c)
}

// setQuestionTagsReq 全量替换题目标签请求。
type setQuestionTagsReq struct {
	QuestionID int
	TagIDs     []int
}

// SetQuestionTags 全量替换题目标签
// @Summary 题目打标
// @Description 全量替换题目的题库标签（管理端/讲师端），返回写入后的标签ID集合
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "标签ID列表" example({"tag_ids":[1,2]})
// @Success 200 {object} response.R{data=service.QuestionTagsResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question/{question_id}/tags [put]
func (h *TrainingCatalogHandler) SetQuestionTags(c *gin.Context) {
	Endpoint[setQuestionTagsReq, service.QuestionTagsResultDTO]{
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
		Invoke: invoke(func(req setQuestionTagsReq) (service.QuestionTagsResultDTO, error) {
			if err := h.svc.SetQuestionTags(req.QuestionID, req.TagIDs); err != nil {
				return service.QuestionTagsResultDTO{}, err
			}
			// 响应即「实际写入的标签集」回显，故在 Invoke 里成型（服务只负责落库）。
			return service.QuestionTagsResultDTO{TagIDs: req.TagIDs}, nil
		}),
	}.WithSuccess(okMsg("题目标签已更新"), http.StatusBadRequest).Handle(c)
}

// ===== 目标证件 =====

// ListPublicCredentials 目标证件列表（公开，仅启用项）GET /api/credentials
// ListPublicCredentials 证件列表 GET /api/credentials
// @Summary 证件列表
// @Description 学员端公开证件列表（目标证件，仅启用项）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.CredentialListDTO} "success"
// @Router /credentials [get]
func (h *TrainingCatalogHandler) ListPublicCredentials(c *gin.Context) {
	Endpoint[struct{}, []service.CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CredentialDict, error) {
			result := h.svc.ListCredentials(true)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CredentialDict, _ error) {
			response.Success(c, service.CredentialListDTO{Credentials: *resp})
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
// @Success 200 {object} response.R{data=service.GroupedCredentialsDTO} "success"
// @Router /credentials/grouped [get]
func (h *TrainingCatalogHandler) ListGroupedCredentials(c *gin.Context) {
	Endpoint[struct{}, service.GroupedCredentialsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.GroupedCredentialsDTO, error) {
			result := h.svc.ListGroupedCredentials()
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.GroupedCredentialsDTO, _ error) {
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// ListCredentials 目标证件列表（管理端，含停用）
// @Summary 证件列表（管理端）
// @Description 管理端目标证件列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.CredentialListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/credentials [get]
func (h *TrainingCatalogHandler) ListCredentials(c *gin.Context) {
	Endpoint[struct{}, []service.CredentialDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]service.CredentialDict, error) {
			result := h.svc.ListCredentials(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]service.CredentialDict, _ error) {
			response.Success(c, service.CredentialListDTO{Credentials: *resp})
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
// @Param body body service.CredentialInput true "证件"
// @Success 201 {object} response.R{data=service.CredentialDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/credential [post]
func (h *TrainingCatalogHandler) CreateCredential(c *gin.Context) {
	Endpoint[service.CredentialInput, service.CredentialDict]{
		Parse:  bindJSONMsgFunc[service.CredentialInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreateCredential),
	}.WithSuccess(created("证件创建成功"), http.StatusBadRequest).Handle(c)
}

// credentialUpdateReq 更新请求
type credentialUpdateReq struct {
	ID int
	In service.CredentialInput
}

// UpdateCredential 更新目标证件
// @Summary 更新证件
// @Description 管理员更新目标证件字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "证件ID"
// @Param body body service.CredentialInput true "证件"
// @Success 200 {object} response.R{data=service.CredentialDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证件不存在"
// @Router /admin/credential/{id} [put]
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
		Invoke: invoke(func(req credentialUpdateReq) (service.CredentialDict, error) {
			return h.svc.UpdateCredential(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("证件更新成功"), http.StatusNotFound).Handle(c)
}

// credentialIDReq ID 路径参数
type credentialIDReq struct {
	ID int
}

// DeleteCredential 删除目标证件
// @Summary 删除证件
// @Description 管理员删除目标证件字典项；无返回载荷
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Param id path int true "证件ID"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证件不存在"
// @Router /admin/credential/{id} [delete]
func (h *TrainingCatalogHandler) DeleteCredential(c *gin.Context) {
	Endpoint[credentialIDReq, struct{}]{
		Parse: func(c *gin.Context) (*credentialIDReq, error) {
			id, err := strconv.Atoi(c.Param("id"))
			if err != nil {
				return nil, badRequest("证件ID无效")
			}
			return &credentialIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req credentialIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteCredential(req.ID)
		}),
	}.WithSuccess(okMsgNoData("证件删除成功"), http.StatusNotFound).Handle(c)
}

// swapCredentialSortReq 交换排序请求
type swapCredentialSortReq struct {
	ID       int
	SwapWith int
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
// @Router /admin/credential/{id}/sort [put]
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
		Invoke: invoke(func(req swapCredentialSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapCredentialSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(okMsgNoData("排序已交换"), http.StatusBadRequest).Handle(c)
}

// GetCurrentCredential 获取当前证件 GET /api/me/credential
// GetCurrentCredential 当前证件 GET /api/me/credential
// @Summary 当前证件
// @Description 查询当前目标证件（hrwai_user/admin/tutor 均可查询）
// @Tags 学员端-目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.CurrentCredentialDTO} "success"
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
		response.Success(c, service.CurrentCredentialDTO{Credential: nil})
		return
	}
	response.Success(c, service.CurrentCredentialDTO{Credential: dict})
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
// @Success 200 {object} response.R{data=service.CurrentCredentialDTO} "success"
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
	response.SuccessWithMsg(c, "当前证件已切换", service.CurrentCredentialDTO{Credential: dict})
}

// ListPositions 岗位列表（管理端含停用项）
// @Summary 岗位列表
// @Description 管理端岗位字典列表（含停用项）
// @Tags 管理端-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.PositionListDTO} "岗位列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/positions [get]
func (h *TrainingCatalogHandler) ListPositions(c *gin.Context) {
	items := h.svc.ListPositions(false)
	// 字节形状不变：{"positions": [...]}，只是从内联 gin.H 换成具名 DTO（注解才能指认 data）
	response.Success(c, service.PositionListDTO{Positions: items})
}

// CreatePosition 创建岗位 POST /api/admin/position
// @Summary 创建岗位
// @Description 管理员创建岗位字典项
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body service.PositionInput true "岗位信息"
// @Success 201 {object} response.R{data=service.PositionDict} "创建成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position [post]
func (h *TrainingCatalogHandler) CreatePosition(c *gin.Context) {
	Endpoint[service.PositionInput, service.PositionDict]{
		Parse:  bindJSONMsgFunc[service.PositionInput]("请求数据无效"),
		Invoke: invoke(h.svc.CreatePosition),
	}.WithSuccess(created("岗位创建成功"), http.StatusBadRequest).Handle(c)
}

// positionUpdateReq 更新岗位请求（ID 来自路径，In 来自 body）。
// 与其它目录实体的 update 请求同形（此前 ID 在 Invoke 里从 path 取，属越层的小样板）。
type positionUpdateReq struct {
	ID int
	In service.PositionInput
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
// @Success 200 {object} response.R{data=service.PositionDict} "更新成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position/{position_id} [put]
func (h *TrainingCatalogHandler) UpdatePosition(c *gin.Context) {
	Endpoint[positionUpdateReq, service.PositionDict]{
		Parse: func(c *gin.Context) (*positionUpdateReq, error) {
			// 先 body 后 path：与既有「Invoke 内先绑定 body、再取 path」的报错优先级逐字一致
			in, err := bindJSONMsg[service.PositionInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			return &positionUpdateReq{ID: id, In: *in}, nil
		},
		Invoke: invoke(func(req positionUpdateReq) (service.PositionDict, error) {
			return h.svc.UpdatePosition(req.ID, req.In)
		}),
	}.WithSuccess(okMsg("岗位已更新"), http.StatusBadRequest).Handle(c)
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
	Endpoint[positionSwapSortReq, struct{}]{
		Parse: func(c *gin.Context) (*positionSwapSortReq, error) {
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			body, err := bindJSONMsg[positionSwapSortReqBody](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &positionSwapSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: invoke(func(req positionSwapSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapPositionSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(okMsgNoData("排序已更新"), http.StatusBadRequest).Handle(c)
}

// positionSwapSortReq 交换岗位排序请求（ID 来自路径，SwapWith 来自 body）。
type positionSwapSortReq struct {
	ID       int
	SwapWith int
}

// positionSwapSortReqBody body 绑定形态（只有 swap_with 一个字段）。
// 与其它 swap 端点一致：此处不做 swap_with <= 0 的前置校验（既有行为——校验留在 service）。
type positionSwapSortReqBody struct {
	SwapWith int `json:"swap_with"`
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
	Endpoint[positionIDReq, struct{}]{
		Parse: func(c *gin.Context) (*positionIDReq, error) {
			id, err := pathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			return &positionIDReq{ID: id}, nil
		},
		Invoke: invoke(func(req positionIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeletePosition(req.ID)
		}),
	}.WithSuccess(okMsgNoData("岗位已删除"), http.StatusBadRequest).Handle(c)
}

// positionIDReq 岗位 ID 路径参数请求。
type positionIDReq struct {
	ID int
}

// ListPublicPositions 岗位字典公开列表
// @Summary 岗位字典
// @Description 学员端/招聘端可用的岗位字典（仅启用项）
// @Tags 招聘域-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.PositionListDTO} "岗位列表"
// @Router /positions [get]
func (h *TrainingCatalogHandler) ListPublicPositions(c *gin.Context) {
	items := h.svc.ListPositions(true)
	response.Success(c, service.PositionListDTO{Positions: items})
}
