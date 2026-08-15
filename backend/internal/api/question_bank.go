// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
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
func RegisterQuestionBankRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.QuestionBankService, fileSvc *service.FileService) {
	h := NewQuestionBankHandler(svc, fileSvc)

	g := rg.Group("/question-bank", middleware.JWTAuth(rd.Session))

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

// listQuestionsReq 题目列表查询参数。
type listQuestionsReq struct {
	Page     int
	PageSize int
	QType    string
	Status   string
	Keyword  string
	TagID    *int
}

// ListQuestions 题目列表分页 GET /api/question-bank/questions
func (h *QuestionBankHandler) ListQuestions(c *gin.Context) {
	Endpoint[listQuestionsReq, map[string]any]{
		Parse: func(c *gin.Context) (*listQuestionsReq, error) {
			return &listQuestionsReq{
				Page:     atoiDefault(c.Query("page"), 1),
				PageSize: atoiDefault(c.Query("page_size"), 20),
				QType:    c.Query("type"),
				Status:   c.Query("status"),
				Keyword:  c.Query("keyword"),
				TagID:    queryIDPtr(c, "tag_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listQuestionsReq) (*map[string]any, error) {
			result := h.svc.ListQuestions(req.Page, req.PageSize, req.QType, req.Status, req.Keyword, req.TagID)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *listQuestionsReq, resp *map[string]any, _ error) {
			response.Success(c, deref(resp))
		},
	}.Handle(c)
}

// createQuestionReq 创建题目请求（含 body、createdBy 指针与类型）。
type createQuestionReq struct {
	Data          map[string]any
	UserID        int
	CreatedByType string
}

// CreateQuestion 创建题目 POST /api/question-bank/questions
func (h *QuestionBankHandler) CreateQuestion(c *gin.Context) {
	Endpoint[createQuestionReq, service.QuestionDTO]{
		Parse: func(c *gin.Context) (*createQuestionReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			role, _ := c.Get(string(middleware.CtxUserRole))
			userID, _ := uid.(int)
			roleStr, _ := role.(string)
			var data map[string]any
			if err := c.ShouldBindJSON(&data); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &createQuestionReq{Data: data, UserID: userID, CreatedByType: roleStr}, nil
		},
		Invoke: func(ctx context.Context, req *createQuestionReq) (*service.QuestionDTO, error) {
			result, err := h.svc.CreateQuestion(req.Data, &req.UserID, req.CreatedByType)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *createQuestionReq, resp *service.QuestionDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Created(c, "题目创建成功", deref(resp))
		},
	}.Handle(c)
}

// batchPublishReq 批量发布请求。
type batchPublishReq struct {
	QuestionIDs []int `json:"question_ids"`
}

// BatchPublish 批量发布（仅管理员）POST /api/question-bank/questions/batch-publish
func (h *QuestionBankHandler) BatchPublish(c *gin.Context) {
	Endpoint[batchPublishReq, map[string]any]{
		Parse: func(c *gin.Context) (*batchPublishReq, error) {
			req, err := bindJSON[batchPublishReq](c)
			if err != nil {
				return nil, err
			}
			if len(req.QuestionIDs) == 0 {
				return nil, badRequest("请选择要发布的题目")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *batchPublishReq) (*map[string]any, error) {
			result := h.svc.BatchPublish(req.QuestionIDs)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *batchPublishReq, resp *map[string]any, _ error) {
			result := deref(resp)
			m := result.(map[string]any)
			response.SuccessWithMsg(c, "成功发布"+strconv.Itoa(m["published_count"].(int))+"道题目", result)
		},
	}.Handle(c)
}

// batchRejectReq 批量驳回请求。
type batchRejectReq struct {
	QuestionIDs []int  `json:"question_ids"`
	Reason      string `json:"reason"`
}

// BatchReject 批量驳回（仅管理员）POST /api/question-bank/questions/batch-reject
func (h *QuestionBankHandler) BatchReject(c *gin.Context) {
	Endpoint[batchRejectReq, map[string]any]{
		Parse: func(c *gin.Context) (*batchRejectReq, error) {
			req, err := bindJSON[batchRejectReq](c)
			if err != nil {
				return nil, err
			}
			if len(req.QuestionIDs) == 0 {
				return nil, badRequest("请选择要驳回的题目")
			}
			return req, nil
		},
		Invoke: func(ctx context.Context, req *batchRejectReq) (*map[string]any, error) {
			result, err := h.svc.BatchReject(req.QuestionIDs, req.Reason)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *batchRejectReq, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			m := deref(resp).(map[string]any)
			response.SuccessWithMsg(c, "成功驳回"+strconv.Itoa(m["rejected_count"].(int))+"道题目", deref(resp))
		},
	}.Handle(c)
}

// batchImportReq 批量导入请求。
type batchImportReq struct {
	Questions []any `json:"questions"`
	UserID    int
}

// BatchImport 批量导入 POST /api/question-bank/questions/batch-import
func (h *QuestionBankHandler) BatchImport(c *gin.Context) {
	Endpoint[batchImportReq, map[string]any]{
		Parse: func(c *gin.Context) (*batchImportReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			userID, _ := uid.(int)
			var req struct {
				Questions []any `json:"questions"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			if len(req.Questions) == 0 {
				return nil, badRequest("导入数据不能为空")
			}
			return &batchImportReq{Questions: req.Questions, UserID: userID}, nil
		},
		Invoke: func(ctx context.Context, req *batchImportReq) (*map[string]any, error) {
			result := h.svc.BatchImport(req.Questions, &req.UserID)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *batchImportReq, resp *map[string]any, _ error) {
			m := deref(resp).(map[string]any)
			response.SuccessWithMsg(c, "成功导入"+strconv.Itoa(m["success_count"].(int))+"道题目", deref(resp))
		},
	}.Handle(c)
}

// questionIDReq 题目 ID 路径参数请求。
type questionIDReq struct {
	ID int
}

// GetQuestion 题目详情 GET /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) GetQuestion(c *gin.Context) {
	Endpoint[questionIDReq, service.QuestionDTO]{
		Parse: func(c *gin.Context) (*questionIDReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			return &questionIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *questionIDReq) (*service.QuestionDTO, error) {
			result, err := h.svc.GetQuestion(req.ID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *questionIDReq, resp *service.QuestionDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, deref(resp))
		},
	}.Handle(c)
}

// updateQuestionReq 更新题目请求。
type updateQuestionReq struct {
	ID   int
	Data map[string]any
}

// UpdateQuestion 更新题目 PUT /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) UpdateQuestion(c *gin.Context) {
	Endpoint[updateQuestionReq, service.QuestionDTO]{
		Parse: func(c *gin.Context) (*updateQuestionReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			var data map[string]any
			if err := c.ShouldBindJSON(&data); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &updateQuestionReq{ID: id, Data: data}, nil
		},
		Invoke: func(ctx context.Context, req *updateQuestionReq) (*service.QuestionDTO, error) {
			result, err := h.svc.UpdateQuestion(req.ID, req.Data)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *updateQuestionReq, resp *service.QuestionDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题目更新成功", deref(resp))
		},
	}.Handle(c)
}

// DeleteQuestion 删除题目 DELETE /api/question-bank/questions/:question_id
func (h *QuestionBankHandler) DeleteQuestion(c *gin.Context) {
	Endpoint[questionIDReq, struct{}]{
		Parse: func(c *gin.Context) (*questionIDReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			return &questionIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *questionIDReq) (*struct{}, error) {
			if err := h.svc.DeleteQuestion(req.ID); err != nil {
				return nil, err
			}
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *questionIDReq, resp *struct{}, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题目删除成功", nil)
		},
	}.Handle(c)
}

// PublishQuestion 发布题目（仅管理员）POST /api/question-bank/questions/:question_id/publish
func (h *QuestionBankHandler) PublishQuestion(c *gin.Context) {
	Endpoint[questionIDReq, service.QuestionDTO]{
		Parse: func(c *gin.Context) (*questionIDReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			return &questionIDReq{ID: id}, nil
		},
		Invoke: func(ctx context.Context, req *questionIDReq) (*service.QuestionDTO, error) {
			result, err := h.svc.PublishQuestion(req.ID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *questionIDReq, resp *service.QuestionDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "题目发布成功", deref(resp))
		},
	}.Handle(c)
}

// rejectQuestionReq 驳回题目请求。
type rejectQuestionReq struct {
	ID     int
	Reason string
}

// RejectQuestion 驳回题目（仅管理员）POST /api/question-bank/questions/:question_id/reject
func (h *QuestionBankHandler) RejectQuestion(c *gin.Context) {
	Endpoint[rejectQuestionReq, service.QuestionDTO]{
		Parse: func(c *gin.Context) (*rejectQuestionReq, error) {
			id, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			var req struct {
				Reason string `json:"reason"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求参数错误")
			}
			return &rejectQuestionReq{ID: id, Reason: req.Reason}, nil
		},
		Invoke: func(ctx context.Context, req *rejectQuestionReq) (*service.QuestionDTO, error) {
			result, err := h.svc.RejectQuestion(req.ID, req.Reason)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *rejectQuestionReq, resp *service.QuestionDTO, err error) {
			if err != nil {
				if err.Error() == "题目不存在" {
					response.NotFound(c, err.Error())
				} else {
					response.BadRequest(c, err.Error())
				}
				return
			}
			response.SuccessWithMsg(c, "题目已驳回", deref(resp))
		},
	}.Handle(c)
}

// GetStats 题库统计 GET /api/question-bank/stats
func (h *QuestionBankHandler) GetStats(c *gin.Context) {
	Endpoint[struct{}, service.QuestionBankStatsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.QuestionBankStatsDTO, error) {
			return h.svc.GetStats(), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.QuestionBankStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
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
