package api

import (
	"forklift-training/internal/checkin"
	"forklift-training/internal/clock"
	"forklift-training/internal/forum"
)

// provideForum 论坛与打卡域：论坛三件（问答／审核／图片清理）共享同一计数器与积分单例；
// 论坛三件已搬 internal/forum。
func provideForum(c *coreSingletons, d *Deps) {
	d.ForumSvc = forum.NewService(c.db, c.fileSvc, c.notifSvc, c.forumCn, c.pointsSvc, c.logger)
	d.ForumModSvc = forum.NewModerationService(c.db, c.fileSvc, c.notifSvc, c.forumCn, c.pointsSvc, c.logger)
	d.ForumImageSvc = forum.NewImageService(c.db, c.fileSvc, c.logger)
	d.CheckInSvc = checkin.NewService(c.db, c.logger, clock.Real(), c.pointsSvc)
}
