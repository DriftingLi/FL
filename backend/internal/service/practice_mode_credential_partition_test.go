// Package service 测试：练习族的证件分区口径（#1007 / ADR-0051）。
//
// 锁定三件事：
//  1. 写入时冻结：SubmitAnswer / 错题重做落的记录带「作答那一刻的当前证件」；
//  2. 三个读面同口径：/history（GetHistory）、/practice-stats（GetPracticeStats）、legacy /stats（GetStats）
//     在「传证件 / 不传证件」两种读法下给出一致的可见集合；
//  3. 口径对账不变式：同一证件下 practice-stats.TotalCount == history.Total == stats.Total
//     （#1007 的缺陷形态正是这条不成立的 0 vs 15）。
package service

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedPracticeCredential 建证件并返回。
func seedPracticeCredential(t *testing.T, db *gorm.DB, code, name string) model.Credential {
	t.Helper()
	c := model.Credential{Code: code, Name: name, Category: "special_operation", Status: 1}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	return c
}

// seedCredentialQuestion 建一道挂指定证件的已发布题（练习池口径：published + 非来源标记标签 + 证件分区）。
func seedCredentialQuestion(t *testing.T, db *gorm.DB, credentialID int, content string) *model.Question {
	t.Helper()
	q := testutil.SeedQuestion(t, db, "single_choice", content, "A")
	if err := db.Model(q).Update("credential_id", credentialID).Error; err != nil {
		t.Fatalf("题目挂证件失败: %v", err)
	}
	return q
}

// TestPracticeReadSurfacesCredentialPartition 三个读面按「写入时冻结的证件」分区且互相对账。
func TestPracticeReadSurfacesCredentialPartition(t *testing.T) {
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "练习分区学员", "x")
	credA := seedPracticeCredential(t, db, "pracA", "叉车司机N1")
	credB := seedPracticeCredential(t, db, "pracB", "低压电工")

	qA1 := seedCredentialQuestion(t, db, credA.ID, "A 证题一")
	qA2 := seedCredentialQuestion(t, db, credA.ID, "A 证题二")
	qB1 := seedCredentialQuestion(t, db, credB.ID, "B 证题")

	for _, tc := range []struct {
		q    *model.Question
		cred *int
	}{{qA1, &credA.ID}, {qA2, &credA.ID}, {qB1, &credB.ID}} {
		if _, err := svc.SubmitAnswer(student.ID, tc.q.ID, "A", "free", tc.cred); err != nil {
			t.Fatalf("提交答案失败: %v", err)
		}
	}

	// 写入时冻结：记录上的分区等于提交时的证件
	var recs []model.QuestionPracticeRecord
	if err := db.Where("student_id = ?", student.ID).Order("id ASC").Find(&recs).Error; err != nil {
		t.Fatalf("查练习记录失败: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("应有 3 条练习记录, got %d", len(recs))
	}
	wantCred := []int{credA.ID, credA.ID, credB.ID}
	for i, rec := range recs {
		if rec.CredentialID == nil || *rec.CredentialID != wantCred[i] {
			t.Fatalf("第 %d 条记录应落证件 %d（写入时冻结）, got %v", i+1, wantCred[i], rec.CredentialID)
		}
	}

	for _, tc := range []struct {
		name string
		cred *int
		want int64
	}{
		{"A 证件分区", &credA.ID, 2},
		{"B 证件分区", &credB.ID, 1},
		{"nil 不分区（看全部）", nil, 3},
	} {
		hist, err := svc.GetHistory(student.ID, tc.cred, 1, 10, "", "", "")
		if err != nil {
			t.Fatalf("GetHistory 失败: %v", err)
		}
		if hist.Total != tc.want {
			t.Fatalf("%s: history.Total=%d, want %d", tc.name, hist.Total, tc.want)
		}
		// 带题型过滤的分支会 JOIN question（两张表都有 credential_id）—— 锁住「分区列仍不歧义」
		byType, err := svc.GetHistory(student.ID, tc.cred, 1, 10, "single_choice", "", "")
		if err != nil {
			t.Fatalf("GetHistory 失败: %v", err)
		}
		if byType.Total != tc.want {
			t.Fatalf("%s: history(type=single_choice).Total=%d, want %d", tc.name, byType.Total, tc.want)
		}
		// 该分支会 JOIN question（两张表都有 created_at/credential_id）：锁住「不再是恒空」——
		// #1095 之前歧义列报错被吞，形状正是 total 正确而 records 恒空（静默 fail-open）。
		if len(byType.Records) != int(tc.want) {
			t.Fatalf("%s: history(type=single_choice).Records 长度=%d, want %d（歧义列修复的回归判据）",
				tc.name, len(byType.Records), tc.want)
		}
		overview, err := svc.GetPracticeStats(student.ID, tc.cred)
		if err != nil {
			t.Fatalf("%s: practice-stats 查询失败: %v", tc.name, err)
		}
		if overview.TotalCount != tc.want {
			t.Fatalf("%s: practice-stats.total_count=%d, want %d", tc.name, overview.TotalCount, tc.want)
		}
		stats, err := svc.GetStats(student.ID, tc.cred)
		if err != nil {
			t.Fatalf("%s: stats 查询失败: %v", tc.name, err)
		}
		if stats.Total != tc.want {
			t.Fatalf("%s: stats.total=%d, want %d", tc.name, stats.Total, tc.want)
		}
		// 口径对账不变式：三个读面在同一证件下必须给同一个数（#1007 的缺陷形态正是 0 ≠ 15）
		if hist.Total != overview.TotalCount || overview.TotalCount != stats.Total {
			t.Fatalf("%s: 三读面口径不一致 history=%d practice-stats=%d stats=%d",
				tc.name, hist.Total, overview.TotalCount, stats.Total)
		}
	}
}

