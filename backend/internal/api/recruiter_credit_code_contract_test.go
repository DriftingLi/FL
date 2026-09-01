// #450 企业招聘账号收敛为与企业 1:1 契约测试：
//   - 创建时信用代码已被占用 → 400 + 可读提示「该企业已存在招聘者账号」，且不入库；
//   - 编辑把信用代码改成别家已占用的值 → 同样被拒；
//   - 自己保持原信用代码编辑 → 放行（不算自我占用）；
//   - 正常路径：不同信用代码可各自建号。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
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

func assertRecruiterCreditCodeUnique(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminCCU", pwd)

	cfg := &config.Config{JWTSecretKey: "credit-code-unique-secret"}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, err := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("issue admin token failed: %v", err)
	}

	createBody := func(username, creditCode string) map[string]any {
		return map[string]any{
			"username": username, "password": "recruit123", "company_name": "测试企业-" + username,
			"credit_code": creditCode, "business_scope": "叉车维修", "contact_name": "联系人-" + username,
			"contact_phone": "13800001111", "contact_email": username + "@example.com",
		}
	}

	// 1. 正常创建第一家
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("ccuA", "91110000CCUAAAAAA"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create first should be 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	data, _ := created["data"].(map[string]any)
	firstID := int(data["id"].(float64))

	// 2. 同一信用代码再建 → 400 + 可读提示 + 不入库
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("ccuB", "91110000CCUAAAAAA"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate credit code create should be 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "该企业已存在招聘者账号") {
		t.Fatalf("duplicate message should mention 该企业已存在招聘者账号, got %s", rec.Body.String())
	}
	var dupCount int64
	db.Model(&model.RecruiterUser{}).Where("username = ?", "ccuB").Count(&dupCount)
	if dupCount != 0 {
		t.Fatalf("duplicate credit code must not insert, count=%d", dupCount)
	}

	// 3. 不同信用代码可各自建号（正常路径，先建别家供后续编辑占用测试）
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", createBody("ccuC", "91110000CCUCCCCCC"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create different credit code should be 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	// 4. 编辑把信用代码改成别家已占用的值 → 400 + 可读提示
	editBody := map[string]any{
		"company_name": "测试企业-ccuA", "credit_code": "91110000CCUCCCCCC", "business_scope": "叉车维修",
		"contact_name": "联系人-ccuA", "contact_phone": "13800001111", "contact_email": "ccuA@example.com",
	}
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/"+itoa(firstID), editBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("edit to occupied credit code should be 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "该企业已存在招聘者账号") {
		t.Fatalf("edit duplicate message should mention 该企业已存在招聘者账号, got %s", rec.Body.String())
	}
	// 5. 自己保持原信用代码编辑 → 放行
	editBody["credit_code"] = "91110000CCUAAAAAA"
	editBody["company_name"] = "改名企业"
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/recruiters/"+itoa(firstID), editBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit keeping own credit code should be 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRecruiterCreditCodeUnique_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruiterCreditCodeUnique(t, db)
}

func TestRecruiterCreditCodeUnique_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruiterCreditCodeUnique(t, db)
}
