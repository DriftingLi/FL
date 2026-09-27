// 投稿域的 nonnil 行为例（批①-B 第④段：判据 5 那一侧的收口）。
//
// 机制见 nonnil_declaration_test.go 的 nonnilOutletTables（分域表 + init 并进汇总）。
//
// ContributionItemDTO.files 是本批唯一**改判**的一格，判据是读投影 + 跑一次填得上的出口两件事：
//   - 声明写的是 `json:"files,omitempty"`，装配点在 contribution_service.go 的 toDTO：
//     `dto.Files` 起手是 nil，只有 `append` 会让它长出元素。⇒ 三种可能的发出形状是
//     「键缺席」（nil 或空集，omitempty 吃掉）与「键在且是数组」（有文件）——
//     **`null` 这一格根本不存在**：`[]T(nil)` 带 omitempty 不会发 null，这是 encoding/json 的
//     规定，不是本仓的约定。
//   - 所以那句 `nullable` 是一句没有出口的谎：它让前端为一条走不到的分支写 `?? []`，
//     并让生成的 TS 承诺 `ContributionFileDTO[] | null`（批①-B 的 flag 一旦补上就落地）。
//     诚实形状是 nonnil + 既有的 `x-optional`（键可缺席 ≠ 键可为 null，两格各说一件事）。
//   - marshalKey 在键缺席时判红（它自己写明「被加了 omitempty ⇒ nonnil 表态就不成立了」），
//     所以举证必须走一条**带文件**的出口：这条出口证的是「键在场时它是数组」，
//     而「键不在场」那一档由 omitempty 与 nil 的组合保证、由上面那条规定保证。
//     两条合起来才是「这一格永不为 null」的完整证明——只跑其中一条都不算。
package service

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nonnilOutletsContribution = map[string]func(t *testing.T) any{
	"service.ContributionItemDTO.files": outletContributionDetailWithFile,
}

func init() {
	nonnilOutletTables = append(nonnilOutletTables, nonnilOutletsContribution)
}

// outletContributionDetailWithFile 作者读自己那篇带一份附件的稿（GetDetail 的 includeAuthorFiles
// 走真库查 user_contribution_files）——files 那一格当场发出数组。
func outletContributionDetailWithFile(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	author := seedForumUser(t, db, "投稿作者")
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
	dto, err := NewContributionService(db, nil, nil, nil, zap.NewNop(), nil).GetDetail(c.ID, author.ID)
	if err != nil {
		t.Fatalf("读投稿详情失败: %v", err)
	}
	if len(dto.Files) != 1 {
		t.Fatalf("files = %d 条, 期望 1 条——这条出口没落到要证的那一格", len(dto.Files))
	}
	return dto
}
