package api

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/captcha"
	"forklift-training/internal/config"
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
	TutorSvc             *service.TutorService
	WrongQuestionSvc     *service.WrongQuestionService
	TrainingCatalogSvc   *service.TrainingCatalogService
	AIAssistantSvc       *service.AIAssistantService
	QuestionCommentSvc   *service.QuestionCommentService
	QuestionNoteSvc      *service.QuestionNoteService
	QuestionKnowledgeSvc *service.QuestionKnowledgeService
}

// NewDeps 构建全部 service 单实例。进程启动早期由 main 调用一次。
// exportStore 经 ExportStore seam 注入（生产为估值模块 pgx adapter）。
func NewDeps(cfg *config.Config, db *gorm.DB, st storage.Storage, logger *zap.Logger, exportStore service.ExportStore) *Deps {
	// 会话唯一实例：签发（AuthService）与校验（中间件/估值模块）共用同一实例
	sess := security.SessionFromConfig(cfg)
	authSvc := service.NewAuthService(db, sess,
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, logger)
	codeSvc := service.NewVerifyCodeService(db, authSvc, cfg.EmailCodeTTL, &service.RedisAuthCodeStore{}, logger)
	captchaSvc := captcha.NewService(captcha.RedisStore{})
	emailCh := service.NewEmailChannel(cfg.SMTP, cfg.IsProd(), logger)
	phoneCh := service.NewSmsChannel(cfg.SMS, cfg.IsProd(), logger)
	wechatAuthSvc := service.NewWechatAuthService(cfg.Wechat, db, authSvc, logger)
	fileSvc := service.NewFileStore(cfg.LibreOfficeSidecarURL, st, logger)
	slideRenderer := service.NewSlideRenderer(cfg.LibreOfficeSidecarURL, st, logger)
	notificationSvc := service.NewNotificationService(db, logger)
	reviewSvc := service.NewProfileReviewService(db, notificationSvc, st, logger)
	authSvc.SetProfileReviewService(reviewSvc)
	aiConfigSvc := service.NewAIConfigService(db, cfg.SecretKey, logger)
	aiSvc := service.NewAIService(db, aiConfigSvc, logger)
	contentGenSvc := service.NewContentGenerateService(db, aiSvc, logger)

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
		AdminSvc:             service.NewAdminService(db, logger),
		AdminCourseSvc:       service.NewAdminCourseService(db, fileSvc, logger),
		ForumSvc:             service.NewForumService(db, fileSvc, notificationSvc, logger),
		CheckInSvc:           service.NewCheckInService(db, logger),
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
		TutorSvc:             service.NewTutorService(db, cfg.UploadFolder, fileSvc, slideRenderer, logger),
		WrongQuestionSvc:     service.NewWrongQuestionService(db, logger),
		TrainingCatalogSvc:   service.NewTrainingCatalogService(db, logger),
		AuditSvc:             service.NewAuditService(db),
		AIAssistantSvc:       service.NewAIAssistantService(db, aiConfigSvc, cfg.SecretKey, logger),
		QuestionCommentSvc:   service.NewQuestionCommentService(db, logger),
		QuestionNoteSvc:      service.NewQuestionNoteService(db, logger),
		QuestionKnowledgeSvc: service.NewQuestionKnowledgeService(db),
	}
	d.AuthH = NewAuthHandler(d.Session, authSvc, fileSvc, st, reviewSvc, logger)
	return d
}

// RouterDeps 投影当前装配根的横切依赖，供 NewRouter 传给各蓝图注册（单一装配点）。
func (d *Deps) RouterDeps() RouterDeps {
	return RouterDeps{Session: d.Session, DB: d.DB, Logger: d.Logger}
}
