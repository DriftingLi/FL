// Package api 实现 HTTP handlers。
// 本文件：培训目录蓝图（专业方向 / 课程等级 / 证书模板 / 题库标签 CRUD + 题目打标 + 学员端查询）。
package api

import (
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
	response.Success(c, h.svc.GetCatalogTree())
}

// ListPublicLevels 课程等级列表（仅启用项）GET /api/levels
func (h *TrainingCatalogHandler) ListPublicLevels(c *gin.Context) {
	response.Success(c, gin.H{"levels": h.svc.ListLevels(true)})
}

// ListPublicTags 题库标签列表（仅启用项）GET /api/tags
func (h *TrainingCatalogHandler) ListPublicTags(c *gin.Context) {
	response.Success(c, gin.H{"tags": h.svc.ListQuestionTags(true)})
}

// GetAdminCatalogTree 管理端目录树（含停用项与章节节点）GET /api/admin/catalog/tree
func (h *TrainingCatalogHandler) GetAdminCatalogTree(c *gin.Context) {
	response.Success(c, h.svc.GetAdminCatalogTree())
}

// ListSpecialties 专业方向列表（含停用项）GET /api/admin/specialties
func (h *TrainingCatalogHandler) ListSpecialties(c *gin.Context) {
	response.Success(c, gin.H{"specialties": h.svc.ListSpecialties(false)})
}

// CreateSpecialty 创建专业方向 POST /api/admin/specialty
func (h *TrainingCatalogHandler) CreateSpecialty(c *gin.Context) {
	var in service.SpecialtyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateSpecialty(in)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "专业方向创建成功", result)
}

// UpdateSpecialty 更新专业方向 PUT /api/admin/specialty/:specialty_id
func (h *TrainingCatalogHandler) UpdateSpecialty(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("specialty_id"))
	if err != nil {
		response.BadRequest(c, "专业方向ID无效")
		return
	}
	var in service.SpecialtyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateSpecialty(id, in)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "专业方向更新成功", result)
}

// SwapSpecialtySort 交换专业方向排序 PUT /api/admin/specialty/:specialty_id/sort（body: {"swap_with": <id>}）
func (h *TrainingCatalogHandler) SwapSpecialtySort(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("specialty_id"))
	if err != nil {
		response.BadRequest(c, "专业方向ID无效")
		return
	}
	var body struct {
		SwapWith int `json:"swap_with"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
		response.BadRequest(c, "swap_with 参数无效")
		return
	}
	if err := h.svc.SwapSpecialtySort(id, body.SwapWith); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "排序已交换", nil)
}

// DeleteSpecialty 删除专业方向 DELETE /api/admin/specialty/:specialty_id
func (h *TrainingCatalogHandler) DeleteSpecialty(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("specialty_id"))
	if err != nil {
		response.BadRequest(c, "专业方向ID无效")
		return
	}
	if err := h.svc.DeleteSpecialty(id); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "专业方向删除成功", nil)
}

// ListLevels 课程等级列表（含停用项）GET /api/admin/levels
func (h *TrainingCatalogHandler) ListLevels(c *gin.Context) {
	response.Success(c, gin.H{"levels": h.svc.ListLevels(false)})
}

// CreateLevel 创建课程等级 POST /api/admin/level
func (h *TrainingCatalogHandler) CreateLevel(c *gin.Context) {
	var in service.LevelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateLevel(in)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "课程等级创建成功", result)
}

// UpdateLevel 更新课程等级 PUT /api/admin/level/:level_id
func (h *TrainingCatalogHandler) UpdateLevel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("level_id"))
	if err != nil {
		response.BadRequest(c, "课程等级ID无效")
		return
	}
	var in service.LevelInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateLevel(id, in)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "课程等级更新成功", result)
}

// SwapLevelSort 交换课程等级排序 PUT /api/admin/level/:level_id/sort（body: {"swap_with": <id>}）
func (h *TrainingCatalogHandler) SwapLevelSort(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("level_id"))
	if err != nil {
		response.BadRequest(c, "课程等级ID无效")
		return
	}
	var body struct {
		SwapWith int `json:"swap_with"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
		response.BadRequest(c, "swap_with 参数无效")
		return
	}
	if err := h.svc.SwapLevelSort(id, body.SwapWith); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "排序已交换", nil)
}

// DeleteLevel 删除课程等级 DELETE /api/admin/level/:level_id
func (h *TrainingCatalogHandler) DeleteLevel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("level_id"))
	if err != nil {
		response.BadRequest(c, "课程等级ID无效")
		return
	}
	if err := h.svc.DeleteLevel(id); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "课程等级删除成功", nil)
}

// ListCertificateTemplates 证书模板列表（含停用项）GET /api/admin/certificate-templates
func (h *TrainingCatalogHandler) ListCertificateTemplates(c *gin.Context) {
	response.Success(c, gin.H{"certificate_templates": h.svc.ListCertificateTemplates(false)})
}

// CreateCertificateTemplate 创建证书模板 POST /api/admin/certificate-template
func (h *TrainingCatalogHandler) CreateCertificateTemplate(c *gin.Context) {
	var in service.CertificateTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateCertificateTemplate(in)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "证书模板创建成功", result)
}

// UpdateCertificateTemplate 更新证书模板 PUT /api/admin/certificate-template/:id
func (h *TrainingCatalogHandler) UpdateCertificateTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "证书模板ID无效")
		return
	}
	var in service.CertificateTemplateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateCertificateTemplate(id, in)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "证书模板更新成功", result)
}

// DeleteCertificateTemplate 删除证书模板 DELETE /api/admin/certificate-template/:id
func (h *TrainingCatalogHandler) DeleteCertificateTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "证书模板ID无效")
		return
	}
	if err := h.svc.DeleteCertificateTemplate(id); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "证书模板删除成功", nil)
}

// ListQuestionTags 题库标签列表（含停用项）GET /api/admin/question-tags
func (h *TrainingCatalogHandler) ListQuestionTags(c *gin.Context) {
	response.Success(c, gin.H{"tags": h.svc.ListQuestionTags(false)})
}

// CreateQuestionTag 创建题库标签 POST /api/admin/question-tag
func (h *TrainingCatalogHandler) CreateQuestionTag(c *gin.Context) {
	var in service.QuestionTagInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateQuestionTag(in)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "题库标签创建成功", result)
}

// UpdateQuestionTag 更新题库标签 PUT /api/admin/question-tag/:id
func (h *TrainingCatalogHandler) UpdateQuestionTag(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "题库标签ID无效")
		return
	}
	var in service.QuestionTagInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.UpdateQuestionTag(id, in)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题库标签更新成功", result)
}

// DeleteQuestionTag 删除题库标签 DELETE /api/admin/question-tag/:id
func (h *TrainingCatalogHandler) DeleteQuestionTag(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "题库标签ID无效")
		return
	}
	if err := h.svc.DeleteQuestionTag(id); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题库标签删除成功", nil)
}

// SetQuestionTags 全量替换题目标签 PUT /api/admin/question/:question_id/tags
func (h *TrainingCatalogHandler) SetQuestionTags(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	var req struct {
		TagIDs []int `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if err := h.svc.SetQuestionTags(id, req.TagIDs); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题目标签已更新", map[string]any{"tag_ids": req.TagIDs})
}
