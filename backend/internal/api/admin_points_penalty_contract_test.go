// #1098 管理员扣罚端点契约：
//   - 目标学员不存在 → 404（pointsErrStatus 域表，不再压成 400）；
//   - 成功路径：响应形状 {"deducted": N} 不变，且扣罚流水与站内信同事务落库。
package api

import (
	"encoding/json"
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

// TestAdminPenaltyContract 域表映射（404）+ 同事务发信（成功路径）。
func TestAdminPenaltyContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "penalty_admin", adminPwd)
	stuPwd, _ := service.HashPassword("student123")
	// 直接建学员行而不走 testutil.SeedStudent：后者会推进进程级 uid 计数器，
	// 使 auth_me 契约测试（在 .001 上锁形状）随本测试文件的存在而漂移。
	student := &model.HrwaiUser{
		UID: 9000000000000000001, Account: "acct_penalty_stu", Username: "penalty_stu",
		Password: stuPwd, Phone: "test_penalty_stu", Status: 1, PointsBalance: 50, CreatedAt: testutil.Now(),
	}
	if err := db.Create(student).Error; err != nil {
		t.Fatalf("建测试学员失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "penalty-contract-secret"}
	deps := newContractDeps(t, db, cfg)
	r := gin.New()
	api := r.Group("/api")
	RegisterAdminPointsRoutes(api, deps.RouterDeps(), deps.PointsSvc)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, err := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发管理员 token 失败: %v", err)
	}

	// 1. 学员不存在 → 404（域表；旧实现一律 400）
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/points/penalty",
		map[string]any{"user_id": 999999, "delta": 10, "reason": "违规"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("扣罚目标不存在应 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "用户不存在") {
		t.Fatalf("404 文案应保留「用户不存在」, got %s", rec.Body.String())
	}

	// 2. 成功：扣 20 → {"deducted":20}；流水 + 站内信同事务落库
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/points/penalty",
		map[string]any{"user_id": student.ID, "delta": 20, "reason": "违规操作"})
	if rec.Code != http.StatusOK {
		t.Fatalf("扣罚应 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			Deducted int `json:"deducted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析信封失败: %v", err)
	}
	if env.Code != http.StatusOK || env.Data.Deducted != 20 {
		t.Fatalf("响应应为 ⟨200, deducted=20⟩, got %+v", env)
	}
	var ledgerCount int64
	if err := db.Model(&model.PointsLedger{}).
		Where("user_id = ? AND reason = ?", student.ID, "admin_penalty").Count(&ledgerCount).Error; err != nil {
		t.Fatalf("统计扣罚流水失败: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("扣罚流水应恰一行, got %d", ledgerCount)
	}
	var notify model.Notification
	if err := db.Where("user_id = ?", student.ID).First(&notify).Error; err != nil {
		t.Fatalf("同事务站内信应存在: %v", err)
	}
	if notify.Title != "积分扣罚" || notify.Content != "您的积分因“违规操作”被扣除 20 分" {
		t.Fatalf("站内信口径不符: %+v", notify)
	}
}
