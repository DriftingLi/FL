// 契约测（ADR-0062 票9）：同一批积分哨兵在三个兑换面上必须同一个码。
// seam S1（HTTP 契约层）。旧形状：/points/* 走 pointsErrStatus（积分不足/已兑换 → 400），
// 而 /real-exam/papers/:id/redeem 挂 errStatusAll(404) ⇒ 同一事件两处不同码，
// 消费端只能靠 message 文案猜语义（前端据此弹过「已解锁本卷」的成功提示）。
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestRedeemErrorParityAcrossSurfaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "redeem-parity-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token := failureTestStudentToken(t, db, cfg, "redeem_parity_stu")

	paper := model.RealExamPaper{CredentialID: 1, Title: "叉车维修真题卷一", Status: 1,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&paper).Error; err != nil {
		t.Fatalf("建真题卷失败: %v", err)
	}
	course := model.Course{Name: "液压系统", CredentialID: intPtr(1), PointsPrice: intPtr(100), Status: 1,
		CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建付费课程失败: %v", err)
	}

	// 同一学员、同样余额不足（新学员余额 0），两个兑换面必须给同一个状态码。
	recPaper := doWithToken(t, r, token, http.MethodPost, "/api/real-exam/papers/1/redeem", nil)
	recCourse := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/course/1/redeem", nil)

	if recCourse.Code != http.StatusBadRequest {
		t.Fatalf("课程兑换的积分族错误应 400（pointsErrStatus 现状）, got %d %s", recCourse.Code, recCourse.Body.String())
	}
	if recPaper.Code != recCourse.Code {
		t.Fatalf("同一事件两处不同码：real-exam=%d points=%d（%s）",
			recPaper.Code, recCourse.Code, recPaper.Body.String())
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recPaper.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if env.Message == "" {
		t.Fatal("积分不足的响应文案不得为空")
	}
}

// TestRedeemShopSurfaceSameCodeFamily 补齐第三面（商城，ADR-0062 票9 / 回记 D2）：
// 商城兑换的**业务拒绝**（sku 未登记读者 ⇒ sku 对账表 deny-by-default）也必须落 400，
// 与课程、真题卷两面同一族。三面同判据的锁若只钉两面，第四面（未来新增 sku）就会重新漂。
func TestRedeemShopSurfaceSameCodeFamily(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "redeem-parity-shop-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token := failureTestStudentToken(t, db, cfg, "redeem_parity_shop_stu")

	item := model.PointsShopItem{SKU: "unlock_parity_probe", Title: "对账探针", Price: 100, Enabled: true}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("建商城行失败: %v", err)
	}

	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/unlock_parity_probe/redeem", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未登记读者的商品须 400（与课程/真题卷同一族），got %d %s", rec.Code, rec.Body.String())
	}
	// 拒兑不得留下权益行也不得扣分：权益读面按同一对键查（写读同源由声明表保证）
	var entCnt, ledgerCnt int64
	if err := db.Model(&model.UserEntitlement{}).Where("sku = ?", "unlock_parity_probe").Count(&entCnt).Error; err != nil {
		t.Fatalf("查权益失败: %v", err)
	}
	if err := db.Model(&model.PointsLedger{}).Where("reason = ?", "redeem_unlock_parity_probe").Count(&ledgerCnt).Error; err != nil {
		t.Fatalf("查流水失败: %v", err)
	}
	if entCnt != 0 || ledgerCnt != 0 {
		t.Fatalf("拒兑后不得留权益行或扣分: entitlement=%d ledger=%d", entCnt, ledgerCnt)
	}
}
