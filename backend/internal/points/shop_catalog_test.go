// 积分商城目录契约（#2548）：分类与积分区间筛选、目录排序、管理端读写与 SKU 登记路径。
// 走内存库 + 真实 Service 出口（不连 PG），筛选/排序/拒收判据在本机即可证伪。
package points

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedCatalog 四件可兑 + 一件下架 + 一件未登记读者的行，覆盖筛选与排序的全部对照。
func seedCatalog(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, sku := range []string{"sku_book", "sku_material", "sku_course", "sku_other"} {
		withShopSKUDeclForTest(t, sku, shopSKUDecl{
			EntitlementSKU: func(refID string) string { return "gold:" + refID },
			ReadBy:         "目录契约测试",
		})
	}
	img := "https://cdn.example.com/book.png"
	desc := "含 3 套真题与解析"
	rows := []model.PointsShopItem{
		{SKU: "sku_book", Title: "图书", Price: 300, Category: "book", SortWeight: 0, Enabled: true, ImageURL: &img, Description: &desc},
		{SKU: "sku_material", Title: "资料", Price: 800, Category: "material", SortWeight: 10, Enabled: true},
		{SKU: "sku_course", Title: "课程", Price: 1200, Category: "course", SortWeight: 5, Enabled: true},
		{SKU: "sku_other", Title: "其他", Price: 500, Category: "other", SortWeight: 0, Enabled: true},
		{SKU: "sku_off", Title: "已下架", Price: 100, Category: "book", Enabled: false},
		{SKU: "sku_unregistered", Title: "未登记读者", Price: 100, Category: "book", Enabled: true},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("建目录行失败: %v", err)
		}
	}
}

func TestListShopItemsFilterAndSort(t *testing.T) {
	svc, db := newPointsSvc(t)
	seedCatalog(t, db)

	skus := func(res *ShopItemsResult) []string {
		t.Helper()
		out := make([]string, 0, len(res.Items))
		for _, it := range res.Items {
			out = append(out, it.SKU)
		}
		return out
	}
	assertOrder := func(name string, got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	all, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{})
	if err != nil {
		t.Fatalf("全量列表失败: %v", err)
	}
	// 排序权重降序 → 同权重按价格升序；下架与未登记读者不出。
	assertOrder("全量", skus(all), "sku_material", "sku_course", "sku_book", "sku_other")
	if all.Items[2].ImageURL == nil || *all.Items[2].ImageURL == "" {
		t.Fatalf("image_url 应随列表下发, got %+v", all.Items[2])
	}
	if all.Items[2].Description == nil || *all.Items[2].Description == "" {
		t.Fatalf("description 应随列表下发, got %+v", all.Items[2])
	}

	book, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{Category: "book"})
	if err != nil {
		t.Fatalf("分类筛选失败: %v", err)
	}
	assertOrder("category=book", skus(book), "sku_book")

	material, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{Category: "material"})
	if err != nil {
		t.Fatalf("分类筛选失败: %v", err)
	}
	assertOrder("category=material", skus(material), "sku_material")

	min := 500
	atLeast, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{PointsMin: &min})
	if err != nil {
		t.Fatalf("下限筛选失败: %v", err)
	}
	assertOrder("points_min=500", skus(atLeast), "sku_material", "sku_course", "sku_other")

	max := 800
	atMost, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{PointsMax: &max})
	if err != nil {
		t.Fatalf("上限筛选失败: %v", err)
	}
	assertOrder("points_max=800", skus(atMost), "sku_material", "sku_book", "sku_other")

	lo, hi := 301, 1000
	between, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{PointsMin: &lo, PointsMax: &hi})
	if err != nil {
		t.Fatalf("区间筛选失败: %v", err)
	}
	assertOrder("points 301..1000", skus(between), "sku_material", "sku_other")

	if _, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{Category: "bogus"}); !errors.Is(err, ErrShopCategoryInvalid) {
		t.Fatalf("非法分类应报 ErrShopCategoryInvalid, got %v", err)
	}
}

