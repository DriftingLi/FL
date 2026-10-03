// 招聘域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名 / 搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
//
// 两格随招聘域从 internal/service 的 nonnilOutletsCore / nonnilOutletsPeople 搬来（P2 波 4e）。
// 第二格的**类型**属简历域（resume.RecruitResumeCard），出口却是本包的读面（Service.Get → Desensitize）
// —— 证据跟真实出口走，不跟类型名的前缀走（判词同 4d 的 course.ChapterDTO.files）。
package recruit

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nonnilOutletsRecruit 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsRecruit = map[string]func(t *testing.T) any{
	"recruit.RecruitListResult.items":                outletRecruitListEmpty,
	"resume.RecruitResumeCard.resume_certifications": outletRecruitCardNoCerts,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsRecruit})
}

// seedBlankJobCard 播一张**四个 JSONB 列都没写过**的简历卡（visibility 由调用方给）。
// 逐字副本：域包测试不得 import 别的包的测试助手（与 internal/resume 各持一份，手册 §11 第③条）。
func seedBlankJobCard(t *testing.T, db *gorm.DB, userID int, visibility string) {
	t.Helper()
	card := model.JobCard{
		UserID: userID, RealName: "张三丰", Visibility: visibility,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("播简历卡失败: %v", err)
	}
}

// outletRecruitListEmpty 脱敏简历库列表：空池时 items 是 make 出来的空集。
func outletRecruitListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), zap.NewNop())
	res, err := svc.List(RecruitListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("空库拉列表失败: %v", err)
	}
	return res
}

// outletRecruitCardNoCerts 脱敏简历卡：持证那一格由 maskCertifications 兜底——脏数据 / 空列
// 都返回 make(0,0)，json.Marshal 出来恒是数组，所以这一格连「列里存 JSON null」那一档都发不出
// null（同一条 maskCertifications 的另两格走 `len(x)==0` 守卫，那一档会漏，见 nullable 表）。
func outletRecruitCardNoCerts(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := testutil.SeedStudent(t, db, "公开简历卡主", "hash")
	seedBlankJobCard(t, db, owner.ID, "open")
	card, err := NewService(db, zap.NewNop()).Get(owner.ID)
	if err != nil {
		t.Fatalf("取脱敏简历卡失败: %v", err)
	}
	return card
}
