// 联络授权读面的三条锁（ADR-0053 §3）：
//  1. 具名值与线上取值域逐字一致（改了常量等于改了协议）
//  2. 三条消费面在同一夹具下给出**一致结论**（同一个事实的三种呈现）
//  3. 字面量清零（防 19 处 `"approved"` 回潮）
package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestContactGrantState_WireValues 具名值的字符串形态就是线上取值域（DB 列与 JSON 都按它走）。
func TestContactGrantState_WireValues(t *testing.T) {
	cases := map[ContactGrantState]string{
		ContactGrantPending:  "pending",
		ContactGrantApproved: "approved",
		ContactGrantRejected: "rejected",
		ContactGrantExpired:  "expired",
		ContactGrantRevoked:  "revoked",
	}
	for state, wire := range cases {
		if string(state) != wire {
			t.Errorf("状态 %q 的线上值为 %q, 期望 %q", state, string(state), wire)
		}
	}
	if string(ContactGrantSourceRecruiter) != "recruiter" || string(ContactGrantSourceApplication) != "application" {
		t.Errorf("来源取值域漂移: %q / %q", ContactGrantSourceRecruiter, ContactGrantSourceApplication)
	}
	if ContactGrantPending.GrantsPlaintext() || ContactGrantRejected.GrantsPlaintext() ||
		ContactGrantExpired.GrantsPlaintext() || ContactGrantRevoked.GrantsPlaintext() {
		t.Error("只有已批准透出明文，其余状态必须为 false")
	}
	if !ContactGrantApproved.GrantsPlaintext() {
		t.Error("已批准必须透出明文")
	}
}

// seedRecruiter 建一条企业账号（授权有效态的另一侧：ADR-0062 决策 9 之后，
// 成对判据会同时问「学员账号在不在」与「企业账号还有效吗」，故夹具必须真的有一行）。
func seedRecruiter(t *testing.T, db *gorm.DB, id int, status int16) {
	t.Helper()
	rec := model.RecruiterUser{
		ID: id, Username: "recAuthz", Password: "x", CompanyName: "授权测试企业",
		ContactPhone: "13800000000", ContactEmail: "rec@example.com", Status: status,
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("播种企业失败: %v", err)
	}
}

// TestContactGrant_ThreeFacesAgree 三条消费面在同一夹具下结论一致：
// 徽章态（批量读面）、招聘方明文门禁（对级谓词）、学员侧明文透出（行级谓词）。
func TestContactGrant_ThreeFacesAgree(t *testing.T) {
	const recruiterID = 101
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.Local)

	cases := []struct {
		name        string
		statuses    []ContactGrantState
		sources     []ContactGrantSource
		wantBadge   ContactGrantState
		wantGranted bool // 对级：可取对方明文
	}{
		{
			name:        "无任何记录 → 未授权",
			wantBadge:   "",
			wantGranted: false,
		},
		{
			name:        "仅 pending → 待确认（不可取明文）",
			statuses:    []ContactGrantState{ContactGrantPending},
			sources:     []ContactGrantSource{ContactGrantSourceRecruiter},
			wantBadge:   ContactGrantPending,
			wantGranted: false,
		},
		{
			name:        "approved → 已授权（可取明文）",
			statuses:    []ContactGrantState{ContactGrantApproved},
			sources:     []ContactGrantSource{ContactGrantSourceRecruiter},
			wantBadge:   ContactGrantApproved,
			wantGranted: true,
		},
		{
			name:        "approved + 更新的 pending → approved 压过 pending",
			statuses:    []ContactGrantState{ContactGrantApproved, ContactGrantPending},
			sources:     []ContactGrantSource{ContactGrantSourceRecruiter, ContactGrantSourceApplication},
			wantBadge:   ContactGrantApproved,
			wantGranted: true,
		},
		{
			name:        "仅 revoked（撤回后）→ 未授权",
			statuses:    []ContactGrantState{ContactGrantRevoked},
			sources:     []ContactGrantSource{ContactGrantSourceApplication},
			wantBadge:   "",
			wantGranted: false,
		},
		{
			name:        "rejected → 未授权",
			statuses:    []ContactGrantState{ContactGrantRejected},
			sources:     []ContactGrantSource{ContactGrantSourceRecruiter},
			wantBadge:   "",
			wantGranted: false,
		},
		{
			name:        "expired → 未授权",
			statuses:    []ContactGrantState{ContactGrantExpired},
			sources:     []ContactGrantSource{ContactGrantSourceRecruiter},
			wantBadge:   "",
			wantGranted: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			// 企业侧账号先要有效——三条消费面「结论一致」的前提是两侧都在册（ADR-0062 决策 9）。
			seedRecruiter(t, db, recruiterID, 1)
			stu := testutil.SeedStudent(t, db, "stuAuthz", "hash")

			for i, status := range tc.statuses {
				src := ContactGrantSourceRecruiter
				if i < len(tc.sources) {
					src = tc.sources[i]
				}
				created := now.Add(time.Duration(i) * time.Minute)
				decided := created
				expires := created.Add(contactDecisionWindow)
				row := model.ContactRequest{
					RecruiterID: recruiterID, StudentUserID: stu.ID, Message: "夹具",
					Status: string(status), Source: string(src),
					CreatedAt: created, UpdatedAt: created, DecidedAt: &decided,
					ExpiresAt: &expires,
				}
				if err := db.Create(&row).Error; err != nil {
					t.Fatalf("seed 授权失败: %v", err)
				}
			}

			// 面 1：批量读面（企业视角徽章）
			grants, err := contactGrantOfManyEffective(db, recruiterID, []int{stu.ID})
			if err != nil {
				t.Fatalf("批量读取失败: %v", err)
			}
			if got := grants[stu.ID].State; got != tc.wantBadge {
				t.Fatalf("徽章态 = %q, 期望 %q", got, tc.wantBadge)
			}

			// 面 2：对级谓词（招聘方明文门禁）
			grant, err := contactGrantEffectiveOf(db, recruiterID, stu.ID)
			if tc.wantGranted {
				if err != nil {
					t.Fatalf("应判为有效授权，却报 %v", err)
				}
				if !grant.Effective() {
					t.Fatal("应判为有效授权")
				}
			} else if err == nil {
				t.Fatal("无有效授权时应报错")
			} else if err != ErrContactNoAuth {
				t.Fatalf("未授权应报 ErrContactNoAuth, got %v", err)
			}

			// 面 3：行级谓词（学员侧明文透出）——与上面两面同源：
			// 「徽章为已授权 / 可取明文」⇔「存在一条已批准记录」
			hasApproved := false
			for _, status := range tc.statuses {
				if status == ContactGrantApproved {
					hasApproved = true
				}
			}
			for _, status := range tc.statuses {
				want := status == ContactGrantApproved
				if got := status.GrantsPlaintext(); got != want {
					t.Fatalf("行级透出（状态 %s）= %v, 期望 %v", status, got, want)
				}
			}
			if hasApproved != tc.wantGranted {
				t.Fatalf("夹具自相矛盾：存在已批准 = %v, 期望可取明文 = %v", hasApproved, tc.wantGranted)
			}
		})
	}
}

