package api

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/admin"
	"forklift-training/internal/aiassistant"
	"forklift-training/internal/auth"
	"forklift-training/internal/checkin"
	"forklift-training/internal/contribution"
	"forklift-training/internal/course"
	"forklift-training/internal/faq"
	"forklift-training/internal/favorite"
	"forklift-training/internal/featured"
	"forklift-training/internal/forum"
	"forklift-training/internal/inspection"
	"forklift-training/internal/material"
	"forklift-training/internal/mockexam"
	"forklift-training/internal/note"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/questioninteraction"
	"forklift-training/internal/realexam"
	"forklift-training/internal/recruit"
	"forklift-training/internal/resume"
	"forklift-training/internal/search"
	"forklift-training/internal/student"
	"forklift-training/internal/training"
	"forklift-training/internal/tutor"
	"forklift-training/internal/wrongquestion"
)

// 域路由注册表（ADR-0047 §6 / spec #933）：一行一域，顺序即注册顺序。
//
// 为什么需要它：新增蓝图以前要在 router.go 的 40 行调用序列里找位置，漏登记的症状是
// 「端点静默 404」；现在一处声明，routes_registry_test.go 用路由总数把它们钉住。
//
// 边界：只收 /api 下的蓝图（RegisterCaptchaRoutes 挂在引擎根、仍在 router.go 内注册）；
// 薄壳里只是调用既有 Register*Routes——签名与内部逻辑零改动（ADR-0047 §6 决策 4）。
type routeRegistrar struct {
	Domain   string
	Register func(api *gin.RouterGroup, rd RouterDeps, deps *Deps)
}

