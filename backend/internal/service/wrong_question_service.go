// Package service 错题本。
package service

import (
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// WrongQuestionService 错题本服务。
type WrongQuestionService struct {
	db *gorm.DB
	// grader 短答 AI 判分 adapter（nil 时简答重做降级，与练习流口径一致）。
	grader ShortAnswerGrader
	// explainer AI 解析 module（与练习提交共用同一 get-or-generate 入口，spec #295/#300）。
	explainer *QuestionExplanation

	logger *zap.Logger
}

// NewWrongQuestionService 创建错题本服务实例。ai 可为 nil（简答判分与解析降级）。
func NewWrongQuestionService(db *gorm.DB, ai *AIService, logger *zap.Logger) *WrongQuestionService {
	return &WrongQuestionService{
		db:        db,
		grader:    shortAnswerGraderOf(ai),
		explainer: NewQuestionExplanation(db, ai, logger),
		logger:    logger,
	}
}

// WrongQuestionDTO 错题本条目（ADR-0009 §2 typed DTO / spec #940 片三）。
//
// 字段按 JSON key 字母序声明（created_at / favorite_id / favorited / id / is_redone /
// is_removed / last_user_answer / last_wrong_at / question / question_id / student_id /
// wrong_count）——旧形态是 map[string]any，encoding/json 对 map 按 key 排序输出，字母序
// 保证换 struct 后序列化字节序不变。Question 带 omitempty：题目行缺失时旧 map 根本不写该 key。
//
// LastUserAnswer（#1077）：学员**最近一次**作答这道题时提交的答案原文，供错题本卡片在
// 折叠态直接做「我的答案 vs 正确答案」对照。从未作答过（错题来自何处无记录）为空串。
type WrongQuestionDTO struct {
	CreatedAt      string       `json:"created_at"`
	FavoriteID     int64        `json:"favorite_id"`
	Favorited      bool         `json:"favorited"`
	ID             int          `json:"id"`
	IsRedone       bool         `json:"is_redone"`
	IsRemoved      bool         `json:"is_removed"`
	LastUserAnswer string       `json:"last_user_answer"`
	LastWrongAt    string       `json:"last_wrong_at"`
	Question       *QuestionDTO `json:"question,omitempty" extensions:"x-optional"`
	QuestionID     int          `json:"question_id"`
	StudentID      int          `json:"student_id"`
	WrongCount     int          `json:"wrong_count"`
}

// WrongQuestionPageDTO 错题本分页（字段按 JSON key 字母序：items / page / page_size / total）。
type WrongQuestionPageDTO struct {
	Items    []WrongQuestionDTO `json:"items" nullability:"nullable"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

// WrongQuestionRemoveResultDTO 移出错题本结果。
type WrongQuestionRemoveResultDTO struct {
	Removed bool `json:"removed"`
}

// WrongQuestionBatchRemoveResultDTO 批量移出错题本结果。
//
// 与单条移除的 DTO 分开：单条是「有没有这条」（bool），批量是「移出了几条」（int）。
// 两个 DTO 的键同名为 removed 但类型不同 —— 分开定型正是为了让这处差异显式，而不是
// 两个 map 里的匿名 int/bool（spec #940 片三）。
type WrongQuestionBatchRemoveResultDTO struct {
	Removed int `json:"removed"`
}

// GetWrongQuestions 错题列表。
// sort: "time_asc" 按最近错误时间升序，其余按降序（默认）；
// favorited: 仅返回已收藏的错题（JOIN favorite，user_id 与 student_id 同源）；
// credentialID: 按题目所属证件分区（与课程/题库同口径，#387；nil 表示不过滤）。
func (s *WrongQuestionService) GetWrongQuestions(studentID, page, pageSize int, qType string, minWrongCount *int, favorited bool, sort string, credentialID *int) (*WrongQuestionPageDTO, error) {
	orderBy := "wrong_question.last_wrong_at DESC"
	if sort == "time_asc" {
		orderBy = "wrong_question.last_wrong_at ASC"
	}
	items, total, page, pageSize, err := paging.Query[model.WrongQuestion](s.db, page, pageSize, 20, orderBy, func(q *gorm.DB) *gorm.DB {
		q = q.Where("student_id = ? AND is_removed = ?", studentID, false)
		if qType != "" || credentialID != nil {
			q = q.Joins("JOIN question ON question.id = wrong_question.question_id")
		}
		if qType != "" {
			q = q.Where("question.type = ?", qType)
		}
		q = EntityOwnedBy(q, "question.credential_id", credentialID)
		if minWrongCount != nil {
			q = q.Where("wrong_question.wrong_count >= ?", *minWrongCount)
		}
		if favorited {
			q = q.Joins("JOIN favorite ON favorite.user_id = wrong_question.student_id AND favorite.target_type = ? AND favorite.target_id = wrong_question.question_id", FavoriteTargetQuestion)
		}
		return q
	})
	if err != nil {
		return nil, err
	}

	questionIDs := make([]int, 0, len(items))
	for i := range items {
		questionIDs = append(questionIDs, items[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, questionIDs)
	favoriteIDs := s.loadFavoriteIDs(studentID, questionIDs)
	lastAnswers := s.loadLastUserAnswers(studentID, questionIDs)

	result := make([]WrongQuestionDTO, 0, len(items))
	for i := range items {
		wq := &items[i]
		favoriteID := favoriteIDs[wq.QuestionID]
		var question *QuestionDTO
		if q, ok := questions[wq.QuestionID]; ok {
			dto := newQuestionDTO(q, true)
			question = &dto
		}
		result = append(result, WrongQuestionDTO{
			CreatedAt:      formatISO(wq.CreatedAt),
			FavoriteID:     favoriteID,
			Favorited:      favoriteID > 0,
			ID:             wq.ID,
			IsRedone:       wq.IsRedone,
			IsRemoved:      wq.IsRemoved,
			LastUserAnswer: lastAnswers[wq.QuestionID],
			LastWrongAt:    formatISO(wq.LastWrongAt),
			Question:       question,
			QuestionID:     wq.QuestionID,
			StudentID:      wq.StudentID,
			WrongCount:     wq.WrongCount,
		})
	}
	return &WrongQuestionPageDTO{
		Items:    result,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// loadLastUserAnswers 批量查询每题「学员最近一次作答」的答案（question_id → user_answer）。
// 「最近」按 (created_at DESC, id DESC) 取首条——同刻并列时用 id 兜底，保证结果稳定可断言。
// 相关子查询在 Postgres 与 SQLite 两方言通用，且一次查询覆盖整页（禁 N+1）。
// 从未作答过的题不出现在结果里，调用方按零值（空串）取值。
func (s *WrongQuestionService) loadLastUserAnswers(studentID int, questionIDs []int) map[int]string {
	result := make(map[int]string, len(questionIDs))
	if len(questionIDs) == 0 {
		return result
	}
	var rows []struct {
		QuestionID int
		UserAnswer string
	}
	if err := s.db.Raw(
		`SELECT r.question_id, r.user_answer
		   FROM question_practice_record r
		  WHERE r.student_id = ?
		    AND r.question_id IN ?
		    AND r.id = (SELECT r2.id
		                  FROM question_practice_record r2
		                 WHERE r2.student_id = r.student_id
		                   AND r2.question_id = r.question_id
		                 ORDER BY r2.created_at DESC, r2.id DESC
		                 LIMIT 1)`,
		studentID, questionIDs,
	).Scan(&rows).Error; err != nil {
		return result
	}
	for _, r := range rows {
		result[r.QuestionID] = r.UserAnswer
	}
	return result
}

// loadFavoriteIDs 批量查询题目收藏 ID（question_id → favorite_id，未收藏为 0）。
func (s *WrongQuestionService) loadFavoriteIDs(studentID int, questionIDs []int) map[int]int64 {
	result := make(map[int]int64, len(questionIDs))
	if len(questionIDs) == 0 {
		return result
	}
	var rows []model.Favorite
	s.db.Where("user_id = ? AND target_type = ? AND target_id IN ?", studentID, FavoriteTargetQuestion, questionIDs).Find(&rows)
	for i := range rows {
		result[rows[i].TargetID] = rows[i].FavoriteID
	}
	return result
}

// RedoWrongQuestion 重做错题：与练习/模拟考试共享同一判分管线（gradeOne）与解析五模块装配。
// 单题即时重做形态（无会话生命周期）；结果落 question_practice_record，正确率/易错项统计含重做。
//
// credentialID 与练习提交同源（本次请求的当前证件，nil = 不分区）：取题按它校验（错题本列表本就按题目证件
// 过滤，这里补上同一道校验，避免绕过列表直调重做别的证件的题）；重做记录的分区也在写入时冻结（ADR-0051），
// 与练习记录同口径。
// 有意**不**施加完整池 scope（published / 排真题）：错题本是「我曾经做错的题」的历史面，题目下架或改标后若
// 在这里被拦，列表会出现点不动的死链 —— 那是错题本读面 scope 的独立议题，不在本票范围。
func (s *WrongQuestionService) RedoWrongQuestion(studentID, questionID int, userAnswer interface{}, credentialID *int) (*SubmitResultDTO, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	q := EntityOwnedBy(s.db.Model(&model.Question{}), "credential_id", credentialID).Where("id = ?", questionID)
	var question model.Question
	if err := q.First(&question).Error; err != nil {
		return nil, errors.New("题目不存在")
	}

	engine := newGradingEngine(s.db)
	flow := gradingFlow{ai: s.grader, maxScore: practiceMaxScore}
	gr := engine.gradeOne(flow, &question, userAnswer, studentID)

	// 重做结果与练习同口径落练习记录（统计事实源单一）。
	rec := model.QuestionPracticeRecord{
		StudentID:    studentID,
		CredentialID: credentialID, // 写入时冻结（ADR-0051）：与练习记录同口径
		QuestionID:   questionID,
		IsCorrect:    gr.IsCorrect != nil && *gr.IsCorrect,
		PracticeType: "redo",
		UserAnswer:   stringifyAnswer(userAnswer),
		CreatedAt:    beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}

	// 错题本状态机：判错路径 gradeOne 已入库计数；此处仅按重做结果维护 is_redone 标记
	// （定向更新，避免覆盖 addToWrongQuestions 刚写入的计数）。
	if gr.IsCorrect != nil {
		if err := s.db.Model(&model.WrongQuestion{}).Where("id = ?", wq.ID).Update("is_redone", *gr.IsCorrect).Error; err != nil {
			return nil, err
		}
	}

	result := &SubmitResultDTO{
		IsCorrect:     gr.IsCorrect,
		CorrectAnswer: question.Answer,
		Explanation:   question.Explanation,
		QuestionID:    questionID,
		UserAnswer:    userAnswer,
	}
	finalizeSubmitResult(s.db, s.explainer, result, gr, &rec, &question)
	return result, nil
}

// RemoveWrongQuestion 移除错题。
func (s *WrongQuestionService) RemoveWrongQuestion(studentID, questionID int) (*WrongQuestionRemoveResultDTO, error) {
	var wq model.WrongQuestion
	if err := s.db.Where("student_id = ? AND question_id = ? AND is_removed = ?", studentID, questionID, false).First(&wq).Error; err != nil {
		return nil, errors.New("错题记录不存在")
	}
	wq.IsRemoved = true
	s.db.Save(&wq)
	return &WrongQuestionRemoveResultDTO{Removed: true}, nil
}

// GetStats 错题统计（经统计聚合 module，一次 GROUP BY）。
// 保留旧语义：仅统计未移除错题；by_type 只含实际存在题型的维度（不零填充）。
func (s *WrongQuestionService) GetStats(studentID int) *WrongQuestionStatsDTO {
	var total int64
	s.db.Model(&model.WrongQuestion{}).Where("student_id = ? AND is_removed = ?", studentID, false).Count(&total)
	byType := groupByCount(
		s.db.Model(&model.WrongQuestion{}).
			Joins("JOIN question ON question.id = wrong_question.question_id").
			Where("wrong_question.student_id = ? AND wrong_question.is_removed = ?", studentID, false),
		"question.type",
	)
	return &WrongQuestionStatsDTO{Total: total, ByType: byType}
}

// ExportWrongQuestions 导出错题。
func (s *WrongQuestionService) ExportWrongQuestions(studentID int) []map[string]any {
	var items []model.WrongQuestion
	s.db.Where("student_id = ? AND is_removed = ?", studentID, false).Find(&items)

	qIDs := make([]int, 0, len(items))
	for i := range items {
		qIDs = append(qIDs, items[i].QuestionID)
	}
	questions := loadQuestionsByIDs(s.db, qIDs)

	exportData := make([]map[string]any, 0, len(items))
	for i := range items {
		wq := &items[i]
		question, ok := questions[wq.QuestionID]
		if !ok {
			continue
		}
		var options interface{}
		if len(question.Options) > 0 {
			_ = jsonUnmarshal(question.Options, &options)
		}
		item := map[string]any{
			"question_id":    question.ID,
			"type":           question.Type,
			"content":        question.Content,
			"options":        options,
			"correct_answer": question.Answer,
			"explanation":    question.Explanation,
			"wrong_count":    wq.WrongCount,
			"image_url":      question.ImageURL,
			"last_wrong_at":  formatISO(wq.LastWrongAt),
		}
		exportData = append(exportData, item)
	}
	return exportData
}

// FormatWrongQuestionsText 格式化错题文本。
func FormatWrongQuestionsText(exportData []map[string]any) string {
	typeMap := map[string]any{
		"single_choice": "单选题",
		"multi_choice":  "多选题",
		"true_false":    "判断题",
		"fault_image":   "故障识图",
		"short_answer":  "简答题",
	}
	now := beijingNow().Format("2006-01-02 15:04:05")
	var sb strings.Builder
	sb.WriteString(strings.Repeat("=", 50))
	sb.WriteString("\n错题本导出\n")
	fmt.Fprintf(&sb, "导出时间: %s\n", now)
	fmt.Fprintf(&sb, "错题总数: %d\n", len(exportData))
	sb.WriteString(strings.Repeat("=", 50))

	for idx, item := range exportData {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "【第%d题】\n", idx+1)
		sb.WriteString(strings.Repeat("-", 40))
		sb.WriteString("\n")
		qType, _ := item["type"].(string)
		fmt.Fprintf(&sb, "题型: %s\n", mapOr(qType, typeMap, qType))
		content, _ := item["content"].(string)
		fmt.Fprintf(&sb, "题目: %s\n", content)

		if options, ok := item["options"].(map[string]any); ok && len(options) > 0 {
			sb.WriteString("选项:\n")
			keys := make([]string, 0, len(options))
			for k := range options {
				keys = append(keys, k)
			}
			sortStrings(keys)
			for _, k := range keys {
				fmt.Fprintf(&sb, "  %s. %v\n", k, options[k])
			}
		}

		correctAnswer, _ := item["correct_answer"].(string)
		fmt.Fprintf(&sb, "正确答案: %s\n", correctAnswer)
		if explanation, ok := item["explanation"].(string); ok && explanation != "" {
			fmt.Fprintf(&sb, "解析: %s\n", explanation)
		}
		wrongCount := toInt(item["wrong_count"])
		fmt.Fprintf(&sb, "错误次数: %d\n", wrongCount)
		if lastWrong, ok := item["last_wrong_at"].(string); ok && lastWrong != "" {
			fmt.Fprintf(&sb, "最近错误时间: %s\n", lastWrong)
		}
		if imgURL, ok := item["image_url"].(string); ok && imgURL != "" {
			fmt.Fprintf(&sb, "图片: %s\n", imgURL)
		}
		sb.WriteString(strings.Repeat("-", 40))
	}

	sb.WriteString("\n")
	fmt.Fprintf(&sb, "\n共 %d 道错题\n", len(exportData))
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("=", 50))
	return sb.String()
}

// BatchRemoveWrongQuestions 批量移出错题本
func (s *WrongQuestionService) BatchRemoveWrongQuestions(studentID int, questionIDs []int) (int, error) {
	if len(questionIDs) == 0 {
		return 0, errors.New("请选择要移除的题目")
	}
	res := s.db.Model(&model.WrongQuestion{}).Where("student_id = ? AND question_id IN ? AND is_removed = ?", studentID, questionIDs, false).Update("is_removed", true)
	return int(res.RowsAffected), res.Error
}

func mapOr(key string, m map[string]any, def any) any {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
