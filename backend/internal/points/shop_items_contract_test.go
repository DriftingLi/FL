// 商城在售列表契约（#2064）：在售判据与兑换同源、已拥有按人算、未登录 401。

package points

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// newShopItemsRouter 自装学员端 /api/points 面（只挂 RegisterRoutes，不走装配根）。
func newShopItemsRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *security.Session) {
	t.Helper()
	testutil.SetTestGinMode()
	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	sess := security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	svc := NewService(db, zap.NewNop(), clock.Real(), notification.NewService(db, zap.NewNop()))
	r := gin.New()
	RegisterRoutes(r.Group("/api"), sess, svc)
	return r, sess
}

// seedShopItems 三行：可兑且在售 / 已下架 / 在售但未登记权益读者。
func seedShopItems(t *testing.T, db *gorm.DB) {
	t.Helper()
	withShopSKUDeclForTest(t, "unlock_gold", shopSKUDecl{
		EntitlementSKU: func(refID string) string { return "gold:" + refID },
		ReadBy:         "契约测试",
	})
	stock := 5
	rows := []model.PointsShopItem{
		{SKU: "unlock_gold", Title: "金牌解锁", Price: 100, Stock: &stock, Enabled: true},
		{SKU: "unlock_silver", Title: "下架商品", Price: 50, Enabled: false},
		{SKU: "unlock_mystery", Title: "未登记读者", Price: 10, Enabled: true},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("建商城行失败: %v", err)
		}
	}
}

func TestShopItemsListContract(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	buyer := testutil.SeedStudent(t, db, "shopBuyer", "pwd")
	other := testutil.SeedStudent(t, db, "shopOther", "pwd")
	seedShopItems(t, db)
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", buyer.ID).Update("points_balance", 500).Error; err != nil {
		t.Fatalf("设余额失败: %v", err)
	}

	r, sess := newShopItemsRouter(t, db)
	token, err := sess.Issue(int(buyer.ID), buyer.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	do := func(tok string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodGet, "/api/points/shop/items", nil)
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 未登录 401（端点挂在 JWTAuth 组内）
	if w := do(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应 401, got %d %s", w.Code, w.Body.String())
	}

	decode := func(w *httptest.ResponseRecorder) ShopItemsResult {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("应 200, got %d %s", w.Code, w.Body.String())
		}
		var env struct {
			Data ShopItemsResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		return env.Data
	}

	before := decode(do(token))
	if len(before.Items) != 1 || before.Items[0].SKU != "unlock_gold" {
		t.Fatalf("在售列表应只有 unlock_gold（下架与未登记读者的行都不出），got %+v", before.Items)
	}
	if before.Items[0].Owned {
		t.Fatalf("未兑换前 owned 应为 false")
	}
	if before.Items[0].Stock == nil || *before.Items[0].Stock != 5 {
		t.Fatalf("库存应回 5, got %+v", before.Items[0].Stock)
	}

	// 兑换后：同一人 owned=true；另一人仍 false（已拥有按人算）
	svc := NewService(db, zap.NewNop(), clock.Real(), notification.NewService(db, zap.NewNop()))
	if _, err := svc.RedeemShop(context.Background(), buyer.ID, "unlock_gold"); err != nil {
		t.Fatalf("兑换失败: %v", err)
	}
	after := decode(do(token))
	if len(after.Items) != 1 || !after.Items[0].Owned {
		t.Fatalf("兑换后本人 owned 应为 true, got %+v", after.Items)
	}
	res, err := svc.ListShopItems(context.Background(), other.ID)
	if err != nil {
		t.Fatalf("他人列表失败: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].Owned {
		t.Fatalf("他人不应看到已拥有, got %+v", res.Items)
	}
}