// routeRegistrars 全部域的注册入口（顺序 = 建路由顺序）。
var routeRegistrars = []routeRegistrar{
	{
		Domain: "认证与账号",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			// 认证蓝图 /api/auth/*：登录/刷新/登出/me/资料/注销（域包自持 handler）
			auth.RegisterRoutes(api, rd.Session, deps.AuthSvc, deps.FileSvc, deps.Storage, deps.ReviewSvc, deps.Logger)
			// 邮箱/手机号验证码注册登录（发码需过图形验证码）
			auth.RegisterEmailAuthRoutes(api, rd.Session, deps.CodeSvc, deps.EmailCh, deps.CaptchaSvc, deps.Cfg.CaptchaEnabled)
			auth.RegisterPhoneAuthRoutes(api, rd.Session, deps.CodeSvc, deps.PhoneCh, deps.CaptchaSvc, deps.Cfg.CaptchaEnabled)
			// 微信扫码登录（框架占位）
			auth.RegisterWechatAuthRoutes(api, deps.WechatAuthSvc)
			// 个人信息页：手机号/邮箱绑定修改
			auth.RegisterProfileBindRoutes(api, rd.Session, deps.CodeSvc, deps.EmailCh, deps.PhoneCh)
		},
	},
	{
		Domain: "培训工作区",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			course.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.CourseSvc)
			student.RegisterRoutes(api, rd.Session, deps.StudentSvc)
			questionbank.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.QuestionBankSvc, deps.FileSvc)
			practicemode.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.PracticeModeSvc)
		},
	},
	{
		Domain: "管理端",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			admin.RegisterRoutes(api, rd.Session, deps.AdminSvc, deps.AuthSvc, deps.AIConfigSvc, deps.ContentGenSvc)
			admin.RegisterAdminRecruiterRoutes(api, rd.Session, deps.AuthSvc)
			course.RegisterAdminRoutes(api, rd.Session, deps.AdminCourseSvc)
		},
	},
	{
		Domain: "招聘域",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			recruit.RegisterRoutes(api, rd.Session, deps.RecruitSvc)
		},
	},
	{
		Domain: "讲师端",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			tutor.RegisterRoutes(api, rd.Session, deps.TutorSvc, deps.FileSvc, uploadVditorImage)
		},
	},
	{
		Domain: "练习与考试",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			wrongquestion.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.WrongQuestionSvc)
			mockexam.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.MockExamSvc)
			realexam.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.RealExamSvc, deps.PointsSvc)
		},
	},
	{
		Domain: "内容与 AI",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			featured.RegisterRoutes(api, rd.Session, deps.FeaturedSvc, deps.FileSvc, uploadVditorImage)
			aiassistant.RegisterRoutes(api, rd.Session, deps.AIAssistantSvc, deps.DiagnosisProxySvc)
		},
	},
	{
		Domain: "论坛与打卡",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			forum.RegisterAdminRoutes(api, rd.Session, deps.ForumSvc, deps.ForumModSvc)
			forum.RegisterRoutes(api, rd.Session, deps.ForumSvc, deps.ForumModSvc, deps.ForumImageSvc)
			checkin.RegisterRoutes(api, rd.Session, deps.CheckInSvc)
		},
	},
	{
		Domain: "积分",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			points.RegisterAdminRoutes(api, rd.Session, deps.PointsSvc)
			points.RegisterRoutes(api, rd.Session, deps.PointsSvc)
		},
	},
	{
		Domain: "审核与治理",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			auth.RegisterAdminRoutes(api, rd.Session, deps.ReviewSvc)
			notification.RegisterRoutes(api, rd.Session, deps.NotificationSvc)
			RegisterAuditRoutes(api, rd, deps.AuditSvc)
			RegisterExportRoutes(api, rd, deps.ExportSvc)
			// 培训域 HTTP 出口三分（handler.go / handler_admin.go / handler_credential.go），
			// 三行合并等价原单条 RegisterTrainingCatalogRoutes（ADR-0070）：学员端读面 → 管理端目录面 → 证件面。
			training.RegisterRoutes(api, rd.Session, deps.TrainingCatalogSvc)
			training.RegisterAdminRoutes(api, rd.Session, deps.TrainingCatalogSvc)
			training.RegisterCredentialRoutes(api, rd.Session, deps.TrainingCatalogSvc)
			questioninteraction.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.QuestionCommentSvc, deps.NoteSvc, deps.QuestionKnowledgeSvc)
		},
	},
	{
		Domain: "个人与检索",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			// 移动端 P1 通用能力（ADR-0018）：通用收藏 / 全局搜索 / 学习资料聚合
			favorite.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.FavoriteSvc)
			search.RegisterRoutes(api, rd.CredentialScope, deps.SearchSvc)
			search.RegisterAdminRoutes(api, rd.Session, deps.SearchSvc)
			material.RegisterRoutes(api, rd.Session, deps.MaterialSvc)
			// 学员笔记（ADR-0055）：题目笔记 + 独立笔记的汇集读面与独立笔记 CRUD
			note.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.NoteSvc)
		},
	},
	{
		Domain: "帮助中心",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			// #1079：学员端只读整页（faq.read）+ 管理端分类与条目 CRUD（faq.manage）
			faq.RegisterRoutes(api, rd.Session, deps.FaqSvc)
		},
	},
	{
		Domain: "简历与职位",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			resume.RegisterRoutes(api, rd.Session, deps.JobCardSvc, deps.FileSvc)
			resume.RegisterViewRoutes(api, rd.Session, deps.JobCardSvc)
			// 两条 PDF 出口分居两域：学员侧在本包（resume），招聘者侧在 recruit（要 RecruitService）。
			resume.RegisterPDFRoutes(api, rd.Session, deps.JobCardSvc, deps.ResumePDFRenderer)
			recruit.RegisterPDFRoutes(api, rd.Session, deps.RecruitSvc, deps.ResumePDFRenderer)
			RegisterContactRoutes(api, rd, deps.ContactSvc)
			RegisterJobRoutes(api, rd, deps.JobPostingSvc)
			RegisterApplicationRoutes(api, rd, deps.JobApplicationSvc)
			RegisterJobReportRoutes(api, rd, deps.JobReportSvc, deps.JobPostingSvc)
			RegisterRecruiterApplicationRoutes(api, rd, deps.JobApplicationSvc)
		},
	},
	{
		Domain: "巡检与投稿",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			inspection.RegisterRoutes(api, rd.Session, deps.InspectionSvc, deps.PointsSvc)
			contribution.RegisterRoutes(api, rd.Session, rd.CredentialScope, deps.ContributionSvc)
			contribution.RegisterAdminRoutes(api, rd.Session, deps.ContributionSvc)
		},
	},
}
