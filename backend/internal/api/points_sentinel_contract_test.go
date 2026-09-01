// Contract test ADR-0024 C1：积分域错误哨兵化后 HTTP 状态码与信封零漂移。
// 哨兵映射：已领取类 → 400、不存在类 → 404/400（沿用既有映射）、参数非法 → 400。
// Main seam：HTTP contract 层（router -> httptest -> 断言状态码 + 文案）。
package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// TestPointsSentinelStatusMapping 哨兵 → 状态码映射零漂移。
func TestPointsSentinelStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	pwd, _ := service.HashPassword("student123")
	student := testutil.SeedStudent(t, db, "sentinel_stu", pwd)

	cfg := &config.Config{
		JWTSecretKey:    "points-sentinel-contract-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	token, err := sess.Issue(student.ID, student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	// 1. 任务不存在 → 404
	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/no_such_task/claim", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("任务不存在应 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "任务不存在") {
		t.Fatalf("任务不存在文案应保留, got %s", rec.Body.String())
	}

	// 2. 课程不存在 → 400
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/shop/course/999/redeem", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("课程不存在应 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "课程不存在") {
		t.Fatalf("课程不存在文案应保留, got %s", rec.Body.String())
	}

	// 3. 商城商品不存在 → 400
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/shop/no_such_sku/redeem", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("商品不存在应 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "商品不存在") {
		t.Fatalf("商品不存在文案应保留, got %s", rec.Body.String())
	}

	// 4. 重复领取（newbie 终身已领）→ 400 且文案「已领取」
	// 播种 newbie 任务并先领一次
	if err := db.Create(&model.PointsTaskConfig{
		Code: "newbie_credential", Title: "选定目标证件", Group: "newbie", Points: 10,
		DailyLimit: 1, TotalLimit: intPtr(1), EventType: "credential_onboarding",
	}).Error; err != nil {
		t.Fatalf("seed newbie task failed: %v", err)
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/newbie_credential/claim", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("首次领取 newbie 应 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/newbie_credential/claim", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("重复领取 newbie 应 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "已领取") {
		t.Fatalf("重复领取文案应保留「已领取」, got %s", rec.Body.String())
	}
}
