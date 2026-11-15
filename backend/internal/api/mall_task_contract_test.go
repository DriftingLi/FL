// 契约测试 #2550：任务中心补「游览商城」「完成积分兑礼」两项。
//
// 守的边界：
//   - browse_mall：上报一次 recorded=true、同日同商品重复 recorded=false；按日去重后
//     不同商品数达 3 ⇒ claimable/3/3；第四件不把进度顶过 3；领取 +5、重复领取 400；
//     未上架 / 未登记读者的 sku 返回 404 且不落事实行。
//   - exchange_gift：三条兑换路径共用 redeem 单点并写 reason=redeem_* 的 points_ledger
//     流水（ADR-0023 / ADR-0041）⇒ 真实兑换后任务 claimable/1/1；领取 +10、重复领取 400；
//     非 redeem_* 流水（ai_tokens）不算。
//   - 跨日：昨日游览 / 昨日兑礼不计入今日。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func assertMallTaskContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	testutil.SetTestGinMode()

	// 生产面 10 行种子（PG 由迁移建出）＋本票新增的两行（SQLite 面补种，两侧断言同一份事实）
	seedPointsTaskConfigs(t, db)
	mustCreate(t, db, &model.PointsTaskConfig{
		Code: "browse_mall", Title: "游览商城", Group: "daily", Points: 5, DailyLimit: 1,
		EventType: "mall_browse", Description: "去商城游览 3 个商品",
	})
	mustCreate(t, db, &model.PointsTaskConfig{
		Code: "exchange_gift", Title: "完成积分兑礼", Group: "daily", Points: 10, DailyLimit: 1,
		EventType: "gift_redeem", Description: "完成 1 次积分兑礼",
	})

	// 商品目录：ai_pro 是可兑商品（对账表已登记）；unlock_real_paper 是价格事实源（Blocked）；
	// B/C/D 只为验证「按日去重不同 SKU」的进度计数（目录扩容前可兑商品只有 ai_pro 一件，
	// 事实行直接构造，端点侧的真实上报已用 ai_pro 验过）。
	items := []model.PointsShopItem{
		{SKU: "ai_pro", Title: "AI 专业版", Price: 1000, Enabled: true},
		{SKU: "unlock_real_paper", Title: "解锁真题套卷1套", Price: 300, Enabled: true},
		{SKU: "sku_mall_b", Title: "商品B", Price: 10, Enabled: true},
		{SKU: "sku_mall_c", Title: "商品C", Price: 10, Enabled: true},
		{SKU: "sku_mall_d", Title: "商品D", Price: 10, Enabled: true},
	}
	for i := range items {
		mustCreate(t, db, &items[i])
	}

	pwd, _ := core.HashPassword("student123")
	stu := seedStudent(t, db, "stuMall", pwd)
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", stu.ID).Update("points_balance", 2000).Error; err != nil {
		t.Fatalf("设置余额失败: %v", err)
	}

	today := clock.DayKey(clock.Now())
	yesterday := clock.DayKey(clock.Now().AddDate(0, 0, -1))
	yesterdayStart := clock.DayStart(clock.Now().AddDate(0, 0, -1))

	// 跨日探针：昨天的游览/兑礼都不该计入今天。
	mustCreate(t, db, &model.MallView{UserID: int(stu.ID), ViewDate: yesterday, SKU: "ai_pro", CreatedAt: testutil.Now()})
	mustCreate(t, db, &model.PointsLedger{
		UserID: int(stu.ID), Delta: -1, Reason: "redeem_course", RefType: "course",
		RefID: "999", CreatedAt: yesterdayStart,
	})

	cfg := &config.Config{JWTSecretKey: "mall-task-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	points.RegisterRoutes(apiGroup, deps.RouterDeps().Session, deps.PointsSvc)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(stu.ID), stu.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	report := func(sku string) (int, bool) {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/"+sku+"/browse", nil)
		if rec.Code != http.StatusOK {
			return rec.Code, false
		}
		var env struct {
			Data struct {
				Recorded bool `json:"recorded"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		return rec.Code, env.Data.Recorded
	}
	claim := func(code string) *httptest.ResponseRecorder {
		return doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/"+code+"/claim", nil)
	}

	// ===== 1. 初始：两项都 todo；昨日事实不计入今日 =====
	row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_mall")
	if row.Status != "todo" || row.Progress != 0 || row.Total != 3 {
		t.Fatalf("未游览应是 todo/0/3, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	if row.Points != 5 || row.Title != "游览商城" {
		t.Fatalf("分值与文案应与配置一致（5 分 / 游览商城）, got %d 分 / %q", row.Points, row.Title)
	}
	row = browseOf(t, fetchBrowseTasks(t, r, token), "exchange_gift")
	if row.Status != "todo" || row.Progress != 0 || row.Total != 1 {
		t.Fatalf("昨日兑礼不该计入今日：应是 todo/0/1, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	if row.Points != 10 || row.Title != "完成积分兑礼" {
		t.Fatalf("分值与文案应与配置一致（10 分 / 完成积分兑礼）, got %d 分 / %q", row.Points, row.Title)
	}

	// ===== 2. 游览上报：首次 true、同日重复 false；未上架/未登记 404 且不落行 =====
	if code, recorded := report("ai_pro"); code != http.StatusOK || !recorded {
		t.Fatalf("首次上报应 200/recorded=true, got %d/%v", code, recorded)
	}
	if code, recorded := report("ai_pro"); code != http.StatusOK || recorded {
		t.Fatalf("同日重复上报应 200/recorded=false（幂等，不是错误）, got %d/%v", code, recorded)
	}
	for _, sku := range []string{"no_such_sku", "unlock_real_paper"} {
		if code, _ := report(sku); code != http.StatusNotFound {
			t.Fatalf("不可浏览的 sku %s 应 404, got %d", sku, code)
		}
	}
	var viewRows int64
	db.Model(&model.MallView{}).Where("user_id = ? AND view_date = ?", stu.ID, today).Count(&viewRows)
	if viewRows != 1 {
		t.Fatalf("同日同商品只该有一行、不可浏览的 sku 不得落行, got %d", viewRows)
	}
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_mall"); row.Progress != 1 || row.Status != "todo" {
		t.Fatalf("游览 1 件应是 todo/1/3, got %s/%d", row.Status, row.Progress)
	}

	// ===== 3. 三件不同商品 ⇒ claimable；第四件不把进度顶过 3 =====
	mustCreate(t, db, &model.MallView{UserID: int(stu.ID), ViewDate: today, SKU: "sku_mall_b", CreatedAt: testutil.Now()})
	mustCreate(t, db, &model.MallView{UserID: int(stu.ID), ViewDate: today, SKU: "sku_mall_c", CreatedAt: testutil.Now()})
	row = browseOf(t, fetchBrowseTasks(t, r, token), "browse_mall")
	if row.Status != "claimable" || row.Progress != 3 {
		t.Fatalf("游览 3 件应 claimable/3/3, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	mustCreate(t, db, &model.MallView{UserID: int(stu.ID), ViewDate: today, SKU: "sku_mall_d", CreatedAt: testutil.Now()})
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_mall"); row.Progress != 3 {
		t.Fatalf("进度封顶 3/3（与浏览课程同口径）, got %d", row.Progress)
	}

	// ===== 4. 领取 +5；重复领取 400 =====
	if rec := claim("browse_mall"); rec.Code != http.StatusOK {
		t.Fatalf("达成后领取应 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := claim("browse_mall"); rec.Code != http.StatusBadRequest {
		t.Fatalf("重复领取应 400, got %d", rec.Code)
	}

	// ===== 5. 非 redeem_* 流水不算兑礼 =====
	mustCreate(t, db, &model.PointsLedger{
		UserID: int(stu.ID), Delta: -1, Reason: "ai_tokens", RefType: "ai", RefID: "req-1", CreatedAt: testutil.Now(),
	})
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "exchange_gift"); row.Status != "todo" {
		t.Fatalf("ai_tokens 流水不该算完成兑礼, got %s", row.Status)
	}

	// ===== 6. 真实兑换（redeem 单点）写 redeem_* 流水 ⇒ exchange_gift claimable/1/1 =====
	if _, err := deps.PointsSvc.RedeemShop(context.Background(), int(stu.ID), "ai_pro"); err != nil {
		t.Fatalf("兑换 ai_pro 失败: %v", err)
	}
	row = browseOf(t, fetchBrowseTasks(t, r, token), "exchange_gift")
	if row.Status != "claimable" || row.Progress != 1 || row.Total != 1 {
		t.Fatalf("兑礼后应 claimable/1/1, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	if rec := claim("exchange_gift"); rec.Code != http.StatusOK {
		t.Fatalf("兑礼后领取应 200, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := claim("exchange_gift"); rec.Code != http.StatusBadRequest {
		t.Fatalf("重复领取应 400, got %d", rec.Code)
	}

	// ===== 7. 领取流水 reason = taskCode（不是 redeem_*，不会自触发下一次兑礼）=====
	var selfTrigger int64
	db.Model(&model.PointsLedger{}).
		Where("user_id = ? AND reason = ? AND created_at >= ?", stu.ID, "exchange_gift", clock.DayStart(clock.Now())).
		Count(&selfTrigger)
	if selfTrigger != 1 {
		t.Fatalf("领取应落一条 reason=exchange_gift 的流水, got %d", selfTrigger)
	}
}

func TestMallTaskContract_OnSqlite(t *testing.T) {
	assertMallTaskContract(t, testutil.NewMemoryDB(t))
}

func TestMallTaskContract_OnPostgres(t *testing.T) {
	assertMallTaskContract(t, testutil.NewPostgresDB(t))
}
