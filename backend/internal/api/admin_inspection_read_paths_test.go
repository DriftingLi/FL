// #1097 巡检读面归位 service：形状锁 + 失败用例。
//
// 形状锁的参照物是**搬迁前实现上实测抓下来的响应体**（不是从新实现反抄的 JSON 字面量）：
// 字段声明序 / 时间形状 / 空切片语义（[] 不是 null）/ 键序任一漂移即红。
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

// setupInspectionRouter 起一台含巡检路由的整机（内存库 + 管理员 token）。
func setupInspectionRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "inspection-shape-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	pwd, err := service.HashPassword("admin123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	admin := testutil.SeedAdmin(t, db, "adminInspection", pwd)
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	token, err := sess.Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return r, db, token
}

// seedInspectionRows 固定时间戳的巡检样本（时间取 UTC，回读形状与搬迁前一致）。
func seedInspectionRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	viewedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 9, 16, 9, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 9, 16, 9, 45, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 9, 30, 9, 45, 0, 0, time.UTC)
	if err := db.Create(&model.RecruitResumeView{ID: 11, RecruiterID: 3, ResumeUserID: 7, ViewedAt: viewedAt}).Error; err != nil {
		t.Fatalf("seed view: %v", err)
	}
	if err := db.Create(&model.ContactRequest{
		ID: 21, RecruiterID: 3, StudentUserID: 7, Message: "巡检测试", Status: "pending", Source: "recruiter",
		CreatedAt: createdAt, UpdatedAt: updatedAt, ExpiresAt: &expiresAt,
	}).Error; err != nil {
		t.Fatalf("seed request: %v", err)
	}
	if err := db.Create(&model.SystemSetting{
		Key: "deleted_after_accepted", Value: "42", Description: "删除已解决帖计数", UpdatedAt: updatedAt,
	}).Error; err != nil {
		t.Fatalf("seed setting failure: %v", err)
	}
}

// TestAdminInspectionReadPathsShapeLock 三条读路径与搬迁前逐字节全等（空态 + 有数据 + 过滤）。
func TestAdminInspectionReadPathsShapeLock(t *testing.T) {
	r, db, token := setupInspectionRouter(t)

	const (
		emptyViews = `{"code":200,"message":"success","data":{"items":[],"page":1,"page_size":20,"total":0}}`
		emptyReqs  = `{"code":200,"message":"success","data":{"items":[],"page":1,"page_size":20,"total":0}}`
		seedViews  = `{"code":200,"message":"success","data":{"items":[{"id":11,"recruiter_id":3,"resume_user_id":7,"viewed_at":"2026-09-17T10:00:00Z"}],"page":1,"page_size":20,"total":1}}`
		seedReqs   = `{"code":200,"message":"success","data":{"items":[{"id":21,"recruiter_id":3,"student_user_id":7,"message":"巡检测试","status":"pending","source":"recruiter","created_at":"2026-09-16T09:30:00Z","updated_at":"2026-09-16T09:45:00Z","expires_at":"2026-09-30T09:45:00Z"}],"page":1,"page_size":20,"total":1}}`
	)

	// 空态：三条路径在无数据时的字节（items 是 [] 不是 null，计数行缺失 = 0 不是错误）。
	for _, tc := range []struct{ name, path, want string }{
		{"空态 /admin/recruit/views", "/api/admin/recruit/views", emptyViews},
		{"空态 /admin/recruit/requests", "/api/admin/recruit/requests", emptyReqs},
		{"空态 /admin/inspection/deleted-after-accepted", "/api/admin/inspection/deleted-after-accepted", `{"code":200,"message":"success","data":{"count":0}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doWithToken(t, r, token, http.MethodGet, tc.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("响应字节漂移（ADR-0009 §2）：\n got = %s\nwant = %s", got, tc.want)
			}
		})
	}

	seedInspectionRows(t, db)

	for _, tc := range []struct{ name, path, want string }{
		{"有数据 /admin/recruit/views", "/api/admin/recruit/views?page=1&page_size=20", seedViews},
		{"有数据 /admin/recruit/requests", "/api/admin/recruit/requests?page=1&page_size=20", seedReqs},
		{"有数据 /admin/inspection/deleted-after-accepted", "/api/admin/inspection/deleted-after-accepted", `{"code":200,"message":"success","data":{"count":42}}`},
		// 过滤位：recruiter_id/student_user_id >0 生效，status 非空生效（命中与不命中各一）。
		{"过滤 /admin/recruit/views?recruiter_id=3&student_user_id=7", "/api/admin/recruit/views?recruiter_id=3&student_user_id=7", seedViews},
		{"过滤不命中 /admin/recruit/views?recruiter_id=999", "/api/admin/recruit/views?recruiter_id=999", emptyViews},
		{"过滤不命中 /admin/recruit/requests?status=approved", "/api/admin/recruit/requests?status=approved", emptyReqs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doWithToken(t, r, token, http.MethodGet, tc.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body = %s", rec.Code, rec.Body.String())
			}
			if got := rec.Body.String(); got != tc.want {
				t.Fatalf("响应字节漂移（ADR-0009 §2）：\n got = %s\nwant = %s", got, tc.want)
			}
		})
	}
}

// TestAdminInspectionReadPathsFailureIs500 失败用例：读库故障 → 500 信封（不再是 200 + 空列表）。
// 注入手段：删表（查询即 ErrNoSuchTable），与 paging 失败注入同族。
func TestAdminInspectionReadPathsFailureIs500(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		table any
	}{
		{"recruit/views 查询失败", "/api/admin/recruit/views", &model.RecruitResumeView{}},
		{"recruit/requests 查询失败", "/api/admin/recruit/requests", &model.ContactRequest{}},
		{"deleted-after-accepted 查询失败", "/api/admin/inspection/deleted-after-accepted", &model.SystemSetting{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, token := setupInspectionRouter(t)
			if err := db.Migrator().DropTable(tc.table); err != nil {
				t.Fatalf("注入故障（删表）失败: %v", err)
			}
			rec := doWithToken(t, r, token, http.MethodGet, tc.path, nil)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("DB 故障应渲染 500 信封，实际 %d body = %s", rec.Code, rec.Body.String())
			}
			var env struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Data    any    `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatalf("解析信封失败: %v", err)
			}
			if env.Code != http.StatusInternalServerError || env.Data != nil || env.Message == "" {
				t.Fatalf("500 信封形状不符：%s", rec.Body.String())
			}
		})
	}
}
