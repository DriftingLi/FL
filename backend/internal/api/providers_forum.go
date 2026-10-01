package api

import (
	"forklift-training/internal/clock"
	"forklift-training/internal/service"
)

// provideForum 论坛与打卡域：问答/审核/图片清理三件共享同一计数器与积分单例。
func provideForum(c *coreSingletons, d *Deps) {
	d.ForumSvc = service.NewForumService(c.db, c.fileSvc, c.notifSvc, c.forumCn, c.pointsSvc, c.logger)
	d.ForumModSvc = service.NewForumModerationService(c.db, c.fileSvc, c.notifSvc, c.forumCn, c.pointsSvc, c.logger)
	d.ForumImageSvc = service.NewForumImageService(c.db, c.fileSvc, c.logger)
	d.CheckInSvc = service.NewCheckInService(c.db, c.logger, clock.Real(), c.pointsSvc)
}
