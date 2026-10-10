// #1621 段4：授权管理端点（角色 CRUD + 管理员挂角色）的行为契约。
//
// 四类判据：① 超管能读到受保护角色与其能力全集；② 受保护角色不可改不可删；
// ③ **最后一个超管不可降级**（防自锁第二层）；④ 端点可达性由**能力表**决定 ——
// 只持有 admin_account.manage 的角色读得到管理员列表、读不到角色列表（403）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"forklift-training/internal/authz"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

type adminRoleRow struct {
	RoleID       int      `json:"role_id"`
	Name         string   `json:"name"`
	Protected    bool     `json:"protected"`
	Capabilities []string `json:"capabilities"`
}

func adminRolesOf(t *testing.T, raw json.RawMessage) []adminRoleRow {
	t.Helper()
	var payload struct {
		Roles []adminRoleRow `json:"roles"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("解析角色列表失败: %v raw=%s", err, raw)
	}
	return payload.Roles
}

func TestAdminAuthzContract(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		JWTSecretKey: "admin-authz-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})

	super := testutil.SeedAdmin(t, db, "authz_super", "x") // SeedAdmin 挂受保护角色
	superTok, err := sess.Issue(super.AdminID, super.Username, "admin")
	if err != nil {
		t.Fatalf("签发超管 token 失败: %v", err)
	}

	// ① 受保护角色在列，且能力集 = 受保护能力全集
	roles := adminRolesOf(t, adminData(t, doWithToken(t, r, superTok, http.MethodGet, "/api/admin/roles", nil), http.StatusOK))
	var protected adminRoleRow
	for _, role := range roles {
		if role.Protected {
			protected = role
		}
	}
	if protected.RoleID == 0 {
		t.Fatalf("角色列表里没有受保护角色: %v", roles)
	}
	if len(protected.Capabilities) != len(authz.ProtectedAdminCapabilities()) {
		t.Fatalf("受保护角色能力集条数 = %d, want %d", len(protected.Capabilities), len(authz.ProtectedAdminCapabilities()))
	}

	// ② 新建角色：合法能力键 201，表外键 400
	raw := adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/roles",
		map[string]any{"name": "运营", "remark": "日常运营", "capabilities": []string{"audit.read", "forum.moderate"}}), http.StatusCreated)
	var created adminRoleRow
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("解析新建角色失败: %v", err)
	}
	if created.RoleID == 0 || len(created.Capabilities) != 2 {
		t.Fatalf("新建角色形状不对: %s", raw)
	}
	if rec := doWithToken(t, r, superTok, http.MethodPost, "/api/admin/roles",
		map[string]any{"name": "坏角色", "capabilities": []string{"nope.nope"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("表外能力键应 400, got %d %s", rec.Code, rec.Body.String())
	}

	// ③ 受保护角色不可改不可删
	if rec := doWithToken(t, r, superTok, http.MethodPut, "/api/admin/roles/"+strconv.Itoa(protected.RoleID),
		map[string]any{"name": protected.Name, "capabilities": []string{"audit.read"}}); rec.Code != http.StatusConflict {
		t.Fatalf("改受保护角色应 409, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, r, superTok, http.MethodDelete, "/api/admin/roles/"+strconv.Itoa(protected.RoleID), nil); rec.Code != http.StatusConflict {
		t.Fatalf("删受保护角色应 409, got %d %s", rec.Code, rec.Body.String())
	}

	// ④ 挂出去的角色不可删（先摘干净）
	ordinary := model.Admin{Username: "authz_ops", Password: "x", Name: "ops", RoleID: &created.RoleID, CreatedAt: testutil.Now()}
	if err := db.Create(&ordinary).Error; err != nil {
		t.Fatalf("建普通管理员失败: %v", err)
	}
	if rec := doWithToken(t, r, superTok, http.MethodDelete, "/api/admin/roles/"+strconv.Itoa(created.RoleID), nil); rec.Code != http.StatusConflict {
		t.Fatalf("删除仍被使用的角色应 409, got %d %s", rec.Code, rec.Body.String())
	}

	// ⑤ 防自锁第二层：最后一个超管不可降级
	if rec := doWithToken(t, r, superTok, http.MethodPut, "/api/admin/accounts/"+strconv.Itoa(super.AdminID)+"/role",
		map[string]any{"role_id": created.RoleID}); rec.Code != http.StatusConflict {
		t.Fatalf("降级最后一个超管应 409, got %d %s", rec.Code, rec.Body.String())
	}
	// 再添一个超管后放行
	second := testutil.SeedAdmin(t, db, "authz_super2", "x")
	if rec := doWithToken(t, r, superTok, http.MethodPut, "/api/admin/accounts/"+strconv.Itoa(second.AdminID)+"/role",
		map[string]any{"role_id": created.RoleID}); rec.Code != http.StatusOK {
		t.Fatalf("有第二个超管时应可改挂, got %d %s", rec.Code, rec.Body.String())
	}

	// ⑥ 可达性由能力表决定：只有 admin_account.manage 的角色读得到账号列表、读不到角色列表
	opsTok, err := sess.Issue(ordinary.AdminID, ordinary.Username, "admin")
	if err != nil {
		t.Fatalf("签发普通管理员 token 失败: %v", err)
	}
	if rec := doWithToken(t, r, opsTok, http.MethodGet, "/api/admin/accounts", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("运营角色没有 admin_account.manage，应 403, got %d", rec.Code)
	}
	if rec := doWithToken(t, r, opsTok, http.MethodGet, "/api/admin/roles", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("运营角色没有 admin_role.manage，应 403, got %d", rec.Code)
	}
}
