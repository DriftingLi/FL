// 契约测（ADR-0062 票2 的 D1）：兑换 `unlock_real_paper` 这一行必须被拒，且**余额一分不动**。
//
// 那一行兼两个身份：① 全部卷价的价格事实源（realPaperPrice 读它，在用）② 一件「可兑换商品」
// （`POST /points/shop/{sku}/redeem` 对任意 enabled 行开放）。兑它写下的 sku="unlock_real_paper"
// 权益行没有任何读面会查（读侧只有 real_paper:{paperID} / course:{courseID}）⇒ 死端，
// 而幂等键补上主体之后每个学员都能成交这笔白扣分。
//
// 定案修法是**拒绝兑换**而不是下线商品：realPaperPrice 只读 enabled=true 的行，置 false 会让
// 卷价静默退回硬编码 300（管理员改价改的是一行读不到的数据）。本文件同时钉住「行仍在当价格源」。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

const realPaperUnlockPrice = 300

func TestPriceOnlyShopSKUCannotBeRedeemed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "shop-sku-redeem-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token, uid := shopRedeemStudent(t, db, cfg, "shop_sku_stu", 5*realPaperUnlockPrice)

	if err := db.Create(&model.PointsShopItem{SKU: "unlock_real_paper", Title: "解锁真题套卷1套",
		Price: realPaperUnlockPrice, Enabled: true}).Error; err != nil {
		t.Fatalf("建商城项失败: %v", err)
	}
	paper := model.RealExamPaper{CredentialID: 1, Title: "叉车维修真题卷一", Status: 1,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&paper).Error; err != nil {
		t.Fatalf("建真题卷失败: %v", err)
	}

	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/unlock_real_paper/redeem", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("兑价格事实源那一行必须 400（白扣分被拒）, got %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// 文案与哨兵同源（一语义一哨兵，见 CONTEXT.md「积分错误哨兵」），且必须给出路：
	// 哨兵文本自带「请到真题页按套兑换」，故不在此另抄字面量。
	if !strings.Contains(body, service.ErrRealPaperUnlockNotRedeemable.Error()) {
		t.Fatalf("响应文案必须是具名哨兵的话（含出路指引）: %s", body)
	}
	// 余额不动：只经外部契约（余额端点）问，不查库
	if got := pointsBalance(t, r, token); got != 5*realPaperUnlockPrice {
		t.Fatalf("被拒的兑换不得扣分，余额应仍是 %d, got %d", 5*realPaperUnlockPrice, got)
	}
	var entCnt int64
	if err := db.Model(&model.UserEntitlement{}).Where("user_id = ? AND sku = ?", uid, "unlock_real_paper").
		Count(&entCnt).Error; err != nil || entCnt != 0 {
		t.Fatalf("被拒的兑换不得写权益行: cnt=%d err=%v", entCnt, err)
	}

	// 该行留着继续当价格事实源：按套兑换同一价格成交（300），说明「拒兑换」没把价格读路径打断。
	if rec := doWithToken(t, r, token, http.MethodPost,
		"/api/real-exam/papers/"+strconv.Itoa(paper.PaperID)+"/redeem", nil); rec.Code != http.StatusOK {
		t.Fatalf("按套兑换真题卷应 200（价格仍取自该行）: got %d %s", rec.Code, rec.Body.String())
	}
	if got := pointsBalance(t, r, token); got != 4*realPaperUnlockPrice {
		t.Fatalf("按套兑换应扣该行声明的 300 分, got %d", got)
	}
}

func TestUndeclaredShopSKUCannotBeRedeemed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "shop-sku-undeclared-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token, _ := shopRedeemStudent(t, db, cfg, "shop_sku_undeclared_stu", 1000)

	// 表里有一行 enabled=true 的商品，但没有登记权益读者 ⇒ 放行判据（对账表）不放。
	if err := db.Create(&model.PointsShopItem{SKU: "gold_gloves", Title: "劳保手套",
		Price: 200, Enabled: true}).Error; err != nil {
		t.Fatalf("建商城项失败: %v", err)
	}
	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/gold_gloves/redeem", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未登记读者的商品必须 400（有商品行、无读者 = 死端）, got %d %s", rec.Code, rec.Body.String())
	}
	if got := pointsBalance(t, r, token); got != 1000 {
		t.Fatalf("被拒的兑换不得扣分, got %d", got)
	}
	// 真正不存在的商品仍报「商品不存在或已下架」（两语义不互相顶替）
	if rec := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/no_such_sku/redeem", nil); !strings.Contains(rec.Body.String(), service.ErrShopItemUnavailable.Error()) {
		t.Fatalf("缺行的商品仍应报商品不存在: %s", rec.Body.String())
	}
}

// ===== 夹具 =====

// shopRedeemStudent 带余额的学员 + 其 access token。
func shopRedeemStudent(t *testing.T, db *gorm.DB, cfg *config.Config, account string, balance int) (string, int) {
	t.Helper()
	pwd, err := service.HashPassword("student123")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	stu := testutil.SeedStudent(t, db, account, pwd)
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", stu.ID).
		UpdateColumn("points_balance", balance).Error; err != nil {
		t.Fatalf("预置余额失败: %v", err)
	}
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(stu.ID, stu.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token, stu.ID
}

// pointsBalance 从 GET /api/points/balance 的信封里取 data.balance（外部契约，不查库）。
func pointsBalance(t *testing.T, r *gin.Engine, token string) int {
	t.Helper()
	rec := doWithToken(t, r, token, http.MethodGet, "/api/points/balance", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("查余额应 200: got %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析余额失败: %v body=%s", err, rec.Body.String())
	}
	return env.Data.Balance
}
