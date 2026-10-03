package api

import (
	"forklift-training/internal/admin"
	"forklift-training/internal/audit"
	"forklift-training/internal/course"
	"forklift-training/internal/faq"
	"forklift-training/internal/favorite"
	"forklift-training/internal/featured"
	"forklift-training/internal/material"
	"forklift-training/internal/search"
	"forklift-training/internal/student"
	"forklift-training/internal/tutor"
	"forklift-training/internal/valuation"
)

// provideTraining 培训工作区（课程/管理端/学员/讲师/检索/收藏/精选/导出/审计/FAQ）。
func provideTraining(c *coreSingletons, d *Deps) {
	d.CourseSvc = course.NewService(c.db, c.slideRenderer, c.logger)
	d.AdminSvc = admin.NewService(c.db, c.sess, c.logger)
	d.AdminCourseSvc = course.NewAdminService(c.db, c.fileSvc, c.logger)
	d.StudentSvc = student.NewService(c.db, c.logger)
	d.TutorSvc = tutor.NewService(c.db, c.cfg.UploadFolder, c.fileSvc, c.slideRenderer, c.logger)
	d.MaterialSvc = material.NewService(c.db, c.logger)
	d.SearchSvc = search.NewService(c.db, c.logger)
	d.FavoriteSvc = favorite.NewService(c.db, c.logger)
	d.FeaturedSvc = featured.NewService(c.db, c.fileSvc, c.logger)
	d.ExportSvc = valuation.NewExportService(c.db, c.export, c.logger)
	d.AuditSvc = audit.NewService(c.db)
	d.FaqSvc = faq.NewService(c.db, c.logger)
}
