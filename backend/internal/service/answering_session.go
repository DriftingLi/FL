// Package service 答题会话 module：练习/模拟考试/定级考试共享的会话语义
// ——守卫（本人+进行中）、题目顺序重建、答题状态三态初始化、场次状态展示。
// 存储形态保持三张存量表不变（practice_progress / mock_exam / exam_participant）。
package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// guardOwnedInProgress 答题会话守卫：记录归属当前学员且处于进行中。
// msgDenied / msgNotInProgress 为各流既有文案（行为冻结）。
func guardOwnedInProgress(recordStudentID int, status string, studentID int, msgDenied, msgNotInProgress string) error {
	if recordStudentID != studentID {
		return errors.New(msgDenied)
	}
	if status != "in_progress" {
		return errors.New(msgNotInProgress)
	}
	return nil
}

// loadOrderedQuestions 按保存顺序加载题目：ids → 查库 → qMap → 有序列表（缺失 id 跳过）。
// 返回 (ordered, qMap)：ordered 保持传入顺序，qMap 供逐题取用。
func loadOrderedQuestions(db *gorm.DB, ids []int) ([]model.Question, map[int]*model.Question) {
	var questions []model.Question
	if len(ids) > 0 {
		db.Where("id IN ?", ids).Find(&questions)
	}
	qMap := map[int]*model.Question{}
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}
	ordered := make([]model.Question, 0, len(ids))
	for _, qid := range ids {
		if q, ok := qMap[qid]; ok {
			ordered = append(ordered, *q)
		}
	}
	return ordered, qMap
}

// answersMapRoundTrip 答题快照 JSONB → map（nil / JSON null 归一为空 map）。
func answersMapRoundTrip(raw model.JSONB) map[string]any {
	var m map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m
}

// initAnswersState answers_state 三态初始化：#142 修复——nil / 空 / 显式 null 一律
// 归一为 {}，禁止 JSONB 'null' 落库产生 SQL NULL；有内容时原样保留。
func initAnswersState(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil || v == nil {
		return json.RawMessage("{}")
	}
	return raw
}

// effectiveExamStatus 场次生效状态（读路径展示用，不落库）：
// 已到开始时间按 ongoing 展示，已过结束时间按 finished 展示，finished 状态恒定。
func effectiveExamStatus(status string, startTime, endTime, now time.Time) string {
	eff := status
	if eff == "upcoming" && now.After(startTime) {
		eff = "ongoing"
	}
	if eff != "finished" && now.After(endTime) {
		eff = "finished"
	}
	return eff
}

// advanceExamStatus 写路径状态推进：upcoming 且已过开始时间 → ongoing。
// 返回 (新状态, 是否推进)。仅由进入考试等写操作调用。
func advanceExamStatus(status string, startTime, now time.Time) (string, bool) {
	if status == "upcoming" && now.After(startTime) {
		return "ongoing", true
	}
	return status, false
}
