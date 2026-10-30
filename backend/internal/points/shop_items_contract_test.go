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

// findShopItem 按 sku 取列表项：#1942 登记 ai_pro（含 migrations/000049 种子行）后，
// 在售列表不再「恒一行」，判据改按 sku 取，别再绑下标。
func findShopItem(items []ShopItemDTO, sku string) (ShopItemDTO, bool) {
	for _, it := range items {
		if it.SKU == sku {
			return it, true
		}
	}
	return ShopItemDTO{}, false
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
	// 在售面 = 登记的测试行 unlock_gold + 迁移种子的 ai_pro（#1942）；下架与未登记读者的行仍不出。
	if len(before.Items) != 2 {
		t.Fatalf("在售列表应为 unlock_gold 与 ai_pro 两行（下架与未登记读者的行都不出），got %+v", before.Items)
	}
	gold, ok := findShopItem(before.Items, "unlock_gold")
	if !ok {
		t.Fatalf("在售列表缺 unlock_gold, got %+v", before.Items)
	}
	if gold.Owned {
		t.Fatalf("未兑换前 owned 应为 false")
	}
	if gold.Stock == nil || *gold.Stock != 5 {
		t.Fatalf("库存应回 5, got %+v", gold.Stock)
	}
	if _, bad := findShopItem(before.Items, "unlock_silver"); bad {
		t.Fatalf("下架行不得出现在列表, got %+v", before.Items)
	}
	if _, bad := findShopItem(before.Items, "unlock_mystery"); bad {
		t.Fatalf("未登记权益读者的行不得出现在列表, got %+v", before.Items)
	}

	// 兑换后：同一人 owned=true；另一人仍 false（已拥有按人算）
	svc := NewService(db, zap.NewNop(), clock.Real(), notification.NewService(db, zap.NewNop()))
	if _, err := svc.RedeemShop(context.Background(), buyer.ID, "unlock_gold"); err != nil {
		t.Fatalf("兑换失败: %v", err)
	}
	after := decode(do(token))
	goldAfter, ok := findShopItem(after.Items, "unlock_gold")
	if !ok || !goldAfter.Owned {
		t.Fatalf("兑换后本人 owned 应为 true, got %+v", after.Items)
	}
	if ai, ok := findShopItem(after.Items, "ai_pro"); ok && ai.Owned {
		t.Fatalf("兑换 unlock_gold 不得让 ai_pro 也变已拥有（按 sku 归位）, got %+v", after.Items)
	}
	// 反证（#2064 AC2）：权益行若按「裸 sku」形态存在（不是兑换写下的权益键 gold:<ref>），
	// 不得被判成已拥有——否则键形态一漂，列表就会把没买过的人标成已拥有。
	if err := db.Create(&model.UserEntitlement{
		UserID: other.ID, SKU: "unlock_gold", RefID: "unlock_gold", CreatedAt: testutil.Now(),
	}).Error; err != nil {
		t.Fatalf("插裸键权益失败: %v", err)
	}
	res, err := svc.ListShopItems(context.Background(), other.ID)
	if err != nil {
		t.Fatalf("他人列表失败: %v", err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("他人在售列表同样应为两行, got %+v", res.Items)
	}
	if otherGold, ok := findShopItem(res.Items, "unlock_gold"); !ok || otherGold.Owned {
		t.Fatalf("他人不应看到已拥有（裸 sku 权益也不算）, got %+v", res.Items)
	}
}
