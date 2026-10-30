// 商城 AI 专业版可兑契约（#1942 口径 B）：登记后进在售列表，兑换写下的权益行由同一读面读回。

package points

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedAiProShopItem 与 migrations/000049 同形（测试库走 AutoMigrate，不执行 migrations/）。
func seedAiProShopItem(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Create(&model.PointsShopItem{SKU: aiProSKU, Title: "AI 专业版", Price: 1000, Enabled: true}).Error; err != nil {
		t.Fatalf("建 ai_pro 商城行失败: %v", err)
	}
}

// TestShopAiProListedAndRedeemable 一件可兑商品的最小闭环：列表 → 兑换 → owned 翻转 → 重复兑被挡。
func TestShopAiProListedAndRedeemable(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1200)
	seedAiProShopItem(t, db)

	before, err := svc.ListShopItems(context.Background(), uid)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if len(before.Items) != 1 || before.Items[0].SKU != aiProSKU || before.Items[0].Owned {
		t.Fatalf("ai_pro 应是唯一在售项且未拥有, got %+v", before.Items)
	}
	if before.Items[0].Price != 1000 {
		t.Fatalf("价格取库（1000）, got %d", before.Items[0].Price)
	}

	res, err := svc.RedeemShop(context.Background(), uid, aiProSKU)
	if err != nil {
		t.Fatalf("登记了读者的 ai_pro 应可兑换: %v", err)
	}
	if res.SKU != aiProSKU || res.RefID != aiProSKU {
		t.Fatalf("权益键应为 (ai_pro, ai_pro), got %+v", res)
	}
	if got := userBalance(t, db, uid); got != 200 {
		t.Fatalf("应扣 1000 留 200, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ? AND reason = ?", uid, "redeem_"+aiProSKU); got != 1 {
		t.Fatalf("应写一条 redeem_ai_pro 流水（useAiPro 的判据）, got %d", got)
	}

	after, err := svc.ListShopItems(context.Background(), uid)
	if err != nil {
		t.Fatalf("兑换后列表失败: %v", err)
	}
	if len(after.Items) != 1 || !after.Items[0].Owned {
		t.Fatalf("兑换后本人 owned 应为 true, got %+v", after.Items)
	}

	if _, err := svc.RedeemShop(context.Background(), uid, aiProSKU); !errors.Is(err, ErrAlreadyRedeemed) {
		t.Fatalf("重复兑换应报 ErrAlreadyRedeemed, got %v", err)
	}
	if got := userBalance(t, db, uid); got != 200 {
		t.Fatalf("被拒的重复兑换不得扣分, got %d", got)
	}

	// 另一个人不复用 seedUserWithBalance：它固定 account/phone，同库二次插入会撞 hrwai_users.phone 的 UNIQUE。
	otherU := testutil.SeedStudent(t, db, "aiProOther", "pwd")
	otherList, err := svc.ListShopItems(context.Background(), otherU.ID)
	if err != nil {
		t.Fatalf("他人列表失败: %v", err)
	}
	if len(otherList.Items) != 1 || otherList.Items[0].Owned {
		t.Fatalf("他人不应拥有（按人算）, got %+v", otherList.Items)
	}
}

// TestShopAiProSeedRowPresent 迁移里的种子行与登记常量必须同一名称：
// 改了行名没改常量（或反之）就是一条「列表里看得见、兑换说不可兑」的错位。
func TestShopAiProSeedRowPresent(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "000049_shop_ai_pro_sku.up.sql")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", path, err)
	}
	if !strings.Contains(string(src), "'"+aiProSKU+"'") {
		t.Fatalf("%s 里没有 %q 种子行：登记表与迁移已错位", path, aiProSKU)
	}
}
