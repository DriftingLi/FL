// 本文件：联络授权的读面（ADR-0053 §3）——「什么算有效授权」的单点。
//
// 三个消费面此前各写一遍判据（同包）：
//  1. GetContact（招聘方取学员明文）—— SQL 过滤 status='approved'；
//  2. toDTO（学员侧看企业明文）—— 行内字符串判定 `m.Status == "approved"`；
//  3. fillContactStates（企业视角徽章）—— 自写「approved > pending」优先级。
//
// 加上写侧的一处内联 SQL（撤投连带置 revoked），授权的状态字面量在服务层出现 19 次，
// 「学员注销即一并失效」这条口径只有 GetContact 一处实现。
//
// 本文件把它收成：**具名五态** + **两个查询形态**（单条 / 批量）+ **两个谓词**：
//   - `GrantsPlaintext()`：行级（该条记录是否透出对方明文）——toDTO 用，零查询、不产生 N+1；
//   - `Effective()`：对级（这一对企业↔学员是否可取明文）——GetContact 用。
//
// 这是 **internal seam**：三个 caller 全在 service 包内，故不新增导出面、不改 deps 装配。
package service

import (
	"errors"
	"sort"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ContactGrantState 授权状态（具名五态，取值域与 contact_requests.status 一一对应）。
type ContactGrantState string

const (
	// ContactGrantPending 待学员裁决。
	ContactGrantPending ContactGrantState = "pending"
	// ContactGrantApproved 已批准：透出对方明文联系方式。
	ContactGrantApproved ContactGrantState = "approved"
	// ContactGrantRejected 学员拒绝（30 天冷却）。
	ContactGrantRejected ContactGrantState = "rejected"
	// ContactGrantExpired pending 超过 14 天未裁决。
	ContactGrantExpired ContactGrantState = "expired"
	// ContactGrantRevoked 已批准后被撤回（撤回投递的连带迁移会置此态）。
	ContactGrantRevoked ContactGrantState = "revoked"
)

// ContactGrantSource 授权来源。
type ContactGrantSource string

const (
	// ContactGrantSourceRecruiter 企业发起（联系方式交换申请）。
	ContactGrantSourceRecruiter ContactGrantSource = "recruiter"
	// ContactGrantSourceApplication 投递产生（学员主动投递即授权）。
	ContactGrantSourceApplication ContactGrantSource = "application"
)

// GrantsPlaintext 该状态是否透出对方明文联系方式——**只有已批准透出**。
// 行级谓词：调用方手上已有一条 contact_requests 记录时用它，不需要再查一次库。
func (s ContactGrantState) GrantsPlaintext() bool { return s == ContactGrantApproved }

// contactGrant 一对 (企业, 学员) 上的授权结论。
type contactGrant struct {
	// State 展示态：approved（存在已批准）/ pending（无已批准但有待裁决）/ 空串（未授权）。
	// 它是「有效授权态的三值投影」，不是另一套状态。
	State ContactGrantState
	// Source 承载 State 的那条记录的来源。
	Source ContactGrantSource
}

// Effective 是否为**有效授权**：可以取对方明文联系方式的资格。
// 语义由读取函数保证（见 contactGrantOfMany 的「学员注销即失效」）。
func (g contactGrant) Effective() bool { return g.State == ContactGrantApproved }

// contactGrantOfMany 批量读取 (企业, 学员集合) 上的授权结论。
//
// 口径（与原三处实现逐字等价）：
//   - 存在已批准 → approved（来源取最新的那条 approved，按 created_at 降序）；
//   - 否则存在待裁决 → pending（同理取最新）；
//   - 其余（拒绝 / 过期 / 已撤回 / 无记录）→ 空串（未授权）。
//
// 「学员注销即一并失效」在这一个入口执行：对判为 approved 的学员做**一次批量**账号存在性
// 校验，已注销的降级为未授权——于是徽章与明文门禁看到的是同一个事实。
func contactGrantOfMany(db *gorm.DB, recruiterID int, studentUserIDs []int) (map[int]contactGrant, error) {
	out := make(map[int]contactGrant, len(studentUserIDs))
	if recruiterID <= 0 || len(studentUserIDs) == 0 {
		return out, nil
	}

	var rows []model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id IN ?", recruiterID, studentUserIDs).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		cur, ok := out[r.StudentUserID]
		state := ContactGrantState(r.Status)
		switch {
		case state == ContactGrantApproved:
			// approved 压过一切（approved 覆盖 pending），取最新一条 approved
			if !ok || cur.State != ContactGrantApproved {
				out[r.StudentUserID] = contactGrant{State: state, Source: ContactGrantSource(r.Source)}
			}
		case state == ContactGrantPending:
			if !ok {
				out[r.StudentUserID] = contactGrant{State: state, Source: ContactGrantSource(r.Source)}
			}
		}
	}
	return out, nil
}

