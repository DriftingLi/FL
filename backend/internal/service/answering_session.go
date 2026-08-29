// Package service 答题会话 module：练习/模拟考试共享的会话语义
// ——守卫（本人+进行中）、题目顺序重建、答题状态三态初始化。
// 存储形态保持存量表不变（practice_progress / mock_exam）。
package service

import (
	"encoding/json"
	"errors"

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
// 批量加载复用 loadQuestionsByIDs；列选择由 loadQuestionsByIDs 的 columns 参数承载。
func loadOrderedQuestions(db *gorm.DB, ids []int) ([]model.Question, map[int]*model.Question) {
	qMap := loadQuestionsByIDs(db, ids)
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
