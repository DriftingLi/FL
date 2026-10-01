package api

import (
	"forklift-training/internal/faq"
	"forklift-training/internal/service"
)

// provideTraining 培训工作区（课程/管理端/学员/讲师/检索/收藏/精选/导出/审计/FAQ）。
func provideTraining(c *coreSingletons, d *Deps) {
	d.CourseSvc = service.NewCourseService(c.db, c.slideRenderer, c.logger)
	d.AdminSvc = service.NewAdminService(c.db, c.sess, c.logger)
	d.AdminCourseSvc = service.NewAdminCourseService(c.db, c.fileSvc, c.logger)
	d.StudentSvc = service.NewStudentService(c.db, c.logger)
	d.TutorSvc = service.NewTutorService(c.db, c.cfg.UploadFolder, c.fileSvc, c.slideRenderer, c.logger)
	d.MaterialSvc = service.NewMaterialService(c.db, c.logger)
	d.SearchSvc = service.NewSearchService(c.db, c.logger)
	d.FavoriteSvc = service.NewFavoriteService(c.db, c.logger)
	d.FeaturedSvc = service.NewFeaturedService(c.db, c.fileSvc, c.logger)
	d.ExportSvc = service.NewExportService(c.db, c.export, c.logger)
	d.AuditSvc = service.NewAuditService(c.db)
	d.FaqSvc = faq.NewService(c.db, c.logger)
}
