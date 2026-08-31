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

// ResumeSetSpec 断点续练协商参数（#385 装配面；三类开始/续练入口既有行为冻结）。
type ResumeSetSpec struct {
	// Mode practice_progress.practice_mode 标识（sequential / tag:<id> / paper:<id>）。
	Mode string
	// FreshIDs 池现集合的目标顺序（tag/顺序练习按 id 升序，按卷练习按卷序）。
	FreshIDs []int
	// ReuseSaved 同集沿用已存顺序（标签/按卷练习 true；顺序练习 false——恒用现顺序）。
	ReuseSaved bool
	// KeepCursorOnRefresh 集合变化时保留未越界游标（顺序练习 true——游标只对越界复位；
	// 标签/按卷练习 false——集合变化即复位 0）。
	KeepCursorOnRefresh bool
	// Sample 新开始时的抽样策略（标签练习按 count 洗牌截断固定顺序；nil = 不抽样）。
	Sample func(ids []int) []int
}

// ResumeSet 断点续练协商单点（#385）：加载进度 → 顺序协商 → 游标截断/复位 → 落库。
// 返回最终题目顺序与起始游标。协商规则（四入口既有语义冻结）：
//   - 进度存在且游标未完成（CurrentIndex < Total）且 ReuseSaved 且已存顺序与现集合
//     同集（sameIDSet）→ 沿用已存顺序与游标（断点续练）；
//   - 顺序练习（KeepCursorOnRefresh）：恒用现集合，游标未越界则保留、越界复位 0；
//   - 其余为新开始：现集合 + 游标 0；Sample 非 nil 且属「新开始」（无进度或已练完，
//     标签练习既有条件）时抽样固定顺序。
func ResumeSet(db *gorm.DB, studentID int, credentialID *int, spec ResumeSetSpec) (ids []int, startIdx int, err error) {
	var prog model.PracticeProgress
	// #414：证件分区定位（nil → IS NULL，Postgres 不接受 IS 参数占位符）
	clause, args := credentialClause(credentialID)
	if err := db.Where("student_id = ? AND practice_mode = ? AND "+clause, append([]any{studentID, spec.Mode}, args...)...).Limit(1).Find(&prog).Error; err != nil {
		return nil, 0, err
	}
	ids = spec.FreshIDs
	startIdx = 0
	switch {
	case spec.KeepCursorOnRefresh:
		// 顺序练习：恒现顺序，游标对「新总数」未越界则保留（跨集合变化保留游标）
		if prog.ID != 0 && prog.CurrentIndex < len(spec.FreshIDs) {
			startIdx = prog.CurrentIndex
		}
	case spec.ReuseSaved && prog.ID != 0 && prog.CurrentIndex < prog.Total:
		// 标签/按卷练习：同集沿用已存顺序与游标；集合已变则按现集合刷新（游标复位）
		var saved []int
		if err := json.Unmarshal(prog.QuestionIDs, &saved); err == nil && len(saved) > 0 && sameIDSet(saved, spec.FreshIDs) {
			ids = saved
			startIdx = prog.CurrentIndex
		}
	}
	if startIdx == 0 && (prog.ID == 0 || prog.CurrentIndex >= prog.Total) && spec.Sample != nil {
		// 新开始（首次进入或已练完）：抽样并固定顺序
		ids = spec.Sample(ids)
	}
	if err := SaveSet(db, studentID, spec.Mode, credentialID, ids, startIdx, len(ids), nil); err != nil {
		return nil, 0, err
	}
	return ids, startIdx, nil
}

// SaveSet practice_progress 单次落库（#385 装配单点，练习进度的唯一写入口）。
//   - ids 非 nil：写入题目顺序与 total（开始/续练流，顺序协商产物）；
//   - ids 为 nil：不触碰既有顺序（进度保存流不得击穿断点续练的已存顺序），
//     记录不存在则建空顺序行，total > 0 时同步 total；
//   - answers 非 nil：写答案快照（进度保存流经三态归一后传入；开始流传 nil，
//     创建时落初始化空对象）。
func SaveSet(db *gorm.DB, studentID int, mode string, credentialID *int, ids []int, startIdx, total int, answers json.RawMessage) error {
	var prog model.PracticeProgress
	clause, args := credentialClause(credentialID)
	if err := db.Where("student_id = ? AND practice_mode = ? AND "+clause, append([]any{studentID, mode}, args...)...).Limit(1).Find(&prog).Error; err != nil {
		return err
	}
	if prog.ID == 0 {
		if ids == nil {
			ids = []int{}
		}
		if answers == nil {
			answers = json.RawMessage("{}")
		}
		prog = model.PracticeProgress{
			StudentID:    studentID,
			PracticeMode: mode,
			CredentialID: credentialID,
			QuestionIDs:  model.JSONB(marshalIDs(ids)),
			CurrentIndex: startIdx,
			Total:        total,
			AnswersState: model.JSONB(answers),
			UpdatedAt:    beijingNow(),
		}
		return db.Create(&prog).Error
	}
	updates := map[string]any{"current_index": startIdx, "updated_at": beijingNow()}
	if ids != nil {
		updates["question_ids"] = model.JSONB(marshalIDs(ids))
		updates["total"] = total
	} else if total > 0 {
		updates["total"] = total
	}
	if answers != nil {
		updates["answers_state"] = model.JSONB(answers)
	}
	return db.Model(&prog).Updates(updates).Error
}

// credentialClause 证件分区 WHERE 片段（#414）：nil → `credential_id IS NULL`（PG 不支持 IS 参数占位）。
func credentialClause(credentialID *int) (string, []any) {
	if credentialID == nil {
		return "credential_id IS NULL", nil
	}
	return "credential_id = ?", []any{*credentialID}
}

// marshalIDs 题目顺序序列化（失败落空数组——顺序仅是缓存态，空数组等价重新协商）。
func marshalIDs(ids []int) []byte {
	b, err := json.Marshal(ids)
	if err != nil {
		return []byte("[]")
	}
	return b
}
