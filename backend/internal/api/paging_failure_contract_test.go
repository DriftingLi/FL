package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// TestListEndpointDBFailureRenders500Envelope（ADR-0056 §1 / issue #1095 验收判据 4）：
// 列表端点的 DB 故障必须渲染 **500 信封**，而不是收口前的 HTTP 200 + items:[] + total:0。
//
// 故障注入：装配完成后关闭底层 *sql.DB 连接池（真实的驱动错误 sql: database is closed）。
// 鉴权走纯 JWT 校验（不查库），因此请求能到达 handler 的查询点；随后由 paging 的
// 错误模式上抛 → Endpoint 骨架 → response.ServerError。
func TestListEndpointDBFailureRenders500Envelope(t *testing.T) {
	cfg := &config.Config{
		JWTSecretKey:    "paging-failure-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	admin := testutil.SeedAdmin(t, db, "adminPagingFail", "x")
	r := NewRouter(newContractDeps(t, db, cfg))
	token, _ := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(admin.AdminID, admin.Username, "admin")

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取底层连接池失败: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("关闭连接池失败: %v", err)
	}

	rec := doWithToken(t, r, token, http.MethodGet, "/api/admin/audit-logs", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("DB 故障必须 500（fail-closed），got %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析 500 信封失败: %v body=%s", err, rec.Body.String())
	}
	if env.Code != 500 || env.Message == "" || string(env.Data) != "null" {
		t.Fatalf("500 信封形状不符（code/message/data）: %s", rec.Body.String())
	}
}
