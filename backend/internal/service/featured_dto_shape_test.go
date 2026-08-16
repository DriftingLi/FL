package service

import (
	"encoding/json"
	"sort"
	"testing"
)

// jsonTopLevelKeys 返回对象 JSON 的顶层 key 集合（保持 map 序列化的字节序无关）。
func jsonTopLevelKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertKeys(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("key 数量 = %d, 期望 %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("key[%d] = %q, 期望 %q\ngot:  %v\nwant: %v", i, got[i], want[i], got, want)
		}
	}
}

// TestFeaturedDTOShapes 锁定 featured typed DTO 的顶层 JSON key 集合，
// 与旧 map 契约逐字一致（ADR-0009 / ADR-0015）。
func TestFeaturedDTOShapes(t *testing.T) {
	pub := "2026-08-16 12:00:00"

	listWant := []string{
		"category", "category_label", "content_id", "cover_image", "created_at",
		"published_at", "sort_order", "source", "status", "summary", "title", "updated_at", "view_count",
	}
	assertKeys(t, jsonTopLevelKeys(t, FeaturedContentDTO{
		Category: "news", CategoryLabel: "资讯", ContentID: 1, CoverImage: "cover.png",
		CreatedAt: pub, PublishedAt: &pub, SortOrder: 1, Source: "src", Status: 1,
		Summary: "summary", Title: "title", UpdatedAt: pub, ViewCount: 2,
	}), listWant)

	detailWant := []string{
		"category", "category_label", "content", "content_id", "cover_image", "created_at",
		"next", "prev", "published_at", "related", "sort_order", "source", "status",
		"summary", "title", "updated_at", "view_count",
	}
	assertKeys(t, jsonTopLevelKeys(t, FeaturedContentDetailDTO{
		Category: "news", CategoryLabel: "资讯", Content: "content", ContentID: 1,
		CoverImage: "cover.png", CreatedAt: pub, PublishedAt: &pub,
		Related: []FeaturedContentDTO{}, SortOrder: 1, Source: "src", Status: 1,
		Summary: "summary", Title: "title", UpdatedAt: pub, ViewCount: 2,
	}), detailWant)

	adminWant := []string{
		"category", "category_label", "content", "content_id", "cover_image", "created_at",
		"published_at", "sort_order", "source", "status", "summary", "title", "updated_at", "view_count",
	}
	assertKeys(t, jsonTopLevelKeys(t, FeaturedContentAdminDetailDTO{
		Category: "news", CategoryLabel: "资讯", Content: "content", ContentID: 1,
		CoverImage: "cover.png", CreatedAt: pub, PublishedAt: &pub,
		SortOrder: 1, Source: "src", Status: 1, Summary: "summary", Title: "title",
		UpdatedAt: pub, ViewCount: 2,
	}), adminWant)

	navWant := []string{"category", "category_label", "content_id", "published_at", "title"}
	assertKeys(t, jsonTopLevelKeys(t, FeaturedNavDTO{
		Category: "news", CategoryLabel: "资讯", ContentID: 1, PublishedAt: &pub, Title: "title",
	}), navWant)
}
