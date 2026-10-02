// 本文件：培训域 HTTP 出口之二——管理端 handler 与 RegisterAdminRoutes。
// 学员端查询见 handler.go，目标证件见 handler_credential.go。
package training

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/sortorder"
	"forklift-training/pkg/httpx"
	"forklift-training/pkg/response"
)

// RegisterAdminRoutes 注册培训域管理端路由：
//   - /api/admin/{specialty,level,certificate-template,position}*：目录 CRUD 与排序（CapCatalogManage）
//   - /api/admin/question-tag*、/api/admin/question/:question_id/tags：题库标签与打标（CapCatalogAuthor，#问题2 导师端需要）
//
// 证件的五条管理端路由随证件面走，见 RegisterCredentialRoutes。
func RegisterAdminRoutes(rg *gin.RouterGroup, session *security.Session, svc *Service) {
	h := newHandler(svc)

	// ===== 管理端 CRUD =====
	g := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapCatalogManage))
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

	// ===== 题库标签与题目打标（admin + tutor，#问题2：导师端题库管理需要） =====
	tagG := rg.Group("/admin", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapCatalogAuthor))
	tagG.GET("/question-tags", h.ListQuestionTags)
	tagG.POST("/question-tag", h.CreateQuestionTag)
	tagG.PUT("/question-tag/:id", h.UpdateQuestionTag)
	tagG.DELETE("/question-tag/:id", h.DeleteQuestionTag)
	tagG.PUT("/question/:question_id/tags", h.SetQuestionTags)
}

// GetAdminCatalogTree 管理端目录树（含停用项与章节节点）
// @Summary 管理端目录树
// @Description 含停用项与章节节点的完整目录树（需 CapCatalogManage）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.CatalogTreeDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/catalog/tree [get]
func (h *handler) GetAdminCatalogTree(c *gin.Context) {
	httpx.Endpoint[struct{}, CatalogTreeDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*CatalogTreeDTO, error) {
			return h.svc.GetAdminCatalogTree(), nil
		},
	}.Handle(c)
}

// ListSpecialties 专业方向列表（含停用项）
// @Summary 专业方向列表
// @Description 管理端专业方向字典列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.SpecialtyListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/specialties [get]
func (h *handler) ListSpecialties(c *gin.Context) {
	httpx.Endpoint[struct{}, []SpecialtyDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]SpecialtyDict, error) {
			result := h.svc.ListSpecialties(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]SpecialtyDict) {
			// 字节形状不变：{"specialties": [...]}，只是从内联 gin.H 换成具名 DTO（注解才能指认 data）
			response.Success(c, SpecialtyListDTO{Specialties: *resp})
		},
	}.Handle(c)
}

// ListLevels 课程等级列表（含停用项）
// @Summary 课程等级列表
// @Description 管理端课程等级字典列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.LevelListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/levels [get]
func (h *handler) ListLevels(c *gin.Context) {
	httpx.Endpoint[struct{}, []LevelDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]LevelDict, error) {
			result := h.svc.ListLevels(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]LevelDict) {
			response.Success(c, LevelListDTO{Levels: *resp})
		},
	}.Handle(c)
}

// ListCertificateTemplates 证书模板列表（含停用项）
// @Summary 证书模板列表
// @Description 管理端证书模板列表（含停用项）
// @Tags 管理端-培训目录
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.CertificateTemplateListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/certificate-templates [get]
func (h *handler) ListCertificateTemplates(c *gin.Context) {
	httpx.Endpoint[struct{}, []CertificateTemplateDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]CertificateTemplateDict, error) {
			result := h.svc.ListCertificateTemplates(false)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *[]CertificateTemplateDict) {
			response.Success(c, CertificateTemplateListDTO{CertificateTemplates: *resp})
		},
	}.Handle(c)
}

