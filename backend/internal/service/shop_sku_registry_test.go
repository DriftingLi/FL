// 锁（ADR-0062 票2 的 D1 结清）：商城 sku 的**写入者**（RedeemShop 建权益行）与**读者**
// （权益读面按 (user, sku, ref_id) 查）之间必须有一张具名声明表 —— 表就是那条对账。
//
// 病根（实测）：points_shop_item 的 ('unlock_real_paper', 300) 一行兼两个身份：
// ① 全部卷价的价格事实源（realPaperPrice 读它，在用）② 一件可兑换商品（POST /points/shop/{sku}/redeem
// 对任意 enabled 行开放）。兑它扣 300 分、写一条 sku="unlock_real_paper" 的权益行，而读侧只查
// real_paper:{paperID} / course:{courseID} ⇒ **没人读 = 死端**，且上一批给兑换键补了主体之后
// 每个学员都能成交这笔白扣分。
//
// 正解是拒绝兑换（**不**下线该商品：realPaperPrice 只读 enabled=true 的行，置 false 会让价格
// 退回硬编码兜底值，管理员改价改的是一行读不到的数据）。
package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// shopSeedRowRE 抓 migrations 里 `INSERT INTO points_shop_item ... VALUES ('<sku>', ...)` 的 sku。
var shopSeedRowRE = regexp.MustCompile(`(?i)INSERT\s+INTO\s+points_shop_item[\s\S]*?VALUES\s*\(\s*'([^']+)'`)

// TestShopSKUSeedRowsAreAllDeclared 每一行**种子商品**都必须在对账表里有一句「兑出去的权益谁读」
// ——「有商品行、无读者」当场判红（历史上就是这么漏出 unlock_real_paper 那笔死端的）。
// 测试库走 AutoMigrate、不执行 migrations/，所以这里读的是 SQL 文本本身。
func TestShopSKUSeedRowsAreAllDeclared(t *testing.T) {
	dir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 migrations 目录失败: %v", err)
	}
	seen := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("读 %s 失败: %v", e.Name(), err)
		}
		for _, m := range shopSeedRowRE.FindAllStringSubmatch(string(src), -1) {
			seen++
			sku := m[1]
			if _, ok := shopSKUDecls[sku]; !ok {
				t.Fatalf("%s 种子的商城商品 %q 没有对账声明（有商品行、无读者）："+
					"须在 shopSKUDecls 登记权益读者，或标为仅价格 + 具名 Blocked", e.Name(), sku)
			}
		}
	}
	if seen == 0 {
		t.Fatal("没扫到任何 points_shop_item 种子行：正则失效了，本锁等同没写")
	}
}

// TestShopSKUDeclInvariants 表自洽：没有读者的行必须给出**具名**拒绝原因，否则它会落进
// 「未登记」那条泛化哨兵里，学员看天书（同一句话该指向「请到真题页按套兑换」这种出路）。
func TestShopSKUDeclInvariants(t *testing.T) {
	for sku, decl := range shopSKUDecls {
		if decl.EntitlementSKU == nil && decl.Blocked == nil {
			t.Fatalf("商品 %q 既无权益读者也无具名拒绝原因：不可兑的行必须给出路文案", sku)
		}
		if decl.EntitlementSKU != nil && decl.Blocked != nil {
			t.Fatalf("商品 %q 同时声明了读者与拒绝原因：判据只能有一侧", sku)
		}
		if strings.TrimSpace(decl.ReadBy) == "" {
			t.Fatalf("商品 %q 未写明读者（ReadBy）：对账表的可审计性靠这一栏", sku)
		}
	}
}

