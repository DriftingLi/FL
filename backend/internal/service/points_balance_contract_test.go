// C1 契约（#509）：余额接口新增 total_spent（支出聚合）、流水条目暴露 expires_at 设计位。
package service

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestPointsBalanceTotalSpent(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	pwd, _ := HashPassword("stu123")
	stu := testutil.SeedStudent(t, db, "balStu", pwd)
	// 收入 +100（earned）、支出 -30 与 -20（spent=50）
	rows := []model.PointsLedger{
		{UserID: stu.ID, Delta: 100, Reason: "task_daily_checkin", RefType: "task", RefID: "1"},
		{UserID: stu.ID, Delta: -30, Reason: "redeem_course", RefType: "course", RefID: "11"},
		{UserID: stu.ID, Delta: -20, Reason: "redeem_paper", RefType: "real_paper", RefID: "22"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("建流水失败: %v", err)
		}
	}
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", stu.ID).Update("points_balance", 50).Error; err != nil {
		t.Fatalf("设余额失败: %v", err)
	}
	svc := NewPointsService(db, nil, nil)
	bal, err := svc.GetBalance(stu.ID)
	if err != nil {
		t.Fatalf("GetBalance 失败: %v", err)
	}
	if bal.TotalSpent != 50 {
		t.Fatalf("total_spent 应为 50（支出绝对值聚合）, got %d", bal.TotalSpent)
	}
	if bal.TotalEarned != 100 {
		t.Fatalf("total_earned 应仍为 100, got %d", bal.TotalEarned)
	}
	if bal.Balance != 50 {
		t.Fatalf("balance 应为 50, got %d", bal.Balance)
	}
}

func TestPointsLedgerExposesExpiresAt(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	pwd, _ := HashPassword("stu123")
	stu := testutil.SeedStudent(t, db, "ledStu", pwd)
	row := model.PointsLedger{UserID: stu.ID, Delta: 10, Reason: "task", RefType: "task", RefID: "1"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("建流水失败: %v", err)
	}
	svc := NewPointsService(db, nil, nil)
	res, err := svc.GetLedger(stu.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("GetLedger 失败: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("应 1 条流水, got %d", len(res.Items))
	}
	// expires_at 设计位：首版恒 nil（模型列预留，未来过期策略点亮）
	if res.Items[0].ExpiresAt != nil {
		t.Fatalf("expires_at 首版应为 nil（永久有效）")
	}
}

// TestPointsLedgerDirectionFilter #512：收支方向筛选——in 仅 delta>0、out 仅 delta<0。
func TestPointsLedgerDirectionFilter(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	pwd, _ := HashPassword("stu123")
	stu := testutil.SeedStudent(t, db, "dirStu", pwd)
	rows := []model.PointsLedger{
		{UserID: stu.ID, Delta: 100, Reason: "task_a", RefType: "task", RefID: "1"},
		{UserID: stu.ID, Delta: -30, Reason: "redeem_course", RefType: "course", RefID: "2"},
		{UserID: stu.ID, Delta: -20, Reason: "ai_tokens", RefType: "ai", RefID: "3"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("建流水失败: %v", err)
		}
	}
	svc := NewPointsService(db, nil, nil)
	inRes, err := svc.GetLedgerFiltered(stu.ID, 1, 20, "", "in")
	if err != nil {
		t.Fatalf("in 查询失败: %v", err)
	}
	if len(inRes.Items) != 1 || inRes.Items[0].Delta != 100 {
		t.Fatalf("in 应只含收入 1 条, got %d", len(inRes.Items))
	}
	outRes, _ := svc.GetLedgerFiltered(stu.ID, 1, 20, "", "out")
	if len(outRes.Items) != 2 {
		t.Fatalf("out 应含支出 2 条, got %d", len(outRes.Items))
	}
	allRes, _ := svc.GetLedger(stu.ID, 1, 20, "")
	if len(allRes.Items) != 3 {
		t.Fatalf("全量应 3 条, got %d", len(allRes.Items))
	}
}
