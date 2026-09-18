package service

import (
	"testing"

	"go.uber.org/zap"
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
		{"RecordPartitionOf/nil=不分区看全部", recordPartitionClause, nil, "", nil},
		{"RecordPartitionOf/指向证件", recordPartitionClause, &credID, column + " = ?", &credID},
		{"RecordPartitionOf/指向不存在证件", recordPartitionClause, &missingID, column + " = ?", &missingID},
		{"PartitionBucket/nil=只取NULL桶", partitionBucketClause, nil, column + " IS NULL", nil},
		{"PartitionBucket/指向证件", partitionBucketClause, &credID, column + " = ?", &credID},
		{"PartitionBucket/指向不存在证件", partitionBucketClause, &missingID, column + " = ?", &missingID},
		{"EntityOwnedBy/nil=不分区看全部", entityOwnedByClause, nil, "", nil},
		{"EntityOwnedBy/指向证件", entityOwnedByClause, &credID, column + " = ?", &credID},
		{"EntityOwnedBy/指向不存在证件", entityOwnedByClause, &missingID, column + " = ?", &missingID},
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

// TestFavoriteTargetSubqueryShape 收藏目标的归属分区子查询：形状与收敛前的内联 SQL 逐字一致
// （ADR-0056 §2 的落点之一；nil → 空串，调用方整支跳过 = 不分区看全部）。
// 期望片段由谓词实现处给出（不在此重抄谓词字面量——静态扫描锁住重抄）。
func TestFavoriteTargetSubqueryShape(t *testing.T) {
	cred := 7
	clause, clauseArgs := entityOwnedByClause("credential_id", &cred)
	if len(clauseArgs) != 1 {
		t.Fatalf("谓词片段参数 = %v", clauseArgs)
	}
	if sub, args := favoriteTargetSubquery(FavoriteTargetCourse, &cred); sub != "SELECT course_id FROM course WHERE "+clause || len(args) != 1 || args[0] != cred {
		t.Fatalf("课程子查询 = %q %v", sub, args)
	}
	if sub, args := favoriteTargetSubquery(FavoriteTargetQuestion, &cred); sub != "SELECT id FROM question WHERE "+clause || len(args) != 1 || args[0] != cred {
		t.Fatalf("题目子查询 = %q %v", sub, args)
	}
	if sub, args := favoriteTargetSubquery(FavoriteTargetCourse, nil); sub != "" || len(args) != 0 {
		t.Fatalf("nil 应整支跳过，got %q %v", sub, args)
	}
}

// TestFavoriteListEntityOwnedByCredential 收藏列表的归属分区端到端（内存库）：
// 混合类型（targetType 空）只过滤 course/question 两个分区、其余类型保持；nil 读作不分区看全部。
// 收敛前这条路径是内联子查询字面量，收敛后由谓词片段拼装——本用例是它的行级证据。
func TestFavoriteListEntityOwnedByCredential(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewFavoriteService(db, zap.NewNop())

	credA, credB := 21, 22
	courseA := model.Course{Name: "课程A", CredentialID: &credA, CreatedAt: testutil.Now()}
	courseB := model.Course{Name: "课程B", CredentialID: &credB, CreatedAt: testutil.Now()}
	for _, c := range []*model.Course{&courseA, &courseB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("创建课程失败: %v", err)
		}
	}
	topic := model.ForumTopic{Title: "主题", CreatedAt: testutil.Now()}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("创建主题失败: %v", err)
	}
	for _, fav := range []model.Favorite{
		{UserID: 1, TargetType: FavoriteTargetCourse, TargetID: courseA.CourseID, CreatedAt: testutil.Now()},
		{UserID: 1, TargetType: FavoriteTargetCourse, TargetID: courseB.CourseID, CreatedAt: testutil.Now()},
		{UserID: 1, TargetType: FavoriteTargetTopic, TargetID: int(topic.ID), CreatedAt: testutil.Now()},
	} {
		if err := db.Create(&fav).Error; err != nil {
			t.Fatalf("创建收藏失败: %v", err)
		}
	}

	all, err := svc.List(1, "", 1, 20, nil)
	if err != nil {
		t.Fatalf("List(nil) 失败: %v", err)
	}
	if all.Total != 3 || len(all.Favorites) != 3 {
		t.Fatalf("nil 应看全部：total=%d items=%d", all.Total, len(all.Favorites))
	}

	part, err := svc.List(1, "", 1, 20, &credA)
	if err != nil {
		t.Fatalf("List(credA) 失败: %v", err)
	}
	if part.Total != 2 || len(part.Favorites) != 2 {
		t.Fatalf("混合类型按 credA 应保留「课程A + 主题」：total=%d items=%d", part.Total, len(part.Favorites))
	}
	for _, item := range part.Favorites {
		if item.TargetType == FavoriteTargetCourse && item.TargetID != courseA.CourseID {
			t.Fatalf("课程B 不该出现：%+v", item)
		}
	}

	only, err := svc.List(1, FavoriteTargetCourse, 1, 20, &credB)
	if err != nil {
		t.Fatalf("List(course, credB) 失败: %v", err)
	}
	if only.Total != 1 || len(only.Favorites) != 1 || only.Favorites[0].TargetID != courseB.CourseID {
		t.Fatalf("单类型按 credB 应只剩课程B：total=%d %+v", only.Total, only.Favorites)
	}
}
