// Package api 实现 HTTP handlers。
package api

import (
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// PracticeModeHandler 题库练习 handler。
type PracticeModeHandler struct {
	svc *service.PracticeModeService
}

// NewPracticeModeHandler 创建题库练习 handler。
func NewPracticeModeHandler(svc *service.PracticeModeService) *PracticeModeHandler {
	return &PracticeModeHandler{svc: svc}
}

// RegisterPracticeModeRoutes 注册 /api/practice-mode 蓝图（题库练习）。
func RegisterPracticeModeRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.PracticeModeService) {
	h := NewPracticeModeHandler(svc)

	g := rg.Group("/practice-mode", middleware.JWTAuth(rd.Session), middleware.RoleRequired("hrwai_user"))

	g.GET("/free", h.GetFreeQuestions)
	g.GET("/tag", h.StartTagPractice)
	g.GET("/sequential", h.StartSequential)
	g.GET("/sequential-progress", h.GetSequentialProgress)
	g.POST("/progress", h.SaveProgress)
	g.GET("/progress", h.GetProgress)
	g.POST("/submit", h.SubmitAnswer)
	g.GET("/stats", h.GetStats)
	g.GET("/history", h.GetHistory)
}

// freeQuestionsReq 随机练习抽题请求（type + count）。
type freeQuestionsReq struct {
	QType string
	Count int
}

// GetFreeQuestions 随机练习抽题
// @Summary 随机练习抽题
// @Description 按题型随机抽题，count 控制题量（默认 20）
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param count query int false "题量" default(20)
// @Param type query string false "题型 single_choice 等"
// @Success 200 {object} response.R{data=[]service.QuestionDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/free [get]
func (h *PracticeModeHandler) GetFreeQuestions(c *gin.Context) {
	Endpoint[freeQuestionsReq, []service.QuestionDTO]{
		Parse: func(c *gin.Context) (*freeQuestionsReq, error) {
			return &freeQuestionsReq{
				QType: c.Query("type"),
				Count: atoiDefault(c.Query("count"), 20),
			}, nil
		},
		Invoke: func(ctx context.Context, req *freeQuestionsReq) (*[]service.QuestionDTO, error) {
			result, err := h.svc.GetFreeQuestions(req.QType, req.Count)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
		Render: func(c *gin.Context, _ *freeQuestionsReq, resp *[]service.QuestionDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, *resp)
		},
	}.Handle(c)
}

// tagPracticeReq 标签练习请求（tag_id 区分缺失/非法 + count）。
type tagPracticeReq struct {
	StudentID int
	TagID     int
	Count     int
}

