package api

import (
	"forklift-training/internal/job"
	"forklift-training/internal/recruit"
	"forklift-training/internal/resume"
)

// provideJobs 招聘与简历域（职位卡/招聘者/职位/投递/举报/简历 PDF）。
func provideJobs(c *coreSingletons, d *Deps) {
	d.JobCardSvc = resume.NewService(c.db, c.fileSvc, c.logger)
	d.ResumePDFRenderer = resume.NewPDFRenderer()
	d.RecruitSvc = recruit.NewService(c.db, c.logger)
	d.ContactSvc = c.contactSvc
	d.JobPostingSvc = job.NewService(c.db, c.logger)
	d.JobApplicationSvc = job.NewApplicationService(c.db, c.logger, c.notifSvc, c.contactSvc)
	d.JobReportSvc = job.NewReportService(c.db, c.logger)
}
