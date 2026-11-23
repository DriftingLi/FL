package practicemode

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/scope"
	"forklift-training/pkg/httpx"
)

type ownedCategoryPracticeStart struct {
	StudentID int
	Input     CategoryPracticeStartInput
}

// @Summary 开始或继续分类练习
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CategoryPracticeStartInput true "一级分类与本次练习模式"
// @Success 200 {object} response.R{data=CategoryPracticeStartDTO} "success"
// @Failure 400 {object} response.R "分类、模式或题池不可用"
// @Failure 409 {object} response.R "本轮已结束，需显式重开"
// @Router /practice-mode/attempts [post]
func (h *handler) StartCategoryPractice(c *gin.Context) {
	httpx.Endpoint[ownedCategoryPracticeStart, CategoryPracticeStartDTO]{
		Parse: func(c *gin.Context) (*ownedCategoryPracticeStart, error) {
			input, err := httpx.BindJSONMsg[CategoryPracticeStartInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			return &ownedCategoryPracticeStart{StudentID: c.GetInt(string(middleware.CtxUserID)), Input: *input}, nil
		},
		Invoke: func(ctx context.Context, req *ownedCategoryPracticeStart) (*CategoryPracticeStartDTO, error) {
			return h.svc.StartCategoryPractice(ctx, req.StudentID, req.Input)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, scope.ErrCategoryUnavailable, ErrPracticeModeInvalid, ErrPracticeTagUnsupported, ErrPracticePoolEmpty).
		WithSentinel(ErrPracticeAttemptFinished, http.StatusConflict).Handle(c)
}

type ownedCategoryPracticeAnswer struct {
	StudentID int
	Input     CategoryPracticeAnswerInput
}

// @Summary 提交分类练习答案
// @Tags 学员端-练习
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CategoryPracticeAnswerInput true "会话标识、题目 ID 和答案"
// @Success 200 {object} response.R{data=SubmitResultDTO} "success"
// @Failure 400 {object} response.R "参数错误或分类已停用"
// @Failure 404 {object} response.R "会话或题目不可用"
// @Router /practice-mode/attempts/answer [post]
func (h *handler) SubmitCategoryPracticeAnswer(c *gin.Context) {
	httpx.Endpoint[ownedCategoryPracticeAnswer, SubmitResultDTO]{
		Parse: func(c *gin.Context) (*ownedCategoryPracticeAnswer, error) {
			input, err := httpx.BindJSONMsg[CategoryPracticeAnswerInput](c, "请求数据无效")
			if err != nil {
				return nil, err
			}
			if len(input.AttemptKey) != 36 || input.QuestionID <= 0 || input.UserAnswer == nil {
				return nil, httpx.BadRequest("请提供会话标识、题目和答案")
			}
			return &ownedCategoryPracticeAnswer{StudentID: c.GetInt(string(middleware.CtxUserID)), Input: *input}, nil
		},
		Invoke: func(ctx context.Context, req *ownedCategoryPracticeAnswer) (*SubmitResultDTO, error) {
			return h.svc.SubmitCategoryPracticeAnswer(ctx, req.StudentID, req.Input)
		},
	}.WithSuccess(httpx.OkMsg("success"), http.StatusInternalServerError).
		WithSentinels(http.StatusBadRequest, scope.ErrCategoryUnavailable, ErrPracticeTagUnsupported).
		WithSentinels(http.StatusNotFound, ErrPracticeAttemptNotFound, questionbank.ErrQuestionNotFound).Handle(c)
}
