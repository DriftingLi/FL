package practicemode

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/scope"
)

var (
	ErrPracticeModeInvalid     = errors.New("练习模式无效")
	ErrPracticePoolEmpty       = errors.New("当前分类暂无符合条件的题目")
	ErrPracticeAttemptFinished = errors.New("本轮已无可继续的题目，可重新开始")
	ErrPracticeAttemptNotFound = errors.New("练习会话不存在或已重新开始")
)

type CategoryPracticeStartInput struct {
	CategoryID int    `json:"category_id"`
	Mode       string `json:"mode"`
	Count      int    `json:"count"`
	Restart    bool   `json:"restart"`
}

type CategoryPracticeStartDTO struct {
	AttemptKey   string                     `json:"attempt_key"`
	Questions    []questionbank.QuestionDTO `json:"questions" nullability:"nonnil"`
	CurrentIndex int                        `json:"current_index"`
	Completed    int                        `json:"completed"`
	AnswersState map[string]any             `json:"answers_state" nullability:"nonnil"`
}

type CategoryPracticeAnswerInput struct {
	AttemptKey string `json:"attempt_key"`
	QuestionID int    `json:"question_id"`
	UserAnswer any    `json:"user_answer"`
}

func categoryPracticePool(mode string, categoryID int) (questionbank.SampleQuestionsOpts, bool, error) {
	opts := questionbank.SampleQuestionsOpts{CategoryID: &categoryID}
	switch {
	case mode == "sequential":
		return opts, true, nil
	case mode == "random":
		return opts, false, nil
	case strings.HasPrefix(mode, "special:"):
		opts.QType = strings.TrimPrefix(mode, "special:")
		if slices.Contains(questionbank.ValidQuestionTypes, opts.QType) {
			return opts, false, nil
		}
	default:
		tagID, ok := pmTagID(PracticeMode(mode))
		if ok && mode == string(PracticeModeTag(tagID)) {
			opts.TagID = tagID
			return opts, true, nil
		}
	}
	return opts, false, ErrPracticeModeInvalid
}

func categoryPracticeAvailable(db *gorm.DB, opts questionbank.SampleQuestionsOpts) error {
	if err := scope.RequireActiveCategory(db, *opts.CategoryID); err != nil {
		return err
	}
	if opts.TagID > 0 {
		var count int64
		if err := db.Model(&model.QuestionTag{}).Where("id = ? AND status = 1 AND is_source_tag = ?", opts.TagID, false).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrPracticeTagUnsupported
		}
	}
	return nil
}

