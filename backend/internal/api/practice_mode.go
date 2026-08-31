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
	g.GET("/practice-stats", h.GetPracticeStats)
	g.GET("/history", h.GetHistory)
}

// freeQuestionsReq 随机练习抽题请求（type + count）。
type freeQuestionsReq struct {
	QType        string
	Count        int
	CredentialID *int
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
				QType:        c.Query("type"),
				Count:        atoiDefault(c.Query("count"), 20),
				CredentialID: queryIDPtr(c, "credential_id"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *freeQuestionsReq) (*[]service.QuestionDTO, error) {
			result, err := h.svc.GetFreeQuestions(req.QType, req.Count, req.CredentialID)
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
	StudentID    int
	TagID        int
	Count        int
	CredentialID *int
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
			return &tagPracticeReq{StudentID: studentID, TagID: tagID, Count: count, CredentialID: queryIDPtr(c, "credential_id")}, nil
		},
		Invoke: func(ctx context.Context, req *tagPracticeReq) (*service.PracticeStartResultDTO, error) {
			return h.svc.StartTagPractice(req.StudentID, req.TagID, req.Count, req.CredentialID)
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
		StudentID    int
		CredentialID *int
	}, service.PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*struct {
			StudentID    int
			CredentialID *int
		}, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &struct {
				StudentID    int
				CredentialID *int
			}{StudentID: studentID, CredentialID: queryIDPtr(c, "credential_id")}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			StudentID    int
			CredentialID *int
		}) (*service.PracticeStartResultDTO, error) {
			return h.svc.StartSequential(req.StudentID, req.CredentialID)
		},
		Render: func(c *gin.Context, _ *struct {
			StudentID    int
			CredentialID *int
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
			// #413：透传证件参数，进度返回体附带实时池总数。
			return h.svc.GetSequentialProgress(req.StudentID, queryIDPtr(c, "credential_id")), nil
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
	CredentialID *int
}

// SaveProgress 保存练习进度
// @Summary 保存练习游标与答题状态
// @Description 支持顺序/标签/按卷练习的断点续练；未知 practice_mode 返回 400（#386）
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "进度" example({"index":5,"practice_mode":"sequential","total":20,"answers_state":{}})
// @Success 200 {object} response.R "success"
// @Failure 400 {object} response.R "参数错误（含未知练习模式）"
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
				CredentialID *int            `json:"credential_id"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, badRequest("请求数据无效")
			}
			if req.PracticeMode == "" {
				req.PracticeMode = string(service.PracticeModeSequential)
			}
			// 练习模式封闭校验（#386）：未知 mode 拒绝，消灭 typo 静默孤儿进度行
			if _, ok := service.ParsePracticeMode(req.PracticeMode); !ok {
				return nil, badRequest("练习模式无效")
			}
			return &practiceSaveProgressReq{
				StudentID:    studentID,
				Index:        req.Index,
				PracticeMode: req.PracticeMode,
				Total:        req.Total,
				AnswersState: req.AnswersState,
				CredentialID: req.CredentialID,
			}, nil
		},
		Invoke: func(ctx context.Context, req *practiceSaveProgressReq) (*struct{}, error) {
			if err := h.svc.SaveProgress(req.StudentID, req.Index, req.PracticeMode, req.Total, req.AnswersState, req.CredentialID); err != nil {
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
	StudentID    int
	Mode         string
	CredentialID *int
}

// GetProgress 查询练习进度
// @Summary 查询练习进度
// @Description 按 mode 查询断点续练进度；mode 为空默认为 sequential，未知 mode 返回 400（#386）
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param mode query string false "练习模式" default(sequential)
// @Success 200 {object} response.R{data=service.ProgressResultDTO} "success"
// @Failure 400 {object} response.R "参数错误（含未知练习模式）"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/progress [get]
func (h *PracticeModeHandler) GetProgress(c *gin.Context) {
	Endpoint[getProgressReq, service.ProgressResultDTO]{
		Parse: func(c *gin.Context) (*getProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			mode := c.Query("mode")
			if mode == "" {
				mode = string(service.PracticeModeSequential)
			}
			// 练习模式封闭校验（#386）：未知 mode 拒绝
			if _, ok := service.ParsePracticeMode(mode); !ok {
				return nil, badRequest("练习模式无效")
			}
			return &getProgressReq{StudentID: studentID, Mode: mode, CredentialID: queryIDPtr(c, "credential_id")}, nil
		},
		Invoke: func(ctx context.Context, req *getProgressReq) (*service.ProgressResultDTO, error) {
			return h.svc.GetProgress(req.StudentID, req.Mode, req.CredentialID), nil
		},
		Render: func(c *gin.Context, _ *getProgressReq, resp *service.ProgressResultDTO, err error) {
			if err != nil {
				response.BadRequest(c, err.Error())
				return
			}
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

// GetPracticeStats 刷题数据展示
// @Summary 刷题数据展示
// @Description 按当前证件分区聚合：今日做题/累计做题/累计做题天数；含重做，均按 question_practice_record；今日按 Asia/Shanghai 自然日，累计天数按自然日去重
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID" minimum(1)
// @Success 200 {object} response.R{data=service.PracticePracticeStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/practice-stats [get]
func (h *PracticeModeHandler) GetPracticeStats(c *gin.Context) {
	Endpoint[practiceStatsReq, service.PracticePracticeStatsDTO]{
		Parse: func(c *gin.Context) (*practiceStatsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &practiceStatsReq{StudentID: studentID, CredentialID: queryIDPtr(c, "credential_id")}, nil
		},
		Invoke: func(ctx context.Context, req *practiceStatsReq) (*service.PracticePracticeStatsDTO, error) {
			return h.svc.GetPracticeStats(req.StudentID, req.CredentialID)
		},
		Render: func(c *gin.Context, _ *practiceStatsReq, resp *service.PracticePracticeStatsDTO, err error) {
			if err != nil {
				response.ServerError(c, "查询失败")
				return
			}
			response.Success(c, resp)
		},
	}.Handle(c)
}

// practiceStatsReq 刷题数据展示请求（学员 ID + 可选证件分区）。
type practiceStatsReq struct {
	StudentID    int
	CredentialID *int
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
