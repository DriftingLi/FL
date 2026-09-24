// 契约测（ADR-0064 决策 8 表态锁的「行为例」补齐）：`ContactRequestListResult.Items`
// 标的是 nonnil（出口恒非 null），而它的组装点在 api 层，服务层那把行为例够不着 ⇒
// 只能在这一层验：**一个从没有过申请记录的学员**拉列表，响应里必须是 `"items":[]` 而不是 `"items":null`。
//
// 为什么值得单独一条：那 5 处 nonnil 表态里，只有这一处的构造发生在 handler 里
// （`contact.go` 拿 service 返回的切片直接组 DTO）。也就是说，切片那半边就算由
// `toDTOs` 的 make(...) 保证，「整份响应里 items 这一格」仍然没人证过——
// 而消费方读的恰恰是这一格。
package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// nonnilOutlets 本文件举证的 nonnil 键（判据 5 的**第二个证据源**）。
//
// 为什么要两个源：这条字段的组装发生在 handler 层（`contact.go` 拿 service 返回的切片直接组 DTO），
// 服务层那张表跑不到它。apitypes 的锁按 AST 读这里的键，值本身不用——它要的是「有没有人跑过」
// 这个集合，而跑的那件事是本文件的用例负责的。
var nonnilOutlets = map[string]bool{
	"service.ContactRequestListResult.items": true,
}

func TestContactRequestListEmptyOutlet(t *testing.T) {
	r, db, _ := newAdminContractEnv(t)

	hashed, err := service.HashPassword("nonnil123")
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	stu := testutil.SeedStudent(t, db, "listshape_stu", hashed)
	// 一个孤立课程：证明「读通了、只是没有记录」，而不是那条路径整条坏掉。
	course := model.Course{Name: "列表形状课", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}

	token, err := security.NewSession(contractJWTSecret, time.Hour, security.CookieConfig{}).
		Issue(stu.ID, stu.Account, service.HrwaiRole)
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}
	rec := doWithToken(t, r, token, http.MethodGet, "/api/resume/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("无记录时列表应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"items":[]`) {
		t.Fatalf("Items 标为 nonnil，空集却发出 null：%s", body)
	}
}
