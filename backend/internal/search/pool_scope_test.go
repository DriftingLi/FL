// Package service 搜索读路径的题库池 scope 测试（3c-1 接缝拆分的 service 一侧）。
package search

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestQuestionSearchScopeCoversPool 搜索题目分区 = 题库池口径（ADR-0050 决策 1）：
// 只有池内题命中，命中数与返回条目一致——draft 与来源标记真题题在任何入口都不可见。
// 3c-1 接缝拆分：原用例（questionbank/pool_test.go 的池三元组扩展）跨了「标签计数」（域内可测）
// 与「搜索分区」（要 Service，留在本包）两条读路径，按接缝一分为二，夹具各带一份。
func TestQuestionSearchScopeCoversPool(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	searchSvc := NewService(db, zap.NewNop())

	cred := &model.Credential{Code: "forklift_n1", Name: "N1证"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	srcTag := model.QuestionTag{Code: "real_exam", Name: "真题", IsSourceTag: true}
	if err := db.Create(&srcTag).Error; err != nil {
		t.Fatalf("建来源标记标签失败: %v", err)
	}
	mk := func(content, status string, tagIDs []int) int {
		t.Helper()
		q := model.Question{Type: "single_choice", Content: content, Answer: "A", Status: status, CredentialID: &cred.ID}
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
		for _, tid := range tagIDs {
			if err := db.Create(&model.QuestionTagRelation{QuestionID: q.ID, TagID: tid}).Error; err != nil {
				t.Fatal(err)
			}
		}
		return q.ID
	}
	inPool := mk("液压泵池内题", "published", nil)
	mk("液压泵草稿题", "draft", nil)
	mk("液压泵真题题", "published", []int{srcTag.ID})
	// 同带主题标签与来源标记标签：源标记排除必须压过主题标签（无证件分区时的漂移点）
	mk("液压泵双标签真题题", "published", []int{srcTag.ID})

	got, err := searchSvc.Search("液压泵", SearchTypeQuestion, 1, 20, &cred.ID)
	if err != nil {
		t.Fatalf("题目分区搜索失败: %v", err)
	}
	page := got.(*SearchPageDTO)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != int64(inPool) {
		t.Fatalf("题目分区应只含池内题 %d, got total=%d items=%+v", inPool, page.Total, page.Items)
	}
}