// StartTagPractice 标签专项练习
// @Summary 标签专项练习
// @Description 按标签 ID 开始/续练专项练习；count=0 表示全部
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tag_id query int true "题库标签ID"
// @Param count query int false "题量 0=全部" default(0)
// @Success 200 {object} response.R{data=service.PracticeStartResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/tag [get]
func (h *PracticeModeHandler) StartTagPractice(c *gin.Context) {
	Endpoint[tagPracticeReq, service.PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*tagPracticeReq, error) {
			tagIDStr := c.Query("tag_id")
			if tagIDStr == "" {
				return nil, badRequest("请指定题库标签")
			}
			tagID, ok := requiredPositiveID(tagIDStr)
			if !ok {
				return nil, badRequest("题库标签ID无效")
			}
			count := atoiDefault(c.Query("count"), 0) // 0=全部
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &tagPracticeReq{StudentID: studentID, TagID: tagID, Count: count}, nil
		},
		Invoke: func(ctx context.Context, req *tagPracticeReq) (*service.PracticeStartResultDTO, error) {
			return h.svc.StartTagPractice(req.StudentID, req.TagID, req.Count)
		},
		Render: func(c *gin.Context, _ *tagPracticeReq, resp *service.PracticeStartResultDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// StartSequential 顺序练习
// @Summary 顺序练习开始/续练
// @Description 返回当前批次题目 + 进度
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.PracticeStartResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/sequential [get]
func (h *PracticeModeHandler) StartSequential(c *gin.Context) {
	Endpoint[struct {
		StudentID int
	}, service.PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*struct {
			StudentID int
		}, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &struct {
				StudentID int
			}{StudentID: studentID}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			StudentID int
		}) (*service.PracticeStartResultDTO, error) {
			return h.svc.StartSequential(req.StudentID)
		},
		Render: func(c *gin.Context, _ *struct {
			StudentID int
		}, resp *service.PracticeStartResultDTO, err error) {
			if err != nil {
				response.NotFound(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// studentIDReq 仅携带学员 ID 的请求。
type studentIDReq struct {
	StudentID int
}

// GetSequentialProgress 顺序练习进度
// @Summary 顺序练习进度
// @Description 用于卡片展示的进度快照
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.ProgressResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/sequential-progress [get]
func (h *PracticeModeHandler) GetSequentialProgress(c *gin.Context) {
	Endpoint[studentIDReq, service.ProgressResultDTO]{
		Parse: h.parseStudentID,
		Invoke: func(ctx context.Context, req *studentIDReq) (*service.ProgressResultDTO, error) {
			return h.svc.GetSequentialProgress(req.StudentID), nil
		},
		Render: func(c *gin.Context, _ *studentIDReq, resp *service.ProgressResultDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// practiceSaveProgressReq 保存练习进度请求（学员 ID + body）。
type practiceSaveProgressReq struct {
	StudentID    int
	Index        int
	PracticeMode string
	Total        int
	AnswersState json.RawMessage
}

// SaveProgress 保存练习进度
// @Summary 保存练习游标与答题状态
// @Description 支持顺序/标签等练习的断点续练
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "进度" example({"index":5,"practice_mode":"sequential","total":20,"answers_state":{}})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/progress [post]
func (h *PracticeModeHandler) SaveProgress(c *gin.Context) {
	Endpoint[practiceSaveProgressReq, struct{}]{
		Parse: func(c *gin.Context) (*practiceSaveProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				Index        int             `json:"index"`
				PracticeMode string          `json:"practice_mode"`
				Total        int             `json:"total"`
				AnswersState json.RawMessage `json:"answers_state"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
			}
			return &practiceSaveProgressReq{
				StudentID:    studentID,
				Index:        req.Index,
				PracticeMode: req.PracticeMode,
				Total:        req.Total,
				AnswersState: req.AnswersState,
			}, nil
		},
		Invoke: func(ctx context.Context, req *practiceSaveProgressReq) (*struct{}, error) {
			if err := h.svc.SaveProgress(req.StudentID, req.Index, req.PracticeMode, req.Total, req.AnswersState); err != nil {
				return nil, err
			}
			return nil, nil
		},
		Render: func(c *gin.Context, req *practiceSaveProgressReq, _ *struct{}, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, map[string]any{"saved": true, "index": req.Index})
		},
	}.Handle(c)
}

// getProgressReq 查询练习进度请求（学员 ID + mode，默认 sequential）。
type getProgressReq struct {
	StudentID int
	Mode      string
}

// GetProgress 查询练习进度
// @Summary 查询练习进度
// @Description 按 mode 查询断点续练进度；mode 为空默认为 sequential
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mode query string false "练习模式" default(sequential)
// @Success 200 {object} response.R{data=service.ProgressResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/progress [get]
func (h *PracticeModeHandler) GetProgress(c *gin.Context) {
	Endpoint[getProgressReq, service.ProgressResultDTO]{
		Parse: func(c *gin.Context) (*getProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			mode := c.Query("mode")
			if mode == "" {
				mode = "sequential"
			}
			return &getProgressReq{StudentID: studentID, Mode: mode}, nil
		},
		Invoke: func(ctx context.Context, req *getProgressReq) (*service.ProgressResultDTO, error) {
			return h.svc.GetProgress(req.StudentID, req.Mode), nil
		},
		Render: func(c *gin.Context, _ *getProgressReq, resp *service.ProgressResultDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// submitAnswerReq 提交答案请求（学员 ID + body）。
type submitAnswerReq struct {
	StudentID    int
	QuestionID   int
	UserAnswer   any
	PracticeType string
}

// SubmitAnswer 提交答案
// @Summary 提交练习答案
// @Description 提交单题答案并即时判分
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "答题" example({"question_id":1,"user_answer":"A","practice_type":"free"})
// @Success 200 {object} response.R{data=service.SubmitResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/submit [post]
func (h *PracticeModeHandler) SubmitAnswer(c *gin.Context) {
	Endpoint[submitAnswerReq, service.SubmitResultDTO]{
		Parse: func(c *gin.Context) (*submitAnswerReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				QuestionID   int         `json:"question_id"`
				UserAnswer   interface{} `json:"user_answer"`
				PracticeType string      `json:"practice_type"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
			}
			if req.QuestionID == 0 {
				return nil, badRequest("题目ID不能为空")
			}
			if req.PracticeType == "" {
				req.PracticeType = "free"
			}
			return &submitAnswerReq{
				StudentID:    studentID,
				QuestionID:   req.QuestionID,
				UserAnswer:   req.UserAnswer,
				PracticeType: req.PracticeType,
			}, nil
		},
		Invoke: func(ctx context.Context, req *submitAnswerReq) (*service.SubmitResultDTO, error) {
			return h.svc.SubmitAnswer(req.StudentID, req.QuestionID, req.UserAnswer, req.PracticeType)
		},
		Render: func(c *gin.Context, _ *submitAnswerReq, resp *service.SubmitResultDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// GetStats 练习统计
// @Summary 练习统计
// @Description 汇总练习正确率/已练题量等
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=service.PracticeStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/stats [get]
func (h *PracticeModeHandler) GetStats(c *gin.Context) {
	Endpoint[studentIDReq, service.PracticeStatsDTO]{
		Parse: h.parseStudentID,
		Invoke: func(ctx context.Context, req *studentIDReq) (*service.PracticeStatsDTO, error) {
			return h.svc.GetStats(req.StudentID), nil
		},
		Render: func(c *gin.Context, _ *studentIDReq, resp *service.PracticeStatsDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// practiceHistoryReq 练习历史请求（学员 ID + 分页 + 过滤）。
type practiceHistoryReq struct {
	StudentID int
	Page      int
	PageSize  int
	QType     string
	StartDate string
	EndDate   string
}

// GetHistory 练习历史
// @Summary 练习历史
// @Description 分页查询练习历史，支持按题型/日期过滤
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param type query string false "题型"
// @Param start_date query string false "开始日期 YYYY-MM-DD"
// @Param end_date query string false "结束日期 YYYY-MM-DD"
// @Success 200 {object} response.R{data=service.HistoryResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/history [get]
func (h *PracticeModeHandler) GetHistory(c *gin.Context) {
	Endpoint[practiceHistoryReq, service.HistoryResultDTO]{
		Parse: func(c *gin.Context) (*practiceHistoryReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &practiceHistoryReq{
				StudentID: studentID,
				Page:      atoiDefault(c.Query("page"), 1),
				PageSize:  atoiDefault(c.Query("page_size"), 20),
				QType:     c.Query("type"),
				StartDate: c.Query("start_date"),
				EndDate:   c.Query("end_date"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *practiceHistoryReq) (*service.HistoryResultDTO, error) {
			return h.svc.GetHistory(req.StudentID, req.Page, req.PageSize, req.QType, req.StartDate, req.EndDate), nil
		},
		Render: func(c *gin.Context, _ *practiceHistoryReq, resp *service.HistoryResultDTO, _ error) {
			response.Success(c, resp)
		},
	}.Handle(c)
}

// parseStudentID 解析学员 ID（来自上下文）。
func (h *PracticeModeHandler) parseStudentID(c *gin.Context) (*studentIDReq, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	return &studentIDReq{StudentID: studentID}, nil
}
