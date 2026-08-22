package service

// 练习流 typed DTO（Ticket #225）。JSON key 与重构前 map 输出逐字一致
// （shape-lock 测试冻结：practice_mode_dto_shape_test.go）。字段声明按 key 字母序。

// PracticeStartResultDTO 标签/顺序练习开始或续练结果
// （旧 StartTagPractice/StartSequential map 输出）。
type PracticeStartResultDTO struct {
	Questions    []QuestionDTO `json:"questions"`
	CurrentIndex int           `json:"current_index"`
	Total        int           `json:"total"`
	Completed    int           `json:"completed"`
}

// ProgressResultDTO 任意模式练习进度（旧 GetProgress/GetSequentialProgress map 输出）。
type ProgressResultDTO struct {
	Completed    int            `json:"completed"`
	Total        int            `json:"total"`
	CurrentIndex int            `json:"current_index"`
	AnswersState map[string]any `json:"answers_state"`
}

// SubmitResultDTO 单题提交判定结果（旧 SubmitAnswer map 输出）。
// IsCorrect 为 *bool：简答题经 AI 判定前为 nil（JSON null），判定后与客观题一样为 true/false。
// 简答题追加 reference_answer / scoring_criteria / max_score；
// AI 评分成功追加 ai_score / ai_comment，降级时追加 ai_fallback。
// 解析增强（spec #284）：全站正确率 accuracy_rate（样本<5 时不返回）与易错项 common_wrong（仅选择题，样本<5 或无错题时不返回）。
type SubmitResultDTO struct {
	IsCorrect       *bool    `json:"is_correct"`
	CorrectAnswer   string   `json:"correct_answer"`
	Explanation     string   `json:"explanation"`
	QuestionID      int      `json:"question_id"`
	UserAnswer      any      `json:"user_answer"`
	ReferenceAnswer string   `json:"reference_answer,omitempty"`
	ScoringCriteria string   `json:"scoring_criteria,omitempty"`
	MaxScore        int      `json:"max_score,omitempty"`
	AIScore         *float64 `json:"ai_score,omitempty"`
	AIComment       string   `json:"ai_comment,omitempty"`
	AIFallback      *bool    `json:"ai_fallback,omitempty"`
	AccuracyRate    *float64 `json:"accuracy_rate,omitempty"`
	CommonWrong     *string  `json:"common_wrong,omitempty"`
	TotalAttempts   int      `json:"total_attempts,omitempty"`
	AIExplanation   string   `json:"ai_explanation,omitempty"`
}

// HistoryResultDTO 练习历史分页结果（旧 GetHistory map 输出）。
type HistoryResultDTO struct {
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Records  []HistoryItemDTO `json:"records"`
}

// HistoryItemDTO 练习历史条目（旧 GetHistory items 内每条 map 输出；命中题目时追加 question）。
type HistoryItemDTO struct {
	ID           int          `json:"id"`
	StudentID    int          `json:"student_id"`
	QuestionID   int          `json:"question_id"`
	IsCorrect    bool         `json:"is_correct"`
	PracticeType string       `json:"practice_type"`
	UserAnswer   string       `json:"user_answer"`
	CreatedAt    string       `json:"created_at"`
	Question     *QuestionDTO `json:"question,omitempty"`
}

// boolPtr 从 bool 构造指针（SubmitResultDTO.AIFallback 等可选布尔字段用）。
func boolPtr(v bool) *bool { return &v }
