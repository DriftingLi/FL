// Ticket 01 API 契约测试：阅读量语义拆分
//   - GET /api/featured-content/:id?no_view=1 不改变 view_count（SSR/爬虫路径）
//   - 不带 no_view 的详情请求保持计数行为
//   - POST /api/featured-content/:id/view 自增并返回最新阅读量；不存在返回 404
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// seedContractFeatured 插入一篇已发布文章，返回其 content_id。
func seedContractFeatured(t *testing.T, db *gorm.DB, title string, viewCount int) int {
	t.Helper()
	now := testutil.Now()
	item := model.FeaturedContent{
		Title:       title,
		Summary:     "摘要",
		Content:     "# 正文",
		Category:    "industry",
		Status:      1,
		ViewCount:   viewCount,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("创建精选内容失败: %v", err)
	}
	return item.ContentID
}

func TestFeaturedDetailNoViewParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	id := seedContractFeatured(t, db, "no_view 契约", 7)

	r := gin.New()
	api := r.Group("/api")
	RegisterFeaturedRoutes(api, newContractDeps(t, db, nil))

	// no_view=1：阅读量不变
	rec := performRequest(r, "GET", "/api/featured-content/"+strconv.Itoa(id)+"?no_view=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got := int(body.Data["view_count"].(float64)); got != 7 {
		t.Errorf("no_view 后 view_count = %d，期望 7", got)
	}

	// 不带参数：保持计数行为（自增）
	rec2 := performRequest(r, "GET", "/api/featured-content/"+strconv.Itoa(id))
	if rec2.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var body2 struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if got := int(body2.Data["view_count"].(float64)); got != 8 {
		t.Errorf("计数请求后 view_count = %d，期望 8", got)
	}
}

func TestFeaturedViewEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	id := seedContractFeatured(t, db, "view 端点契约", 10)

	r := gin.New()
	api := r.Group("/api")
	RegisterFeaturedRoutes(api, newContractDeps(t, db, nil))

	rec := performRequest(r, "POST", "/api/featured-content/"+strconv.Itoa(id)+"/view")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			ContentID int `json:"content_id"`
			ViewCount int `json:"view_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Data.ContentID != id || body.Data.ViewCount != 11 {
		t.Errorf("响应 = %+v，期望 content_id=%d view_count=11", body.Data, id)
	}

	// 不存在的 ID → 404
	rec404 := performRequest(r, "POST", "/api/featured-content/99999/view")
	if rec404.Code != http.StatusNotFound {
		t.Errorf("不存在的 ID 期望 404, got %d", rec404.Code)
	}
}
