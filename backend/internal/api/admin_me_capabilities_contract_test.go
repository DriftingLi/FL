// #1618 段1：GET /api/admin/me/capabilities 的行为契约。
//
// 四档：超管（受保护角色）拿到管理能力全集、未挂角色的管理员拿到**空数组**（不是 null）、
// 非管理员 403、未认证 401。第一档的「全集」判据取自 authz 自己的声明（不在这里另抄清单）。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/authz"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestAdminMeCapabilitiesContract(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		JWTSecretKey: "admin-me-caps-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})

	// 1) 超管：受保护角色 → 能力全集
	super := testutil.SeedAdmin(t, db, "caps_super", "x")
	superTok, err := sess.Issue(super.AdminID, super.Username, "admin")
	if err != nil {
		t.Fatalf("签发超管 token 失败: %v", err)
	}
	rec := doWithToken(t, r, superTok, http.MethodGet, "/api/admin/me/capabilities", nil)
	raw := adminData(t, rec, http.StatusOK)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("解析 data 失败: %v raw=%s", err, raw)
	}
	if granted, _ := data["granted"].(bool); !granted {
		t.Fatalf("超管应 granted=true, raw=%s", raw)
	}
	items, _ := data["capabilities"].([]any)
	got := map[string]bool{}
	for _, it := range items {
		if s, ok := it.(string); ok {
			got[s] = true
		}
	}
	for _, want := range authz.ProtectedAdminCapabilities() {
		if !got[string(want)] {
			t.Fatalf("超管能力集缺 %q（受保护角色应持有全集）", want)
		}
	}
	if len(items) != len(authz.ProtectedAdminCapabilities()) {
		t.Fatalf("超管能力集条数 = %d, want %d（不多不少：受保护角色的可达面就是这张清单）",
			len(items), len(authz.ProtectedAdminCapabilities()))
	}

	// 2) 未挂角色的管理员：登录成功但无任何管理能力（fail closed），capabilities 是**空数组**
	norole := model.Admin{Username: "caps_norole", Password: "x", Name: "norole", CreatedAt: testutil.Now()}
	if err := db.Create(&norole).Error; err != nil {
		t.Fatalf("建无角色管理员失败: %v", err)
	}
	noroleTok, err := sess.Issue(norole.AdminID, norole.Username, "admin")
	if err != nil {
		t.Fatalf("签发无角色 token 失败: %v", err)
	}
	rec = doWithToken(t, r, noroleTok, http.MethodGet, "/api/admin/me/capabilities", nil)
	raw = adminData(t, rec, http.StatusOK)
	if !strings.Contains(string(raw), "\"capabilities\":[]") {
		t.Fatalf("未挂角色时 capabilities 必须是空数组而非 null, raw=%s", raw)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("解析 data 失败: %v raw=%s", err, raw)
	}
	if granted, _ := data["granted"].(bool); granted {
		t.Fatalf("未挂角色应 granted=false, raw=%s", raw)
	}

	// 3) 非管理员（学员）→ 403
	stu := testutil.SeedStudent(t, db, "caps_stu", "x")
	stuTok, err := sess.Issue(stu.ID, stu.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}
	if rec = doWithToken(t, r, stuTok, http.MethodGet, "/api/admin/me/capabilities", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("学员读管理端能力集应 403, got %d %s", rec.Code, rec.Body.String())
	}

	// 4) 未认证 → 401
	if rec = doWithToken(t, r, "", http.MethodGet, "/api/admin/me/capabilities", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证应 401, got %d %s", rec.Code, rec.Body.String())
	}
}
