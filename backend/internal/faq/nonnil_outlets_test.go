// 帮助中心域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 为什么证据跟着域包走（ADR-0070）：判据 5 的表是「哪些域声明 nonnil、谁举证」的对应关系，
// 域包拆出去之后域的实现与它的举证住在同一个包里，改名/搬目录不会让两侧各自漂。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package faq

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nonnilOutletsFaq 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
// 前缀 nonnilOutlets 是约定的证据表名：apitypes 的表态锁按它扫目录收键（见
// internal/apitypes/nullability_lock_test.go 的 nonNilEvidenceSources）。
var nonnilOutletsFaq = map[string]func(t *testing.T) any{
	"faq.FaqResult.categories":                outletFaqPublishedEmpty,
	"faq.FaqCategoryDTO.entries":              outletFaqPublishedEmptyCategory,
	"faq.AdminFaqCategoriesResult.categories": outletAdminFaqCategoriesEmpty,
	"faq.AdminFaqEntriesResult.entries":       outletAdminFaqEntriesEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsFaq})
}

func outletFaqPublishedEmpty(t *testing.T) any {
	t.Helper()
	res, err := NewService(testutil.NewMemoryDB(t), zap.NewNop()).ListPublished()
	if err != nil {
		t.Fatalf("学员端帮助中心失败: %v", err)
	}
	return res
}

// outletFaqPublishedEmptyCategory entries 的形状要单独播一条**启用分类且其下零条已发布条目**
// ——ListPublished 对这种分类显式回填 `[]FaqEntryDTO{}`（源码注释：前端免判空）。
// 空库取不到分类节点，也就取不到这一格。
func outletFaqPublishedEmptyCategory(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	cat := model.FaqCategory{Code: "nonnil-faq", Title: "空分类", SortOrder: 1, Enabled: true}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("播种 FAQ 分类失败: %v", err)
	}
	res, err := NewService(db, zap.NewNop()).ListPublished()
	if err != nil {
		t.Fatalf("学员端帮助中心失败: %v", err)
	}
	if len(res.Categories) == 0 {
		t.Fatal("帮助中心里没有分类节点：这条证据没有落地")
	}
	return res.Categories[0]
}

func outletAdminFaqCategoriesEmpty(t *testing.T) any {
	t.Helper()
	items, err := NewService(testutil.NewMemoryDB(t), zap.NewNop()).AdminListCategories()
	if err != nil {
		t.Fatalf("管理端分类清单失败: %v", err)
	}
	return AdminFaqCategoriesResult{Categories: items}
}

func outletAdminFaqEntriesEmpty(t *testing.T) any {
	t.Helper()
	items, err := NewService(testutil.NewMemoryDB(t), zap.NewNop()).AdminListEntries(nil)
	if err != nil {
		t.Fatalf("管理端条目清单失败: %v", err)
	}
	return AdminFaqEntriesResult{Entries: items}
}
