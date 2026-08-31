// #417 企业招聘者编辑与重置密码契约测试：
//   - 非管理员 403；
//   - 编辑后列表读到新值（企业名/联系人更新生效）；
//   - 重置密码后旧口令失效、新口令可登录；
//   - 响应与错误信息不回显任何口令字段。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func assertRecruiterEditReset(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`admin123`)
	admin := testutil.SeedAdmin(t, db, `admin1`, pwd)
	stuPwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, stuPwd)

	cfg := &config.Config{
		JWTSecretKey: `recruiter-edit-secret`,
	}
	deps := newContractDeps(t, db, cfg)
	r := NewRouter(deps)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, err := adminSess.Issue(admin.AdminID, admin.Username, `admin`)
	if err != nil {
		t.Fatalf(`issue admin token failed: %v`, err)
	}
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, err := stuSess.Issue(student.ID, student.Username, `hrwai_user`)
	if err != nil {
		t.Fatalf(`issue student token failed: %v`, err)
	}

	// 建一个招聘者
	createBody := map[string]any{
		`username`: `recruit_edit`, `password`: `oldpass123`, `company_name`: `旧企业`,
		`credit_code`: `CCX`, `business_scope`: `叉车`, `contact_name`: `旧联系人`,
		`contact_phone`: `13800000001`, `contact_email`: `edit@ex.com`,
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, `/api/admin/recruiters`, createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf(`create should be 201, got %d body=%s`, rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	data, _ := created[`data`].(map[string]any)
	id := int(data[`id`].(float64))

	// 1. 非管理员编辑 403
	editBody := map[string]any{`company_name`: `新企业`, `credit_code`: `CCX`, `business_scope`: `叉车维修`, `contact_name`: `新联系人`, `contact_phone`: `13800000002`, `contact_email`: `edit2@ex.com`}
	rec = doWithToken(t, r, stuToken, http.MethodPut, `/api/admin/recruiters/`+itoa(id), editBody)
	if rec.Code != http.StatusForbidden {
		t.Fatalf(`non-admin edit should be 403, got %d`, rec.Code)
	}
	// 2. 编辑成功，列表读到新值
	rec = doWithToken(t, r, adminToken, http.MethodPut, `/api/admin/recruiters/`+itoa(id), editBody)
	if rec.Code != http.StatusOK {
		t.Fatalf(`edit should be 200, got %d body=%s`, rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `password`) {
		t.Fatalf(`edit response must not contain password: %s`, rec.Body.String())
	}
	items, _ := fetchRecruiters(t, r, adminToken, ``)
	found := false
	for _, it := range items {
		if int(it[`id`].(float64)) == id && it[`company_name`] == `新企业` && it[`contact_name`] == `新联系人` {
			found = true
		}
	}
	if !found {
		t.Fatalf(`edited values not reflected in list: %v`, items)
	}
	// 3. 重置密码：旧口令失效，新口令可登录
	resetBody := map[string]any{`password`: `newpass456`}
	rec = doWithToken(t, r, adminToken, http.MethodPut, `/api/admin/recruiters/`+itoa(id)+`/password`, resetBody)
	if rec.Code != http.StatusOK {
		t.Fatalf(`reset password should be 200, got %d body=%s`, rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `newpass456`) {
		t.Fatalf(`reset response must not echo password: %s`, rec.Body.String())
	}
	// 旧口令登录失败
	loginBody := map[string]any{`username`: `recruit_edit`, `password`: `oldpass123`}
	rec = doJSON(t, r, http.MethodPost, `/api/auth/recruiter-login`, loginBody)
	if rec.Code == http.StatusOK {
		t.Fatalf(`old password should fail, got %d`, rec.Code)
	}
	// 新口令登录成功
	loginBody[`password`] = `newpass456`
	rec = doJSON(t, r, http.MethodPost, `/api/auth/recruiter-login`, loginBody)
	if rec.Code != http.StatusOK {
		t.Fatalf(`new password should login, got %d body=%s`, rec.Code, rec.Body.String())
	}
}

func itoa(v int) string {
	return fmt.Sprintf(`%d`, v)
}

func TestRecruiterEditResetContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruiterEditReset(t, db)
}

func TestRecruiterEditResetContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruiterEditReset(t, db)
}
