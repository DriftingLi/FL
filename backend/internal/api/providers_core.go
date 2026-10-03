package api

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/auth"
	"forklift-training/internal/captcha"
	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/course"
	"forklift-training/internal/filestore"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation"
)

// coreSingletons 是**跨域共享**的单例与横切依赖：domain provider 都从它取，不再各自 new 一份。
//
// 为什么单独一个结构：这些实例的「唯一性」本身就是判据（会话单例、论坛计数器唯一写入口、
// 积分服务唯一实例、单一模型端口——ADR-0029/0031/0011）。把它们收在一处，读的人一眼能看见
// 「哪些东西全进程只有一份」，而各域 provider 只负责自己那几个 service。
type coreSingletons struct {
	cfg     *config.Config
	db      *gorm.DB
	st      storage.Storage
	logger  *zap.Logger
	export  valuation.ExportStore
	sess    *security.Session
	forumCn core.ForumCounter

	authSvc       *auth.Service
	codeSvc       *auth.VerifyCodeService
	captchaSvc    *captcha.Service
	emailCh       auth.CodeChannel
	phoneCh       auth.CodeChannel
	mailSender    core.MailSender
	wechatAuthSvc *auth.WechatService
	fileSvc       *filestore.FileStore
	slideRenderer *course.SlideRenderer
	notifSvc      *notification.Service
	reviewSvc     *auth.ProfileReviewService
	aiConfigSvc   *aiassistant.ConfigService
	pointsSvc     *points.Service
	aiModelPort   aiassistant.ModelPort
	aiSvc         *aiassistant.GenerationService
	contentGenSvc *core.ContentGenerateService
	contactSvc    *core.ContactService
}

// provideCore 建横切单例。**构造顺序与原单函数逐字一致**（会话 → 计数器 → 认证 → 通道 →
// 存储/渲染 → 通知/审核 → AI 配置 → 积分 → 模型端口 → AI → 内容生成 → 联系方式），
// 因为其中夹着一条后置装配（authSvc.SetProfileReviewService）与若干「先有 A 才有 B」的单例。
func provideCore(cfg *config.Config, db *gorm.DB, st storage.Storage, logger *zap.Logger, exportStore valuation.ExportStore) *coreSingletons {
	c := &coreSingletons{cfg: cfg, db: db, st: st, logger: logger, export: exportStore}

	// 会话唯一实例：签发（AuthService）与校验（中间件/估值模块）共用同一实例
	c.sess = security.SessionFromConfig(cfg)
	// 论坛计数器唯一实例：ForumService / ForumModerationService 与 AuthService 共享（计数列唯一写入口，spec #297）
	c.forumCn = core.NewForumCounter()
	c.authSvc = auth.NewService(db, c.sess, c.forumCn,
		cfg.DefaultPasswords.Admin, cfg.DefaultPasswords.Tutor, cfg.DefaultPasswords.Student, logger)
	c.codeSvc = auth.NewVerifyCodeService(db, c.authSvc, cfg.EmailCodeTTL, &auth.RedisAuthCodeStore{}, logger)
	c.captchaSvc = captcha.NewService(captcha.RedisStore{})
	c.emailCh = auth.NewEmailChannel(cfg.SMTP, cfg.IsProd(), logger)
	// 邮件发送器单点（spec #449 决定 15）：联系方式交换与投递通知共用，不再注入 nil 只写日志。
	c.mailSender = core.NewMailSender(cfg.SMTP, cfg.IsProd(), logger)
	c.phoneCh = auth.NewSmsChannel(cfg.SMS, cfg.IsProd(), logger)
	c.wechatAuthSvc = auth.NewWechatService(cfg.Wechat.MiniProgram, db, c.authSvc, logger)
	c.fileSvc = filestore.NewFileStore(cfg.LibreOfficeSidecarURL, st, logger)
	c.slideRenderer = course.NewSlideRenderer(cfg.LibreOfficeSidecarURL, st, logger)
	c.notifSvc = notification.NewService(db, logger)
	c.reviewSvc = auth.NewProfileReviewService(db, c.notifSvc, st, logger)
	c.authSvc.SetProfileReviewService(c.reviewSvc)
	c.aiConfigSvc = aiassistant.NewConfigService(db, cfg.SecretKey, logger)
	// 积分服务唯一实例：积分端点与真题卷权益校验共用
	c.pointsSvc = points.NewService(db, logger, clock.Real(), c.notifSvc)
	// 单一模型端口（ADR-0029 T2）：唯一 eino adapter 实例，阻塞/流式消费方共享同一 client 签名缓存。
	// 计量闸门（ADR-0031）作为装饰器挂在该端口上：所有 LLM 消费（含会话自动命名）过同一道闸，
	// 生产 meter 即积分域 *points.Service（预检与扣费下限同源），装配单点在此。
	// 第二实现：外部诊断 RAG 助手（fault_diagnosis）经 routing adapter 按功能键分发
	// （baseURL 来自 cfg.DiagnosisAssistantURL，不走管理端模型绑定）。
	aiRouting := aiassistant.NewRoutingModel(
		aiassistant.NewEinoModel(c.aiConfigSvc, logger),
		aiassistant.NewDiagnosisAssistantModel(cfg.DiagnosisAssistantURL, logger),
	)
	c.aiModelPort = aiassistant.NewMeteredModel(aiRouting, c.pointsSvc, logger)
	c.aiSvc = aiassistant.NewGenerationService(db, c.aiModelPort, logger)
	c.contentGenSvc = core.NewContentGenerateService(db, c.aiSvc, logger)
	// 联系方式交换唯一实例：申请/授权状态机（EnsureApproved）与投递侧共用（ADR-0027 C5）
	c.contactSvc = core.NewContactService(db, logger, c.notifSvc, c.mailSender)
	return c
}
