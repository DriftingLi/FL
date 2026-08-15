// Package service 保存会话进度 module：练习/模拟考试/定级考试共享的
// 「保存会话进度快照」语义 —— 守卫裁定（本人+在途校验策略由调用方声明）、
// 答案快照 JSONB 三态归一、快照写回（load → 改 JSONB → db.Save）。
// 存储形态保持三张存量表不变（practice_progress / mock_exam / exam_participant），
// 守卫统一为最严口径：提交后/已结束的会话不再接受进度保存（提交晚到静默忽略）。
package service

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// SessionProgressSpec 声明单一「保存会话进度」流的持久化与守卫裁定策略。
// 三个流只注入此 spec（表/字段 + 在途校验策略），共享 saveSessionProgress 的
// 唯一写回实现：load → guard 守卫裁定 → 快照 JSONB 三态归一 → db.Save。
//
// 在途校验策略由调用方声明：
//   - mock/level：校验 status == "in_progress"（提交后/已结束拒绝保存）；
//   - practice：practice_progress 无 status 字段（schema 冻结 ADR-0010），
//     经 (student_id, practice_mode) 定位即天然归属本人，且无终端状态，恒视为在途。
type SessionProgressSpec[T any] struct {
	// notFoundErr 记录不存在时的错误文案（各流既有文案，行为冻结）。
	// load 返回非 nil 错误即按该文案返回（原实现任何 First 错误都折叠为「不存在」）。
	notFoundErr string
	// load 按流声明的查询条件加载记录；非 nil error 表示不存在/加载失败。
	load func(db *gorm.DB) (rec T, err error)
	// guard 守卫裁定：记录归属当前学员且处于进行中。返回非 nil 即拒绝。
	guard func(rec T) error
	// write 将归一化后的答案快照与剩余时间写回记录（调用方声明改哪些字段）。
	write func(rec *T, snapshot model.JSONB, remainingTime int)
}

// saveSessionProgress 快照写回唯一实现：load → guard 守卫裁定 → 答案快照 JSONB
// 三态归一 → db.Save。提交晚到的保存经 guard 拒绝且不落库（静默忽略），三流行为一致。
// 深模块形态：单一入口 + spec 三回调，把守卫裁定、JSONB 归一、回写实现集中于此，
// 三个流（练习/模拟/定级）只注入表/字段 + 在途校验策略，语义只实现一次。
func saveSessionProgress[T any](db *gorm.DB, spec SessionProgressSpec[T], answers map[string]any, remainingTime int) error {
	rec, err := spec.load(db)
	if err != nil {
		return errors.New(spec.notFoundErr)
	}
	if err := spec.guard(rec); err != nil {
		return err
	}
	spec.write(&rec, marshalAnswersSnapshot(answers), remainingTime)
	return db.Save(&rec).Error
}

// marshalAnswersSnapshot 答案快照归一落库：#142 修复——nil / 空 / 显式 null 一律落库
// 为 {}，禁止 JSONB 'null' 写库产生 SQL NULL；有内容时原样保留。
func marshalAnswersSnapshot(answers map[string]any) model.JSONB {
	if len(answers) == 0 {
		return model.JSONB([]byte("{}"))
	}
	b, err := json.Marshal(answers)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return model.JSONB([]byte("{}"))
	}
	return model.JSONB(b)
}
