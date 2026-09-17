// Package api 实现 HTTP handlers。
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

// questionBankErrStatus 题库域哨兵→状态码表（#611）：题目不存在 → 404，
// 其余（状态/原因校验等业务错误）兜底 400。
var questionBankErrStatus = &errStatusTable{
	entries: []errStatusEntry{
		{service.ErrQuestionNotFound, http.StatusNotFound},
	},
	fallback: http.StatusBadRequest,
}

// QuestionBankHandler 题库管理 handler。
type QuestionBankHandler struct {
	svc     *service.QuestionBankService
	fileSvc *service.FileStore
}

// NewQuestionBankHandler 创建题库管理 handler。
func NewQuestionBankHandler(svc *service.QuestionBankService, fileSvc *service.FileStore) *QuestionBankHandler {
	return &QuestionBankHandler{svc: svc, fileSvc: fileSvc}
}

// RegisterQuestionBankRoutes 注册 /api/question-bank 蓝图。
func RegisterQuestionBankRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.QuestionBankService, fileSvc *service.FileStore) {
	h := NewQuestionBankHandler(svc, fileSvc)

	g := rg.Group("/question-bank", middleware.JWTAuth(rd.Session), middleware.CredentialScoped(rd.CredentialScope))

	// ===== 题目 CRUD =====
	g.GET("/questions", h.ListQuestions)
	g.POST("/questions", middleware.CapabilityRequired(authz.CapQuestionAuthor), h.CreateQuestion)
	// 注意：Gin 路由树中静态路径优先于参数路径，batch-publish/batch-import 需在 :question_id 之前注册
	g.POST("/questions/batch-publish", middleware.CapabilityRequired(authz.CapQuestionReview), h.BatchPublish)
	g.POST("/questions/batch-reject", middleware.CapabilityRequired(authz.CapQuestionReview), h.BatchReject)
	g.POST("/questions/batch-import", middleware.CapabilityRequired(authz.CapQuestionAuthor), h.BatchImport)
	g.GET("/questions/:question_id", h.GetQuestion)
	g.PUT("/questions/:question_id", middleware.CapabilityRequired(authz.CapQuestionAuthor), h.UpdateQuestion)
	g.DELETE("/questions/:question_id", middleware.CapabilityRequired(authz.CapQuestionAuthor), h.DeleteQuestion)
	g.POST("/questions/:question_id/publish", middleware.CapabilityRequired(authz.CapQuestionReview), h.PublishQuestion)
	g.POST("/questions/:question_id/reject", middleware.CapabilityRequired(authz.CapQuestionReview), h.RejectQuestion)
	g.GET("/stats", h.GetStats)
	g.POST("/upload-image", middleware.CapabilityRequired(authz.CapQuestionAuthor), h.UploadImage)
}

// listQuestionsReq 题目列表查询参数。
type listQuestionsReq struct {
	Page         int
	PageSize     int
	QType        string
	Status       string
	Keyword      string
	TagID        *int
	CredentialID *int
	// Sort 排序口径（#412）：缺省 = 现状（最新提交优先）；讲师端显式传 id_asc。
	Sort string
}

