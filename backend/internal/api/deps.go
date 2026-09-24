package api

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/captcha"
	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/daemon"
	"forklift-training/internal/middleware"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
)

// RouterDeps 聚合蓝图注册所需的横切依赖（Session/DB/Logger）。
// Register*Routes 统一收 RouterDeps 替代逐处重复传 sess *security.Session；
// 业务 service 仍按需注入（不吞整个 Deps），保留 ADR-0009 按需注入精神。
type RouterDeps struct {
	Session *security.Session
	DB      *gorm.DB
	Logger  *zap.Logger
	// CredentialScope 证件作用域解析器（ADR-0047 §4）：受作用域端点用它解析「本次请求按哪个
	// 证件过滤」，事实源在服务端（用户当前证件），客户端漏传不再静默返回全量。
	CredentialScope middleware.CredentialResolver
}

// Deps 是后端 service 装配根：全部 service 在此构建一次，经 NewRouter 注入各蓝图注册。
// 蓝图注册函数不再自行构造 service，实例归属唯一（对应 spec #75 D9）。
type Deps struct {
	Cfg     *config.Config
	DB      *gorm.DB
	Storage storage.Storage
	Logger  *zap.Logger
	Session *security.Session

	AuthSvc         *service.AuthService
	CodeSvc         *service.VerifyCodeService
	EmailCh         service.CodeChannel
	PhoneCh         service.CodeChannel
	CaptchaSvc      *captcha.Service
	WechatAuthSvc   *service.WechatAuthService
	FileSvc         *service.FileStore
	SlideRenderer   *service.SlideRenderer
	NotificationSvc *service.NotificationService
	ReviewSvc       *service.ProfileReviewService
	AuditSvc        *service.AuditService
	AIConfigSvc     *service.AIConfigService
	ContentGenSvc   *service.ContentGenerateService
	ExportStore     service.ExportStore
	AuthH           *AuthHandler

	CourseSvc            *service.CourseService
	AdminSvc             *service.AdminService
	AdminCourseSvc       *service.AdminCourseService
	ForumSvc             *service.ForumService
	ForumModSvc          *service.ForumModerationService
	CheckInSvc           *service.CheckInService
	ForumImageSvc        *service.ForumImageService
	FeaturedSvc          *service.FeaturedService
	FavoriteSvc          *service.FavoriteService
	SearchSvc            *service.SearchService
	MaterialSvc          *service.MaterialService
	ExportSvc            *service.ExportService
	StudentSvc           *service.StudentService
	QuestionBankSvc      *service.QuestionBankService
	PracticeModeSvc      *service.PracticeModeService
	MockExamSvc          *service.MockExamService
	RealExamSvc          *service.RealExamService
	TutorSvc             *service.TutorService
	WrongQuestionSvc     *service.WrongQuestionService
	TrainingCatalogSvc   *service.TrainingCatalogService
	AIAssistantSvc       *service.AIAssistantService
	DiagnosisProxySvc    *service.DiagnosisProxyService
	QuestionCommentSvc   *service.QuestionCommentService
	NoteSvc              *service.NoteService
	QuestionKnowledgeSvc *service.QuestionKnowledgeService
	FaqSvc               *service.FaqService
	PointsSvc            *service.PointsService
	JobCardSvc           *service.JobCardService
	ResumePDFRenderer    *service.ResumePDFRenderer
	RecruitSvc           *service.RecruitService
	ContactSvc           *service.ContactService
	JobPostingSvc        *service.JobPostingService
	JobApplicationSvc    *service.JobApplicationService
	JobReportSvc         *service.JobReportService
	InspectionSvc        *service.InspectionService
	ContributionSvc      *service.ContributionService

	// Daemons 进程内周期守护的**登记表**（ADR-0061 §1）：NewDeps 声明，cmd/server 经 daemon.StartAll 启动。
	// 登记 ≠ 启动——契约测试构造 Deps 时不会拉起任何 goroutine，而漏登记会被 daemons_contract_test.go 判红。
	// 口径：只有「跨服务、需要独立周期」的守护入表；middleware 自己构造时装配的（rate-limit-cleanup）不入表，
	// 它的生命周期属于该组件，摘出来反而更容易漏。
	Daemons []daemon.Task
}

