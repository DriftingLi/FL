// #411 巡检问答积分流水按业务域收口契约测试：
//   - 不传 ref_type = 跨业务域全量（管理员知情切换仍可用）；
//   - ref_type=forum_topic → 只回问答域流水；
//   - 分页生效且页间不重叠；非管理员 403。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
	"net/http"
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

// fetchLedgerPage 请求流水接口并以 map 解信封（不冻结 struct tag 形状，只锁行为）。
func fetchLedgerPage(t *testing.T, r *gin.Engine, token, query string) ([]map[string]any, float64, float64) {
	t.Helper()
	path := `/api/admin/points/ledger`
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
	if code, ok := env[`code`].(float64); !ok || code != http.StatusOK {
		t.Fatalf(`envelope code should be 200, got %v`, env[`code`])
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
	pages, _ := data[`pages`].(float64)
	return items, total, pages
}

func assertLedgerDomainFilter(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`admin123`)
	admin := testutil.SeedAdmin(t, db, `admin1`, pwd)
	stuPwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, stuPwd)
	cfg := &config.Config{
		JWTSecretKey: `ledger-contract-secret`,
	}
	r := gin.New()
	api := r.Group(`/api`)
	deps := newContractDeps(t, db, cfg)
	RegisterAdminInspectionRoutes(api, deps.RouterDeps(), db, deps.PointsSvc)

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

	rows := []model.PointsLedger{
		{UserID: student.ID, Delta: 40, Reason: `accept_reward`, RefType: `forum_topic`, RefID: `42`},
		{UserID: student.ID, Delta: 5, Reason: `daily_checkin`, RefType: `task`, RefID: `daily_checkin`},
		{UserID: student.ID, Delta: -100, Reason: `redeem_course`, RefType: `course`, RefID: `7`},
		{UserID: student.ID, Delta: -30, Reason: `ai_tokens`, RefType: `ai_chat`, RefID: `request-1`},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf(`seed ledger rows failed: %v`, err)
	}

	// 1. 非管理员 403
	rec := doWithToken(t, r, stuToken, http.MethodGet, `/api/admin/points/ledger`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf(`non-admin should be 403, got %d`, rec.Code)
	}
	// 2. 默认（不传 ref_type）= 跨业务域全量
	allItems, allTotal, _ := fetchLedgerPage(t, r, adminToken, ``)
	if allTotal != 4 || len(allItems) != 4 {
		t.Fatalf(`all-domain should be 4 rows, got total=%v len=%d`, allTotal, len(allItems))
	}
	// 3. 问答域过滤
	qaItems, qaTotal, _ := fetchLedgerPage(t, r, adminToken, `ref_type=forum_topic`)
	if qaTotal != 1 || len(qaItems) != 1 {
		t.Fatalf(`qa domain should be 1 row, got total=%v len=%d`, qaTotal, len(qaItems))
	}
	for _, it := range qaItems {
		if ref, _ := it[`ref_type`].(string); ref != `forum_topic` {
			t.Fatalf(`qa row should be forum_topic, got %v`, it[`ref_type`])
		}
	}
	// 4. 分页：page_size=2 → 2 页，页间不重叠
	p1, _, pages := fetchLedgerPage(t, r, adminToken, `page=1&page_size=2`)
	if len(p1) != 2 || pages != 2 {
		t.Fatalf(`page1 should be 2 items & 2 pages, got %d / %v`, len(p1), pages)
	}
	p2, _, _ := fetchLedgerPage(t, r, adminToken, `page=2&page_size=2`)
	if len(p2) != 2 {
		t.Fatalf(`page2 should be 2 items, got %d`, len(p2))
	}
	seen := map[float64]bool{}
	for _, it := range p1 {
		if id, ok := it[`id`].(float64); ok {
			seen[id] = true
		}
	}
	for _, it := range p2 {
		if id, ok := it[`id`].(float64); ok && seen[id] {
			t.Fatalf(`page2 overlaps page1 (id=%v)`, id)
		}
	}
}

// SQLite 适配器（恒绿，锁回归）。
func TestPointsLedgerContract_DomainFilterOnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertLedgerDomainFilter(t, db)
}

// Postgres 适配器（真实 SQL 迁移建表；无 DATABASE_URL 自动跳过）。
func TestPointsLedgerContract_DomainFilterOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertLedgerDomainFilter(t, db)
}
