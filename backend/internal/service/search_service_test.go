package service

import (
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// #980：LIKE 通配符必须按**字面**匹配 —— 用户串里的 % 与 _ 不得当通配符。
// 这三条用例按「用户输入什么就搜什么」断言，与实现无关（不校验 SQL 形状）。
func TestSearchKeywordWildcardsAreLiteral(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)
	spID, lvID := 1, 1
	mk := func(name string) {
		c := model.Course{Name: name, Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("插入课程失败: %v", err)
		}
	}
	mk("液压系统100%检修")
	mk("液压系统ABC检修")
	mk("a_b 故障码")
	mk("axb 故障码")

	names := func(keyword string) []string {
		t.Helper()
		got, err := svc.Search(keyword, SearchTypeCourse, 1, 20, nil)
		if err != nil {
			t.Fatalf("搜索 %q 失败: %v", keyword, err)
		}
		page := got.(*SearchPageDTO)
		out := make([]string, 0, len(page.Items))
		for _, it := range page.Items {
			out = append(out, it.Title)
		}
		return out
	}

	// % 必须是字面量：只命中真含「100%」的那门课
	if got := names("100%"); len(got) != 1 || got[0] != "液压系统100%检修" {
		t.Fatalf("搜 %q 应只命中字面含 100%% 的课程, got %v", "100%", got)
	}
	// 单独一个 % 不得命中全表
	if got := names("%"); len(got) != 1 || got[0] != "液压系统100%检修" {
		t.Fatalf("搜 %q 不得当通配符命中全表, got %v", "%", got)
	}
	// _ 必须是字面量：a_b 不得匹配 axb
	if got := names("a_b"); len(got) != 1 || got[0] != "a_b 故障码" {
		t.Fatalf("搜 %q 应只命中字面 a_b, got %v", "a_b", got)
	}
}

// #980：关键词长度上限（超长拒绝，边界内放行）。
func TestSearchKeywordLengthCap(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewSearchService(db, nil)

	if _, err := svc.Search(strings.Repeat("液", maxSearchKeywordLen), SearchTypeCourse, 1, 20, nil); err != nil {
		t.Fatalf("恰好 %d 字符应放行, got %v", maxSearchKeywordLen, err)
	}
	if _, err := svc.Search(strings.Repeat("液", maxSearchKeywordLen+1), SearchTypeCourse, 1, 20, nil); err == nil {
		t.Fatalf("超过 %d 字符应报错", maxSearchKeywordLen)
	}
}
