// 契约测（ADR-0062 票9）：不存在的 AI 配置必须回具名业务语义，
// 而不是把驱动原文经「更新失败: 」前缀送进中文界面（旧形状 = errStatusAllPrefix(500) +
// service 返回裸 gorm.ErrRecordNotFound）。
package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestUpdateMissingAIConfigRenders404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "ai-config-404-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	admin := testutil.SeedAdmin(t, db, "aiConfig404Admin", "x")
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	rec := doWithToken(t, r, token, http.MethodPut, "/api/admin/ai-configs/999999", map[string]any{
		"name": "改个不存在的配置", "base_url": "https://api.example.com", "model": "deepseek-chat",
	})
	body := rec.Body.String()
	if rec.Code != http.StatusNotFound {
		t.Fatalf("更新不存在的 AI 配置应 404, got %d %s", rec.Code, body)
	}
	if strings.Contains(body, "record not found") || strings.Contains(body, "gorm") {
		t.Fatalf("响应不得携带驱动原文: %s", body)
	}
	if strings.Contains(body, "更新失败") {
		t.Fatalf("改判后不应再走「更新失败: 」前缀: %s", body)
	}
	// 文案必须是具名业务语义（与 service 哨兵同源）
	if !strings.Contains(body, service.ErrAIConfigNotFound.Error()) {
		t.Fatalf("响应文案应为「%s」: %s", service.ErrAIConfigNotFound.Error(), body)
	}
}
