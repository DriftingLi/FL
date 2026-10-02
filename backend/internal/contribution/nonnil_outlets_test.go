// Package contribution 域内 nonnil 证据表（ADR-0070 决策 9：证据表住域包；
// 跨包「一条事实一个证据」由 internal/apitypes/nullability_lock_test.go 钉住）。
package contribution

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nonnilOutletsContribution 本域三格 nonnil 声明的出口。
// files 那格：ContributionFileDTO 无 x-nullable、Files 带 omitempty ⇒ 键可能整个不存在（x-optional），
// 但一旦出现就必须是数组（nullability:"nonnil"）——所以举证出口必须真的带文件。
var nonnilOutletsContribution = map[string]func(t *testing.T) any{
	"contribution.ContributionItemDTO.files":          outletContributionDetailWithFile,
	"contribution.ContributionPageResult.items":       outletContributionPageEmpty,
	"contribution.ContributionReportPageResult.items": outletContributionReportPageEmpty,
}

func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsContribution})
}

// outletContributionDetailWithFile 作者读自己那篇带一份附件的稿（GetDetail 的 includeAuthorFiles
// 走真库查 user_contribution_files）——files 那一格当场发出数组。
func outletContributionDetailWithFile(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	author := seedContributionUser(t, db, "投稿作者", 0)
	c := model.UserContribution{
		UserID: author.ID, CredentialID: 0, Title: "带附件的稿", Intro: "含一份文件",
		Status: ContributionStatusApproved, CreatedAt: testutil.Now(),
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("播投稿失败: %v", err)
	}
	f := model.UserContributionFile{
		ContributionID: c.ID, FileURL: "/uploads/contribution/wave16b.pdf", FileName: "wave16b.pdf",
		FileSize: 1024, ContentType: "document", CreatedAt: testutil.Now(),
	}
	if err := db.Create(&f).Error; err != nil {
		t.Fatalf("播投稿附件失败: %v", err)
	}
	dto, err := NewService(db, nil, nil, nil, zap.NewNop(), nil).GetDetail(c.ID, author.ID)
	if err != nil {
		t.Fatalf("读投稿详情失败: %v", err)
	}
	if len(dto.Files) != 1 {
		t.Fatalf("files = %d 条, 期望 1 条——这条出口没落到要证的那一格", len(dto.Files))
	}
	return dto
}

// outletContributionPageEmpty 投稿审核队列：零投稿时 items 是空集。
func outletContributionPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, nil, nil, zap.NewNop(), nil)
	res, err := svc.ListPending(1, 20)
	if err != nil {
		t.Fatalf("投稿队列失败: %v", err)
	}
	return res
}

// outletContributionReportPageEmpty 投稿举报队列：零举报时 items 是空集。
func outletContributionReportPageEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, nil, nil, zap.NewNop(), nil)
	res, err := svc.ListReports(1, 20, nil)
	if err != nil {
		t.Fatalf("投稿举报队列失败: %v", err)
	}
	return res
}
