// #1621 段4：授权管理端点（角色 CRUD + 管理员挂角色）的行为契约。
//
// 四类判据：① 超管能读到受保护角色与其能力全集；② 受保护角色不可改不可删；
// ③ **最后一个超管不可降级**（防自锁第二层）；④ 端点可达性由**能力表**决定 ——
// 只持有 admin_account.manage 的角色读得到管理员列表、读不到角色列表（403）。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"forklift-training/internal/authz"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
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

// adminAccountRow 管理员账号行的响应形状（与 admin.AdminAccountDTO 对齐）。
type adminAccountRow struct {
	AdminID   int    `json:"admin_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	RoleID    int    `json:"role_id"`
	RoleName  string `json:"role_name"`
	Protected bool   `json:"protected"`
}

func adminAccountOf(t *testing.T, raw json.RawMessage) adminAccountRow {
	t.Helper()
	var row adminAccountRow
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatalf("解析管理员失败: %v raw=%s", err, raw)
	}
	return row
}

// TestAdminAccountCrudContract #1632：超管**新建 / 删除管理员账号**。
//
// 收敛前这两件事没有端点（只有 GET 列表与 PUT 改挂角色），界面上点不动「新建/删除」。
// 判据：① 新建 201 且**能用该口令真的登进来**（口令走 core 的哈希与长度规则，不是明文）；
// ② 账号名重复 409、口令不合规 400、角色不存在 400；
// ③ 未挂角色的新账号登得进但什么都看不到（能力解析对 NULL role_id fail closed）；
// ④ 删除：删自己 409、删**最后一个超管** 409（可达性：拿 admin_account.manage 的普通角色
// 也能走到这条判据）、删普通管理员 200 且库里真的没了。
func TestAdminAccountCrudContract(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		JWTSecretKey: "admin-account-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})

	super := testutil.SeedAdmin(t, db, "acct_super", "x")
	superTok, err := sess.Issue(super.AdminID, super.Username, "admin")
	if err != nil {
		t.Fatalf("签发超管 token 失败: %v", err)
	}
	role := adminRoleRow{}
	if err := json.Unmarshal(adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/roles",
		map[string]any{"name": "客服", "capabilities": []string{"audit.read"}}), http.StatusCreated), &role); err != nil {
		t.Fatalf("解析新建角色失败: %v", err)
	}

	// ① 新建（挂角色）：201 + 形状 + 口令可登录
	created := adminAccountOf(t, adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_new", "name": "新管理员", "password": "newpass123", "role_id": role.RoleID}),
		http.StatusCreated))
	if created.AdminID == 0 || created.Username != "acct_new" || created.Name != "新管理员" || created.RoleID != role.RoleID {
		t.Fatalf("新建管理员形状不对: %+v", created)
	}
	// 口令是 bcrypt 落的：既不是明文，也能被登录路径的校验认出来
	var stored model.Admin
	if err := db.Where("admin_id = ?", created.AdminID).First(&stored).Error; err != nil {
		t.Fatalf("新建的管理员没落库: %v", err)
	}
	if stored.Password == "newpass123" || !core.VerifyPassword("newpass123", stored.Password) {
		t.Fatalf("口令不是 bcrypt 哈希: %q", stored.Password)
	}
	loginRec := doJSON(t, r, http.MethodPost, "/api/auth/admin-login",
		map[string]any{"username": "acct_new", "password": "newpass123"})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("新建的管理员应能用该口令登录, got %d %s", loginRec.Code, loginRec.Body.String())
	}
	if token := loginTokenOf(t, loginRec); token == "" {
		t.Fatalf("登录响应没有下发 token: %s", loginRec.Body.String())
	}

	// ② 新建（不挂角色）：登得进，但什么都看不到（fail closed）
	norole := adminAccountOf(t, adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_norole", "name": "无角色", "password": "newpass123"}), http.StatusCreated))
	if norole.RoleID != 0 || norole.Protected {
		t.Fatalf("未挂角色的新账号应为 role_id=0 非受保护: %+v", norole)
	}
	noroleTok := loginTokenOf(t, doJSON(t, r, http.MethodPost, "/api/auth/admin-login",
		map[string]any{"username": "acct_norole", "password": "newpass123"}))
	if noroleTok == "" {
		t.Fatal("未挂角色的账号应能登录（未授权不是无法登录）")
	}
	if rec := doWithToken(t, r, noroleTok, http.MethodGet, "/api/admin/accounts", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("未授权账号读管理员列表应 403, got %d %s", rec.Code, rec.Body.String())
	}

	// ③ 三条拒绝：账号名重复 / 口令不合规 / 角色不存在
	if rec := doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_new", "name": "重名", "password": "newpass123"}); rec.Code != http.StatusConflict {
		t.Fatalf("账号名重复应 409, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_bad_pwd", "name": "短口令", "password": "123"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("口令过短应 400, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_bad_role", "name": "坏角色", "password": "newpass123", "role_id": 999999}); rec.Code != http.StatusBadRequest {
		t.Fatalf("角色不存在应 400, got %d %s", rec.Code, rec.Body.String())
	}

	// ④ 删除：删自己 409
	if rec := doWithToken(t, r, superTok, http.MethodDelete, "/api/admin/accounts/"+strconv.Itoa(super.AdminID), nil); rec.Code != http.StatusConflict {
		t.Fatalf("删自己应 409, got %d %s", rec.Code, rec.Body.String())
	}

	// ④b 最后一个超管：操作者是一个持 admin_account.manage 的**普通**角色（能力可授 ⇒ 这条判据可达）
	opsRole := adminRoleRow{}
	if err := json.Unmarshal(adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/roles",
		map[string]any{"name": "账号专员", "capabilities": []string{"admin_account.manage"}}), http.StatusCreated), &opsRole); err != nil {
		t.Fatalf("解析运营角色失败: %v", err)
	}
	ops := adminAccountOf(t, adminData(t, doWithToken(t, r, superTok, http.MethodPost, "/api/admin/accounts",
		map[string]any{"username": "acct_ops", "name": "账号专员", "password": "newpass123", "role_id": opsRole.RoleID}),
		http.StatusCreated))
	if ops.AdminID == 0 {
		t.Fatal("前置：账号专员没建出来")
	}
	opsTok := loginTokenOf(t, doJSON(t, r, http.MethodPost, "/api/auth/admin-login",
		map[string]any{"username": "acct_ops", "password": "newpass123"}))
	if rec := doWithToken(t, r, opsTok, http.MethodDelete, "/api/admin/accounts/"+strconv.Itoa(super.AdminID), nil); rec.Code != http.StatusConflict {
		t.Fatalf("删最后一个超管应 409, got %d %s", rec.Code, rec.Body.String())
	}

	// ④c 参数与存在性：id 非正数 400、账号不存在 404
	if rec := doWithToken(t, r, superTok, http.MethodDelete, "/api/admin/accounts/0", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("管理员 ID 非正数应 400, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, r, superTok, http.MethodDelete, "/api/admin/accounts/999999", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("删不存在的账号应 404, got %d %s", rec.Code, rec.Body.String())
	}

	// ④d 「删成功」那条（200 + 落库消失 + 旧 refresh 当场失效）打在 internal/admin 的服务面：
	// 删除要先写吊销标记，而契约装配里**服务持有的会话是 Redis 版**（NewDeps 的 coreSingletons 用
	// SessionFromConfig 建，helper 只换掉了 d.Session），这一层没有 Redis。同一 seam 的注释见
	// internal/admin/admin_account_test.go。
}

// loginTokenOf 从登录响应里取出 access token（空串 = 没登成）。
func loginTokenOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析登录响应失败: %v body=%s", err, rec.Body.String())
	}
	return payload.Data.Token
}
