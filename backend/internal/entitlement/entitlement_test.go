package entitlement

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestSKUShapes sku 词汇表单点：两条 sku 的字面形状是**写入侧与读取侧的契约**，
// 形状一分家就是 unlock_real_paper 那笔死端的形态（写了没人读），故逐字钉住。
func TestSKUShapes(t *testing.T) {
	t.Parallel()
	if got := CourseSKU(7); got != "course:7" {
		t.Fatalf("CourseSKU(7) = %q，期望 \"course:7\"", got)
	}
	if got := RealPaperSKU(12); got != "real_paper:12" {
		t.Fatalf("RealPaperSKU(12) = %q，期望 \"real_paper:12\"", got)
	}
}

// TestHoldsTriState 权益判据三态：写过 ⇒ true；没写过 ⇒ false；别人写过 ⇒ false，
// 且 (主体, sku, ref_id) 三元组任一不同都不得命中。
func TestHoldsTriState(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	if err := db.Create(&model.UserEntitlement{UserID: 11, SKU: CourseSKU(3), RefID: "3"}).Error; err != nil {
		t.Fatalf("播种权益失败: %v", err)
	}
	if got, err := Holds(db, 11, CourseSKU(3), "3"); err != nil || !got {
		t.Fatalf("本人已兑换应回 (true, nil)，实际 (%v, %v)", got, err)
	}
	if got, err := Holds(db, 12, CourseSKU(3), "3"); err != nil || got {
		t.Fatalf("他人未兑换应回 (false, nil)，实际 (%v, %v)", got, err)
	}
	if got, err := Holds(db, 11, CourseSKU(4), "4"); err != nil || got {
		t.Fatalf("换了 sku/ref_id 不应命中（按套粒度），实际 (%v, %v)", got, err)
	}
}

// TestHoldsPropagatesQueryError 「查不动不得被读成没兑换过」（ADR-0062 票6）。
// 反向那一半才是承重的一半：回 (false, nil) 会让已付费的学员在自己买过的内容前被拒。
func TestHoldsPropagatesQueryError(t *testing.T) {
	t.Parallel()
	db := testutil.NewMemoryDB(t)
	if err := db.Exec("DROP TABLE user_entitlement").Error; err != nil {
		t.Fatalf("删表失败: %v", err)
	}
	got, err := Holds(db, 11, CourseSKU(3), "3")
	if err == nil {
		t.Fatalf("权益表都读不到却回 (false, nil)：DB 抖动被读成「没兑换过」")
	}
	if got {
		t.Fatalf("查不动却回 true")
	}
}