// ListQuestionTags 题库标签列表（含停用项）
// @Summary 题库标签列表
// @Description 管理端题库标签列表（含停用项与题目计数）
// @Tags 题库管理
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.QuestionTagListDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question-tags [get]
func (h *handler) ListQuestionTags(c *gin.Context) {
	httpx.Endpoint[struct{}, []QuestionTagDict]{
		Invoke: func(ctx context.Context, _ *struct{}) (*[]QuestionTagDict, error) {
			result, err := h.svc.ListQuestionTags(false, true, nil) // 管理端：全部可见、不分区
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

// CreateSpecialty 创建专业方向
// @Summary 创建专业方向
// @Description 管理员创建专业方向字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.SpecialtyInput true "专业方向"
// @Success 201 {object} response.R{data=training.SpecialtyDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/specialty [post]
func (h *handler) CreateSpecialty(c *gin.Context) {
	httpx.Endpoint[SpecialtyInput, SpecialtyDict]{
		Parse:  httpx.BindJSONMsgFunc[SpecialtyInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreateSpecialty),
	}.WithSuccess(httpx.Created("专业方向创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateLevel 创建课程等级
// @Summary 创建课程等级
// @Description 管理员创建全局课程等级字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.LevelInput true "课程等级"
// @Success 201 {object} response.R{data=training.LevelDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/level [post]
func (h *handler) CreateLevel(c *gin.Context) {
	httpx.Endpoint[LevelInput, LevelDict]{
		Parse:  httpx.BindJSONMsgFunc[LevelInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreateLevel),
	}.WithSuccess(httpx.Created("课程等级创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateCertificateTemplate 创建证书模板
// @Summary 创建证书模板
// @Description 管理员创建证书模板（有效期单位天）
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.CertificateTemplateInput true "证书模板"
// @Success 201 {object} response.R{data=training.CertificateTemplateDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/certificate-template [post]
func (h *handler) CreateCertificateTemplate(c *gin.Context) {
	httpx.Endpoint[CertificateTemplateInput, CertificateTemplateDict]{
		Parse:  httpx.BindJSONMsgFunc[CertificateTemplateInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreateCertificateTemplate),
	}.WithSuccess(httpx.Created("证书模板创建成功"), http.StatusBadRequest).Handle(c)
}

// CreateQuestionTag 创建题库标签
// @Summary 创建题库标签
// @Description 管理员/讲师创建题库标签
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.QuestionTagInput true "题库标签"
// @Success 201 {object} response.R{data=training.QuestionTagDict} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question-tag [post]
func (h *handler) CreateQuestionTag(c *gin.Context) {
	httpx.Endpoint[QuestionTagInput, QuestionTagDict]{
		Parse:  httpx.BindJSONMsgFunc[QuestionTagInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreateQuestionTag),
	}.WithSuccess(httpx.Created("题库标签创建成功"), http.StatusBadRequest).Handle(c)
}

// catalogUpdateReq 目录实体更新请求的公共容器（ID 来自路径，In 来自 body）。
//
// 六个目录实体（专业方向 / 课程等级 / 证书模板 / 题库标签 / 目标证件 / 岗位）的更新请求完全同形，
// 只此一份声明（ADR-0060 决策 10）：实体差异只在 In 的 typed input 上，不进 wire
// —— 本容器不经 JSON 绑定（body 由 Parse 里的 httpx.BindJSONMsg / ShouldBindJSON 绑成 training.XInput），
// swagger 注解指认的请求体仍是 training.XInput，契约零变更。
// 岗位一项的留痕：此前 ID 在 Invoke 里从 path 取，属越层的小样板，已收进本容器。
type catalogUpdateReq[I any] struct {
	ID int
	In I
}

// UpdateSpecialty 更新专业方向
// @Summary 更新专业方向
// @Description 管理员更新专业方向字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param specialty_id path int true "专业方向ID"
// @Param body body training.SpecialtyInput true "专业方向"
// @Success 200 {object} response.R{data=training.SpecialtyDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "专业方向不存在"
// @Router /admin/specialty/{specialty_id} [put]
func (h *handler) UpdateSpecialty(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[SpecialtyInput], SpecialtyDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[SpecialtyInput], error) {
			id, err := httpx.PathInt(c, "specialty_id", "专业方向ID无效")
			if err != nil {
				return nil, err
			}
			var in SpecialtyInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &catalogUpdateReq[SpecialtyInput]{ID: id, In: in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[SpecialtyInput]) (SpecialtyDict, error) {
			return h.svc.UpdateSpecialty(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("专业方向更新成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrSpecialtyNotFound, http.StatusNotFound).Handle(c)
}

// UpdateLevel 更新课程等级
// @Summary 更新课程等级
// @Description 管理员更新课程等级字典项
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param level_id path int true "等级ID"
// @Param body body training.LevelInput true "课程等级"
// @Success 200 {object} response.R{data=training.LevelDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "课程等级不存在"
// @Router /admin/level/{level_id} [put]
func (h *handler) UpdateLevel(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[LevelInput], LevelDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[LevelInput], error) {
			id, err := httpx.PathInt(c, "level_id", "课程等级ID无效")
			if err != nil {
				return nil, err
			}
			var in LevelInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &catalogUpdateReq[LevelInput]{ID: id, In: in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[LevelInput]) (LevelDict, error) {
			return h.svc.UpdateLevel(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("课程等级更新成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseLevelNotFound, http.StatusNotFound).Handle(c)
}

// UpdateCertificateTemplate 更新证书模板
// @Summary 更新证书模板
// @Description 管理员更新证书模板
// @Tags 管理端-培训目录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "模板ID"
// @Param body body training.CertificateTemplateInput true "证书模板"
// @Success 200 {object} response.R{data=training.CertificateTemplateDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "证书模板不存在"
// @Router /admin/certificate-template/{id} [put]
func (h *handler) UpdateCertificateTemplate(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[CertificateTemplateInput], CertificateTemplateDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[CertificateTemplateInput], error) {
			id, err := httpx.PathInt(c, "id", "证书模板ID无效")
			if err != nil {
				return nil, err
			}
			var in CertificateTemplateInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &catalogUpdateReq[CertificateTemplateInput]{ID: id, In: in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[CertificateTemplateInput]) (CertificateTemplateDict, error) {
			return h.svc.UpdateCertificateTemplate(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("证书模板更新成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCertificateTemplateNotFound, http.StatusNotFound).Handle(c)
}

// UpdateQuestionTag 更新题库标签
// @Summary 更新题库标签
// @Description 管理员/讲师更新题库标签
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "标签ID"
// @Param body body training.QuestionTagInput true "题库标签"
// @Success 200 {object} response.R{data=training.QuestionTagDict} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "题库标签不存在"
// @Router /admin/question-tag/{id} [put]
func (h *handler) UpdateQuestionTag(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[QuestionTagInput], QuestionTagDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[QuestionTagInput], error) {
			id, err := httpx.PathInt(c, "id", "题库标签ID无效")
			if err != nil {
				return nil, err
			}
			var in QuestionTagInput
			if err := c.ShouldBindJSON(&in); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			return &catalogUpdateReq[QuestionTagInput]{ID: id, In: in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[QuestionTagInput]) (QuestionTagDict, error) {
			return h.svc.UpdateQuestionTag(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("题库标签更新成功"), http.StatusInternalServerError).
		WithSentinel(ErrQuestionTagNotFound, http.StatusNotFound).Handle(c)
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
func (h *handler) DeleteSpecialty(c *gin.Context) {
	httpx.Endpoint[specialtyIDReq, struct{}]{
		Parse: func(c *gin.Context) (*specialtyIDReq, error) {
			id, err := httpx.PathInt(c, "specialty_id", "专业方向ID无效")
			if err != nil {
				return nil, err
			}
			return &specialtyIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req specialtyIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteSpecialty(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("专业方向删除成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrSpecialtyNotFound, http.StatusNotFound).Handle(c)
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
func (h *handler) DeleteLevel(c *gin.Context) {
	httpx.Endpoint[levelIDReq, struct{}]{
		Parse: func(c *gin.Context) (*levelIDReq, error) {
			id, err := httpx.PathInt(c, "level_id", "课程等级ID无效")
			if err != nil {
				return nil, err
			}
			return &levelIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req levelIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteLevel(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("课程等级删除成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCourseLevelNotFound, http.StatusNotFound).Handle(c)
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
func (h *handler) DeleteCertificateTemplate(c *gin.Context) {
	httpx.Endpoint[certificateTemplateIDReq, struct{}]{
		Parse: func(c *gin.Context) (*certificateTemplateIDReq, error) {
			id, err := httpx.PathInt(c, "id", "证书模板ID无效")
			if err != nil {
				return nil, err
			}
			return &certificateTemplateIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req certificateTemplateIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteCertificateTemplate(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("证书模板删除成功"), http.StatusInternalServerError).
		WithSentinel(model.ErrCertificateTemplateNotFound, http.StatusNotFound).Handle(c)
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
func (h *handler) DeleteQuestionTag(c *gin.Context) {
	httpx.Endpoint[questionTagIDReq, struct{}]{
		Parse: func(c *gin.Context) (*questionTagIDReq, error) {
			id, err := httpx.PathInt(c, "id", "题库标签ID无效")
			if err != nil {
				return nil, err
			}
			return &questionTagIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req questionTagIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeleteQuestionTag(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("题库标签删除成功"), http.StatusInternalServerError).
		WithSentinel(ErrQuestionTagNotFound, http.StatusNotFound).Handle(c)
}

// SortFacts400 是「交换排序」这一族端点共用的输入不合法事实（ADR-0065 决策 3）。
// 课程域与培训域各自建表；两枚哨兵的唯一出处是本包依赖的 internal/sortorder（波 4d 起，
// 不再跨包复用本表 —— 课程域 import 不了本包：internal/training/catalog_service.go 已 import 它）。
// 5 个端点挂同一份表，默认面一律 500：改之前它们是 `WithSuccess(…, 400)`，于是
// 「写库/查库失败」与「不支持排序」「待交换的项不存在」挤在同一格，还把驱动原文
// （`SQL logic error: no such table: …`）当 400 的说明发给客户端。
var SortFacts400 = []error{sortorder.ErrEntityNotSortable, sortorder.ErrSwapItemNotFound}

// catalogSwapSortReq 交换排序请求（ID 来自路径，SwapWith 来自 body）。
// 三个可排序的目录实体（专业方向 / 课程等级 / 目标证件）的 swap 请求完全同形，
// 只此一份声明（ADR-0060 决策 10）；岗位的 swap 请求另有一份，见 positionSwapSortReq。
type catalogSwapSortReq struct {
	ID       int
	SwapWith int
}

// catalogSwapSortParse 交换排序端点的 Parse 单点（ADR-0060 决策 10）：三处除「路径参数名 + ID
// 无效文案」外逐字相同，实参按端点传入，不留第二份实现。
// body 绑定形态仍是只带 swap_with 一个字段的匿名 struct（JSON 字段名不动，wire 契约零变更）；
// swap_with <= 0 的前置校验照旧做在这里——岗位端点刻意不做（既有行为，见 positionSwapSortReqBody）。
func catalogSwapSortParse(idParam, idMsg string) httpx.ParseFunc[catalogSwapSortReq] {
	return func(c *gin.Context) (*catalogSwapSortReq, error) {
		id, err := httpx.PathInt(c, idParam, idMsg)
		if err != nil {
			return nil, err
		}
		var body struct {
			SwapWith int `json:"swap_with"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
			return nil, httpx.BadRequest("swap_with 参数无效")
		}
		return &catalogSwapSortReq{ID: id, SwapWith: body.SwapWith}, nil
	}
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
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/specialty/{specialty_id}/sort [put]
func (h *handler) SwapSpecialtySort(c *gin.Context) {
	httpx.Endpoint[catalogSwapSortReq, struct{}]{
		Parse: catalogSwapSortParse("specialty_id", "专业方向ID无效"),
		Invoke: httpx.Invoke(func(req catalogSwapSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapSpecialtySort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(httpx.OkMsgNoData("排序已交换"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, SortFacts400...).Handle(c)
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
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/level/{level_id}/sort [put]
func (h *handler) SwapLevelSort(c *gin.Context) {
	httpx.Endpoint[catalogSwapSortReq, struct{}]{
		Parse: catalogSwapSortParse("level_id", "课程等级ID无效"),
		Invoke: httpx.Invoke(func(req catalogSwapSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapLevelSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(httpx.OkMsgNoData("排序已交换"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, SortFacts400...).Handle(c)
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
// @Success 200 {object} response.R{data=training.QuestionTagsResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/question/{question_id}/tags [put]
func (h *handler) SetQuestionTags(c *gin.Context) {
	httpx.Endpoint[setQuestionTagsReq, QuestionTagsResultDTO]{
		Parse: func(c *gin.Context) (*setQuestionTagsReq, error) {
			id, err := httpx.PathInt(c, "question_id", "题目ID无效")
			if err != nil {
				return nil, err
			}
			var req struct {
				TagIDs []int `json:"tag_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, httpx.BadRequest("请求参数错误")
			}
			return &setQuestionTagsReq{QuestionID: id, TagIDs: req.TagIDs}, nil
		},
		Invoke: httpx.Invoke(func(req setQuestionTagsReq) (QuestionTagsResultDTO, error) {
			if err := h.svc.SetQuestionTags(req.QuestionID, req.TagIDs); err != nil {
				return QuestionTagsResultDTO{}, err
			}
			// 响应即「实际写入的标签集」回显，故在 Invoke 里成型（服务只负责落库）。
			return QuestionTagsResultDTO{TagIDs: req.TagIDs}, nil
		}),
	}.WithSuccess(httpx.OkMsg("题目标签已更新"), http.StatusBadRequest).Handle(c)
}

// ListPositions 岗位列表（管理端含停用项）
// @Summary 岗位列表
// @Description 管理端岗位字典列表（含停用项）
// @Tags 管理端-岗位字典
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=training.PositionListDTO} "岗位列表"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/positions [get]
func (h *handler) ListPositions(c *gin.Context) {
	items := h.svc.ListPositions(false)
	// 字节形状不变：{"positions": [...]}，只是从内联 gin.H 换成具名 DTO（注解才能指认 data）
	response.Success(c, PositionListDTO{Positions: items})
}

// CreatePosition 创建岗位 POST /api/admin/position
// @Summary 创建岗位
// @Description 管理员创建岗位字典项
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body training.PositionInput true "岗位信息"
// @Success 201 {object} response.R{data=training.PositionDict} "创建成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position [post]
func (h *handler) CreatePosition(c *gin.Context) {
	httpx.Endpoint[PositionInput, PositionDict]{
		Parse:  httpx.BindJSONMsgFunc[PositionInput]("请求数据无效"),
		Invoke: httpx.Invoke(h.svc.CreatePosition),
	}.WithSuccess(httpx.Created("岗位创建成功"), http.StatusBadRequest).Handle(c)
}

// UpdatePosition 更新岗位 PUT /api/admin/position/:position_id
// @Summary 更新岗位
// @Description 管理员更新岗位字典项
// @Tags 管理端-岗位字典
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param position_id path int true "岗位 ID"
// @Param body body training.PositionInput true "岗位信息"
// @Success 200 {object} response.R{data=training.PositionDict} "更新成功"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /admin/position/{position_id} [put]
func (h *handler) UpdatePosition(c *gin.Context) {
	httpx.Endpoint[catalogUpdateReq[PositionInput], PositionDict]{
		Parse: func(c *gin.Context) (*catalogUpdateReq[PositionInput], error) {
			// 先 body 后 path：与既有「Invoke 内先绑定 body、再取 path」的报错优先级逐字一致
			in, err := httpx.BindJSONMsg[PositionInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			id, err := httpx.PathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			return &catalogUpdateReq[PositionInput]{ID: id, In: *in}, nil
		},
		Invoke: httpx.Invoke(func(req catalogUpdateReq[PositionInput]) (PositionDict, error) {
			return h.svc.UpdatePosition(req.ID, req.In)
		}),
	}.WithSuccess(httpx.OkMsg("岗位已更新"), http.StatusBadRequest).Handle(c)
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
// @Failure 500 {object} response.R "写库或查库失败"
// @Router /admin/position/{position_id}/sort [put]
func (h *handler) SwapPositionSort(c *gin.Context) {
	httpx.Endpoint[positionSwapSortReq, struct{}]{
		Parse: func(c *gin.Context) (*positionSwapSortReq, error) {
			id, err := httpx.PathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			body, err := httpx.BindJSONMsg[positionSwapSortReqBody](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &positionSwapSortReq{ID: id, SwapWith: body.SwapWith}, nil
		},
		Invoke: httpx.Invoke(func(req positionSwapSortReq) (struct{}, error) {
			return struct{}{}, h.svc.SwapPositionSort(req.ID, req.SwapWith)
		}),
	}.WithSuccess(httpx.OkMsgNoData("排序已更新"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, SortFacts400...).Handle(c)
}

// positionSwapSortReq 交换岗位排序请求（ID 来自路径，SwapWith 来自 body）。
// 字段与 catalogSwapSortReq 相同但不同源：岗位端点的 Parse 走 httpx.PathInt + httpx.BindJSONMsg（错误优先级与
// 文案都不同，且不做 swap_with <= 0 的前置校验），是既有行为，不并进共用容器。
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
func (h *handler) DeletePosition(c *gin.Context) {
	httpx.Endpoint[positionIDReq, struct{}]{
		Parse: func(c *gin.Context) (*positionIDReq, error) {
			id, err := httpx.PathInt(c, "position_id", "岗位 ID 无效")
			if err != nil {
				return nil, err
			}
			return &positionIDReq{ID: id}, nil
		},
		Invoke: httpx.Invoke(func(req positionIDReq) (struct{}, error) {
			return struct{}{}, h.svc.DeletePosition(req.ID)
		}),
	}.WithSuccess(httpx.OkMsgNoData("岗位已删除"), http.StatusBadRequest).Handle(c)
}

// positionIDReq 岗位 ID 路径参数请求。
type positionIDReq struct {
	ID int
}
