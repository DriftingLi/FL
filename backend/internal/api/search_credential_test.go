// Package api #393 回归：全局搜索聚合路径（type 缺省）的 course/question
// 分区按「当前证件」分区，与显式 type 路径同口径。
package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// TestSearchAggregationFollowsCredential 聚合搜索（type 缺省各分区 top5）
// 必须把当前证件透传到底层分区查询：course/question 只返回当前证件的内容；
// 不带证件时不过滤（显式 type 路径本就正确，一并对齐断言）。
func TestSearchAggregationFollowsCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	svc := service.NewSearchService(db, nil)

	credA, credB := 1, 2
	spID, lvID := 1, 1
	mkCourse := func(name string, cred *int) {
		c := model.Course{Name: name, Status: 1, CredentialID: cred, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("插入课程失败: %v", err)
		}
	}
	mkQuestion := func(content string, cred *int) {
		q := model.Question{Type: "single_choice", Content: content, Answer: "A", Status: "published", CredentialID: cred, CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
		if err := db.Create(&q).Error; err != nil {
			t.Fatalf("插入题目失败: %v", err)
		}
	}
	mkCourse("叉车课程证件A", &credA)
	mkCourse("叉车课程证件B", &credB)
	mkQuestion("叉车题目证件A", &credA)
	mkQuestion("叉车题目证件B", &credB)

	titles := func(items []service.SearchItemDTO) map[string]bool {
		seen := map[string]bool{}
		for _, it := range items {
			seen[it.Title] = true
		}
		return seen
	}

	// 聚合路径 + 证件 A：course/question 分区只含 A
	all, err := svc.Search("叉车", "", 1, 20, &credA)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	got := all.(*service.SearchAllDTO)
	if got.Courses.Total != 1 || !titles(got.Courses.Items)["叉车课程证件A"] {
		t.Fatalf("聚合 course 分区应只含证件 A, got %+v", got.Courses)
	}
	if got.Questions.Total != 1 || !titles(got.Questions.Items)["叉车题目证件A"] {
		t.Fatalf("聚合 question 分区应只含证件 A, got %+v", got.Questions)
	}

	// 聚合路径 + 证件 B：只含 B
	all, err = svc.Search("叉车", "", 1, 20, &credB)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	got = all.(*service.SearchAllDTO)
	if got.Courses.Total != 1 || !titles(got.Courses.Items)["叉车课程证件B"] {
		t.Fatalf("聚合 course 分区应只含证件 B, got %+v", got.Courses)
	}
	if got.Questions.Total != 1 || !titles(got.Questions.Items)["叉车题目证件B"] {
		t.Fatalf("聚合 question 分区应只含证件 B, got %+v", got.Questions)
	}

	// 聚合路径不带证件：不过滤（两分区各 2 条）
	all, err = svc.Search("叉车", "", 1, 20)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	got = all.(*service.SearchAllDTO)
	if got.Courses.Total != 2 || got.Questions.Total != 2 {
		t.Fatalf("无证件聚合应各分区 2 条, got courses=%d questions=%d", got.Courses.Total, got.Questions.Total)
	}

	// 显式 type 路径（本就正确，对齐断言）：按证件过滤
	page, err := svc.Search("叉车", service.SearchTypeCourse, 1, 20, &credB)
	if err != nil {
		t.Fatalf("显式搜索失败: %v", err)
	}
	gotPage := page.(*service.SearchPageDTO)
	if gotPage.Total != 1 || !titles(gotPage.Items)["叉车课程证件B"] {
		t.Fatalf("显式 course 路径应只含证件 B, got %+v", gotPage)
	}
}
