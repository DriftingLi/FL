package scope

import (
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCredentialScopeClauseNilTriState 三族具名谓词的 nil 三态（SQL 片段形态）：
// nil / 指向某证件 / 指向不存在的证件。
// 三族的 nil 语义**相反**正是本 module 的存在理由（ADR-0056 §2）——断言按族分别写死，
// 不许「统一成看起来更合理的那种」。
func TestCredentialScopeClauseNilTriState(t *testing.T) {
	const column = "practice_progress.credential_id"
	credID, missingID := 7, 999999
	cases := []struct {
		name       string
		clause     func(string, *int) (string, []any)
		cred       *int
		wantClause string
		wantArg    *int // nil = 无参数
	}{
		{"RecordPartitionOf/nil=不分区看全部", RecordPartitionClause, nil, "", nil},
		{"RecordPartitionOf/指向证件", RecordPartitionClause, &credID, column + " = ?", &credID},
		{"RecordPartitionOf/指向不存在证件", RecordPartitionClause, &missingID, column + " = ?", &missingID},
		{"PartitionBucket/nil=只取NULL桶", PartitionBucketClause, nil, column + " IS NULL", nil},
		{"PartitionBucket/指向证件", PartitionBucketClause, &credID, column + " = ?", &credID},
		{"PartitionBucket/指向不存在证件", PartitionBucketClause, &missingID, column + " = ?", &missingID},
		{"EntityOwnedBy/nil=不分区看全部", EntityOwnedByClause, nil, "", nil},
		{"EntityOwnedBy/指向证件", EntityOwnedByClause, &credID, column + " = ?", &credID},
		{"EntityOwnedBy/指向不存在证件", EntityOwnedByClause, &missingID, column + " = ?", &missingID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clause, args := tc.clause(column, tc.cred)
			if clause != tc.wantClause {
				t.Fatalf("clause = %q, want %q", clause, tc.wantClause)
			}
			if tc.wantArg == nil {
				if len(args) != 0 {
					t.Fatalf("args = %v, want 空", args)
				}
				return
			}
			if len(args) != 1 || args[0] != *tc.wantArg {
				t.Fatalf("args = %v, want [%d]", args, *tc.wantArg)
			}
		})
	}
}

// TestCredentialScopePredicatesNilTriStateRows 三族谓词的行级行为（内存库）：
// 同形参数、不同族 → 不同结果集。最尖锐的一条：PartitionBucket(nil) 只回 NULL 桶，
// 而 RecordPartitionOf(nil) / EntityOwnedBy(nil) 回全部。
// 「指向不存在的证件」一律 = 该证件那一格（0 行），**不回落**成 nil 语义。
func TestCredentialScopePredicatesNilTriStateRows(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	credA, credB, missing := 11, 12, 999999

	for _, cred := range []*int{nil, &credA, &credB} {
		rec := model.QuestionPracticeRecord{StudentID: 1, CredentialID: cred, QuestionID: 1, PracticeType: "free", CreatedAt: testutil.Now()}
		if err := db.Create(&rec).Error; err != nil {
			t.Fatalf("播练习记录失败: %v", err)
		}
		prog := model.PracticeProgress{StudentID: 1, PracticeMode: "sequential", CredentialID: cred,
			QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: testutil.Now()}
		if err := db.Create(&prog).Error; err != nil {
			t.Fatalf("播练习进度失败: %v", err)
		}
		q := model.Question{Type: "single_choice", Content: "题干", Answer: "A", Status: "published",
			CredentialID: cred, CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
		if err := db.Create(&q).Error; err != nil {
			t.Fatalf("播题目失败: %v", err)
		}
	}

	count := func(q *gorm.DB) int64 {
		t.Helper()
		var n int64
		if err := q.Count(&n).Error; err != nil {
			t.Fatalf("计数失败: %v", err)
		}
		return n
	}
	record := func(cred *int) *gorm.DB {
		return RecordPartitionOf(db.Model(&model.QuestionPracticeRecord{}), "question_practice_record.credential_id", cred)
	}
	bucket := func(cred *int) *gorm.DB {
		return PartitionBucket(db.Model(&model.PracticeProgress{}), "credential_id", cred)
	}
	owned := func(cred *int) *gorm.DB {
		return EntityOwnedBy(db.Model(&model.Question{}), "credential_id", cred)
	}
	cases := []struct {
		name string
		q    *gorm.DB
		want int64
	}{
		{"RecordPartitionOf/nil=看全部", record(nil), 3},
		{"RecordPartitionOf/指向 credA", record(&credA), 1},
		{"RecordPartitionOf/指向不存在证件", record(&missing), 0},
		{"PartitionBucket/nil=只取 NULL 桶（不是看全部）", bucket(nil), 1},
		{"PartitionBucket/指向 credA", bucket(&credA), 1},
		{"PartitionBucket/指向不存在证件（不是 NULL 桶）", bucket(&missing), 0},
		{"EntityOwnedBy/nil=看全部", owned(nil), 3},
		{"EntityOwnedBy/指向 credA", owned(&credA), 1},
		{"EntityOwnedBy/指向不存在证件", owned(&missing), 0},
	}
	for _, tc := range cases {
		if got := count(tc.q); got != tc.want {
			t.Errorf("%s: 命中 %d 行, want %d", tc.name, got, tc.want)
		}
	}
}