// TestPracticeStatsCountsRedoRecordsUnderTheirCredential 重做记录也按其分区计数（与练习同源）。
func TestPracticeStatsCountsRedoRecordsUnderTheirCredential(t *testing.T) {
	svc, db := newPracticeSvc(t)
	student := testutil.SeedStudent(t, db, "重做分区学员", "x")
	credA := seedPracticeCredential(t, db, "redoA", "叉车司机N1")
	credB := seedPracticeCredential(t, db, "redoB", "低压电工")
	qA := seedCredentialQuestion(t, db, credA.ID, "A 证题")

	if _, err := svc.SubmitAnswer(student.ID, qA.ID, "A", "free", &credA.ID); err != nil {
		t.Fatalf("提交答案失败: %v", err)
	}
	rec := model.QuestionPracticeRecord{
		StudentID: student.ID, CredentialID: &credB.ID, QuestionID: qA.ID,
		IsCorrect: true, PracticeType: "redo", CreatedAt: beijingNow(),
	}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatalf("插入重做记录失败: %v", err)
	}

	overviewA, err := svc.GetPracticeStats(student.ID, &credA.ID)
	if err != nil {
		t.Fatalf("A 证件 stats 查询失败: %v", err)
	}
	if overviewA.TotalCount != 1 {
		t.Fatalf("A 证件 total_count=%d, want 1", overviewA.TotalCount)
	}
	overviewB, err := svc.GetPracticeStats(student.ID, &credB.ID)
	if err != nil {
		t.Fatalf("B 证件 stats 查询失败: %v", err)
	}
	if overviewB.TotalCount != 1 {
		t.Fatalf("B 证件 total_count=%d, want 1（重做记录按其分区计数）", overviewB.TotalCount)
	}
	histB, err := svc.GetHistory(student.ID, &credB.ID, 1, 10, "", "", "")
	if err != nil {
		t.Fatalf("GetHistory 失败: %v", err)
	}
	if got := histB.Total; got != 1 {
		t.Fatalf("B 证件 history.Total=%d, want 1", got)
	}
}
