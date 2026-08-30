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

// WrongQuestionHandler 错题本 handler。
type WrongQuestionHandler struct {
	svc *service.WrongQuestionService
}

// NewWrongQuestionHandler 创建错题本 handler。
func NewWrongQuestionHandler(svc *service.WrongQuestionService) *WrongQuestionHandler {
	return &WrongQuestionHandler{svc: svc}
}

// RegisterWrongQuestionRoutes 注册 /api/wrong-questions 蓝图。
func RegisterWrongQuestionRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.WrongQuestionService) {
	h := NewWrongQuestionHandler(svc)

	g := rg.Group("/wrong-questions", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	// GET /api/wrong-questions  错题列表（分页+过滤）
	g.GET("", h.List)
	// POST /api/wrong-questions/:question_id/redo  重做错题
	g.POST("/:question_id/redo", h.Redo)
	// POST /api/wrong-questions/batch-remove  批量移出
	g.POST("/batch-remove", h.BatchRemove)
	// POST /api/wrong-questions/:question_id/remove  移出错题本
	g.POST("/:question_id/remove", h.Remove)
	// GET /api/wrong-questions/stats  错题统计
	g.GET("/stats", h.GetStats)
	// GET /api/wrong-questions/export  导出错题本（纯文本附件）
	g.GET("/export", h.Export)
}

// listWrongQuestionsReq 错题列表查询请求。
type listWrongQuestionsReq struct {
	StudentID     int
	Page          int
	PageSize      int
	QType         string
	MinWrongCount *int
	Favorited     bool
	Sort          string
	CredentialID  *int
}

// List 错题列表
// @Summary 错题列表
// @Description 分页查询错题，支持按题型/错误次数/收藏过滤与时间排序
// @Tags 学员端-错题本
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param type query string false "题型"
// @Param min_wrong_count query int false "最小错误次数"
// @Param favorited query bool false "仅看收藏"
// @Param sort query string false "排序 time_desc/time_asc" default(time_desc)
// @Param credential_id query int false "目标证件ID（按题目所属证件分区）"
// @Success 200 {object} response.R "success"
// @Failure 401 {object} response.R "未认证"
// @Router /wrong-questions [get]
func (h *WrongQuestionHandler) List(c *gin.Context) {
	Endpoint[listWrongQuestionsReq, map[string]any]{
		Parse: func(c *gin.Context) (*listWrongQuestionsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &listWrongQuestionsReq{
				StudentID:     studentID,
				Page:          atoiDefault(c.Query("page"), 1),
				PageSize:      atoiDefault(c.Query("page_size"), 20),
				QType:         c.Query("type"),
				MinWrongCount: queryIntPtr(c, "min_wrong_count"),
				Favorited:     c.Query("favorited") == "true",
				Sort:          c.Query("sort"),
				CredentialID:  queryIDPtr(c, "credential_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *listWrongQuestionsReq) (*map[string]any, error) {
			result := h.svc.GetWrongQuestions(req.StudentID, req.Page, req.PageSize, req.QType, req.MinWrongCount, req.Favorited, req.Sort, req.CredentialID)
			return &result, nil
		},
		Render: func(c *gin.Context, _ *listWrongQuestionsReq, resp *map[string]any, _ error) {
			response.Success(c, deref(resp))
		},
	}.Handle(c)
}

// redoWrongQuestionReq 重做错题请求。
type redoWrongQuestionReq struct {
	StudentID  int
	QuestionID int
	UserAnswer interface{}
}

// Redo 重做错题
// @Summary 重做错题
// @Description 提交错题重做答案并判分
// @Tags 学员端-错题本
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Param body body object true "答案" example({"user_answer":"A"})
// @Success 200 {object} response.R{data=service.SubmitResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /wrong-questions/{question_id}/redo [post]
func (h *WrongQuestionHandler) Redo(c *gin.Context) {
	Endpoint[redoWrongQuestionReq, service.SubmitResultDTO]{
		Parse: func(c *gin.Context) (*redoWrongQuestionReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			questionID, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			var req struct {
				UserAnswer interface{} `json:"user_answer"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &redoWrongQuestionReq{StudentID: studentID, QuestionID: questionID, UserAnswer: req.UserAnswer}, nil
		},
		Invoke: func(ctx context.Context, req *redoWrongQuestionReq) (*service.SubmitResultDTO, error) {
			return h.svc.RedoWrongQuestion(req.StudentID, req.QuestionID, req.UserAnswer)
		},
		Render: func(c *gin.Context, _ *redoWrongQuestionReq, resp *service.SubmitResultDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, deref(resp))
		},
	}.Handle(c)
}

// removeWrongQuestionReq 移出错题本请求。
type removeWrongQuestionReq struct {
	StudentID  int
	QuestionID int
}

// Remove 移出错题本
// @Summary 移出错题本
// @Description 将指定题目移出错题本
// @Tags 学员端-错题本
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param question_id path int true "题目ID"
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /wrong-questions/{question_id}/remove [post]
func (h *WrongQuestionHandler) Remove(c *gin.Context) {
	Endpoint[removeWrongQuestionReq, map[string]any]{
		Parse: func(c *gin.Context) (*removeWrongQuestionReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			questionID, err := strconv.Atoi(c.Param("question_id"))
			if err != nil {
				return nil, badRequest("题目ID无效")
			}
			return &removeWrongQuestionReq{StudentID: studentID, QuestionID: questionID}, nil
		},
		Invoke: func(ctx context.Context, req *removeWrongQuestionReq) (*map[string]any, error) {
			result, err := h.svc.RemoveWrongQuestion(req.StudentID, req.QuestionID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *removeWrongQuestionReq, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已移出错题本", deref(resp))
		},
	}.Handle(c)
}

// BatchRemove 批量移出错题本
// @Summary 批量移出
// @Description 批量将错题移出（is_removed=true）
// @Tags 学员端-错题本
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "题目IDs" example({"question_ids":[1,2,3]})
// @Success 200 {object} response.R "success"
// @Router /wrong-questions/batch-remove [post]
func (h *WrongQuestionHandler) BatchRemove(c *gin.Context) {
	Endpoint[batchRemoveReq, map[string]any]{
		Parse: func(c *gin.Context) (*batchRemoveReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				QuestionIDs []int `json:"question_ids"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &batchRemoveReq{StudentID: studentID, QuestionIDs: req.QuestionIDs}, nil
		},
		Invoke: func(ctx context.Context, req *batchRemoveReq) (*map[string]any, error) {
			cnt, err := h.svc.BatchRemoveWrongQuestions(req.StudentID, req.QuestionIDs)
			if err != nil {
				return nil, err
			}
			m := map[string]any{"removed": cnt}
			return &m, nil
		},
		Render: func(c *gin.Context, _ *batchRemoveReq, resp *map[string]any, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.SuccessWithMsg(c, "已批量移出", deref(resp))
		},
	}.Handle(c)
}

type batchRemoveReq struct {
	StudentID   int
	QuestionIDs []int
}

// getWrongStatsReq 错题统计请求。
type getWrongStatsReq struct {
	StudentID int
}

// GetStats 错题统计
// @Summary 错题统计
// @Description 汇总错题数量/题型分布等
// @Tags 学员端-错题本
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.WrongQuestionStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /wrong-questions/stats [get]
func (h *WrongQuestionHandler) GetStats(c *gin.Context) {
	Endpoint[getWrongStatsReq, service.WrongQuestionStatsDTO]{
		Parse: func(c *gin.Context) (*getWrongStatsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &getWrongStatsReq{StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *getWrongStatsReq) (*service.WrongQuestionStatsDTO, error) {
			return h.svc.GetStats(req.StudentID), nil
		},
		Render: func(c *gin.Context, _ *getWrongStatsReq, resp *service.WrongQuestionStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// exportWrongQuestionsReq 导出错题本请求。
type exportWrongQuestionsReq struct {
	StudentID int
}

// Export 导出错题本
// @Summary 导出错题本
// @Description 导出为纯文本附件（text/plain）
// @Tags 学员端-错题本
// @Produce plain
// @Security BearerAuth
// @Success 200 {string} string "错题文本"
// @Failure 401 {object} response.R "未认证"
// @Router /wrong-questions/export [get]
func (h *WrongQuestionHandler) Export(c *gin.Context) {
	Endpoint[exportWrongQuestionsReq, struct{}]{
		Parse: func(c *gin.Context) (*exportWrongQuestionsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &exportWrongQuestionsReq{StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *exportWrongQuestionsReq) (*struct{}, error) {
			data := h.svc.ExportWrongQuestions(req.StudentID)
			text := service.FormatWrongQuestionsText(data)
			c.Header("Content-Disposition", "attachment; filename=wrong_questions.txt")
			c.Data(200, "text/plain; charset=utf-8", []byte(text))
			return &struct{}{}, nil
		},
		Render: func(c *gin.Context, _ *exportWrongQuestionsReq, _ *struct{}, _ error) {
		},
	}.Handle(c)
}