// ListQuestions 题目列表分页
// @Summary 题目列表
// @Description 题库分页查询（可按题型/状态/关键词/标签/证件过滤；sort=id_asc 按 ID 升序，缺省最新提交优先）
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param type query string false "题型 single_choice|multi_choice|true_false|fault_image|short_answer"
// @Param status query string false "状态 draft|pending|published"
// @Param keyword query string false "关键词"
// @Param tag_id query int false "题库标签ID"
// @Param credential_id query int false "目标证件ID"
// @Param sort query string false "排序口径 id_asc"
// @Success 200 {object} response.R{data=service.QuestionPageDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions [get]
func (h *QuestionBankHandler) ListQuestions(c *gin.Context) {
	Endpoint[listQuestionsReq, service.QuestionPageDTO]{
		Parse: func(c *gin.Context) (*listQuestionsReq, error) {
			return &listQuestionsReq{
				Page:         atoiDefault(c.Query("page"), 1),
				PageSize:     atoiDefault(c.Query("page_size"), 20),
				QType:        c.Query("type"),
				Status:       c.Query("status"),
				Keyword:      c.Query("keyword"),
				TagID:        queryIDPtr(c, "tag_id"),
				CredentialID: middleware.CredentialIDPtr(c),
				Sort:         c.Query("sort"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listQuestionsReq) (*service.QuestionPageDTO, error) {
			return h.svc.ListQuestions(req.Page, req.PageSize, req.QType, req.Status, req.Keyword, req.TagID, req.CredentialID, req.Sort)
		},
		Render: func(c *gin.Context, _ *listQuestionsReq, resp *service.QuestionPageDTO, err error) {
			if err != nil {
				response.ServerError(c, err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// createQuestionReq 创建题目请求（含 body、createdBy 指针与类型）。
type createQuestionReq struct {
	Data          map[string]any
	UserID        int
	CreatedByType string
}

// CreateQuestion 创建题目
// @Summary 创建题目
// @Description 创建题目（讲师/管理员，需 CapQuestionAuthor）
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "题目" example({"type":"single_choice","content":"题干","options":{"A":"选项A"},"answer":"A","score":3})
// @Success 201 {object} response.R{data=service.QuestionDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions [post]
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

// BatchPublish 批量发布（仅管理员）
// @Summary 批量发布题目
// @Description 批量把待审题目置为已发布，返回发布条数
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "题目ID列表" example({"question_ids":[1,2]})
// @Success 200 {object} response.R{data=service.QuestionPublishResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions/batch-publish [post]
func (h *QuestionBankHandler) BatchPublish(c *gin.Context) {
	Endpoint[batchPublishReq, service.QuestionPublishResultDTO]{
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
		Invoke: func(ctx context.Context, req *batchPublishReq) (*service.QuestionPublishResultDTO, error) {
			return h.svc.BatchPublish(req.QuestionIDs), nil
		},
		Render: func(c *gin.Context, _ *batchPublishReq, resp *service.QuestionPublishResultDTO, _ error) {
			response.SuccessWithMsg(c, "成功发布"+strconv.Itoa(resp.PublishedCount)+"道题目", *resp)
		},
	}.Handle(c)
}

// batchRejectReq 批量驳回请求。
type batchRejectReq struct {
	QuestionIDs []int  `json:"question_ids"`
	Reason      string `json:"reason"`
}

// BatchReject 批量驳回（仅管理员）
// @Summary 批量驳回题目
// @Description 批量驳回题目并记录原因，返回驳回条数
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "题目ID列表与原因" example({"question_ids":[1,2],"reason":"题干不完整"})
// @Success 200 {object} response.R{data=service.QuestionRejectResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions/batch-reject [post]
func (h *QuestionBankHandler) BatchReject(c *gin.Context) {
	Endpoint[batchRejectReq, service.QuestionRejectResultDTO]{
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
		Invoke: func(ctx context.Context, req *batchRejectReq) (*service.QuestionRejectResultDTO, error) {
			return h.svc.BatchReject(req.QuestionIDs, req.Reason)
		},
		Render: func(c *gin.Context, _ *batchRejectReq, resp *service.QuestionRejectResultDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "成功驳回"+strconv.Itoa(resp.RejectedCount)+"道题目", *resp)
		},
	}.Handle(c)
}

// batchImportReq 批量导入请求。
type batchImportReq struct {
	Questions []any `json:"questions"`
	UserID    int
}

// BatchImport 批量导入
// @Summary 批量导入题目
// @Description 批量导入题目，返回成功/失败条数与逐条失败原因
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "题目数组" example({"questions":[{"type":"single_choice","content":"题干","answer":"A"}]})
// @Success 200 {object} response.R{data=service.QuestionImportResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions/batch-import [post]
func (h *QuestionBankHandler) BatchImport(c *gin.Context) {
	Endpoint[batchImportReq, service.QuestionImportResultDTO]{
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
		Invoke: func(ctx context.Context, req *batchImportReq) (*service.QuestionImportResultDTO, error) {
			return h.svc.BatchImport(req.Questions, &req.UserID), nil
		},
		Render: func(c *gin.Context, _ *batchImportReq, resp *service.QuestionImportResultDTO, _ error) {
			response.SuccessWithMsg(c, "成功导入"+strconv.Itoa(resp.SuccessCount)+"道题目", *resp)
		},
	}.Handle(c)
}

// questionIDReq 题目 ID 路径参数请求。
type questionIDReq struct {
	ID int
}

// GetQuestion 题目详情
// @Summary 题目详情
// @Description 按 ID 查询题目（含答案/解析，管理面口径）
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R{data=service.QuestionDTO} "success"
// @Failure 404 {object} response.R "题目不存在"
// @Router /question-bank/questions/{question_id} [get]
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
			// 分流读路径（#981）：题库作者/审核者走编辑面（可读 draft），其余（学员）走题库池口径。
			if middleware.HasCapability(c, authz.CapQuestionAuthor) || middleware.HasCapability(c, authz.CapQuestionReview) {
				result, err := h.svc.GetQuestion(req.ID)
				if err != nil {
					return nil, err
				}
				return &result, nil
			}
			result, err := h.svc.GetQuestionForStudent(req.ID, middleware.CredentialIDPtr(c))
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

// UpdateQuestion 更新题目
// @Summary 更新题目
// @Description 按 ID 更新题目字段（讲师/管理员，需 CapQuestionAuthor）
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "题目字段（部分更新）"
// @Success 200 {object} response.R{data=service.QuestionDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions/{question_id} [put]
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

// DeleteQuestion 删除题目
// @Summary 删除题目
// @Description 按 ID 删除题目（讲师/管理员，需 CapQuestionAuthor）；无返回载荷
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R "success"
// @Failure 404 {object} response.R "题目不存在"
// @Router /question-bank/questions/{question_id} [delete]
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

// PublishQuestion 发布题目（仅管理员）
// @Summary 发布题目
// @Description 单题发布（需 CapQuestionReview），返回发布后的题目
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R{data=service.QuestionDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Failure 404 {object} response.R "题目不存在"
// @Router /question-bank/questions/{question_id}/publish [post]
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

// RejectQuestion 驳回题目（仅管理员）
// @Summary 驳回题目
// @Description 单题驳回并记录原因（需 CapQuestionReview），返回驳回后的题目
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "驳回原因" example({"reason":"题干不完整"})
// @Success 200 {object} response.R{data=service.QuestionDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/questions/{question_id}/reject [post]
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
				questionBankErrStatus.renderError(c, err) // #611：错误映射退表，成功文案保留定制
				return
			}
			response.SuccessWithMsg(c, "题目已驳回", deref(resp))
		},
	}.Handle(c)
}

// GetStats 题库统计
// @Summary 题库统计
// @Description 按当前证件题库池口径统计总数/按题型/按状态
// @Tags 题库管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.QuestionBankStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/stats [get]
func (h *QuestionBankHandler) GetStats(c *gin.Context) {
	Endpoint[struct{}, service.QuestionBankStatsDTO]{
		Invoke: func(ctx context.Context, _ *struct{}) (*service.QuestionBankStatsDTO, error) {
			// #413：总数按当前证件题库池口径（拦截器已注入 credential_id；缺省 = 不分区）。
			return h.svc.GetStats(middleware.CredentialIDPtr(c)), nil
		},
		Render: func(c *gin.Context, _ *struct{}, resp *service.QuestionBankStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// UploadImage 上传题目图片
// @Summary 上传题目图片
// @Description multipart 上传题干图片，返回可访问的图片直链
// @Tags 题库管理
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param image formData file true "图片文件"
// @Success 200 {object} response.R{data=service.QuestionImageUploadDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /question-bank/upload-image [post]
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
	ok, msg := h.fileSvc.ValidateImage(file.Filename, file.Size)
	if !ok {
		response.BadRequest(c, msg)
		return
	}
	url, err := h.fileSvc.Save(buf, file.Filename, "images/questions")
	if err != nil {
		response.ServerError(c, "图片上传失败: "+err.Error())
		return
	}
	response.SuccessWithMsg(c, "图片上传成功", service.QuestionImageUploadDTO{URL: url})
}