// TestRedeemShopRejectsPriceOnlySKU 兑换价格事实源那一行：具名哨兵 + 余额不动 + 零权益行。
func TestRedeemShopRejectsPriceOnlySKU(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1000)
	seedShopItem(t, db, realPaperUnlockSKU, 300)

	if _, err := svc.RedeemShop(context.Background(), uid, realPaperUnlockSKU); !errors.Is(err, ErrRealPaperUnlockNotRedeemable) {
		t.Fatalf("兑价格事实源行应报 ErrRealPaperUnlockNotRedeemable, got %v", err)
	}
	if got := userBalance(t, db, uid); got != 1000 {
		t.Fatalf("被拒的兑换不得扣分，余额应仍是 1000, got %d", got)
	}
	if got := entitlementCount(t, db, uid, realPaperUnlockSKU); got != 0 {
		t.Fatalf("被拒的兑换不得写权益行, got %d", got)
	}
	if got := ledgerCount(t, db, "user_id = ?", uid); got != 0 {
		t.Fatalf("被拒的兑换不得写流水, got %d", got)
	}
	// 行本身必须留着当价格事实源（下线它 = realPaperPrice 退回硬编码兜底值）
	if price := svc.realPaperPrice(); price != 300 {
		t.Fatalf("拒绝兑换不得影响卷价（仍读该行）: got %d", price)
	}
}

// TestRedeemShopRejectsUndeclaredSKU 有商品行、无读者 ⇒ 当场拒兑（deny-by-default）：
// 放行判据来自对账表，不再来自「表里有这么一行 enabled=true」。
func TestRedeemShopRejectsUndeclaredSKU(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1000)
	seedShopItem(t, db, "gold_gloves", 200)

	if _, err := svc.RedeemShop(context.Background(), uid, "gold_gloves"); !errors.Is(err, ErrShopItemNotRedeemable) {
		t.Fatalf("未登记读者的商品应报 ErrShopItemNotRedeemable, got %v", err)
	}
	if got := userBalance(t, db, uid); got != 1000 {
		t.Fatalf("未登记的兑换不得扣分, got %d", got)
	}
}

// TestRedeemShopDeclaredSKUWritesReadableEntitlement 写读同源（这张表存在的理由）：
// 登记了读者的商品，RedeemShop 写下的权益行键形状**就是**权益读面查询的那一对键。
// 旧形状里两侧各抄一遍 sku ⇒ 抄歪一处就是一条静默死端。
func TestRedeemShopDeclaredSKUWritesReadableEntitlement(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1000)
	seedShopItem(t, db, "unlock_gold", 300)
	withShopSKUDeclForTest(t, "unlock_gold", shopSKUDecl{
		EntitlementSKU: func(refID string) string { return "gold:" + refID },
		ReadBy:         "测试读者：gold:<sku>",
	})

	res, err := svc.RedeemShop(context.Background(), uid, "unlock_gold")
	if err != nil {
		t.Fatalf("登记了读者的商品应可兑换: %v", err)
	}
	if res.SKU != "gold:unlock_gold" || res.RefID != "unlock_gold" {
		t.Fatalf("权益键应取声明表的形状, got %+v", res)
	}
	// 读侧经权益读面单点查同一对键：查得到 = 这条兑换不是死端
	entitled, err := svc.HasEntitlement(uid, "gold:unlock_gold", "unlock_gold")
	if err != nil || !entitled {
		t.Fatalf("兑换写下的权益必须被读面命中: entitled=%v err=%v", entitled, err)
	}
	if got := userBalance(t, db, uid); got != 700 {
		t.Fatalf("登记读者的正常兑换应扣价 300, got %d", got)
	}
}

// ===== 夹具 =====

func seedShopItem(t *testing.T, db *gorm.DB, sku string, price int) {
	t.Helper()
	if err := db.Create(&model.PointsShopItem{SKU: sku, Title: "测试商品 " + sku, Price: price, Enabled: true}).Error; err != nil {
		t.Fatalf("建商城项失败: %v", err)
	}
}

func entitlementCount(t *testing.T, db *gorm.DB, userID int, sku string) int64 {
	t.Helper()
	var cnt int64
	if err := db.Model(&model.UserEntitlement{}).
		Where("user_id = ? AND sku = ?", userID, sku).Count(&cnt).Error; err != nil {
		t.Fatalf("统计权益行失败: %v", err)
	}
	return cnt
}

// withShopSKUDeclForTest 测试期登记一条声明（用例结束自动回滚，别把假商品留在表里）。
func withShopSKUDeclForTest(t *testing.T, sku string, decl shopSKUDecl) {
	t.Helper()
	prev, existed := shopSKUDecls[sku]
	shopSKUDecls[sku] = decl
	t.Cleanup(func() {
		if existed {
			shopSKUDecls[sku] = prev
		} else {
			delete(shopSKUDecls, sku)
		}
	})
}
