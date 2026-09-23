// 卡面那一格的三条边界（ADR-0064 决策 5 / ADR-0062 决策 9 / ADR-0062 票6）。
//
// fillContactStates 是列表与详情卡的**唯一装配点**，所以「企业可用性」这一维也只在这里落一次；
// 本文件钉的是它的三个边界：可用 ⇒ 缺席；被禁用 ⇒ 出现且只出现在 approved 上；
// **查不动 ⇒ 不猜成「已停用」**（把 DB 故障报成一个处置事实，比少说一格更坏）。
package service

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func seedGrantPair(t *testing.T, db *gorm.DB, status, source string) (recruiterID, studentID int) {
	t.Helper()
	pwd, err := HashPassword("seedpass123")
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	rec := testutil.SeedRecruiter(t, db, "cardflag_rec", pwd)
	stu := testutil.SeedStudent(t, db, "cardflag_stu", pwd)
	row := model.ContactRequest{RecruiterID: rec.ID, StudentUserID: stu.ID, Status: status, Source: source}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("播种授权行失败: %v", err)
	}
	return rec.ID, stu.ID
}

// cardsOf 以 caller 企业身份装配一批卡（与 List/GetForRecruiter 同一条路径）。
func cardsOf(db *gorm.DB, recruiterID, studentID int) []RecruitResumeCard {
	cards := []RecruitResumeCard{{UserID: studentID}}
	fillContactStates(db, recruiterID, cards)
	return cards
}

func TestFillContactStatesCardAvailability(t *testing.T) {
	t.Run("企业可用 ⇒ 那一格缺席", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		recID, stuID := seedGrantPair(t, db, "approved", "recruiter")
		cards := cardsOf(db, recID, stuID)
		if cards[0].ContactState != string(ContactGrantApproved) {
			t.Fatalf("夹具应是 approved，实际 %q", cards[0].ContactState)
		}
		if cards[0].CompanyDisabled {
			t.Fatal("企业可用时不该给出可用性说明")
		}
	})

	t.Run("企业被禁用 ⇒ 徽章不降级、那一格出现", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		recID, stuID := seedGrantPair(t, db, "approved", "recruiter")
		if err := db.Model(&model.RecruiterUser{}).Where("id = ?", recID).Update("status", 0).Error; err != nil {
			t.Fatalf("置禁用失败: %v", err)
		}
		cards := cardsOf(db, recID, stuID)
		if cards[0].ContactState != string(ContactGrantApproved) {
			t.Fatalf("处置不改写授权事实 ⇒ 徽章仍应是 approved，实际 %q", cards[0].ContactState)
		}
		if !cards[0].CompanyDisabled {
			t.Fatal("明文取不到必须在卡面上说得出，不能静默缺席")
		}
	})

	t.Run("pending 不给出这一格", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		recID, stuID := seedGrantPair(t, db, "pending", "recruiter")
		if err := db.Model(&model.RecruiterUser{}).Where("id = ?", recID).Update("status", 0).Error; err != nil {
			t.Fatalf("置禁用失败: %v", err)
		}
		cards := cardsOf(db, recID, stuID)
		if cards[0].CompanyDisabled {
			t.Fatal("可用性说明只跟着「授权在而明文取不到」那一格；pending 本来就没有明文可收")
		}
	})

	// 「查不动」不得被读成「已停用」（票6 的同一判据）：这一格说的是处置事实，
	// 把它当成 DB 故障的投影会让管理员以为企业被处置过。
	t.Run("recruiter_users 读不动 ⇒ 不猜已停用", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		recID, stuID := seedGrantPair(t, db, "approved", "recruiter")
		if err := db.Exec("DROP TABLE recruiter_users").Error; err != nil {
			t.Fatalf("注入故障（删 recruiter_users 表）失败: %v", err)
		}
		cards := cardsOf(db, recID, stuID)
		if cards[0].ContactState != string(ContactGrantApproved) {
			t.Fatalf("徽章那一维不依赖 recruiter_users，应保持 approved，实际 %q", cards[0].ContactState)
		}
		if cards[0].CompanyDisabled {
			t.Fatal("查不动被读成了「企业已停用」⇒ 把故障报成处置事实")
		}
	})

	// caller 不是这家企业（recruiterID<=0）时整格不装：不得凭空给出可用性说明。
	t.Run("无 caller 身份 ⇒ 两格都不装", func(t *testing.T) {
		db := testutil.NewMemoryDB(t)
		recID, stuID := seedGrantPair(t, db, "approved", "recruiter")
		if err := db.Model(&model.RecruiterUser{}).Where("id = ?", recID).Update("status", 0).Error; err != nil {
			t.Fatalf("置禁用失败: %v", err)
		}
		cards := cardsOf(db, 0, stuID)
		if cards[0].ContactState != "" || cards[0].CompanyDisabled {
			t.Fatalf("无 caller 时两格都该缺席，实际 state=%q disabled=%v", cards[0].ContactState, cards[0].CompanyDisabled)
		}
	})
}
