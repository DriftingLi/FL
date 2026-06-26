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

// RegisterQuestionBankRoutes 注册 /api/question-bank 蓝图。
// 对应 Python app/api/question_bank.py。
func RegisterQuestionBankRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewQuestionBankService(db)
	fileSvc := service.NewFileService(cfg.UploadFolder)

	g := rg.Group("/question-bank", middleware.JWTAuth(cfg))

	// ===== 题目 CRUD =====

	// GET /api/question-bank/questions  题目列表分页
	g.GET("/questions", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 20)
		level := c.Query("level")
		qType := c.Query("type")
		status := c.Query("status")
		keyword := c.Query("keyword")
		var kpID *int
		if s := c.Query("knowledge_point_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				kpID = &id
			}
		}
		response.Success(c, svc.ListQuestions(page, pageSize, level, qType, kpID, status, keyword))
	})

	// POST /api/question-bank/questions  创建题目
	g.POST("/questions", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		role, _ := c.Get(string(middleware.CtxUserRole))
		userID, _ := uid.(int)
		roleStr, _ := role.(string)
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateQuestion(data, &userID, roleStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "题目创建成功", result)
	})

	// GET /api/question-bank/questions/batch-publish  与下方 :question_id 路由冲突处理
	// 注意：Gin 路由树中静态路径优先于参数路径，batch-publish/batch-import 需在 :question_id 之前注册
	// POST /api/question-bank/questions/batch-publish  批量发布
	g.POST("/questions/batch-publish", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		var req struct {
			QuestionIDs []int `json:"question_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.QuestionIDs) == 0 {
			response.BadRequest(c, "请选择要发布的题目")
			return
		}
		result := svc.BatchPublish(req.QuestionIDs)
		response.SuccessWithMsg(c, "成功发布"+strconv.Itoa(result["published_count"].(int))+"道题目", result)
	})

	// POST /api/question-bank/questions/batch-import  批量导入
	g.POST("/questions/batch-import", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		uid, _ := c.Get(string(middleware.CtxUserID))
		userID, _ := uid.(int)
		var req struct {
			Questions []interface{} `json:"questions"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if len(req.Questions) == 0 {
			response.BadRequest(c, "导入数据不能为空")
			return
		}
		result := svc.BatchImport(req.Questions, &userID)
		response.SuccessWithMsg(c, "成功导入"+strconv.Itoa(result["success_count"].(int))+"道题目", result)
	})

	// GET /api/question-bank/questions/:question_id  题目详情
	g.GET("/questions/:question_id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		result, err := svc.GetQuestion(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// PUT /api/question-bank/questions/:question_id  更新题目
	g.PUT("/questions/:question_id", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateQuestion(id, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题目更新成功", result)
	})

	// DELETE /api/question-bank/questions/:question_id  删除题目
	g.DELETE("/questions/:question_id", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		if err := svc.DeleteQuestion(id); err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题目删除成功", nil)
	})

	// POST /api/question-bank/questions/:question_id/publish  发布题目
	g.POST("/questions/:question_id/publish", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("question_id"))
		if err != nil {
			response.BadRequest(c, "题目ID无效")
			return
		}
		result, err := svc.PublishQuestion(id)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "题目发布成功", result)
	})

	// GET /api/question-bank/stats  题库统计
	g.GET("/stats", func(c *gin.Context) {
		response.Success(c, svc.GetStats())
	})

	// ===== 知识点 =====

	// GET /api/question-bank/knowledge-points  知识点列表
	g.GET("/knowledge-points", func(c *gin.Context) {
		level := c.Query("level")
		var parentID *int
		if s := c.Query("parent_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				parentID = &id
			}
		}
		response.Success(c, svc.GetKnowledgePoints(level, parentID))
	})

	// POST /api/question-bank/knowledge-points  创建知识点
	g.POST("/knowledge-points", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.CreateKnowledgePoint(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "知识点创建成功", result)
	})

	// PUT /api/question-bank/knowledge-points/:kp_id  更新知识点
	g.PUT("/knowledge-points/:kp_id", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("kp_id"))
		if err != nil {
			response.BadRequest(c, "知识点ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := svc.UpdateKnowledgePoint(id, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "知识点更新成功", result)
	})

	// DELETE /api/question-bank/knowledge-points/:kp_id  删除知识点
	g.DELETE("/knowledge-points/:kp_id", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("kp_id"))
		if err != nil {
			response.BadRequest(c, "知识点ID无效")
			return
		}
		if err := svc.DeleteKnowledgePoint(id); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "知识点删除成功", nil)
	})

	// ===== 图片上传 =====

	// POST /api/question-bank/upload-image  上传题目图片
	g.POST("/upload-image", middleware.RoleRequired("tutor", "admin"), func(c *gin.Context) {
		file, err := c.FormFile("image")
		if err != nil {
			response.BadRequest(c, "未找到上传文件")
			return
		}
		if file.Filename == "" {
			response.BadRequest(c, "未选择文件")
			return
		}
		content, err := file.Open()
		if err != nil {
			response.ServerError(c, "图片上传失败")
			return
		}
		defer content.Close()
		buf := make([]byte, file.Size)
		if _, err := content.Read(buf); err != nil {
			response.ServerError(c, "图片上传失败")
			return
		}
		ok, msg := fileSvc.ValidateImageFile(file.Filename, file.Size)
		if !ok {
			response.BadRequest(c, msg)
			return
		}
		url, _ := fileSvc.SaveFile(buf, file.Filename, "images/questions")
		response.SuccessWithMsg(c, "图片上传成功", gin.H{"url": url})
	})
}
