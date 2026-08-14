package service

import "testing"

// paging_shape_lock_test 锁定三处 QueryWithScan 列表接口的分页信封 shape：
// forum ListTopics / notification List / profile_review ListRequests 深化后字段名零漂移。
// 若任何字段增删/改名在此暴露（前端契约是最高优先级约束）。

func TestPagingResultShapeLock(t *testing.T) {
	assertShapeLock(t, &ForumTopicPageResult{}, "page", "pages", "topics", "total")
	assertShapeLock(t, &NotificationListPageResult{}, "items", "page", "pages", "total", "unread_count")
	assertShapeLock(t, &ProfileChangeRequestPageResult{}, "page", "pages", "requests", "total")
}
