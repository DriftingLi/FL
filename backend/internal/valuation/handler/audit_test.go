package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
	vservice "forklift-training/internal/valuation/service"
)

// TestValuationAdminWriteAudited 锁定 ADR-0012 §7：
// /api/valuation/admin/* 写操作接入 AuditLog，与主体系管理端同一留痕口径。
func TestValuationAdminWriteAudited(t *testing.T) {
	gin.SetMode(gin.TestMode)

	auditDB := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "test-secret"}
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: "hrwai_token"})

	r := gin.New()
	dict := newSeedMemDict()
	evalStore := newMemEvalStore()
	batteryStore := &memBatteryStore{}
	valuationSvc, err := vservice.NewValuationService(dict, evalStore)
	if err != nil {
		t.Fatalf("构造估值服务失败: %v", err)
	}
	RegisterRoutes(r, sess, zap.NewNop(), cfg, auditDB,
		dict, evalStore, batteryStore,
		valuationSvc, vservice.NewBatteryRULService(),
		&memReportGenerator{}, &memStorage{},
		nil)

	token, err := sess.Issue(1, "admin1", "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}

	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"name": "合力", "code": "HELI", "sort_order": 1}, "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}

	var logs []model.AuditLog
	if err := auditDB.Find(&logs).Error; err != nil {
		t.Fatalf("查询审计日志失败: %v", err)
	}
	if len(logs) == 0 {
		t.Fatal("管理员字典写操作应产生审计日志")
	}
	entry := logs[0]
	if entry.ActorRole != "admin" || entry.ActorID != 1 {
		t.Fatalf("审计条目操作者异常: role=%s id=%d", entry.ActorRole, entry.ActorID)
	}
	if entry.Path != "/api/valuation/admin/brands" || entry.Method != http.MethodPost {
		t.Fatalf("审计条目路径/方法异常: %s %s", entry.Method, entry.Path)
	}
	if entry.Status != http.StatusOK {
		t.Fatalf("审计条目状态应为 200, got %d", entry.Status)
	}
}

// TestValuationAdminWriteNotAuditedWithoutDB 审计 DB 未装配时写操作不 panic（测试降级路径）。
func TestValuationAdminWriteNotAuditedWithoutDB(t *testing.T) {
	r, _, _, _ := newTestValuationEngineWithStorage(t, &memStorage{})

	// hrwai_user token：JWTAuth 放行、RoleRequired 拒绝（403），
	// 证明审计 DB 未装配时中间件链不受影响、写操作不 panic。
	w := performRequestWithAuth(r, http.MethodPost, "/api/valuation/admin/brands",
		map[string]interface{}{"name": "合力", "code": "HELI", "sort_order": 1}, authHeader(t, 1))
	if w.Code != http.StatusForbidden {
		t.Fatalf("hrwai_user 写操作状态码 = %d, 期望 403: %s", w.Code, w.Body.String())
	}
	_ = context.TODO()
}
