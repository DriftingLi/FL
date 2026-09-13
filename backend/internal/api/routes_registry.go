package api

import (
	"github.com/gin-gonic/gin"
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
			// 邮箱/手机号验证码注册登录（发码需过图形验证码）
			RegisterEmailAuthRoutes(api, rd, deps.CodeSvc, deps.EmailCh, deps.CaptchaSvc, deps.Cfg.CaptchaEnabled)
			RegisterPhoneAuthRoutes(api, rd, deps.CodeSvc, deps.PhoneCh, deps.CaptchaSvc, deps.Cfg.CaptchaEnabled)
			// 微信扫码登录（框架占位）
			RegisterWechatAuthRoutes(api, deps.WechatAuthSvc)
			// 个人信息页：手机号/邮箱绑定修改
			RegisterProfileBindRoutes(api, rd, deps.CodeSvc, deps.EmailCh, deps.PhoneCh)
		},
	},
	{
		Domain: "培训工作区",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterCoursesRoutes(api, rd, deps.CourseSvc)
			RegisterStudentRoutes(api, rd, deps.StudentSvc)
			RegisterQuestionBankRoutes(api, rd, deps.QuestionBankSvc, deps.FileSvc)
			RegisterPracticeModeRoutes(api, rd, deps.PracticeModeSvc)
		},
	},
	{
		Domain: "管理端",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterAdminRoutes(api, rd, deps.AdminSvc, deps.AdminCourseSvc, deps.AuthSvc, deps.AIConfigSvc, deps.ContentGenSvc)
			RegisterAdminRecruiterRoutes(api, rd, deps.AuthSvc)
		},
	},
	{
		Domain: "招聘域",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterRecruitRoutes(api, rd, deps.RecruitSvc)
		},
	},
	{
		Domain: "讲师端",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterTutorRoutes(api, rd, deps.TutorSvc, deps.FileSvc)
		},
	},
	{
		Domain: "练习与考试",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterWrongQuestionRoutes(api, rd, deps.WrongQuestionSvc)
			RegisterMockExamRoutes(api, rd, deps.MockExamSvc)
			RegisterRealExamRoutes(api, rd, deps.RealExamSvc, deps.PointsSvc)
		},
	},
	{
		Domain: "内容与 AI",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterFeaturedRoutes(api, rd, deps.FeaturedSvc, deps.FileSvc)
			RegisterAIAssistantRoutes(api, rd, deps.AIAssistantSvc)
			RegisterDiagnosisRoutes(api.Group("/ai-assistant"), rd, deps.DiagnosisProxySvc)
		},
	},
	{
		Domain: "论坛与打卡",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterForumRoutes(api, rd, deps.ForumSvc, deps.ForumImageSvc)
			RegisterCheckInRoutes(api, rd, deps.CheckInSvc)
		},
	},
	{
		Domain: "积分",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterAdminPointsRoutes(api, rd, deps.PointsSvc, deps.NotificationSvc)
			RegisterPointsRoutes(api, rd, deps.PointsSvc)
		},
	},
	{
		Domain: "审核与治理",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterProfileReviewRoutes(api, rd, deps.ReviewSvc)
			RegisterNotificationRoutes(api, rd, deps.NotificationSvc)
			RegisterAuditRoutes(api, rd, deps.AuditSvc)
			RegisterExportRoutes(api, rd, deps.ExportSvc)
			RegisterTrainingCatalogRoutes(api, rd, deps.TrainingCatalogSvc)
			RegisterQuestionInteractionRoutes(api, rd, deps.QuestionCommentSvc, deps.QuestionNoteSvc, deps.QuestionKnowledgeSvc)
		},
	},
	{
		Domain: "个人与检索",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			// 移动端 P1 通用能力（ADR-0018）：通用收藏 / 全局搜索 / 学习资料聚合
			RegisterFavoriteRoutes(api, rd, deps.FavoriteSvc)
			RegisterSearchRoutes(api, rd, deps.SearchSvc)
			RegisterMaterialRoutes(api, rd, deps.MaterialSvc)
		},
	},
	{
		Domain: "简历与职位",
		Register: func(api *gin.RouterGroup, rd RouterDeps, deps *Deps) {
			RegisterJobCardRoutes(api, rd, deps.JobCardSvc, deps.FileSvc)
			RegisterResumeViewRoutes(api, rd, deps.RecruitSvc)
			RegisterResumePDFRoutes(api, rd, deps.RecruitSvc, deps.ResumePDFRenderer)
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
			RegisterAdminInspectionRoutes(api, rd, deps.DB, deps.PointsSvc)
			RegisterContributionRoutes(api, rd, deps.ContributionSvc)
		},
	},
}