// NewDeps 构建全部 service 单实例。进程启动早期由 main 调用一次。
// exportStore 经 ExportStore seam 注入（生产为估值模块 pgx adapter）。
func NewDeps(cfg *config.Config, db *gorm.DB, st storage.Storage, logger *zap.Logger, exportStore service.ExportStore) *Deps {
	// 会话唯一实例：签发（AuthService）与校验（中间件/估值模块）共用同一实例
	sess := security.SessionFromConfig(cfg)
	// 论坛计数器唯一实例：ForumService / ForumModerationService 与 AuthService 共享（计数列唯一写入口，spec #297）
	forumCnt := service.NewForumCounter()
	authSvc := service.NewAuthService(db, sess, forumCnt,
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, logger)
	codeSvc := service.NewVerifyCodeService(db, authSvc, cfg.EmailCodeTTL, &service.RedisAuthCodeStore{}, logger)
	captchaSvc := captcha.NewService(captcha.RedisStore{})
	emailCh := service.NewEmailChannel(cfg.SMTP, cfg.IsProd(), logger)
	// 邮件发送器单点（spec #449 决定 15）：联系方式交换与投递通知共用，不再注入 nil 只写日志。
	mailSender := service.NewMailSender(cfg.SMTP, cfg.IsProd(), logger)
	phoneCh := service.NewSmsChannel(cfg.SMS, cfg.IsProd(), logger)
	wechatAuthSvc := service.NewWechatAuthService(cfg.Wechat.MiniProgram, db, authSvc, logger)
	fileSvc := service.NewFileStore(cfg.LibreOfficeSidecarURL, st, logger)
	slideRenderer := service.NewSlideRenderer(cfg.LibreOfficeSidecarURL, st, logger)
	notificationSvc := service.NewNotificationService(db, logger)
	reviewSvc := service.NewProfileReviewService(db, notificationSvc, st, logger)
	authSvc.SetProfileReviewService(reviewSvc)
	aiConfigSvc := service.NewAIConfigService(db, cfg.SecretKey, logger)
	// 积分服务唯一实例：积分端点与真题卷权益校验共用
	pointsSvc := service.NewPointsService(db, logger, clock.Real(), notificationSvc)
	// 单一模型端口（ADR-0029 T2）：唯一 eino adapter 实例，阻塞/流式消费方共享同一 client 签名缓存。
	// 计量闸门（ADR-0031）作为装饰器挂在该端口上：所有 LLM 消费（含会话自动命名）过同一道闸，
	// 生产 meter 即积分域 *PointsService（预检与扣费下限同源），装配单点在此。
	// 第二实现：外部诊断 RAG 助手（fault_diagnosis）经 routing adapter 按功能键分发
	// （baseURL 来自 cfg.DiagnosisAssistantURL，不走管理端模型绑定）。
	aiRouting := service.NewRoutingAIModel(
		service.NewEinoAIModel(aiConfigSvc, logger),
		service.NewDiagnosisAssistantModel(cfg.DiagnosisAssistantURL, logger),
	)
	aiModelPort := service.NewMeteredAIModel(aiRouting, pointsSvc, logger)
	aiSvc := service.NewAIService(db, aiModelPort, logger)
	contentGenSvc := service.NewContentGenerateService(db, aiSvc, logger)
	// 联系方式交换唯一实例：申请/授权状态机（EnsureApproved）与投递侧共用（ADR-0027 C5）
	contactSvc := service.NewContactService(db, logger, notificationSvc, mailSender)

	d := &Deps{
		Cfg:                  cfg,
		DB:                   db,
		Storage:              st,
		Logger:               logger,
		Session:              sess,
		AuthSvc:              authSvc,
		CodeSvc:              codeSvc,
		EmailCh:              emailCh,
		PhoneCh:              phoneCh,
		CaptchaSvc:           captchaSvc,
		WechatAuthSvc:        wechatAuthSvc,
		FileSvc:              fileSvc,
		SlideRenderer:        slideRenderer,
		NotificationSvc:      notificationSvc,
		ReviewSvc:            reviewSvc,
		AIConfigSvc:          aiConfigSvc,
		ContentGenSvc:        contentGenSvc,
		CourseSvc:            service.NewCourseService(db, slideRenderer, logger),
		AdminSvc:             service.NewAdminService(db, sess, logger),
		AdminCourseSvc:       service.NewAdminCourseService(db, fileSvc, logger),
		ForumSvc:             service.NewForumService(db, fileSvc, notificationSvc, forumCnt, pointsSvc, logger),
		ForumModSvc:          service.NewForumModerationService(db, fileSvc, notificationSvc, forumCnt, pointsSvc, logger),
		CheckInSvc:           service.NewCheckInService(db, logger, clock.Real(), pointsSvc),
		ForumImageSvc:        service.NewForumImageService(db, fileSvc, logger),
		FeaturedSvc:          service.NewFeaturedService(db, fileSvc, logger),
		FavoriteSvc:          service.NewFavoriteService(db, logger),
		SearchSvc:            service.NewSearchService(db, logger),
		MaterialSvc:          service.NewMaterialService(db, logger),
		ExportSvc:            service.NewExportService(db, exportStore, logger),
		StudentSvc:           service.NewStudentService(db, logger),
		QuestionBankSvc:      service.NewQuestionBankService(db, fileSvc, logger),
		PracticeModeSvc:      service.NewPracticeModeService(db, aiSvc, logger),
		MockExamSvc:          service.NewMockExamService(db, aiSvc, logger),
		RealExamSvc:          service.NewRealExamService(db, pointsSvc, logger),
		TutorSvc:             service.NewTutorService(db, cfg.UploadFolder, fileSvc, slideRenderer, logger),
		WrongQuestionSvc:     service.NewWrongQuestionService(db, aiSvc, logger),
		TrainingCatalogSvc:   service.NewTrainingCatalogService(db, logger),
		AuditSvc:             service.NewAuditService(db),
		AIAssistantSvc:       service.NewAIAssistantService(db, aiConfigSvc, fileSvc, cfg.SecretKey, logger, aiModelPort),
		DiagnosisProxySvc:    service.NewDiagnosisProxyService(cfg.DiagnosisAssistantURL, logger),
		QuestionCommentSvc:   service.NewQuestionCommentService(db, logger),
		NoteSvc:              service.NewNoteService(db, logger),
		QuestionKnowledgeSvc: service.NewQuestionKnowledgeService(db),
		FaqSvc:               service.NewFaqService(db, logger),
		PointsSvc:            pointsSvc,
		JobCardSvc:           service.NewJobCardService(db, fileSvc, logger),
		ResumePDFRenderer:    service.NewResumePDFRenderer(),
		RecruitSvc:           service.NewRecruitService(db, logger),
		ContactSvc:           contactSvc,
		JobPostingSvc:        service.NewJobPostingService(db, logger),
		JobApplicationSvc:    service.NewJobApplicationService(db, logger, notificationSvc, contactSvc),
		JobReportSvc:         service.NewJobReportService(db, logger),
		InspectionSvc:        service.NewInspectionService(db),
		ContributionSvc:      service.NewContributionService(db, fileSvc, notificationSvc, pointsSvc, logger, clock.Real()),
	}
	// 守护登记（ADR-0061 §1）：加守护 = 往这张表加一条，不需要在 cmd/server 里再手写一次 start。
	// 闭包读 d 上的 service 字段（此刻已构造完），故登记排在 Deps 字面量之后。
	d.Daemons = []daemon.Task{
		// 两个悬空文件清理都是 6 小时差集扫描（算法在各 service 里，这里只声明节奏）。
		{Name: "forum-image-cleanup", Interval: 6 * time.Hour, Run: func(ctx context.Context) {
			if cleaned := d.ForumImageSvc.CleanupOrphans(ctx); cleaned > 0 {
				logger.Info("论坛悬空图片清理完成", zap.Int("cleaned", cleaned))
			}
		}},
		{Name: "contribution-file-cleanup", Interval: 6 * time.Hour, Run: func(ctx context.Context) {
			if cleaned := d.ContributionSvc.CleanupOrphanFiles(ctx); cleaned > 0 {
				logger.Info("投稿悬空文件清理完成", zap.Int("cleaned", cleaned))
			}
		}},
		// 联系方式交换的裁决窗口收敛（#1197）：把窗口已闭却仍挂 pending 的行落为 expired。
		// 1 小时对 14 天窗口纯属精度余量；正在重试的企业由 Create 的定向落态当场兜住，
		// 本守护只负责没人再触碰的那些行。
		{Name: "contact-request-expire", Interval: time.Hour, Run: func(ctx context.Context) {
			if _, err := d.ContactSvc.ExpirePending(clock.Now()); err != nil {
				logger.Warn("contact expire 失败", zap.Error(err))
			}
		}},
	}
	// 投递通知与联系方式交换共用邮件单点（spec #449 决定 15）
	if d.JobApplicationSvc != nil && mailSender != nil {
		d.JobApplicationSvc.SetMailer(mailSender)
	}
	if d.JobReportSvc != nil && mailSender != nil {
		d.JobReportSvc.SetMailer(mailSender)
	}
	d.AuthH = NewAuthHandler(d.Session, authSvc, fileSvc, st, reviewSvc, logger)
	return d
}

// StartDaemons 启动本装配根登记的全部守护（ADR-0061 §1）。
// 登记与启动收在同一个方法上，「表里有、却没起」因此可被测试问到（daemons_contract_test.go）；
// opts 透传给 Runner，测试用它换掉真时钟。
func (d *Deps) StartDaemons(ctx context.Context, opts ...daemon.RunnerOption) []*daemon.Runner {
	return daemon.StartAll(ctx, d.Logger, d.Daemons, opts...)
}

// RouterDeps 投影当前装配根的横切依赖，供 NewRouter 传给各蓝图注册（单一装配点）。
func (d *Deps) RouterDeps() RouterDeps {
	return RouterDeps{Session: d.Session, DB: d.DB, Logger: d.Logger,
		CredentialScope: d.TrainingCatalogSvc,
	}
}
