package service

import "forklift-training/internal/notification"

import "testing"

// paging_shape_lock_test 锁定两处 QueryWithScan 列表接口的分页信封 shape：
// notification List / profile_review ListRequests 深化后字段名零漂移。
// 若任何字段增删/改名在此暴露（前端契约是最高优先级约束）。
// （forum ListTopics 的信封形状已随域包搬去 internal/forum/，ADR-0070 波 2b-2。）

func TestPagingResultShapeLock(t *testing.T) {
	assertShapeLock(t, &notification.NotificationListPageResult{}, "items", "page", "pages", "total", "unread_count")
	assertShapeLock(t, &ProfileChangeRequestPageResult{}, "page", "pages", "requests", "total")
}
