package api

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/auth"
	"forklift-training/internal/captcha"
	"forklift-training/internal/checkin"
	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/contribution"
	"forklift-training/internal/course"
	"forklift-training/internal/daemon"
	"forklift-training/internal/faq"
	"forklift-training/internal/favorite"
	"forklift-training/internal/featured"
	"forklift-training/internal/filestore"
	"forklift-training/internal/forum"
	"forklift-training/internal/inspection"
	"forklift-training/internal/material"
	"forklift-training/internal/middleware"
	"forklift-training/internal/mockexam"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/realexam"
	"forklift-training/internal/search"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/internal/student"
	"forklift-training/internal/training"
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

	AuthSvc         *auth.Service
	CodeSvc         *auth.VerifyCodeService
	EmailCh         auth.CodeChannel
	PhoneCh         auth.CodeChannel
	CaptchaSvc      *captcha.Service
	WechatAuthSvc   *auth.WechatService
	FileSvc         *filestore.FileStore
	SlideRenderer   *course.SlideRenderer
	NotificationSvc *notification.Service
	ReviewSvc       *auth.ProfileReviewService
	AuditSvc        *service.AuditService
	AIConfigSvc     *aiassistant.ConfigService
	ContentGenSvc   *service.ContentGenerateService
	ExportStore     service.ExportStore

	CourseSvc            *course.Service
	AdminSvc             *service.AdminService
	AdminCourseSvc       *course.AdminService
	ForumSvc             *forum.Service
	ForumModSvc          *forum.ModerationService
	CheckInSvc           *checkin.Service
	ForumImageSvc        *forum.ImageService
	FeaturedSvc          *featured.Service
	FavoriteSvc          *favorite.Service
	SearchSvc            *search.Service
	MaterialSvc          *material.Service
	ExportSvc            *service.ExportService
	StudentSvc           *student.Service
	QuestionBankSvc      *questionbank.Service
	PracticeModeSvc      *practicemode.Service
	MockExamSvc          *mockexam.Service
	RealExamSvc          *realexam.Service
	TutorSvc             *service.TutorService
	WrongQuestionSvc     *service.WrongQuestionService
	TrainingCatalogSvc   *training.Service
	AIAssistantSvc       *aiassistant.Service
	DiagnosisProxySvc    *aiassistant.DiagnosisProxyService
	QuestionCommentSvc   *service.QuestionCommentService
	NoteSvc              *service.NoteService
	QuestionKnowledgeSvc *service.QuestionKnowledgeService
	FaqSvc               *faq.Service
	PointsSvc            *points.Service
	JobCardSvc           *service.JobCardService
	ResumePDFRenderer    *service.ResumePDFRenderer
	RecruitSvc           *service.RecruitService
	ContactSvc           *service.ContactService
	JobPostingSvc        *service.JobPostingService
	JobApplicationSvc    *service.JobApplicationService
	JobReportSvc         *service.JobReportService
	InspectionSvc        *inspection.Service
	ContributionSvc      *contribution.Service

	// Daemons 进程内周期守护的**登记表**（ADR-0061 §1）：NewDeps 声明，cmd/server 经 daemon.StartAll 启动。
	// 登记 ≠ 启动——契约测试构造 Deps 时不会拉起任何 goroutine，而漏登记会被 daemons_contract_test.go 判红。
	// 口径：只有「跨服务、需要独立周期」的守护入表；middleware 自己构造时装配的（rate-limit-cleanup）不入表，
	// 它的生命周期属于该组件，摘出来反而更容易漏。
	Daemons []daemon.Task
}

// NewDeps 构建全部 service 单实例。进程启动早期由 main 调用一次。
//
// 装配顺序 = **core（横切单例）→ 各域 provider（一域一个文件）→ 守护登记 → 后置装配**。
// 各域 provider 只写「自己那几个 service」，横切单例一律从 coreSingletons 取（providers_core.go），
// 于是「全进程只有一份的东西」与「某域自己的东西」在文件层面就分得开。
// exportStore 经 ExportStore seam 注入（生产为估值模块 pgx adapter）。
func NewDeps(cfg *config.Config, db *gorm.DB, st storage.Storage, logger *zap.Logger, exportStore service.ExportStore) *Deps {
	core := provideCore(cfg, db, st, logger, exportStore)

	d := &Deps{
		Cfg:     cfg,
		DB:      db,
		Storage: st,
		Logger:  logger,
		Session: core.sess,
		// 横切单例在 Deps 上的投影（路由装配与蓝图注册直接读这几个字段）
		FileSvc:         core.fileSvc,
		SlideRenderer:   core.slideRenderer,
		NotificationSvc: core.notifSvc,
	}

	// 各域装配：一行一域，顺序即依赖序（域之间只经 core 的共享单例交互）。
	provideAuth(core, d)
	provideAI(core, d)
	provideForum(core, d)
	provideTraining(core, d)
	provideExam(core, d)
	provideJobs(core, d)
	provideContribution(core, d)

	// 守护登记（ADR-0061 §1）：加守护 = 往这张表加一条，不需要在 cmd/server 里再手写一次 start。
	// 闭包读 d 上的 service 字段（此刻已构造完），故登记排在各域装配之后。
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
	if d.JobApplicationSvc != nil && core.mailSender != nil {
		d.JobApplicationSvc.SetMailer(core.mailSender)
	}
	if d.JobReportSvc != nil && core.mailSender != nil {
		d.JobReportSvc.SetMailer(core.mailSender)
	}
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