// contactGrantOf 单条形态（批量取一个），供只关心一对 (企业, 学员) 的调用方使用。
// 无授权记录时返回零值（State 为空串 = 未授权）。
func contactGrantOf(db *gorm.DB, recruiterID, studentUserID int) (contactGrant, error) {
	grants, err := contactGrantOfMany(db, recruiterID, []int{studentUserID})
	if err != nil {
		return contactGrant{}, err
	}
	return grants[studentUserID], nil
}

// contactGrantEffectiveOf 单条 + 「学员注销即失效」语义（明文门禁用）。
//
// 返回 (授权结论, 是否有效)。学员账号已不在时结论降级为未授权，并以 ErrStudentGone 区分
// 「无授权」与「学员已注销」两种失败原因（既有文案逐字不变）。
func contactGrantEffectiveOf(db *gorm.DB, recruiterID, studentUserID int) (contactGrant, error) {
	grant, err := contactGrantOf(db, recruiterID, studentUserID)
	if err != nil {
		return contactGrant{}, err
	}
	if !grant.Effective() {
		return grant, ErrContactNoAuth
	}
	if err := studentAccountAlive(db, studentUserID); err != nil {
		return contactGrant{}, err
	}
	return grant, nil
}

// studentAccountAlive 学员账号是否存在（hrwai_users 无此 id 即视为已注销）。
func studentAccountAlive(db *gorm.DB, studentUserID int) error {
	var stu model.HrwaiUser
	if err := db.First(&stu, studentUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStudentGone
		}
		return err
	}
	return nil
}

// contactGrantOfManyEffective 批量形态 + 「学员注销即失效」（企业视角徽章用）。
// 「在册」判定对整批只查一次，不随卡片数增长。
func contactGrantOfManyEffective(db *gorm.DB, recruiterID int, studentUserIDs []int) (map[int]contactGrant, error) {
	grants, err := contactGrantOfMany(db, recruiterID, studentUserIDs)
	if err != nil {
		return nil, err
	}
	if err := contactGrantDropGoneStudents(db, grants); err != nil {
		return nil, err
	}
	return grants, nil
}

// contactGrantDropGoneStudents 把「判为已批准但学员账号已不在」的条目降级为未授权
// （与明文门禁同口径：注销即一并失效）。一次 pluck 覆盖整批。
func contactGrantDropGoneStudents(db *gorm.DB, grants map[int]contactGrant) error {
	ids := make([]int, 0, len(grants))
	for id, g := range grants {
		if g.Effective() {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Ints(ids)
	var alive []int
	if err := db.Model(&model.HrwaiUser{}).Where("id IN ?", ids).Pluck("id", &alive).Error; err != nil {
		return err
	}
	aliveSet := make(map[int]bool, len(alive))
	for _, id := range alive {
		aliveSet[id] = true
	}
	for _, id := range ids {
		if !aliveSet[id] {
			delete(grants, id)
		}
	}
	return nil
}
