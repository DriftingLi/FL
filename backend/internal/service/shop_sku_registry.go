// Package service 实现业务服务层。
// 本文件：商城 sku 的「写入者 ↔ 读者」对账单点（ADR-0062 票2 的 D1 结清）。
//
// 病根：points_shop_item 的一行同时可以是一件可兑换商品（`POST /points/shop/{sku}/redeem`
// 对任意 enabled 行开放）与一份别的事实（价格、文案…），而**没有任何东西**记录
// 「兑出去的那条权益行谁来读」。于是 ('unlock_real_paper', 300) 那一行既当**全部卷价的事实源**
// （realPaperPrice 读它，在用）又当商品：兑它扣 300 分、写一条 sku="unlock_real_paper" 的权益行，
// 读侧却只查 real_paper:{paperID} / course:{courseID} ⇒ 没人读 = 死端，
// 且幂等键补上主体后**每个学员都能成交这笔白扣分**。
//
// 本表把那件事升级为具名声明：一行 sku 必须显式回答「兑出去的权益按什么键、被谁读」；
// 答不上即**不可兑换**（deny-by-default）。RedeemShop 的放行判据与权益键形状同取此处，
// 写侧与读侧不再各抄一遍 sku。锁见 shop_sku_registry_test.go（种子行未登记即红）。
//
// 注意：**不要靠把行置 enabled=false 来停售价格事实源**——realPaperPrice 只读 enabled=true 的行，
// 置 false 会让全部卷价静默退回硬编码兜底值，管理员改价改的是一行读不到的数据。
package service

import "errors"

// 商城兑换拒绝的具名哨兵（CONTEXT.md「积分错误哨兵」：一语义一哨兵，handler 以 errors.Is 映射码）。
var (
	// ErrRealPaperUnlockNotRedeemable 「解锁真题套卷」只是价格事实源，不是一件可兑商品：
	// 真题按套兑换有它自己的入口（POST /real-exam/papers/{paper_id}/redeem，写 real_paper:{id} 且有人读）。
	ErrRealPaperUnlockNotRedeemable = errors.New("解锁真题不单独兑换，请到真题页按套兑换该卷")
	// ErrShopItemNotRedeemable 商城里有这一行、但没登记权益读者：兑出去就是一条没人读的权益行。
	ErrShopItemNotRedeemable = errors.New("该商品暂不可兑换")
)

// shopSKUDecl 一行商城 sku 的具名声明。
type shopSKUDecl struct {
	// EntitlementSKU 兑换写下的权益行 sku 值（入参为权益行 ref_id，商城兑换取 sku 本身）。
	// **权益读面必须按同一对键查**（见 holdsEntitlement 与 CourseSKU / RealPaperSKU 的先例）。
	// nil = 该商品不产生任何可被读取的权益 ⇒ 不可兑换，Blocked 必填。
	EntitlementSKU func(refID string) string
	// ReadBy 这条权益由谁读（人读定位；EntitlementSKU 为 nil 时写「没有读者」的原因与出路）。
	ReadBy string
	// Blocked EntitlementSKU 为 nil 时给调用方的具名哨兵（给出路，不给「未知错误」）。
	Blocked error
}

// shopSKUDecls 商城 sku 声明表（key = points_shop_item.sku）。新增商品必须在这里登记读者。
var shopSKUDecls = map[string]shopSKUDecl{
	realPaperUnlockSKU: {
		EntitlementSKU: nil,
		ReadBy: "无读者：本行只是**全部卷价的事实源**（realPaperPrice 读 price 列）。按套兑换走 " +
			"POST /real-exam/papers/{paper_id}/redeem，写 real_paper:{paper_id} 权益，由真题读面查",
		Blocked: ErrRealPaperUnlockNotRedeemable,
	},
}

// shopRedeemEntitlementKey 兑换的放行判据 + 权益键形状（同一个函数，故两侧不可能不同源）。
// 返回 err = 拒兑（具名哨兵），调用方不得扣分。
func shopRedeemEntitlementKey(sku string) (entitlementSKU, refID string, err error) {
	decl, declared := shopSKUDecls[sku]
	if !declared {
		return "", "", ErrShopItemNotRedeemable
	}
	if decl.EntitlementSKU == nil {
		return "", "", decl.Blocked
	}
	return decl.EntitlementSKU(sku), sku, nil
}
