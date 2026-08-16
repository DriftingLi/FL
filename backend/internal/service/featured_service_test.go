package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// newFeaturedTestSvc 构造内容精选服务 + 内存数据库。
func newFeaturedTestSvc(t *testing.T) (*FeaturedService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := NewFeaturedService(db, nil, zap.NewNop())
	return svc, db
}

// seedPublishedFeatured 插入一篇已发布文章，返回其 ID。
func seedPublishedFeatured(t *testing.T, db *gorm.DB, title string, viewCount int) int {
	t.Helper()
	now := testutil.Now()
	item := model.FeaturedContent{
		Title:       title,
		Summary:     "摘要-" + title,
		Content:     "# " + title + "\n正文",
		Category:    "industry",
		Source:      "测试来源",
		Status:      1,
		ViewCount:   viewCount,
		SortOrder:   0,
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("创建精选内容失败: %v", err)
	}
	return item.ContentID
}

// TestGetPublicDetailNoViewSkipsCount 阅读量语义拆分：
// countView=false（SSR/爬虫路径）不改变 view_count，countView=true 才自增。
func TestGetPublicDetailNoViewSkipsCount(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	id := seedPublishedFeatured(t, db, "计数语义", 5)

	// no_view：阅读量不变
	detail, err := svc.GetPublicDetail(id, false)
	if err != nil {
		t.Fatalf("GetPublicDetail(no_view) 失败: %v", err)
	}
	if got := detail.ViewCount; got != 5 {
		t.Errorf("no_view 后 view_count = %v，期望 5", got)
	}
	if detail.Title != "计数语义" {
		t.Errorf("详情标题 = %v，期望 计数语义", detail.Title)
	}

	// 正常请求：自增一次
	detail2, err := svc.GetPublicDetail(id, true)
	if err != nil {
		t.Fatalf("GetPublicDetail(count) 失败: %v", err)
	}
	if got := detail2.ViewCount; got != 6 {
		t.Errorf("计数请求后 view_count = %v，期望 6", got)
	}
}

// TestGetPublicDetailUnpublishedRejected 未发布内容对公开详情不可见（含 no_view 路径）。
func TestGetPublicDetailUnpublishedRejected(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	now := testutil.Now()
	draft := model.FeaturedContent{
		Title:     "草稿",
		Category:  "news",
		Status:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("创建草稿失败: %v", err)
	}

	if _, err := svc.GetPublicDetail(draft.ContentID, false); err == nil {
		t.Error("草稿应返回错误（no_view 路径）")
	}
	if _, err := svc.GetPublicDetail(99999, true); err == nil {
		t.Error("不存在的 ID 应返回错误")
	}
}

// TestIncrementViewCount 客户端计数端点：自增并返回最新值；不存在/未发布返回错误。
func TestIncrementViewCount(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)
	id := seedPublishedFeatured(t, db, "计数端点", 3)

	count, err := svc.IncrementViewCount(id)
	if err != nil {
		t.Fatalf("IncrementViewCount 失败: %v", err)
	}
	if count != 4 {
		t.Errorf("IncrementViewCount 返回 %d，期望 4", count)
	}

	// 未发布内容不可计数
	now := testutil.Now()
	draft := model.FeaturedContent{
		Title:     "草稿计数",
		Category:  "news",
		Status:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("创建草稿失败: %v", err)
	}
	if _, err := svc.IncrementViewCount(draft.ContentID); err == nil {
		t.Error("草稿计数应返回错误")
	}
	if _, err := svc.IncrementViewCount(99999); err == nil {
		t.Error("不存在的 ID 计数应返回错误")
	}
}
