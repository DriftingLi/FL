// Package service 课程考核（基于 JSON 题库），对应 Python exam_service。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// 单选/多选分值，与 Python 一致。
const (
	examSingleScore = 3
	examMultiScore  = 4
)

// examQuestionsFile 题库 JSON 文件路径，启动时解析。
var examQuestionsFile = filepath.Join("data", "exam_questions.json")

var (
	examQuestionsCache     map[string]interface{}
	examQuestionsCacheOnce sync.Once
	examQuestionsCacheErr  error
)

// ExamService 课程考核服务。
type ExamService struct {
	db *gorm.DB
}

// NewExamService 创建考核服务。
func NewExamService(db *gorm.DB) *ExamService {
	return &ExamService{db: db}
}

// SetExamQuestionsFile 覆盖题库文件路径（测试用）。
func SetExamQuestionsFile(path string) {
	examQuestionsFile = path
	examQuestionsCache = nil
	examQuestionsCacheOnce = sync.Once{}
}

// loadQuestions 加载并缓存 JSON 题库，对应 Python _load_questions。
func loadQuestions() (map[string]interface{}, error) {
	examQuestionsCacheOnce.Do(func() {
		b, err := os.ReadFile(examQuestionsFile)
		if err != nil {
			examQuestionsCacheErr = fmt.Errorf("题库文件读取失败: %w", err)
			return
		}
		var data map[string]interface{}
		if err := json.Unmarshal(b, &data); err != nil {
			examQuestionsCacheErr = fmt.Errorf("题库文件解析失败: %w", err)
			return
		}
		examQuestionsCache = data
	})
	return examQuestionsCache, examQuestionsCacheErr
}

// GetExamQuestions 获取课程考核题目（乱序、选项乱序），对应 Python get_exam_questions。
func (s *ExamService) GetExamQuestions(courseID int) (map[string]interface{}, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	all, err := loadQuestions()
	if err != nil {
		return nil, err
	}
	courseKey := fmt.Sprintf("%d", courseID)
	courseData, ok := all[courseKey].(map[string]interface{})
	if !ok {
		return nil, errors.New("该课程暂无考核题目")
	}
	rawQuestions, _ := courseData["questions"].([]interface{})
	questions := make([]map[string]interface{}, 0, len(rawQuestions))
	for _, rq := range rawQuestions {
		q, ok := rq.(map[string]interface{})
		if !ok {
			continue
		}
		cp := copyQuestion(q)
		shuffleOptions(cp)
		questions = append(questions, cp)
	}
	rand.Shuffle(len(questions), func(i, j int) { questions[i], questions[j] = questions[j], questions[i] })

	totalScore := 0
	for _, q := range rawQuestions {
		if t, _ := q.(map[string]interface{})["type"].(string); t == "multi_choice" {
			totalScore += examMultiScore
		} else {
			totalScore += examSingleScore
		}
	}
	return map[string]interface{}{
		"course_id":       courseID,
		"course_name":     courseData["course_name"],
		"total_score":     totalScore,
		"total_questions": len(questions),
		"single_score":    examSingleScore,
		"multi_score":     examMultiScore,
		"questions":       questions,
	}, nil
}

// SubmitExam 提交考核，评分并保存 ExamRecord，对应 Python submit_exam。
func (s *ExamService) SubmitExam(studentID, courseID int, answers map[string]interface{}) (map[string]interface{}, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	all, err := loadQuestions()
	if err != nil {
		return nil, err
	}
	courseKey := fmt.Sprintf("%d", courseID)
	courseData, ok := all[courseKey].(map[string]interface{})
	if !ok {
		return nil, errors.New("该课程暂无考核题目")
	}
	rawQuestions, _ := courseData["questions"].([]interface{})

	totalScore := 0
	correctCount := 0
	details := make([]map[string]interface{}, 0, len(rawQuestions))

	for _, rq := range rawQuestions {
		q, _ := rq.(map[string]interface{})
		qID := fmt.Sprintf("%v", q["question_id"])
		qType, _ := q["type"].(string)
		userAnswer := answers[qID]

		var isCorrect bool
		var questionScore int
		if qType == "single_choice" {
			questionScore = examSingleScore
			isCorrect = stringifyAnswer(userAnswer) == stringifyAnswer(q["answer"])
		} else if qType == "multi_choice" {
			questionScore = examMultiScore
			correct := normalizeAnswerList(stringifyAnswer(q["answer"]))
			user := normalizeUserAnswerList(userAnswer)
			isCorrect = user != nil && stringSliceEqual(correct, user)
		}
		if isCorrect {
			correctCount++
		}
		if qType == "multi_choice" {
			totalScore += examMultiScore
		} else {
			totalScore += examSingleScore
		}

		earned := 0
		if isCorrect {
			earned = questionScore
		}
		details = append(details, map[string]interface{}{
			"question_id":    q["question_id"],
			"type":           qType,
			"question_text":  q["question_text"],
			"user_answer":    userAnswer,
			"correct_answer": q["answer"],
			"is_correct":     isCorrect,
			"score":          earned,
			"total_score":    questionScore,
			"explanation":    q["explanation"],
			"options":        q["options"],
		})
	}

	finalScore := 0
	for _, d := range details {
		finalScore += toInt(d["score"])
	}

	detailsJSON, _ := json.Marshal(details)
	var rec model.ExamRecord
	err = s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).First(&rec).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		rec = model.ExamRecord{
			StudentID: studentID,
			CourseID:  courseID,
			Score:     floatPtr(float64(finalScore)),
			Answers:   model.JSONB(detailsJSON),
			ExamDate:  beijingNow(),
		}
		if err := s.db.Create(&rec).Error; err != nil {
			return nil, err
		}
	case err == nil:
		rec.Score = floatPtr(float64(finalScore))
		rec.Answers = model.JSONB(detailsJSON)
		rec.ExamDate = beijingNow()
		if err := s.db.Save(&rec).Error; err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	_ = cache.InvalidatePattern(context.Background(), cache.SafeKey("exam", "result", fmt.Sprintf("%d", studentID), fmt.Sprintf("%d", courseID)))
	_ = cache.InvalidatePattern(context.Background(), cache.SafeKey("student", "profile", fmt.Sprintf("%d", studentID)))
	_ = cache.InvalidatePattern(context.Background(), "admin:stats")
	return map[string]interface{}{
		"exam_id":         rec.ExamID,
		"score":           finalScore,
		"total_score":     totalScore,
		"correct_count":   correctCount,
		"total_questions": len(rawQuestions),
		"details":         details,
	}, nil
}

