package api

import (
	"forklift-training/internal/clock"
	"forklift-training/internal/service"
)

// provideContribution 巡检与投稿域。
func provideContribution(c *coreSingletons, d *Deps) {
	d.InspectionSvc = service.NewInspectionService(c.db)
	d.ContributionSvc = service.NewContributionService(c.db, c.fileSvc, c.notifSvc, c.pointsSvc, c.logger, clock.Real())
}