// TestContactGrant_StudentGoneInvalidates 不变式：学员注销 → 授权失效。
// 对级谓词（招聘方明文门禁）必须拒绝，且给出「学员已注销」而不是「无授权」；
// 企业视角徽章同样不再显示已授权——两处看到的是同一个事实。
func TestContactGrant_StudentGoneInvalidates(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	const recruiterID = 101
	seedRecruiter(t, db, recruiterID, 1)
	stu := testutil.SeedStudent(t, db, "stuGone", "hash")
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.Local)

	approved := model.ContactRequest{
		RecruiterID: recruiterID, StudentUserID: stu.ID, Message: "已批准",
		Status: string(ContactGrantApproved), Source: string(ContactGrantSourceRecruiter),
		// 无窗口：approved 是永久授权（ADR-0061 §2），本夹具顺带锁住「读面不看 expires_at」。
		CreatedAt: now, UpdatedAt: now, DecidedAt: &now,
	}
	if err := db.Create(&approved).Error; err != nil {
		t.Fatalf("seed 授权失败: %v", err)
	}

	// 在册时：有效授权
	if _, err := contactGrantEffectiveOf(db, recruiterID, stu.ID); err != nil {
		t.Fatalf("在册时应判为有效授权: %v", err)
	}
	grants, err := contactGrantOfManyEffective(db, recruiterID, []int{stu.ID})
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if !grants[stu.ID].Effective() {
		t.Fatal("在册时徽章应为已授权")
	}

	// 注销（hrwai_users 记录消失）
	if err := db.Where("id = ?", stu.ID).Delete(&model.HrwaiUser{}).Error; err != nil {
		t.Fatalf("删除学员失败: %v", err)
	}

	if _, err := contactGrantEffectiveOf(db, recruiterID, stu.ID); err != ErrStudentGone {
		t.Fatalf("注销后应报 ErrStudentGone, got %v", err)
	}
	grants, err = contactGrantOfManyEffective(db, recruiterID, []int{stu.ID})
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if grants[stu.ID].Effective() {
		t.Fatal("学员注销后徽章不应再显示已授权（与明文门禁同口径）")
	}
}