// GetExamResult 查询考核结果，对应 Python get_exam_result。
func (s *ExamService) GetExamResult(studentID, courseID int) (map[string]interface{}, error) {
	cacheKey := cache.SafeKey("exam", "result", fmt.Sprintf("%d", studentID), fmt.Sprintf("%d", courseID))
	var result map[string]interface{}
	err := cache.GetOrSetJSON(context.Background(), cacheKey, cache.TTLStats, &result, func() (interface{}, error) {
		var rec model.ExamRecord
		err := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).First(&rec).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		all, _ := loadQuestions()
		courseKey := fmt.Sprintf("%d", courseID)
		totalScore := 0
		if courseData, ok := all[courseKey].(map[string]interface{}); ok {
			if raws, _ := courseData["questions"].([]interface{}); ok {
				for _, rq := range raws {
					if q, _ := rq.(map[string]interface{}); q != nil {
						if t, _ := q["type"].(string); t == "multi_choice" {
							totalScore += examMultiScore
						} else {
							totalScore += examSingleScore
						}
					}
				}
			}
		}
		var details []interface{}
		if len(rec.Answers) > 0 {
			_ = json.Unmarshal(rec.Answers, &details)
		}
		correctCount := 0
		for _, d := range details {
			if m, ok := d.(map[string]interface{}); ok && m["is_correct"] == true {
				correctCount++
			}
		}
		score := 0.0
		if rec.Score != nil {
			score = *rec.Score
		}
		return map[string]interface{}{
			"exam_id":         rec.ExamID,
			"score":           score,
			"total_score":     totalScore,
			"correct_count":   correctCount,
			"total_questions": len(details),
			"details":         details,
			"exam_date":       formatISO(rec.ExamDate),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetExamHistory 学员考核历史，对应 Python get_exam_history。
func (s *ExamService) GetExamHistory(studentID int) []map[string]interface{} {
	var records []model.ExamRecord
	s.db.Where("student_id = ?", studentID).Order("exam_date DESC").Find(&records)
	out := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		var course model.Course
		courseName := "未知课程"
		if err := s.db.First(&course, r.CourseID).Error; err == nil {
			courseName = course.Name
		}
		score := 0.0
		if r.Score != nil {
			score = *r.Score
		}
		out = append(out, map[string]interface{}{
			"exam_id":     r.ExamID,
			"course_id":   r.CourseID,
			"course_name": courseName,
			"score":       score,
			"exam_date":   formatISO(r.ExamDate),
		})
	}
	return out
}

// copyQuestion 深拷贝题目 map（含 options）。
func copyQuestion(q map[string]interface{}) map[string]interface{} {
	cp := make(map[string]interface{}, len(q))
	for k, v := range q {
		if k == "options" {
			if opts, ok := v.(map[string]interface{}); ok {
				optCopy := make(map[string]interface{}, len(opts))
				for k2, v2 := range opts {
					optCopy[k2] = v2
				}
				cp[k] = optCopy
			} else {
				cp[k] = v
			}
		} else {
			cp[k] = v
		}
	}
	return cp
}

// shuffleOptions 单选题选项乱序，对应 Python _shuffle_options。
func shuffleOptions(q map[string]interface{}) {
	qType, _ := q["type"].(string)
	if qType != "single_choice" {
		return
	}
	opts, ok := q["options"].(map[string]interface{})
	if !ok {
		return
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	rand.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	shuffled := make(map[string]interface{}, len(opts))
	for _, k := range keys {
		shuffled[k] = opts[k]
	}
	q["options"] = shuffled
	_ = sort.StringSlice(nil)
}
