// 契约测（ADR-0064 决策 8 表态锁的「行为例」补齐）：`ContactRequestListResult.Items`
// 标的是 nonnil（出口恒非 null），而它的组装点在 api 层，服务层那张表够不着 ⇒
// 只能在这一层验：**一个从没有过申请记录的学员**拉列表，响应里必须是 `"items":[]` 而不是 `"items":null`。
//
// 为什么值得单独一条：那处 nonnil 表态里只有它的构造发生在 handler 里
// （`contact.go` 拿 service 返回的切片直接组 DTO）。切片那半边就算由 `toDTOs` 的 make(...) 保证，
// 「整份响应里 items 这一格」仍然没人证过——而消费方读的恰恰是这一格。
package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// nonnilOutlets 本包举证的 nonnil 键 → 走真实路由拿响应体的闭包。
//
// 为什么要两张表（这张 + service 那张）：证据必须住在**组装发生的那一层**，服务层的表跑不到
// handler 里拼出来的那一格。键由 apitypes 的判据 5 按变量名前缀扫目录读，值由下面的用例执行。
//
// 值不能省：一张只被 AST 读、没有任何 Go 代码引用的表会被 unused 判死（CI backend-lint 实测
// 红过一次）。而那恰好也是这张表该有的形状——**证据必须真的被跑过**，不是一个供人查名字的花名册。
var nonnilOutlets = map[string]func(t *testing.T) string{
	"service.ContactRequestListResult.items": contactRequestListBody,
	// 三条分页壳：service 侧的切片由 gorm 的 Find 填，恒非 null，但「整份响应里 items /
	// favorites 这一格」是 handler 拼出来的 ⇒ 举证必须在路由这一层（批①-A 委托时也确认过一次：
	// 同一个 items，服务层跑不到）。
	"service.NotePageDTO.items":               notePageBody,
	"service.QuestionCommentPageResult.items": commentPageBody,
	"service.FavoritePageResult.favorites":    favoritePageBody,
	// 反方向的谎：这一格从前声明 nullable 且契约上落了 x-nullable，而空审计表拉列表实测发的是
	// `[]` —— 契约在承诺一个永远不来的 null。批①-B 把它改判 nonnil 并摘掉 x-nullable
	//（判据 2 不许两者同在；摘掉后生成的 TS 从 `T[] | null` 收回 `T[]`，是收窄不是加负担）。
	"api.AuditLogPageResult.items": auditLogPageBody,
}

// auditLogPageBody 空审计表拉列表的响应体。
func auditLogPageBody(t *testing.T) string {
	t.Helper()
	r, _, adminToken := newAdminContractEnv(t)
	rec := doWithToken(t, r, adminToken, http.MethodGet, "/api/admin/audit-logs?page_size=20", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("空审计表拉列表应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// getJSON 打一次 GET 并要求 200，返回响应体。
func getJSON(t *testing.T, f *poolLeakFixture, path string) string {
	t.Helper()
	code, body := doAndBody(t, f, f.studentToken, http.MethodGet, path, nil)
	if code != http.StatusOK {
		t.Fatalf("GET %s 应 200，实际 %d：%s", path, code, body)
	}
	return body
}

func notePageBody(t *testing.T) string {
	t.Helper()
	return getJSON(t, newPoolLeakFixture(t), "/api/notes?page_size=10")
}

func commentPageBody(t *testing.T) string {
	t.Helper()
	f := newPoolLeakFixture(t)
	return getJSON(t, f, "/api/questions/"+strconv.Itoa(f.poolQ.ID)+"/comments?page_size=10")
}

func favoritePageBody(t *testing.T) string {
	t.Helper()
	return getJSON(t, newPoolLeakFixture(t), "/api/favorites?page_size=10")
}

func TestNonNilOutletsNeverEmitNull(t *testing.T) {
	for key, outlet := range nonnilOutlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			body := outlet(t)
			if want := `"` + jsonKey + `":[]`; !strings.Contains(body, want) {
				t.Fatalf("声明 nonnil 的 %s 没发出 %s：%s", key, want, body)
			}
		})
	}
}

// contactRequestListBody 空列表那一格的真实响应体。
func contactRequestListBody(t *testing.T) string {
	t.Helper()
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
	return rec.Body.String()
}
