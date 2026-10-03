// 装配根签出的 access 必须真的可用（P2 波 3a 血账的本地钉子）。
//
// 来由：internal/auth/account_deletion_postgres_contract_test.go 的两条用例走真实路由面 + 真 access，
// 但它们只在 CI 上跑（testutil.NewPostgresDB 缺 DATABASE_URL 时干净 skip），于是
// 「newContractDeps 给的字面量 cfg 缺 JWTExpiresHours ⇒ JWTExpiry() = 0 ⇒ 签出的 access 立即过期
// ⇒ 401 Token无效或已过期」这件事本地全绿、CI 判红。本用例用同一条装配链（内存库，不需要 PG）
// 把其中一步单独钉住：装配根签出的 access 打到带 JWTAuth 的真实路由上，不许是 401。
//
// 钉的是 config.Config 的零值回退（config.go 的 JWTExpiry：零值按 Load 的默认口径回退 2h）——
// 那条回退一旦被删，这里立刻判红，不必等到 CI 的带 PG 的 backend-test。
package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/auth"
	"forklift-training/internal/core"
	"forklift-training/internal/testutil"
)

func TestContractDepsAccessIsNotExpired(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	student := seedStudent(t, db, "dep_access_guard", "hash")
	// 显式传 nil：走 newContractDeps 的默认装配，也就是 CI 上那条 PG 用例走的路。
	deps := newContractDeps(t, db, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	auth.RegisterRoutes(r.Group("/api"), deps.Session, deps.AuthSvc, nil, nil, nil, zap.NewNop())

	tok, _, err := deps.Session.IssuePair(student.ID, "dep_access_guard", core.HrwaiRole)
	if err != nil {
		t.Fatalf("签发 access 失败: %v", err)
	}
	rec := testutil.CodeAuthRequest(r, http.MethodDelete, "/api/auth/account", nil, tok)
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("装配根签出的 access 被判 401（token 一签发就过期？查 config.Config 的 JWT 有效期回退），body=%s", rec.Body.String())
	}
}
