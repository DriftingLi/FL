// 契约测试 问题6：招聘者用户名格式校验与管理员改用户名。
//   - 创建时用户名含非法字符（中文/空格/连字符）→ 400，不入库；
//   - 长度 <4 或 >20 → 400；
//   - 管理员编辑可修改用户名（合法新名生效，新用户名可登录）；
//   - 改成已占用用户名 → 400。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func assertRecruiterUsernameContract(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminUser", pwd)

	cfg := &config.Config{JWTSecretKey: "username-secret", JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	createBody := func(username string) map[string]any {
		return map[string]any{
			"username": username, "password": "recruit123", "company_name": "测试企业-" + username,
			"credit_code": "91110000" + username, "business_scope": "叉车维修", "contact_name": "王五",
			"contact_phone": "13800001111", "contact_email": username + "@example.com",
		}
	}

	// 1. 非法用户名（含中文）→ 400 + 不入库
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("张三丰"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("chinese username should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "字母、数字和下划线") {
		t.Fatalf("invalid username message should mention format, got %s", rec.Body.String())
	}
	var cnt int64
	db.Model(&model.RecruiterUser{}).Where("username = ?", "张三丰").Count(&cnt)
	if cnt != 0 {
		t.Fatalf("invalid username must not insert, count=%d", cnt)
	}

	// 2. 非法用户名（含空格/连字符）→ 400
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("recruit test"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("space username should 400, got %d", rec.Code)
	}
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("recruit-test"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("hyphen username should 400, got %d", rec.Code)
	}

	// 3. 长度 <4 → 400
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("abc"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short username should 400, got %d", rec.Code)
	}

	// 4. 合法用户名正常创建
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("recruit_a1"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid username should 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	data, _ := created["data"].(map[string]any)
	id := int(data["id"].(float64))

	// 5. 管理员改用户名：改成合法新名 → 成功，新用户名可登录
	editBody := map[string]any{
		"username": "recruit_new", "company_name": "测试企业-recruit_a1", "credit_code": "91110000recruit_a1",
		"business_scope": "叉车维修", "contact_name": "王五", "contact_phone": "13800001111", "contact_email": "recruit_a1@example.com",
	}
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/"+itoa(id), editBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit username should 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "recruit_new") {
		t.Fatalf("edit response should contain new username, got %s", rec.Body.String())
	}
	// 新用户名登录成功
	rec = doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruit_new", "password": "recruit123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login with new username should 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 6. 改成已占用用户名 → 400
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("another_rec"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create second recruiter should 201, got %d", rec.Code)
	}
	editBody["username"] = "another_rec"
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/"+itoa(id), editBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("edit to taken username should 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "用户名已被注册") {
		t.Fatalf("taken username message should mention 用户名已被注册, got %s", rec.Body.String())
	}
}

func TestRecruiterUsernameContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruiterUsernameContract(t, db)
}

func TestRecruiterUsernameContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruiterUsernameContract(t, db)
}