// TestContactGrant_CompanyDisabledInvalidates 不变式（ADR-0062 决策 9）：**企业被禁用 → 授权失效**。
// 与上一例同形，只是把「哪一侧账号不成立」换成了企业侧——两例并排才叫「双向对称」，
// 只有上一例时判据就只是学员侧那半边（本波的实测缺陷）。
//
// 禁用不是注销：授权事实不改写（徽章仍按 approved 投影，「授权存在 ≠ 授权可用」），
// 改的是可用性；解禁后当场恢复（判据读当前状态，不读历史快照）。
func TestContactGrant_CompanyDisabledInvalidates(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	const recruiterID = 101
	seedRecruiter(t, db, recruiterID, 1)
	stu := testutil.SeedStudent(t, db, "stuCompanyDisabled", "hash")
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.Local)

	approved := model.ContactRequest{
		RecruiterID: recruiterID, StudentUserID: stu.ID, Message: "已批准",
		Status: string(ContactGrantApproved), Source: string(ContactGrantSourceRecruiter),
		CreatedAt: now, UpdatedAt: now, DecidedAt: &now,
	}
	if err := db.Create(&approved).Error; err != nil {
		t.Fatalf("seed 授权失败: %v", err)
	}
	// 对照：启用中两侧都成立
	if _, err := contactGrantEffectiveOf(db, recruiterID, stu.ID); err != nil {
		t.Fatalf("启用中应判为有效授权: %v", err)
	}

	// 处置动作：禁用该企业（recruiter_users.status = 0，等价于 ToggleRecruiterStatus）
	if err := db.Model(&model.RecruiterUser{}).Where("id = ?", recruiterID).
		Update("status", int16(0)).Error; err != nil { // 0 = 禁用（ToggleRecruiterStatus 的落地值）
		t.Fatalf("禁用企业失败: %v", err)
	}

	if _, err := contactGrantEffectiveOf(db, recruiterID, stu.ID); err != ErrCompanyUnavailable {
		t.Fatalf("企业被禁用后应报 ErrCompanyUnavailable（不是笼统的无授权）, got %v", err)
	}
	// 授权事实仍在：徽章面（批量读面）按授权事实投影，不因处置而改写
	grants, err := contactGrantOfManyEffective(db, recruiterID, []int{stu.ID})
	if err != nil {
		t.Fatalf("批量读取失败: %v", err)
	}
	if !grants[stu.ID].Effective() {
		t.Fatal("徽章是授权事实的投影，企业被禁用不该改写它（可用性呈现在明文位置）")
	}
	// 行级装配：明文位置降级为「不可用 + 具名说明」
	svc := NewContactService(db, nil, nil, nil)
	row, err := toContactRowOf(db, recruiterID, stu.ID)
	if err != nil {
		t.Fatalf("取授权行失败: %v", err)
	}
	dto := svc.toDTO(row)
	if dto.ContactPhone != "" || dto.ContactEmail != "" || dto.Wechat != "" {
		t.Fatalf("企业被禁用后明文一律不得透出，实际 phone=%q email=%q wechat=%q",
			dto.ContactPhone, dto.ContactEmail, dto.Wechat)
	}
	if dto.Status != string(ContactGrantApproved) || !dto.CompanyDisabled {
		t.Fatalf("应保留 approved 并给出具名说明 company_disabled，实际 status=%q disabled=%v",
			dto.Status, dto.CompanyDisabled)
	}
	if dto.CompanyName == "" {
		t.Fatal("企业名不是 L3 明文，禁用后仍应回填")
	}

	// 解禁 ⇒ 当场恢复（同一份授权行，无重新授权）
	if err := db.Model(&model.RecruiterUser{}).Where("id = ?", recruiterID).
		Update("status", recruiterStatusActive).Error; err != nil {
		t.Fatalf("解禁企业失败: %v", err)
	}
	if _, err := contactGrantEffectiveOf(db, recruiterID, stu.ID); err != nil {
		t.Fatalf("解禁后应恢复有效授权: %v", err)
	}
	if dto := svc.toDTO(row); dto.ContactPhone == "" || dto.CompanyDisabled {
		t.Fatalf("解禁后明文应恢复且不再报 company_disabled，实际 phone=%q disabled=%v",
			dto.ContactPhone, dto.CompanyDisabled)
	}
}

// toContactRowOf 取那一行 approved 授权（行级装配的输入）。
func toContactRowOf(db *gorm.DB, recruiterID, studentUserID int) (*model.ContactRequest, error) {
	var row model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id = ?", recruiterID, studentUserID).
		First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// TestContactAuthz_NoGrantLiterals 字面量清零锁：联络授权域的两个文件里不得再出现
// 五态与来源的裸字符串（今天曾有 19 处 `"approved"`）。
//
// 扫描面刻意只含联络授权域自己的文件——其它域（题库审核、资料审核、举报处理…）用
// 同名字面量表达各自的状态，属不同取值域，不该被这条锁管。
func TestContactAuthz_NoGrantLiterals(t *testing.T) {
	banned := []string{`"approved"`, `"pending"`, `"rejected"`, `"expired"`, `"revoked"`, `"application"`}
	// 具名值自身的声明行是字面量的**唯一**合法落点（`ContactGrantApproved ContactGrantState = "approved"`）
	declRE := regexp.MustCompile(`^\s*ContactGrant\w+\s+ContactGrant(State|Source)\s*=\s*"`)
	for _, name := range []string{"contact_service.go", "contact_authz.go"} {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if declRE.MatchString(line) {
				continue
			}
			// 允许出现在注释里（说明口径时引用线上取值），故按行剔除注释后再判
			code := line
			if idx := strings.Index(code, "//"); idx >= 0 {
				code = code[:idx]
			}
			for _, lit := range banned {
				if strings.Contains(code, lit) {
					t.Errorf("%s:%d 出现裸状态字面量 %s，请改用 ContactGrant* 具名值：%s",
						name, i+1, lit, strings.TrimSpace(line))
				}
			}
		}
	}
}
