// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// QuestionBankHandler 题库管理 handler。
type QuestionBankHandler struct {
	svc     *service.QuestionBankService
	fileSvc *service.FileService
}

// NewQuestionBankHandler 创建题库管理 handler。
func NewQuestionBankHandler(svc *service.QuestionBankService, fileSvc *service.FileService) *QuestionBankHandler {
	return &QuestionBankHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterQuestionBankRoutes 注册 /api/question-bank 蓝图。
func RegisterQuestionBankRoutes(rg *gin.RouterGroup, sess *security.Session, svc *service.QuestionBankService, fileSvc *service.FileService) {
	h := NewQuestionBankHandler(svc, fileSvc)

	g := rg.Group("/question-bank", middleware.JWTAuth(sess))

	// ===== 题目 CRUD =====
	g.GET("/questions", h.ListQuestions)
	g.POST("/questions", middleware.RoleRequired("tutor", "admin"), h.CreateQuestion)
	// 注意：Gin 路由树中静态路径优先于参数路径，batch-publish/batch-import 需在 :question_id 之前注册
	g.POST("/questions/batch-publish", middleware.RoleRequired("admin"), h.BatchPublish)
	g.POST("/questions/batch-reject", middleware.RoleRequired("admin"), h.BatchReject)
	g.POST("/questions/batch-import", middleware.RoleRequired("tutor", "admin"), h.BatchImport)
	g.GET("/questions/:question_id", h.GetQuestion)
	g.PUT("/questions/:question_id", middleware.RoleRequired("tutor", "admin"), h.UpdateQuestion)
	g.DELETE("/questions/:question_id", middleware.RoleRequired("tutor", "admin"), h.DeleteQuestion)
	g.POST("/questions/:question_id/publish", middleware.RoleRequired("admin"), h.PublishQuestion)
	g.POST("/questions/:question_id/reject", middleware.RoleRequired("admin"), h.RejectQuestion)
	g.GET("/stats", h.GetStats)
	g.POST("/upload-image", middleware.RoleRequired("tutor", "admin"), h.UploadImage)
}

// ListQuestions 题目列表分页 GET /api/question-bank/questions
func (h *QuestionBankHandler) ListQuestions(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	pageSize := atoiDefault(c.Query("page_size"), 20)
	qType := c.Query("type")
	status := c.Query("status")
	keyword := c.Query("keyword")
	var tagID *int
	if s := c.Query("tag_id"); s != "" {
		if id, err := strconv.Atoi(s); err == nil {
			tagID = &id
		}
	}
	response.Success(c, h.svc.ListQuestions(page, pageSize, qType, status, keyword, tagID))
}

// CreateQuestion 创建题目 POST /api/question-bank/questions
func (h *QuestionBankHandler) CreateQuestion(c *gin.Context) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	role, _ := c.Get(string(middleware.CtxUserRole))
	userID, _ := uid.(int)
	roleStr, _ := role.(string)
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		response.BadRequest(c, "请求数据无效")
		return
	}
	result, err := h.svc.CreateQuestion(data, &userID, roleStr)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, "题目创建成功", result)
}

// BatchPublish 批量发布（仅管理员）POST /api/question-bank/questions/batch-publish
func (h *QuestionBankHandler) BatchPublish(c *gin.Context) {
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
	result := h.svc.BatchPublish(req.QuestionIDs)
	response.SuccessWithMsg(c, "成功发布"+strconv.Itoa(result["published_count"].(int))+"道题目", result)
}

// BatchReject 批量驳回（仅管理员）POST /api/question-bank/questions/batch-reject
func (h *QuestionBankHandler) BatchReject(c *gin.Context) {
	var req struct {
		QuestionIDs []int  `json:"question_ids"`
		Reason      string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	if len(req.QuestionIDs) == 0 {
		response.BadRequest(c, "请选择要驳回的题目")
		return
	}
	result, err := h.svc.BatchReject(req.QuestionIDs, req.Reason)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "成功驳回"+strconv.Itoa(result["rejected_count"].(int))+"道题目", result)
}

// BatchImport 批量导入 POST /api/question-bank/questions/batch-import
func (h *QuestionBankHandler) BatchImport(c *gin.Context) {
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
	result := h.svc.BatchImport(req.Questions, &userID)
	response.SuccessWithMsg(c, "成功导入"+strconv.Itoa(result["success_count"].(int))+"道题目", result)
}

// GetQuestion 题目详情 GET /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) GetQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	result, err := h.svc.GetQuestion(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, result)
}

// UpdateQuestion 更新题目 PUT /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) UpdateQuestion(c *gin.Context) {
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
	result, err := h.svc.UpdateQuestion(id, data)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题目更新成功", result)
}

// DeleteQuestion 删除题目 DELETE /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) DeleteQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	if err := h.svc.DeleteQuestion(id); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题目删除成功", nil)
}

// PublishQuestion 发布题目（仅管理员）POST /api/question-bank/questions/:question_id/publish
func (h *QuestionBankHandler) PublishQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	result, err := h.svc.PublishQuestion(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.SuccessWithMsg(c, "题目发布成功", result)
}

// RejectQuestion 驳回题目（仅管理员）POST /api/question-bank/questions/:question_id/reject
func (h *QuestionBankHandler) RejectQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("question_id"))
	if err != nil {
		response.BadRequest(c, "题目ID无效")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}
	result, err := h.svc.RejectQuestion(id, req.Reason)
	if err != nil {
		if err.Error() == "题目不存在" {
			response.NotFound(c, err.Error())
		} else {
			response.BadRequest(c, err.Error())
		}
		return
	}
	response.SuccessWithMsg(c, "题目已驳回", result)
}

// GetStats 题库统计 GET /api/question-bank/stats
func (h *QuestionBankHandler) GetStats(c *gin.Context) {
	response.Success(c, h.svc.GetStats())
}

// UploadImage 上传题目图片 POST /api/question-bank/upload-image
func (h *QuestionBankHandler) UploadImage(c *gin.Context) {
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
	ok, msg := h.fileSvc.ValidateImageFile(file.Filename, file.Size)
	if !ok {
		response.BadRequest(c, msg)
		return
	}
	url, err := h.fileSvc.SaveFile(buf, file.Filename, "images/questions")
	if err != nil {
		response.ServerError(c, "图片上传失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "图片上传成功", gin.H{"url": url})
}
