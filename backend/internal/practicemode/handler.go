// 本文件：练习域 HTTP 出口（/api/practice-mode 蓝图，ADR-0070 的 handler.go 形态）。
package practicemode

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/authz"
	"forklift-training/internal/middleware"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/security"
	"forklift-training/pkg/httpx"
)

// handler 练习域 HTTP handler。
type handler struct {
	svc *Service
}

// newHandler 创建练习域 HTTP handler。
func newHandler(svc *Service) *handler {
	return &handler{svc: svc}
}

// RegisterRoutes 注册 /api/practice-mode 蓝图（练习域）。
func RegisterRoutes(rg *gin.RouterGroup, session *security.Session, credRes middleware.CredentialResolver, svc *Service) {
	h := newHandler(svc)

	g := rg.Group("/practice-mode", middleware.JWTAuth(session), middleware.CapabilityRequired(authz.CapQuestionPractice), middleware.CredentialScoped(credRes))

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
// @Success 200 {object} response.R{data=[]questionbank.QuestionDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/free [get]
func (h *handler) GetFreeQuestions(c *gin.Context) {
	httpx.Endpoint[freeQuestionsReq, []questionbank.QuestionDTO]{
		Parse: func(c *gin.Context) (*freeQuestionsReq, error) {
			return &freeQuestionsReq{
				QType:        c.Query("type"),
				Count:        httpx.QueryIntDefault(c, "count", 20),
				CredentialID: middleware.CredentialIDPtr(c),
			}, nil
		},
		Invoke: func(ctx context.Context, req *freeQuestionsReq) (*[]questionbank.QuestionDTO, error) {
			result, err := h.svc.GetFreeQuestions(req.QType, req.Count, req.CredentialID)
			if err != nil {
				return nil, err
			}
			return &result, nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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
// @Success 200 {object} response.R{data=PracticeStartResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/tag [get]
func (h *handler) StartTagPractice(c *gin.Context) {
	httpx.Endpoint[tagPracticeReq, PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*tagPracticeReq, error) {
			tagIDStr := c.Query("tag_id")
			if tagIDStr == "" {
				return nil, httpx.BadRequest("请指定题库标签")
			}
			tagID, ok := httpx.PositiveID(tagIDStr)
			if !ok {
				return nil, httpx.BadRequest("题库标签ID无效")
			}
			count := httpx.QueryIntDefault(c, "count", 0) // 0=全部
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &tagPracticeReq{StudentID: studentID, TagID: tagID, Count: count, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *tagPracticeReq) (*PracticeStartResultDTO, error) {
			return h.svc.StartTagPractice(req.StudentID, req.TagID, req.Count, req.CredentialID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinel(ErrPracticeTagRequired, http.StatusBadRequest).
		WithSentinel(ErrPracticeTagUnsupported, http.StatusBadRequest).Handle(c)
}

// StartSequential 顺序练习
// @Summary 顺序练习开始/续练
// @Description 返回当前批次题目 + 进度
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.R{data=PracticeStartResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/sequential [get]
func (h *handler) StartSequential(c *gin.Context) {
	httpx.Endpoint[struct {
		StudentID    int
		CredentialID *int
	}, PracticeStartResultDTO]{
		Parse: func(c *gin.Context) (*struct {
			StudentID    int
			CredentialID *int
		}, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &struct {
				StudentID    int
				CredentialID *int
			}{StudentID: studentID, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *struct {
			StudentID    int
			CredentialID *int
		}) (*PracticeStartResultDTO, error) {
			return h.svc.StartSequential(req.StudentID, req.CredentialID)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
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
// @Success 200 {object} response.R{data=ProgressResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/sequential-progress [get]
func (h *handler) GetSequentialProgress(c *gin.Context) {
	httpx.Endpoint[studentIDReq, ProgressResultDTO]{
		Parse: h.parseStudentID,
		Invoke: func(ctx context.Context, req *studentIDReq) (*ProgressResultDTO, error) {
			// #413：透传证件参数，进度返回体附带实时池总数。
			return h.svc.GetSequentialProgress(req.StudentID, middleware.CredentialIDPtr(c)), nil
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
// @Success 200 {object} response.R{data=ProgressSaveResultDTO} "success"
// @Failure 400 {object} response.R "参数错误（含未知练习模式）"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/progress [post]
func (h *handler) SaveProgress(c *gin.Context) {
	httpx.Endpoint[practiceSaveProgressReq, ProgressSaveResultDTO]{
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
				return nil, httpx.BadRequest("请求数据无效")
			}
			if req.PracticeMode == "" {
				req.PracticeMode = string(PracticeModeSequential)
			}
			// 练习模式封闭校验（#386）：未知 mode 拒绝，消灭 typo 静默孤儿进度行
			if _, ok := ParsePracticeMode(req.PracticeMode); !ok {
				return nil, httpx.BadRequest("练习模式无效")
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
		Invoke: func(ctx context.Context, req *practiceSaveProgressReq) (*ProgressSaveResultDTO, error) {
			if err := h.svc.SaveProgress(req.StudentID, req.Index, req.PracticeMode, req.Total, req.AnswersState, req.CredentialID); err != nil {
				return nil, err
			}
			return &ProgressSaveResultDTO{Index: req.Index, Saved: true}, nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusBadRequest).Handle(c)
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
// @Success 200 {object} response.R{data=ProgressResultDTO} "success"
// @Failure 400 {object} response.R "参数错误（含未知练习模式）"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/progress [get]
func (h *handler) GetProgress(c *gin.Context) {
	httpx.Endpoint[getProgressReq, ProgressResultDTO]{
		Parse: func(c *gin.Context) (*getProgressReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			mode := c.Query("mode")
			if mode == "" {
				mode = string(PracticeModeSequential)
			}
			// 练习模式封闭校验（#386）：未知 mode 拒绝
			if _, ok := ParsePracticeMode(mode); !ok {
				return nil, httpx.BadRequest("练习模式无效")
			}
			return &getProgressReq{StudentID: studentID, Mode: mode, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *getProgressReq) (*ProgressResultDTO, error) {
			return h.svc.GetProgress(req.StudentID, req.Mode, req.CredentialID), nil
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusBadRequest).Handle(c)
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
// @Success 200 {object} response.R{data=SubmitResultDTO} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/submit [post]
func (h *handler) SubmitAnswer(c *gin.Context) {
	httpx.Endpoint[submitAnswerReq, SubmitResultDTO]{
		Parse: func(c *gin.Context) (*submitAnswerReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			var req struct {
				QuestionID   int         `json:"question_id"`
				UserAnswer   interface{} `json:"user_answer"`
				PracticeType string      `json:"practice_type"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				return nil, httpx.BadRequest("请求数据无效")
			}
			if req.QuestionID == 0 {
				return nil, httpx.BadRequest("题目ID不能为空")
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
		Invoke: func(ctx context.Context, req *submitAnswerReq) (*SubmitResultDTO, error) {
			return h.svc.SubmitAnswer(req.StudentID, req.QuestionID, req.UserAnswer, req.PracticeType, middleware.CredentialIDPtr(c))
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusBadRequest).Handle(c)
}

// GetPracticeStats 刷题数据展示
// @Summary 刷题数据展示
// @Description 按当前证件分区聚合：今日做题/累计做题/累计做题天数；含重做，均按 question_practice_record；今日按 Asia/Shanghai 自然日，累计天数按自然日去重
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID" minimum(1)
// @Success 200 {object} response.R{data=PracticePracticeStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/practice-stats [get]
func (h *handler) GetPracticeStats(c *gin.Context) {
	httpx.Endpoint[practiceStatsReq, PracticePracticeStatsDTO]{
		Parse: func(c *gin.Context) (*practiceStatsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &practiceStatsReq{StudentID: studentID, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *practiceStatsReq) (*PracticePracticeStatsDTO, error) {
			return h.svc.GetPracticeStats(req.StudentID, req.CredentialID)
		},
		ErrStatus: httpx.ErrStatusAllMsg(http.StatusInternalServerError, "查询失败"),
	}.Handle(c)
}

// practiceStatsReq 练习统计类请求（学员 ID + 可选证件分区）：/practice-stats 与 /stats 共用。
// 证件来自 CredentialScoped 中间件（显式 query 优先，否则服务端当前证件）；nil = 未设置，按不分区处理。
type practiceStatsReq struct {
	StudentID    int
	CredentialID *int
}

// GetStats 练习统计
// @Summary 练习统计
// @Description 汇总练习正确率/已练题量等（按当前证件分区：credential_id 可选，经拦截器注入当前证件）
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID"
// @Success 200 {object} response.R{data=PracticeStatsDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/stats [get]
func (h *handler) GetStats(c *gin.Context) {
	httpx.Endpoint[practiceStatsReq, PracticeStatsDTO]{
		Parse: func(c *gin.Context) (*practiceStatsReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &practiceStatsReq{StudentID: studentID, CredentialID: middleware.CredentialIDPtr(c)}, nil
		},
		Invoke: func(ctx context.Context, req *practiceStatsReq) (*PracticeStatsDTO, error) {
			return h.svc.GetStats(req.StudentID, req.CredentialID)
		},
	}.Handle(c)
}

// practiceHistoryReq 练习历史请求（学员 ID + 当前证件 + 分页 + 过滤）。
type practiceHistoryReq struct {
	StudentID    int
	CredentialID *int
	Page         int
	PageSize     int
	QType        string
	StartDate    string
	EndDate      string
}

// GetHistory 练习历史
// @Summary 练习历史
// @Description 分页查询练习历史，支持按题型/日期过滤（按当前证件分区：credential_id 可选，经拦截器注入当前证件）
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param credential_id query int false "目标证件ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param type query string false "题型"
// @Param start_date query string false "开始日期 YYYY-MM-DD"
// @Param end_date query string false "结束日期 YYYY-MM-DD"
// @Success 200 {object} response.R{data=HistoryResultDTO} "success"
// @Failure 401 {object} response.R "未认证"
// @Router /practice-mode/history [get]
func (h *handler) GetHistory(c *gin.Context) {
	httpx.Endpoint[practiceHistoryReq, HistoryResultDTO]{
		Parse: func(c *gin.Context) (*practiceHistoryReq, error) {
			uid, _ := c.Get(string(middleware.CtxUserID))
			studentID, _ := uid.(int)
			return &practiceHistoryReq{
				StudentID:    studentID,
				CredentialID: middleware.CredentialIDPtr(c),
				Page:         httpx.QueryIntDefault(c, "page", 1),
				PageSize:     httpx.QueryIntDefault(c, "page_size", 20),
				QType:        c.Query("type"),
				StartDate:    c.Query("start_date"),
				EndDate:      c.Query("end_date"),
			}, nil
		},
		Invoke: func(ctx context.Context, req *practiceHistoryReq) (*HistoryResultDTO, error) {
			return h.svc.GetHistory(req.StudentID, req.CredentialID, req.Page, req.PageSize, req.QType, req.StartDate, req.EndDate)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).Handle(c)
}

// parseStudentID 解析学员 ID（来自上下文）。
func (h *handler) parseStudentID(c *gin.Context) (*studentIDReq, error) {
	uid, _ := c.Get(string(middleware.CtxUserID))
	studentID, _ := uid.(int)
	return &studentIDReq{StudentID: studentID}, nil
}
