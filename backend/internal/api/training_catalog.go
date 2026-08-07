// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterTrainingCatalogRoutes 注册培训目录蓝图：
//   - /api/admin/*：专业方向 / 课程等级 / 证书模板 / 题库标签 CRUD 与题目打标（管理端）
//   - /api/catalog/*、/api/specialties、/api/levels、/api/tags：学员端查询
func RegisterTrainingCatalogRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewTrainingCatalogService(db)

	// ===== 学员端查询（公开） =====

	// GET /api/catalog/tree  课程目录树：专业方向 → 等级 → 课程（含章节数）
	rg.GET("/catalog/tree", func(c *gin.Context) {
		response.Success(c, svc.GetCatalogTree())
	})

	// GET /api/specialties  专业方向列表（仅启用项）
	rg.GET("/specialties", func(c *gin.Context) {
		response.Success(c, svc.ListSpecialties(true))
	})

	// GET /api/levels  课程等级列表（仅启用项）
	rg.GET("/levels", func(c *gin.Context) {
		response.Success(c, svc.ListLevels(true))
	})

	// GET /api/tags  题库标签列表（仅启用项）
	rg.GET("/tags", func(c *gin.Context) {
		response.Success(c, svc.ListQuestionTags(true))
	})

	// ===== 管理端 CRUD =====

	g := rg.Group("/admin", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// GET /api/admin/catalog/tree  管理端目录树（含停用项与章节节点）
	g.GET("/catalog/tree", func(c *gin.Context) {
		response.Success(c, svc.GetAdminCatalogTree())
	})

	// ---- 专业方向 ----

	// GET /api/admin/specialties  专业方向列表（含停用项）
	g.GET("/specialties", func(c *gin.Context) {
		response.Success(c, svc.ListSpecialties(false))
	})

	// POST /api/admin/specialty  创建专业方向
	g.POST("/specialty", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateSpecialty(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "专业方向创建成功", result)
	})

	// PUT /api/admin/specialty/:specialty_id  更新专业方向
	g.PUT("/specialty/:specialty_id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("specialty_id"))
		if err != nil {
			response.BadRequest(c, "专业方向ID无效")
			return
		}
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateSpecialty(id, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "专业方向更新成功", result)
	})

	// PUT /api/admin/specialty/:specialty_id/sort  交换专业方向排序（body: {"swap_with": <id>}）
	g.PUT("/specialty/:specialty_id/sort", func(c *gin.Context) {
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
		if err := svc.SwapSpecialtySort(id, body.SwapWith); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "排序已交换", nil)
	})

	// DELETE /api/admin/specialty/:specialty_id  删除专业方向
	g.DELETE("/specialty/:specialty_id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("specialty_id"))
		if err != nil {
			response.BadRequest(c, "专业方向ID无效")
			return
		}
		if err := svc.DeleteSpecialty(id); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "专业方向删除成功", nil)
	})

	// ---- 课程等级 ----

	// GET /api/admin/levels  课程等级列表（含停用项）
	g.GET("/levels", func(c *gin.Context) {
		response.Success(c, svc.ListLevels(false))
	})

	// POST /api/admin/level  创建课程等级
	g.POST("/level", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateLevel(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "课程等级创建成功", result)
	})

	// PUT /api/admin/level/:level_id  更新课程等级
	g.PUT("/level/:level_id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("level_id"))
		if err != nil {
			response.BadRequest(c, "课程等级ID无效")
			return
		}
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateLevel(id, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程等级更新成功", result)
	})

	// PUT /api/admin/level/:level_id/sort  交换课程等级排序（body: {"swap_with": <id>}）
	g.PUT("/level/:level_id/sort", func(c *gin.Context) {
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
		if err := svc.SwapLevelSort(id, body.SwapWith); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "排序已交换", nil)
	})

	// DELETE /api/admin/level/:level_id  删除课程等级
	g.DELETE("/level/:level_id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("level_id"))
		if err != nil {
			response.BadRequest(c, "课程等级ID无效")
			return
		}
		if err := svc.DeleteLevel(id); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程等级删除成功", nil)
	})

	// ---- 证书模板 ----

	// GET /api/admin/certificate-templates  证书模板列表（含停用项）
	g.GET("/certificate-templates", func(c *gin.Context) {
		response.Success(c, svc.ListCertificateTemplates(false))
	})

	// POST /api/admin/certificate-template  创建证书模板
	g.POST("/certificate-template", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateCertificateTemplate(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "证书模板创建成功", result)
	})

	// PUT /api/admin/certificate-template/:id  更新证书模板
	g.PUT("/certificate-template/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "证书模板ID无效")
			return
		}
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateCertificateTemplate(id, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "证书模板更新成功", result)
	})

	// DELETE /api/admin/certificate-template/:id  删除证书模板
	g.DELETE("/certificate-template/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "证书模板ID无效")
			return
		}
		if err := svc.DeleteCertificateTemplate(id); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "证书模板删除成功", nil)
	})

	// ---- 题库标签 ----

	// GET /api/admin/question-tags  题库标签列表（含停用项）
	g.GET("/question-tags", func(c *gin.Context) {
		response.Success(c, svc.ListQuestionTags(false))
	})

	// POST /api/admin/question-tag  创建题库标签
	g.POST("/question-tag", func(c *gin.Context) {
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateQuestionTag(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "题库标签创建成功", result)
	})

	// PUT /api/admin/question-tag/:id  更新题库标签
	g.PUT("/question-tag/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "题库标签ID无效")
			return
		}
		var data map[string]any
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateQuestionTag(id, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题库标签更新成功", result)
	})

	// PUT /api/admin/question-tag/:id/sort  交换题库标签排序（body: {"swap_with": <id>}）
	g.PUT("/question-tag/:id/sort", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "标签ID无效")
			return
		}
		var body struct {
			SwapWith int `json:"swap_with"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.SwapWith <= 0 {
			response.BadRequest(c, "swap_with 参数无效")
			return
		}
		if err := svc.SwapQuestionTagSort(id, body.SwapWith); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "排序已交换", nil)
	})

	// DELETE /api/admin/question-tag/:id  删除题库标签
	g.DELETE("/question-tag/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			response.BadRequest(c, "题库标签ID无效")
			return
		}
		if err := svc.DeleteQuestionTag(id); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题库标签删除成功", nil)
	})

	// ---- 题目打标 ----

	// GET /api/admin/question/:question_id/tags  查询题目标签
	g.GET("/question/:question_id/tags", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		result, err := svc.GetQuestionTags(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// PUT /api/admin/question/:question_id/tags  全量替换题目标签
	g.PUT("/question/:question_id/tags", func(c *gin.Context) {
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
		if err := svc.SetQuestionTags(id, req.TagIDs); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题目标签已更新", map[string]any{"tag_ids": req.TagIDs})
	})
}
