// #418 招聘留痕/申请记录巡检契约：
//   - 非管理员不可访问；
//   - 响应字段集合不含明文联系方式（手机号/微信/精确现居地/PDF/证书原图等凭据式字段）；
//   - 查看记录/申请记录可过滤（recruiter_id / student_user_id）并分页。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
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

// forbiddenCredentialKeys 管理面绝不允许出现的凭据式字段（#418）。
var forbiddenCredentialKeys = []string{
	`contact_phone`, `wechat`, `region`, `real_name`, `resume_file_url`, `photos`,
	`phone`, `mobile`, `email`, `address`, `cert_no`, `image_urls`,
}

func assertRecruitTrail(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`admin123`)
	admin := testutil.SeedAdmin(t, db, `admin1`, pwd)
	stuPwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, stuPwd)
	// 种招聘者 + 查看留痕 + 申请记录
	recruiter := testutil.SeedRecruiter(t, db, `rec1`, pwd)
	if err := db.Create(&model.RecruitResumeView{RecruiterID: recruiter.ID, ResumeUserID: student.ID, ViewedAt: time.Now()}).Error; err != nil {
		t.Fatalf(`seed view failed: %v`, err)
	}
	if err := db.Create(&model.ContactRequest{RecruiterID: recruiter.ID, StudentUserID: student.ID, Message: `想了解您的求职意向`, Status: `pending`, CreatedAt: time.Now(), UpdatedAt: time.Now(), ExpiresAt: time.Now().Add(14 * 24 * time.Hour)}).Error; err != nil {
		t.Fatalf(`seed request failed: %v`, err)
	}

	cfg := &config.Config{
		JWTSecretKey: `trail-contract-secret`,
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

	// 1. 非管理员 403（两个端点）
	rec := doWithToken(t, r, stuToken, http.MethodGet, `/api/admin/recruit/views`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf(`non-admin views should be 403, got %d`, rec.Code)
	}
	rec = doWithToken(t, r, stuToken, http.MethodGet, `/api/admin/recruit/requests`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf(`non-admin requests should be 403, got %d`, rec.Code)
	}

	// 2. 查看留痕：字段集合不含明文
	rec = doWithToken(t, r, adminToken, http.MethodGet, `/api/admin/recruit/views`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf(`views should be 200, got %d body=%s`, rec.Code, rec.Body.String())
	}
	assertNoCredentialFields(t, rec.Body.String(), `views`)
	// 3. 申请记录：字段集合不含明文，且覆盖状态
	rec = doWithToken(t, r, adminToken, http.MethodGet, `/api/admin/recruit/requests`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf(`requests should be 200, got %d body=%s`, rec.Code, rec.Body.String())
	}
	assertNoCredentialFields(t, rec.Body.String(), `requests`)
	if !strings.Contains(rec.Body.String(), `pending`) {
		t.Fatalf(`request status should be present: %s`, rec.Body.String())
	}
	// 4. 按招聘者过滤
	rec = doWithToken(t, r, adminToken, http.MethodGet, `/api/admin/recruit/views?recruiter_id=`+itoa(recruiter.ID), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `recruiter_id`) {
		t.Fatalf(`filtered views failed: %d %s`, rec.Code, rec.Body.String())
	}
}

func assertNoCredentialFields(t *testing.T, body, label string) {
	t.Helper()
	for _, key := range forbiddenCredentialKeys {
		if strings.Contains(body, `"`+key+`"`) {
			t.Fatalf(`%s must not expose %q: %s`, label, key, body)
		}
	}
}

func TestRecruitTrailContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruitTrail(t, db)
}

func TestRecruitTrailContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruitTrail(t, db)
}
