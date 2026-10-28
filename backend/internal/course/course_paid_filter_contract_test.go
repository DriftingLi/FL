// 免费/付费筛选契约（#2071）：判据 = 积分兑换价 points_price，**不是**现金价；
// 省略不过滤、true = 付费（价 > 0）、false = 免费（价为空或 ≤ 0）、非法值 400。

package course

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestCoursePaidFilterContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	ptr := func(v int) *int { return &v }

	spec := model.Specialty{Code: "paid-spec", Name: "免费付费", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "paid-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	seed := func(name string, price *int) {
		t.Helper()
		c := model.Course{Name: name, Status: 1, PointsPrice: price,
			SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("创建课程失败: %v", err)
		}
	}
	// 本用例跑真 PG：迁移 000002 的 CHECK（IS NULL OR > 0）拒 0，零价课播不进去 ⇒ 另在
	// TestCoursePaidFilterTreatsZeroPriceAsFree 里用 AutoMigrate 库（无该 CHECK）覆盖「0 算免费」。
	seed("免费课", nil)
	seed("付费课", ptr(50))

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := newCourseContractRouter(t, db, cfg)

	list := func(query string) CoursePageResult {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, "/api/courses"+query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应 200, got %d %s", query, w.Code, w.Body.String())
		}
		var env struct {
			Data CoursePageResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		return env.Data
	}
	names := func(res CoursePageResult) map[string]bool {
		out := map[string]bool{}
		for _, c := range res.Courses {
			out[c.Name] = true
		}
		return out
	}

	// 迁移基线自带种子课程，断言不能靠总数，只能靠「两档互相排斥 + 逐项满足判据」。
	all := list("")
	paidRes := list("?paid=true")
	freeRes := list("?paid=false")
	paid := names(paidRes)
	free := names(freeRes)

	if !paid["付费课"] || paid["免费课"] {
		t.Fatalf("paid=true 应含付费课且不含免费课, got %+v", paid)
	}
	if !free["免费课"] || free["付费课"] {
		t.Fatalf("paid=false 应含免费课且不含付费课, got %+v", free)
	}
	for _, c := range paidRes.Courses {
		if c.PointsPrice == nil || *c.PointsPrice <= 0 {
			t.Fatalf("paid=true 返回了非付费课程: %s points_price=%v", c.Name, c.PointsPrice)
		}
	}
	for _, c := range freeRes.Courses {
		if c.PointsPrice != nil && *c.PointsPrice > 0 {
			t.Fatalf("paid=false 返回了付费课程: %s points_price=%v", c.Name, *c.PointsPrice)
		}
	}

	// 两档互斥且并集 = 未过滤全集（无其他筛选时）
	if paidRes.Total+freeRes.Total != all.Total {
		t.Fatalf("两档并集应等于全集: paid=%d free=%d total=%d", paidRes.Total, freeRes.Total, all.Total)
	}

	// 与既有筛选组合（#2071 AC3）：paid 与 specialty_id 同时生效
	mixed := list(fmt.Sprintf("?paid=false&specialty_id=%d", spec.SpecialtyID))
	if mixed.Total != 1 || len(mixed.Courses) != 1 || mixed.Courses[0].Name != "免费课" {
		t.Fatalf("paid=false 与 specialty_id 组合应只返回本方向的免费课, got %+v", mixed.Courses)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/courses?paid=maybe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 paid 应 400, got %d %s", w.Code, w.Body.String())
	}
}

// 零价课在「没有那条 CHECK」的库上（AutoMigrate 测试库）也必须算免费——#2071 的 AC 是「为空或 0」。
// 真 PG 上这条数据插不进去（000002 的 CHECK 拒 0），所以 <= 0 那一支只能在这里覆盖。
func TestCoursePaidFilterTreatsZeroPriceAsFree(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	ptr := func(v int) *int { return &v }

	spec := model.Specialty{Code: "zero-spec", Name: "零价", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "zero-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	for _, tc := range []struct {
		name  string
		price *int
	}{{"免费课", nil}, {"零价课", ptr(0)}, {"付费课", ptr(50)}} {
		c := model.Course{Name: tc.name, Status: 1, PointsPrice: tc.price,
			SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("创建课程失败: %v", err)
		}
	}

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := newCourseContractRouter(t, db, cfg)
	list := func(query string) map[string]bool {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, "/api/courses"+query, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应 200, got %d %s", query, w.Code, w.Body.String())
		}
		var env struct {
			Data CoursePageResult `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		out := map[string]bool{}
		for _, c := range env.Data.Courses {
			out[c.Name] = true
		}
		return out
	}

	free := list("?paid=false")
	if !free["免费课"] || !free["零价课"] || free["付费课"] {
		t.Fatalf("paid=false 应含免费课与零价课、不含付费课, got %+v", free)
	}
	paid := list("?paid=true")
	if !paid["付费课"] || paid["零价课"] || paid["免费课"] {
		t.Fatalf("paid=true 应只含付费课, got %+v", paid)
	}
}