func (s *Service) StartCategoryPractice(ctx context.Context, studentID int, input CategoryPracticeStartInput) (*CategoryPracticeStartDTO, error) {
	opts, durable, err := categoryPracticePool(input.Mode, input.CategoryID)
	if err != nil {
		return nil, err
	}
	var result CategoryPracticeStartDTO
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := categoryPracticeAvailable(tx, opts); err != nil {
			return err
		}
		pool, err := questionbank.SampleQuestionsByOpts(tx, opts)
		if err != nil {
			return err
		}
		progress := model.PracticeProgress{
			StudentID: studentID, PracticeMode: input.Mode, CategoryID: &input.CategoryID,
			QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: s.clk.Now(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&progress).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ? AND practice_mode = ? AND category_id = ?", studentID, input.Mode, input.CategoryID).
			Take(&progress).Error; err != nil {
			return err
		}
		var ids []int
		if err := json.Unmarshal(progress.QuestionIDs, &ids); err != nil {
			return err
		}
		answers := AnswersMapRoundTrip(progress.AnswersState)
		key := progress.AttemptKey
		if len(ids) == 0 || input.Restart || !durable {
			if len(pool) == 0 {
				return ErrPracticePoolEmpty
			}
			if input.Mode == "random" || opts.TagID > 0 {
				count := input.Count
				if input.Mode == "random" && count <= 0 {
					count = 20
				}
				pool = questionbank.ShuffleTruncate(pool, count)
			}
			ids = make([]int, len(pool))
			for i := range pool {
				ids[i] = pool[i].ID
			}
			answers = map[string]any{}
			key = nil
		}
		byID := make(map[int]*model.Question, len(pool))
		for i := range pool {
			byID[pool[i].ID] = &pool[i]
		}
		result.Questions = make([]questionbank.QuestionDTO, 0, len(ids))
		result.AnswersState = make(map[string]any)
		result.CurrentIndex = -1
		for _, id := range ids {
			answer := answers[strconv.Itoa(id)]
			_, answered := answer.(map[string]any)
			if answered {
				result.Completed++
				result.AnswersState[strconv.Itoa(id)] = answer
			}
			if q, eligible := byID[id]; eligible {
				if !answered && result.CurrentIndex == -1 {
					result.CurrentIndex = len(result.Questions)
				}
				result.Questions = append(result.Questions, questionbank.NewQuestionDTO(q, false))
			}
		}
		if result.CurrentIndex == -1 {
			return ErrPracticeAttemptFinished
		}
		if key == nil {
			result.AttemptKey = uuid.NewString()
		} else {
			result.AttemptKey = *key
		}
		state, err := json.Marshal(answers)
		if err != nil {
			return err
		}
		return tx.Model(&progress).Updates(map[string]any{
			"attempt_key": result.AttemptKey, "question_ids": model.JSONB(marshalIDs(ids)),
			"answers_state": model.JSONB(state), "current_index": result.CurrentIndex, "total": len(ids), "updated_at": s.clk.Now(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) SubmitCategoryPracticeAnswer(ctx context.Context, studentID int, input CategoryPracticeAnswerInput) (*SubmitResultDTO, error) {
	var result SubmitResultDTO
	var answeredQuestion model.Question
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var progress model.PracticeProgress
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("student_id = ? AND attempt_key = ? AND category_id IS NOT NULL", studentID, input.AttemptKey).
			Take(&progress).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPracticeAttemptNotFound
		}
		if err != nil {
			return err
		}
		opts, _, err := categoryPracticePool(progress.PracticeMode, *progress.CategoryID)
		if err != nil {
			return err
		}
		if err := categoryPracticeAvailable(tx, opts); err != nil {
			return err
		}
		var ids []int
		if err := json.Unmarshal(progress.QuestionIDs, &ids); err != nil {
			return err
		}
		if !slices.Contains(ids, input.QuestionID) {
			return questionbank.ErrQuestionNotFound
		}
		if err := questionbank.PoolFilter(tx.Model(&model.Question{}), opts).Where("id = ?", input.QuestionID).Take(&answeredQuestion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return questionbank.ErrQuestionNotFound
			}
			return err
		}
		answers := AnswersMapRoundTrip(progress.AnswersState)
		id := strconv.Itoa(input.QuestionID)
		if saved, ok := answers[id].(map[string]any); ok {
			raw, err := json.Marshal(saved)
			if err != nil {
				return err
			}
			return json.Unmarshal(raw, &result)
		}
		grade := NewGradingEngine(tx).GradeOne(GradingFlow{AI: s.grader, MaxScore: PracticeMaxScore, CategoryID: progress.CategoryID}, &answeredQuestion, input.UserAnswer, studentID)
		if grade.WrongWriteError != nil {
			return grade.WrongWriteError
		}
		result = SubmitResultDTO{
			IsCorrect: grade.IsCorrect, QuestionID: input.QuestionID, UserAnswer: input.UserAnswer,
			CorrectAnswer: answeredQuestion.Answer, Explanation: answeredQuestion.Explanation,
		}
		applyShortAnswerResult(&result, grade, &answeredQuestion)
		practiceType := progress.PracticeMode
		if practiceType == "random" || opts.QType != "" {
			practiceType = "free"
		}
		if opts.TagID > 0 {
			practiceType = "tag"
		}
		if err := tx.Create(&model.QuestionPracticeRecord{
			StudentID: studentID, CategoryID: progress.CategoryID, QuestionID: input.QuestionID,
			IsCorrect: result.IsCorrect != nil && *result.IsCorrect, PracticeType: practiceType,
			UserAnswer: questionbank.StringifyAnswer(input.UserAnswer), CreatedAt: s.clk.Now(),
		}).Error; err != nil {
			return err
		}
		answers[id] = result
		state, err := json.Marshal(answers)
		if err != nil {
			return err
		}
		return tx.Model(&progress).Updates(map[string]any{"answers_state": model.JSONB(state), "updated_at": s.clk.Now()}).Error
	})
	if err != nil {
		return nil, err
	}
	addSubmitFeedback(s.db, s.explainer, &result, &answeredQuestion)
	return &result, nil
}
