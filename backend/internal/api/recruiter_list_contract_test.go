// #416 管理端企业招聘者列表契约测试：真列表（分页+关键字）替换硬编码空数组桩；
// 响应字段白名单不含任何凭据（口令哈希）；非管理员 403。
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
	"net/http"
	"net/url"
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

func fetchRecruiters(t *testing.T, r *gin.Engine, token, query string) ([]map[string]any, float64) {
	t.Helper()
	path := `/api/admin/recruiters`
	if query != `` {
		path += `?` + query
	}
	rec := doWithToken(t, r, token, http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf(`GET %s should be 200, got %d body=%s`, path, rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf(`parse envelope failed: %v`, err)
	}
	data, _ := env[`data`].(map[string]any)
	items := []map[string]any{}
	if raw, ok := data[`items`].([]any); ok {
		for _, it := range raw {
			if m, ok := it.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	total, _ := data[`total`].(float64)
	return items, total
}

func assertRecruiterList(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`admin123`)
	admin := testutil.SeedAdmin(t, db, `admin1`, pwd)
	stuPwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, stuPwd)
	// 种 3 个招聘者（2 个企业名含「叉车」，1 个不含）
	seed := []model.RecruiterUser{
		{Username: `recruit_a`, Password: pwd, CompanyName: `上海叉车租赁`, CreditCode: `CC1`, BusinessScope: `叉车维修`, ContactName: `甲`, ContactPhone: `13800000001`, ContactEmail: `a@ex.com`, Status: 1, CreatedAt: time.Now()},
		{Username: `recruit_b`, Password: pwd, CompanyName: `北京叉车贸易`, CreditCode: `CC2`, BusinessScope: `叉车销售`, ContactName: `乙`, ContactPhone: `13800000002`, ContactEmail: `b@ex.com`, Status: 0, CreatedAt: time.Now()},
		{Username: `recruit_c`, Password: pwd, CompanyName: `物流设备公司`, CreditCode: `CC3`, BusinessScope: `物流`, ContactName: `丙`, ContactPhone: `13800000003`, ContactEmail: `c@ex.com`, Status: 1, CreatedAt: time.Now()},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf(`seed recruiters failed: %v`, err)
	}

	cfg := &config.Config{
		JWTSecretKey: `recruiter-list-secret`,
	}
	r := gin.New()
	api := r.Group(`/api`)
	deps := newContractDeps(t, db, cfg)
	RegisterAdminRecruiterRoutes(api, deps.RouterDeps(), deps.AuthSvc)
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

	// 1. 非管理员 403
	rec := doWithToken(t, r, stuToken, http.MethodGet, `/api/admin/recruiters`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf(`non-admin should be 403, got %d`, rec.Code)
	}
	// 2. 全量列表：3 条，字段白名单无凭据
	items, total := fetchRecruiters(t, r, adminToken, ``)
	if total != 3 || len(items) != 3 {
		t.Fatalf(`all recruiters should be 3, got total=%v len=%d`, total, len(items))
	}
	for _, it := range items {
		if _, ok := it[`password`]; ok {
			t.Fatalf(`password must not appear in recruiter list: %v`, it)
		}
		if _, ok := it[`password_hash`]; ok {
			t.Fatalf(`password_hash must not appear: %v`, it)
		}
		if _, ok := it[`company_name`]; !ok {
			t.Fatalf(`company_name missing: %v`, it)
		}
	}
	// 3. 关键字过滤：企业名含「叉车」2 条
	kwItems, kwTotal := fetchRecruiters(t, r, adminToken, `keyword=`+url.QueryEscape(`叉车`))
	if kwTotal != 2 || len(kwItems) != 2 {
		t.Fatalf(`keyword filter should be 2, got total=%v len=%d`, kwTotal, len(kwItems))
	}
	// 4. 分页：page_size=2 → 2 页
	p1, total1 := fetchRecruiters(t, r, adminToken, `page=1&page_size=2`)
	if len(p1) != 2 || total1 != 3 {
		t.Fatalf(`page1 should be 2 items of 3, got len=%d total=%v`, len(p1), total1)
	}
}

func TestRecruiterListContract_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertRecruiterList(t, db)
}

func TestRecruiterListContract_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertRecruiterList(t, db)
}
