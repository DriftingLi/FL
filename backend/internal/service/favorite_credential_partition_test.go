package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/scope"
	"forklift-training/internal/testutil"
)

// TestFavoriteTargetSubqueryShape 收藏目标的归属分区子查询：形状与收敛前的内联 SQL 逐字一致
// （ADR-0056 §2 的落点之一；nil → 空串，调用方整支跳过 = 不分区看全部）。
// 期望片段由谓词实现处给出（不在此重抄谓词字面量——静态扫描锁住重抄）。
func TestFavoriteTargetSubqueryShape(t *testing.T) {
	cred := 7
	clause, clauseArgs := scope.EntityOwnedByClause("credential_id", &cred)
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
