package api

import (
	"forklift-training/internal/clock"
	"forklift-training/internal/contribution"
	"forklift-training/internal/inspection"
)

// provideContribution 巡检与投稿域。
func provideContribution(c *coreSingletons, d *Deps) {
	d.InspectionSvc = inspection.NewService(c.db)
	d.ContributionSvc = contribution.NewService(c.db, c.fileSvc, c.notifSvc, c.pointsSvc, c.logger, clock.Real())
}
