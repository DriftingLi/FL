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
//   - `GrantsPlaintext()`：行级（该条记录的状态是否透出对方明文）；
//   - `Effective()`：对级（这一对企业↔学员是否存在已批准的授权）。
//
// **第十四波补的对称半边（ADR-0062 决策 9）**：上面两个谓词都只答「授权在不在」，
// 而 CONTEXT.md「授权有效态」要的是「**现在还可不可用**」——判据是「存在已批准授权 **且双方账号
// 均有效**（未注销、未被禁用）」。原实现里存活判据只有学员侧一条（`studentAccountAlive`），
// 学员看企业那条走零查询的行级谓词，于是**企业被禁用后已授权学员仍看得到其电话/邮箱/微信**。
// 现在三个事实各有一条具名判据、合取式只写一遍（`contactPairUsable`），两个读取形态共用：
//   - `contactGrantEffectiveOf`：成对形态（各查一次）——GetContact 用；
//   - `toDTOs` + `contactPairUsable`：行级形态（一次批量取回后逐行判）——两侧列表用，不产生 N+1。
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
//
// ⚠️ 它只答「授权事实批准了没有」，**不答「现在还能不能用」**——后者是 contactPairUsable
// （把 `Effective` 一词兼表两义的旧用法拆开；ADR-0062 决策 9）。
func (s ContactGrantState) GrantsPlaintext() bool { return s == ContactGrantApproved }

// contactPairUsable 明文门禁的**合取式单点**：存在已批准授权 ∧ 学员账号有效 ∧ 企业账号有效。
//
// 两个读取形态共用这一条，区别只在三个事实怎么取（成对形态各查一次；行级形态批量取回后逐行判）。
// 本波的缺陷正是「合取式被写了两遍、其中一遍少了企业那一维」——写两遍才会漏一半，
// 收成一条后「补一维」只需改一个入参的来源。
func contactPairUsable(grantApproved, studentUsable, companyUsable bool) bool {
	return grantApproved && studentUsable && companyUsable
}

// contactGrant 一对 (企业, 学员) 上的授权结论。
type contactGrant struct {
	// State 展示态：approved（存在已批准）/ pending（无已批准但有待裁决）/ 空串（未授权）。
	// 它是「有效授权态的三值投影」，不是另一套状态。
	State ContactGrantState
	// Source 承载 State 的那条记录的来源。
	Source ContactGrantSource
}

// Effective 授权事实这一维是否已批准（≠「现在可用」——可用性还要两侧账号都有效，
// 那一位在 contactPairUsable 里；本波的缺陷就是只算了这一维）。
func (g contactGrant) Effective() bool { return g.State == ContactGrantApproved }

// contactGrantOfMany 批量读取 (企业, 学员集合) 上的授权结论（**只读授权事实**，不查账号）。
//
// 口径（与原三处实现逐字等价）：
//   - 存在已批准 → approved（来源取最新的那条 approved，按 created_at 降序）；
//   - 否则存在待裁决 → pending（同理取最新）；
//   - 其余（拒绝 / 过期 / 已撤回 / 无记录）→ 空串（未授权）。
//
// 账号存活的那一维由 Effective 后缀的两个入口补齐（contactGrantOfManyEffective 一次批量、
// contactGrantEffectiveOf 各查一次）——「查不动」一律上抛，不当成「查得空」。
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

// contactGrantEffectiveOf 明文门禁的**成对判据**（两个方向共用这一条，ADR-0062 决策 9）：
// 存在已批准授权 ∧ 双方账号均有效。
//
// 返回 (授权结论, 是否可用)。任一侧账号不成立时按**那一侧的原因**具名报错——
// 「学员已注销」与「企业已停用」不得被并成笼统的「无授权」（授权存在 ≠ 授权可用，
// 两者的处置动作完全不同：前者授权本就随之消失，后者是平台对企业的处置）。
func contactGrantEffectiveOf(db *gorm.DB, recruiterID, studentUserID int) (contactGrant, error) {
	grant, err := contactGrantOf(db, recruiterID, studentUserID)
	if err != nil {
		return contactGrant{}, err
	}
	if !grant.Effective() {
		return grant, ErrContactNoAuth
	}
	studentErr := studentAccountAlive(db, studentUserID)
	companyErr := recruiterAccountUsable(db, recruiterID)
	if !contactPairUsable(grant.Effective(), studentErr == nil, companyErr == nil) {
		// 走到这里一定是账号那一维不成立：分侧给出原因（先被联系的一方，再是被处置的一方）。
		if studentErr != nil {
			return contactGrant{}, studentErr
		}
		return contactGrant{}, companyErr
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

// recruiterStatusActive recruiter_users.status 的「有效」取值（1 有效 / 0 禁用）。
// 「禁用是处置动作」⇒ 这一列的读法在联络域只允许住在 recruiterRowUsable 里，
// 常量与它同处一地，免得第二处 caller 又手拼一次 `status`。
const recruiterStatusActive int16 = 1

// recruiterRowUsable 企业有效性判据的**行内形态**：手上这一行仍是有效账号。
// 行是否存在由调用方的取回结果承载（取不到即无行可用 = 已注销，同判）。
func recruiterRowUsable(rec *model.RecruiterUser) bool {
	return rec != nil && rec.Status == recruiterStatusActive
}

// recruiterAccountUsable 企业账号是否仍有效：recruiter_users 行存在且未被禁用。
// 与 studentAccountAlive **同形**的一侧判据——两侧各一条，caller 不手拼 status。
// 真实可达路径是「禁用」（ToggleRecruiterStatus，违规处置动作，同时吊销该企业全部会话）：
// access 令牌在有效期内仍过 JWTAuth，故联系面必须自己判这一维。
func recruiterAccountUsable(db *gorm.DB, recruiterID int) error {
	var rec model.RecruiterUser
	if err := db.First(&rec, recruiterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCompanyUnavailable
		}
		return err
	}
	if !recruiterRowUsable(&rec) {
		return ErrCompanyUnavailable
	}
	return nil
}

// contactGrantOfManyEffective 批量形态 + 「学员注销即失效」（企业视角徽章用）。
// 「在册」判定对整批只查一次，不随卡片数增长。
//
// 这一维照 ADR-0053 §3 保留（徽章与明文门禁看到同一个事实）；但**不叠加企业那一维**——
// 徽章是 caller（企业自己）看出去的投影，企业被禁用时该改的不是徽章而是明文位置
// （ADR-0062 决策 9「授权存在 ≠ 授权可用」+ ADR-0064 决策 5：那一维由 fillContactStates 另用
// company_disabled 一格说出，仍不降级徽章）。
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
