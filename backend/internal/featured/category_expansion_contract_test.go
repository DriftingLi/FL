// 内容精选分类扩两态契约（#2063）：考情 / 公告可选、可筛、标签正确；旧四态与非法值行为不变。

package featured

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

func TestFeaturedCategoryExpansionContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	svc := NewService(db, nil, zap.NewNop())

	// 字典：六态齐、标签正确、旧四态一字不动
	for cat, want := range map[string]string{
		"company": "公司动态", "industry": "行业新闻", "product": "产品资讯", "news": "政策法规",
		"exam": "考情", "notice": "公告",
	} {
		if got := svc.CategoryLabel(cat); got != want {
			t.Fatalf("分类 %s 标签应为 %s, got %s", cat, want, got)
		}
		if !svc.IsValidCategory(cat) {
			t.Fatalf("分类 %s 应合法", cat)
		}
	}
	if svc.IsValidCategory("nope") {
		t.Fatal("未知分类不应合法")
	}

	// 建两篇新类别内容（一篇考情、一篇公告）
	ids := make([]int, 0, 2)
	for _, cat := range []string{"exam", "notice"} {
		created, err := svc.Create(FeaturedContentInput{
			Title: cat + " 内容", Category: cat, Summary: "摘要", Content: "# 正文",
		})
		if err != nil {
			t.Fatalf("创建 %s 内容失败: %v", cat, err)
		}
		ids = append(ids, created.ContentID)
	}
	// 非法类别仍拒
	if _, err := svc.Create(FeaturedContentInput{Title: "非法", Category: "nope"}); !errors.Is(err, ErrFeaturedCategoryInvalid) {
		t.Fatalf("非法分类应返回 ErrFeaturedCategoryInvalid, got %v", err)
	}

	// 发布走域出口：新类别同样要能发布（不是只测「列表能筛」）
	for _, id := range ids {
		if _, err := svc.Publish(id, nil); err != nil {
			t.Fatalf("发布 %d 失败: %v", id, err)
		}
	}
	res, err := svc.GetPublicList(1, 10, "exam")
	if err != nil {
		t.Fatalf("按 exam 取列表失败: %v", err)
	}
	if res.Total != 1 || len(res.Items) != 1 || res.Items[0].CategoryLabel != "考情" {
		t.Fatalf("按 exam 过滤应命中 1 条且标签为考情, got %+v", res.Items)
	}

	// HTTP 面：公开列表按类别筛 + 标签外发
	r := newFeaturedContractEnv(t, db)
	req, _ := http.NewRequest(http.MethodGet, "/api/featured-contents?category=notice", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("公开列表应 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data FeaturedContentPageResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if env.Data.Total != 1 || len(env.Data.Items) != 1 || env.Data.Items[0].CategoryLabel != "公告" {
		t.Fatalf("按 notice 过滤应命中 1 条且标签为公告, got %+v", env.Data.Items)
	}
}