// TestShopItemsHTTPFilterParams 走真实 HTTP 出口验 query 解析（curl 验收口径的最短复现）。
func TestShopItemsHTTPFilterParams(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "shopFilterHTTP", "x")
	seedCatalog(t, db)
	r, sess := newShopItemsRouter(t, db)
	token, err := sess.Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	get := func(query string) (*httptest.ResponseRecorder, ShopItemsResult) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, "/api/points/shop/items"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var out ShopItemsResult
		if w.Code == http.StatusOK {
			var env struct {
				Data ShopItemsResult `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
				t.Fatalf("解析响应失败: %v", err)
			}
			out = env.Data
		}
		return w, out
	}

	w, got := get("?category=book&points_min=200&points_max=400")
	if w.Code != http.StatusOK || len(got.Items) != 1 || got.Items[0].SKU != "sku_book" {
		t.Fatalf("三参组合应只筛出 sku_book, got %d %+v", w.Code, got.Items)
	}
	w, got = get("?category=material")
	if w.Code != http.StatusOK || len(got.Items) != 1 || got.Items[0].SKU != "sku_material" {
		t.Fatalf("category=material 应只筛出 sku_material, got %d %+v", w.Code, got.Items)
	}
	if w, _ := get("?category=bogus"); w.Code != http.StatusBadRequest {
		t.Fatalf("非法分类应 400, got %d %s", w.Code, w.Body.String())
	}
}

func TestAdminShopUpsertAndRegistrationPath(t *testing.T) {
	svc, _ := newPointsSvc(t)

	// 未登记读者的 SKU 不允许上架：否则运营会建出一行永远不出现在在售列表的商品。
	if _, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", AdminShopItemInput{
		Title: "新品", Price: 100, Enabled: true, Category: "book",
	}); !errors.Is(err, ErrShopSKUUnregistered) {
		t.Fatalf("未登记就上架应报 ErrShopSKUUnregistered, got %v", err)
	}

	// 下架态可以先建（预录入），登记读者后再上架。
	off, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", AdminShopItemInput{
		Title: "新品", Price: 100, Enabled: false, Category: "book",
	})
	if err != nil {
		t.Fatalf("下架态建行应成功: %v", err)
	}
	if off.Redeemable || off.Enabled {
		t.Fatalf("未登记 + 下架应 redeemable=false/enabled=false, got %+v", off)
	}

	withShopSKUDeclForTest(t, "sku_new", shopSKUDecl{
		EntitlementSKU: func(refID string) string { return "gold_new:" + refID },
		ReadBy:         "管理端契约测试",
	})
	img := "  "
	desc := "  运营文案  "
	on, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", AdminShopItemInput{
		Title: " 新品上架 ", Price: 100, Enabled: true, Category: "book",
		ImageURL: &img, Description: &desc, SortWeight: 7,
	})
	if err != nil {
		t.Fatalf("登记读者后上架应成功: %v", err)
	}
	if !on.Enabled || !on.Redeemable {
		t.Fatalf("登记读者后应 enabled/redeemable, got %+v", on)
	}
	if on.ImageURL != nil {
		t.Fatalf("纯空白 image_url 应归一为 NULL, got %q", *on.ImageURL)
	}
	if on.Description == nil || *on.Description != "运营文案" {
		t.Fatalf("description 应 trim 后落库, got %+v", on.Description)
	}
	if on.SortWeight != 7 {
		t.Fatalf("sort_weight 应保存 7, got %d", on.SortWeight)
	}

	// 目录列表含下架行；改价/下架是同一入口。
	all, err := svc.AdminListShopItems(context.Background())
	if err != nil || len(all.Items) != 1 {
		t.Fatalf("管理端目录应有一行, got %+v err=%v", all, err)
	}
	offAgain, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", AdminShopItemInput{
		Title: "新品下架", Price: 150, Enabled: false, Category: "book",
	})
	if err != nil {
		t.Fatalf("下架应成功: %v", err)
	}
	if offAgain.Price != 150 || offAgain.Enabled {
		t.Fatalf("改价+下架应落库, got %+v", offAgain)
	}
	list, err := svc.ListShopItems(context.Background(), 1, ShopItemFilter{})
	if err != nil || len(list.Items) != 0 {
		t.Fatalf("下架后学员列表应为空, got %+v err=%v", list, err)
	}

	// 价格事实源不能下架：下架会让 RealPaperPrice 退回硬编码兜底。
	if _, err := svc.AdminUpsertShopItem(context.Background(), realPaperUnlockSKU, AdminShopItemInput{
		Title: "解锁真题套卷1套", Price: 400, Enabled: false, Category: "material",
	}); !errors.Is(err, ErrShopPriceSourcePinned) {
		t.Fatalf("下架价格事实源应报 ErrShopPriceSourcePinned, got %v", err)
	}
	price, err := svc.AdminUpsertShopItem(context.Background(), realPaperUnlockSKU, AdminShopItemInput{
		Title: "解锁真题套卷1套", Price: 400, Enabled: true, Category: "material",
	})
	if err != nil || price.Price != 400 || price.Redeemable {
		t.Fatalf("价格事实源应可改价且 redeemable=false, got %+v err=%v", price, err)
	}
	if got := svc.RealPaperPrice(); got != 400 {
		t.Fatalf("改价后 RealPaperPrice 应读 400, got %d", got)
	}

	// 参数与分类校验。
	bad := []AdminShopItemInput{
		{Title: "", Price: 100, Enabled: false, Category: "book"},
		{Title: "x", Price: 0, Enabled: false, Category: "book"},
		{Title: "x", Price: 100, Enabled: false, Category: "bogus"},
	}
	for _, in := range bad {
		if _, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", in); !errors.Is(err, ErrShopItemInvalid) && !errors.Is(err, ErrShopCategoryInvalid) {
			t.Fatalf("非法参数应被拒, in=%+v got %v", in, err)
		}
	}
	neg := -1
	if _, err := svc.AdminUpsertShopItem(context.Background(), "sku_new", AdminShopItemInput{
		Title: "x", Price: 100, Stock: &neg, Enabled: false, Category: "book",
	}); !errors.Is(err, ErrShopItemInvalid) {
		t.Fatalf("负库存应报 ErrShopItemInvalid, got %v", err)
	}
}
