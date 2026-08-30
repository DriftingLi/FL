// Package config 负责加载与校验应用配置。
// 所有配置通过环境变量注入（Viper AutomaticEnv + 集中 SetDefault 默认值）。
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 持有应用运行所需的全部配置。
type Config struct {
	AppEnv                string
	Port                  string
	SecretKey             string
	JWTSecretKey          string
	JWTExpiresHours       int
	JWTRefreshExpiresDays int
	DatabaseURL           string
	CORSOrigins           []string
	UploadFolder          string
	VolumeMountPath       string
	MaxContentLength      int64
	// LibreOfficeSidecarURL LibreOffice sidecar HTTP 地址(如 http://libreoffice:8000)。
	// 为空时降级到本地 exec 调用(向后兼容)。
	LibreOfficeSidecarURL string
	// Storage 文件存储配置（local 本地磁盘 / r2 Cloudflare R2 对象存储）。
	Storage StorageConfig
	// AI 服务配置（OpenAI 兼容格式）。优先级：AI_* > DEEPSEEK_* > ZHIPU_* > OPENAI_API_KEY。
	AIAPIKey  string
	AIBaseURL string
	AIModel   string
	Valuation ValuationConfig
	Redis     RedisConfig
	RateLimit RateLimitConfig
	// CaptchaEnabled 图形验证码开关（生产默认开启、其他默认关闭；显式 true/false 可覆盖）。
	CaptchaEnabled   bool
	Swagger          SwaggerConfig
	DefaultPasswords DefaultPasswordsConfig
	// SMTP 邮件发送配置（邮箱验证码注册/登录）。
	SMTP SMTPConfig
	// SMS 腾讯云短信配置（手机验证码注册/登录）。
	SMS SMSConfig
	// EmailCodeTTL 邮箱验证码有效期（EMAIL_CODE_TTL_MINUTES，默认 5 分钟）。
	EmailCodeTTL time.Duration
	// Wechat 微信登录凭证配置——小程序与开放平台严格区分：
	//   - MiniProgram 小程序凭证（code2session 登录，wx-login 消费）
	//   - OpenPlatform 开放平台网页应用凭证（扫码登录，占位待接入）
	Wechat WechatConfig
	// AuthCookie 登录态 Cookie 配置（父域名共享登录）。
	AuthCookie AuthCookieConfig
	// RecruiterCookie 企业招聘者登录态 Cookie（host-only，不共享父域）。
	RecruiterCookie RecruiterCookieConfig
	// Log 统一系统运行日志配置（zap）。
	Log LogConfig
}

// SMTPConfig SMTP 邮件发送配置。
type SMTPConfig struct {
	Host     string // SMTP_HOST
	Port     int    // SMTP_PORT，默认 587
	Username string // SMTP_USERNAME
	Password string // SMTP_PASSWORD
	From     string // SMTP_FROM 发件人邮箱
	FromName string // SMTP_FROM_NAME，默认 和润天下
}

// SMSConfig 腾讯云短信配置（手机验证码注册/登录）。
// SecretID/SecretKey 为云 API 密钥（cam/capi），不是短信控制台应用详情页的 AppKey。
// 模板按用途拆分，各对应一个已审核模板：登录模板双参数（验证码+分钟数），其余单参数。
type SMSConfig struct {
	SecretID     string // TENCENT_SMS_SECRET_ID 腾讯云 API 密钥 SecretId
	SecretKey    string // TENCENT_SMS_SECRET_KEY 腾讯云 API 密钥 SecretKey
	SdkAppID     string // TENCENT_SMS_SDK_APP_ID 短信应用 SdkAppId
	SignName     string // TENCENT_SMS_SIGN_NAME 已审核通过的短信签名
	Region       string // TENCENT_SMS_REGION 接入地域，默认 ap-guangzhou
	TplRegister  string // TENCENT_SMS_TEMPLATE_REGISTER 注册验证码模板（{1}=验证码）
	TplLogin     string // TENCENT_SMS_TEMPLATE_LOGIN 登录验证码模板（{1}=验证码 {2}=有效分钟数）
	TplPassword  string // TENCENT_SMS_TEMPLATE_PASSWORD 密码重置/修改密码模板（{1}=验证码）
	TplBindPhone string // TENCENT_SMS_TEMPLATE_BIND_PHONE 绑定/修改手机号、修改账号模板（{1}=验证码）
}

// Configured 返回短信通道是否已完整配置（生产发送必需）。
func (c SMSConfig) Configured() bool {
	return c.SecretID != "" && c.SecretKey != "" && c.SdkAppID != "" && c.SignName != "" &&
		c.TplRegister != "" && c.TplLogin != "" && c.TplPassword != "" && c.TplBindPhone != ""
}

// WechatAppConfig 一组微信应用凭证（AppID 为公开标识，AppSecret 必须仅存服务端）。
type WechatAppConfig struct {
	AppID     string // AppID，如 wx 開頭的小程序 ID
	AppSecret string // AppSecret
}

// Configured 返回该组凭证是否已配置。
func (c WechatAppConfig) Configured() bool {
	return c.AppID != "" && c.AppSecret != ""
}

// WechatConfig 微信登录凭证——小程序与开放平台网页应用两套独立凭证，
// 不可混用：code2session 只认小程序凭证，扫码登录只认开放平台网页应用凭证。
type WechatConfig struct {
	MiniProgram  WechatAppConfig // WECHAT_MINI_PROGRAM_APP_ID / WECHAT_MINI_PROGRAM_APP_SECRET
	OpenPlatform WechatAppConfig // WECHAT_OPEN_PLATFORM_APP_ID / WECHAT_OPEN_PLATFORM_APP_SECRET（扫码登录占位）
}

// AuthCookieConfig 登录态 Cookie 配置。
// Domain 需配置为父域名（如 localhost / example.com），各子域名才能共享登录态。
type AuthCookieConfig struct {
	Name   string // AUTH_COOKIE_NAME，默认 hrwai_token
	Domain string // AUTH_COOKIE_DOMAIN，默认 localhost
	Secure bool   // AUTH_COOKIE_SECURE，生产默认 true（HTTPS 才发送）
}

// RecruiterCookieConfig 招聘者登录态 Cookie 配置（host-only 隔离，不共享父域）。
// 与学员侧 hrwai_token 完全独立，Domain 为空表示 host-only。
type RecruiterCookieConfig struct {
	Name   string // RECRUITER_COOKIE_NAME，默认 recruiter_token
	Domain string // RECRUITER_COOKIE_DOMAIN，默认 ""（host-only）
	Secure bool   // RECRUITER_COOKIE_SECURE，生产默认 true
}

// StorageConfig 文件存储配置。
// Driver 为 "local" 时使用本地磁盘（UploadFolder），为 "r2" 时使用对象存储
// （S3 兼容：缺失 R2Endpoint 时指向 Cloudflare R2，配置时指向自建 RGW 等）。
type StorageConfig struct {
	Driver            string // STORAGE_DRIVER，默认 "local"
	R2AccountID       string // R2_ACCOUNT_ID
	R2AccessKeyID     string // R2_ACCESS_KEY_ID
	R2SecretAccessKey string // R2_SECRET_ACCESS_KEY
	R2Bucket          string // R2_BUCKET
	R2PublicDomain    string // R2_PUBLIC_DOMAIN，如 https://cdn.example.com
	R2Endpoint        string // R2_ENDPOINT，空用 R2 默认 endpoint；自建 S3（RGW）时配置
}

// DefaultPasswordsConfig 默认账号密码配置，生产环境必须覆盖开发默认值。
type DefaultPasswordsConfig struct {
	Admin   string
	Tutor   string
	Student string
}

// ValuationConfig 残值评估模块配置。
type ValuationConfig struct {
	PDFOutputDir      string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime int
}

// RedisConfig Redis 缓存配置。
type RedisConfig struct {
	Addr         string        // REDIS_ADDR，默认 "localhost:6379"
	Password     string        // REDIS_PASSWORD，生产环境从环境变量注入
	DB           int           // REDIS_DB，默认 0
	PoolSize     int           // REDIS_POOL_SIZE，默认 10
	MinIdleConns int           // REDIS_MIN_IDLE_CONNS，默认 3
	MaxRetries   int           // REDIS_MAX_RETRIES，默认 3
	Prefix       string        // REDIS_KEY_PREFIX，统一 key 前缀，默认 "fl:"
	DialTimeout  time.Duration // REDIS_DIAL_TIMEOUT，默认 5s
	ReadTimeout  time.Duration // REDIS_READ_TIMEOUT，默认 3s
	WriteTimeout time.Duration // REDIS_WRITE_TIMEOUT，默认 3s
	PoolTimeout  time.Duration // REDIS_POOL_TIMEOUT，默认 4s
	IdleTimeout  time.Duration // REDIS_IDLE_TIMEOUT，默认 5m
}

// RateLimitConfig 限流配置（基于客户端 IP 的 token bucket）。
// 生产环境建议开启，防止暴力枚举/撞库/爬虫。
type RateLimitConfig struct {
	Enabled bool    // RATE_LIMIT_ENABLED，默认 production 开启、其他关闭
	RPS     float64 // RATE_LIMIT_RPS，每 IP 每秒请求数，默认 20
	Burst   int     // RATE_LIMIT_BURST，突发上限，默认 40
}

// LogConfig 统一系统运行日志（zap）配置。
type LogConfig struct {
	Level      string // LOG_LEVEL，debug|info|warn|error，默认 info
	Format     string // LOG_FORMAT，console|json，默认 console
	OutputDir  string // LOG_DIR，日志文件目录；为空仅 stdout，非空双写（生产挂载卷）
	MaxSizeMB  int    // LOG_MAX_SIZE_MB，单文件轮转上限，默认 100
	MaxBackups int    // LOG_MAX_BACKUPS，保留旧文件份数，默认 7
	MaxAgeDays int    // LOG_MAX_AGE_DAYS，旧文件最长保留天数，默认 30
	Compress   bool   // LOG_COMPRESS，轮转压缩归档，默认 true
}

// SwaggerConfig Swagger 文档与 BasicAuth 配置（C 方案）。
// Enabled 由 SWAGGER_ENABLED 控制（未显式设置时 dev 默认 true、prod 默认 false）；
// User/Pass 由 SWAGGER_USER/PASS 注入，Enabled=true 且两者非空时 /swagger/*any 走 BasicAuth。
type SwaggerConfig struct {
	Enabled bool   // SWAGGER_ENABLED
	User    string // SWAGGER_USER
	Pass    string // SWAGGER_PASS
}

// setDefaults 集中定义全部配置默认值。
func setDefaults() {
	viper.SetDefault("app_env", "development")
	viper.SetDefault("port", "8080")
	viper.SetDefault("secret_key", "dev-secret-key")
	viper.SetDefault("jwt_secret_key", "jwt-secret-key")
	viper.SetDefault("jwt_expires_hours", 2)
	viper.SetDefault("jwt_refresh_expires_days", 7)
	viper.SetDefault("max_content_length_mb", 250)
	viper.SetDefault("database_url", "")
	viper.SetDefault("cors_origins", "http://localhost:5173,http://localhost:5174,"+
		"http://training.localhost:5173,http://valuation.localhost:5173,"+
		"http://mentor.localhost:5173,http://manage.localhost:5173,http://recruit.localhost:5173")
	viper.SetDefault("upload_folder", "")
	viper.SetDefault("volume_mount_path", "")
	viper.SetDefault("libreoffice_sidecar_url", "")
	viper.SetDefault("storage_driver", "local")
	viper.SetDefault("r2_account_id", "")
	viper.SetDefault("r2_access_key_id", "")
	viper.SetDefault("r2_secret_access_key", "")
	viper.SetDefault("r2_bucket", "")
	viper.SetDefault("r2_public_domain", "")
	viper.SetDefault("ai_base_url", "https://api.deepseek.com")
	viper.SetDefault("ai_model", "deepseek-v4-flash")
	viper.SetDefault("valuation_pdf_output_dir", "storage/reports")
	viper.SetDefault("valuation_db_max_open_conns", 20)
	viper.SetDefault("valuation_db_max_idle_conns", 5)
	viper.SetDefault("valuation_db_conn_max_lifetime", 3600)
	viper.SetDefault("redis_addr", "localhost:6379")
	viper.SetDefault("redis_password", "")
	viper.SetDefault("redis_db", 0)
	viper.SetDefault("redis_pool_size", 20)
	viper.SetDefault("redis_min_idle_conns", 5)
	viper.SetDefault("redis_max_retries", 3)
	viper.SetDefault("redis_key_prefix", "fl:")
	viper.SetDefault("redis_dial_timeout", "2s")
	viper.SetDefault("redis_read_timeout", "2s")
	viper.SetDefault("redis_write_timeout", "2s")
	viper.SetDefault("redis_pool_timeout", "3s")
	viper.SetDefault("redis_idle_timeout", "5m")
	viper.SetDefault("rate_limit_rps", 20.0)
	viper.SetDefault("rate_limit_burst", 40)
	viper.SetDefault("admin_default_password", "admin123")
	viper.SetDefault("tutor_default_password", "tutor123")
	viper.SetDefault("student_default_password", "student123")
	viper.SetDefault("smtp_host", "")
	viper.SetDefault("smtp_port", 587)
	viper.SetDefault("smtp_username", "")
	viper.SetDefault("smtp_password", "")
	viper.SetDefault("smtp_from", "")
	viper.SetDefault("smtp_from_name", "和润天下")
	viper.SetDefault("email_code_ttl_minutes", 5)
	viper.SetDefault("tencent_sms_secret_id", "")
	viper.SetDefault("tencent_sms_secret_key", "")
	viper.SetDefault("tencent_sms_sdk_app_id", "")
	viper.SetDefault("tencent_sms_sign_name", "")
	viper.SetDefault("tencent_sms_template_register", "")
	viper.SetDefault("tencent_sms_template_login", "")
	viper.SetDefault("tencent_sms_template_password", "")
	viper.SetDefault("tencent_sms_template_bind_phone", "")
	viper.SetDefault("tencent_sms_region", "ap-guangzhou")
	viper.SetDefault("wechat_mini_program_app_id", "")
	viper.SetDefault("wechat_mini_program_app_secret", "")
	viper.SetDefault("wechat_open_platform_app_id", "")
	viper.SetDefault("wechat_open_platform_app_secret", "")
	viper.SetDefault("auth_cookie_name", "hrwai_token")
	viper.SetDefault("auth_cookie_domain", "localhost")
	viper.SetDefault("recruiter_cookie_name", "recruiter_token")
	viper.SetDefault("recruiter_cookie_domain", "")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_format", "console")
	viper.SetDefault("log_dir", "")
	viper.SetDefault("log_max_size_mb", 100)
	viper.SetDefault("log_max_backups", 7)
	viper.SetDefault("log_max_age_days", 30)
	viper.SetDefault("log_compress", true)
	viper.SetDefault("swagger_user", "")
	viper.SetDefault("swagger_pass", "")
}

// Load 从环境变量加载配置。非 production 环境会自动加载 .env 文件。
func Load() (*Config, error) {
	viper.AutomaticEnv()
	setDefaults()

	appEnv := viper.GetString("app_env")
	if appEnv != "production" {
		_ = godotenv.Load()
	}

	// 限流与 Cookie 安全位特殊：显式 "true" 任意环境开启；
	// 生产默认开启，且仅显式 "false" 可关闭（需区分"未设置"与"显式空值"）。
	rateLimitEnabled := envBoolOr("rate_limit_enabled", appEnv == "production")
	authCookieSecure := envBoolOr("auth_cookie_secure", appEnv == "production")
	captchaEnabled := envBoolOr("captcha_enabled", appEnv == "production")
	swaggerEnabled := envBoolOr("swagger_enabled", appEnv != "production")

	cfg := &Config{
		AppEnv:                appEnv,
		Port:                  viper.GetString("port"),
		SecretKey:             viper.GetString("secret_key"),
		JWTSecretKey:          viper.GetString("jwt_secret_key"),
		JWTExpiresHours:       positiveInt("jwt_expires_hours", 2),
		JWTRefreshExpiresDays: positiveInt("jwt_refresh_expires_days", 7),
		DatabaseURL:           viper.GetString("database_url"),
		// 本地开发默认允许所有子域名 origin（生产环境必须通过 CORS_ORIGINS 注入实际域名）
		CORSOrigins:     splitOrigins(viper.GetString("cors_origins")),
		UploadFolder:    viper.GetString("upload_folder"),
		VolumeMountPath: viper.GetString("volume_mount_path"),
		// LibreOffice sidecar HTTP 地址;为空则降级到本地 exec(向后兼容)
		MaxContentLength:      int64(positiveInt("max_content_length_mb", 250)) * 1024 * 1024,
		LibreOfficeSidecarURL: viper.GetString("libreoffice_sidecar_url"),
		Storage: StorageConfig{
			Driver:            viper.GetString("storage_driver"),
			R2AccountID:       viper.GetString("r2_account_id"),
			R2AccessKeyID:     viper.GetString("r2_access_key_id"),
			R2SecretAccessKey: viper.GetString("r2_secret_access_key"),
			R2Bucket:          viper.GetString("r2_bucket"),
			R2PublicDomain:    viper.GetString("r2_public_domain"),
			R2Endpoint:        viper.GetString("r2_endpoint"),
		},
		AIAPIKey:  viper.GetString("ai_api_key"),
		AIBaseURL: viper.GetString("ai_base_url"),
		AIModel:   viper.GetString("ai_model"),
		Valuation: ValuationConfig{
			PDFOutputDir:      viper.GetString("valuation_pdf_output_dir"),
			DBMaxOpenConns:    positiveInt("valuation_db_max_open_conns", 20),
			DBMaxIdleConns:    positiveInt("valuation_db_max_idle_conns", 5),
			DBConnMaxLifetime: positiveInt("valuation_db_conn_max_lifetime", 3600),
		},
		Redis: RedisConfig{
			Addr:         viper.GetString("redis_addr"),
			Password:     viper.GetString("redis_password"),
			DB:           nonNegInt("redis_db", 0),
			PoolSize:     positiveInt("redis_pool_size", 20),
			MinIdleConns: positiveInt("redis_min_idle_conns", 5),
			MaxRetries:   positiveInt("redis_max_retries", 3),
			Prefix:       viper.GetString("redis_key_prefix"),
			DialTimeout:  positiveDuration("redis_dial_timeout", 2*time.Second),
			ReadTimeout:  positiveDuration("redis_read_timeout", 2*time.Second),
			WriteTimeout: positiveDuration("redis_write_timeout", 2*time.Second),
			PoolTimeout:  positiveDuration("redis_pool_timeout", 3*time.Second),
			IdleTimeout:  positiveDuration("redis_idle_timeout", 5*time.Minute),
		},
		RateLimit: RateLimitConfig{
			Enabled: rateLimitEnabled,
			RPS:     positiveFloat("rate_limit_rps", 20),
			Burst:   positiveInt("rate_limit_burst", 40),
		},
		CaptchaEnabled: captchaEnabled,
		Swagger: SwaggerConfig{
			Enabled: swaggerEnabled,
			User:    viper.GetString("swagger_user"),
			Pass:    viper.GetString("swagger_pass"),
		},
		DefaultPasswords: DefaultPasswordsConfig{
			Admin:   viper.GetString("admin_default_password"),
			Tutor:   viper.GetString("tutor_default_password"),
			Student: viper.GetString("student_default_password"),
		},
		SMTP: SMTPConfig{
			Host:     viper.GetString("smtp_host"),
			Port:     positiveInt("smtp_port", 587),
			Username: viper.GetString("smtp_username"),
			Password: viper.GetString("smtp_password"),
			From:     viper.GetString("smtp_from"),
			FromName: viper.GetString("smtp_from_name"),
		},
		SMS: SMSConfig{
			SecretID:     viper.GetString("tencent_sms_secret_id"),
			SecretKey:    viper.GetString("tencent_sms_secret_key"),
			SdkAppID:     viper.GetString("tencent_sms_sdk_app_id"),
			SignName:     viper.GetString("tencent_sms_sign_name"),
			Region:       viper.GetString("tencent_sms_region"),
			TplRegister:  viper.GetString("tencent_sms_template_register"),
			TplLogin:     viper.GetString("tencent_sms_template_login"),
			TplPassword:  viper.GetString("tencent_sms_template_password"),
			TplBindPhone: viper.GetString("tencent_sms_template_bind_phone"),
		},
		EmailCodeTTL: time.Duration(positiveInt("email_code_ttl_minutes", 5)) * time.Minute,
		Wechat: WechatConfig{
			MiniProgram:  wechatAppConfig("wechat_mini_program_app_id", "wechat_mini_program_app_secret", "wechat_app_id", "wechat_app_secret"),
			OpenPlatform: wechatAppConfig("wechat_open_platform_app_id", "wechat_open_platform_app_secret"),
		},
		AuthCookie: AuthCookieConfig{
			Name:   viper.GetString("auth_cookie_name"),
			Domain: viper.GetString("auth_cookie_domain"),
			Secure: authCookieSecure,
		},
		RecruiterCookie: RecruiterCookieConfig{
			Name:   viper.GetString("recruiter_cookie_name"),
			Domain: viper.GetString("recruiter_cookie_domain"),
			Secure: envBoolOr("recruiter_cookie_secure", appEnv == "production"),
		},
		Log: LogConfig{
			Level:      viper.GetString("log_level"),
			Format:     viper.GetString("log_format"),
			OutputDir:  viper.GetString("log_dir"),
			MaxSizeMB:  positiveInt("log_max_size_mb", 100),
			MaxBackups: positiveInt("log_max_backups", 7),
			MaxAgeDays: positiveInt("log_max_age_days", 30),
			Compress:   viper.GetBool("log_compress"),
		},
	}

	// 默认上传目录
	if cfg.UploadFolder == "" {
		if cfg.VolumeMountPath != "" {
			cfg.UploadFolder = cfg.VolumeMountPath + "/uploads"
		} else {
			cfg.UploadFolder = "static/uploads"
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// CORSConfigWarnings 返回生产环境 CORS 配置自检告警列表（供启动阶段以统一日志输出）。
// 若仍使用本地开发默认值或为空，前端跨域请求会被浏览器拦截。
func (c *Config) CORSConfigWarnings() []string {
	if !c.IsProd() {
		return nil
	}
	var warns []string
	if len(c.CORSOrigins) == 0 {
		warns = append(warns, "CORS_ORIGINS 为空，生产环境前端跨域请求将被浏览器拦截；请在部署环境变量中设置前端页面源")
	}
	for _, o := range c.CORSOrigins {
		if strings.Contains(o, "localhost") {
			warns = append(warns, "CORS_ORIGINS 仍包含本地开发地址，生产环境前端跨域可能被拦截: "+strings.Join(c.CORSOrigins, ","))
			break
		}
	}
	return warns
}

// Validate 在 production 环境校验必填项。
func (c *Config) Validate() error {
	if c.AppEnv != "production" {
		return nil
	}
	var missing []string
	if c.SecretKey == "" || c.SecretKey == "dev-secret-key" {
		missing = append(missing, "SECRET_KEY")
	}
	if c.JWTSecretKey == "" || c.JWTSecretKey == "jwt-secret-key" {
		missing = append(missing, "JWT_SECRET_KEY")
	}
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.Redis.Addr == "" {
		missing = append(missing, "REDIS_ADDR")
	}
	// R2 对象存储校验：driver=r2 时必填 R2 凭证
	// （自建 RGW 时 R2_ACCOUNT_ID 可给占位，endpoint 由 R2_ENDPOINT 指定）
	if c.Storage.Driver == "r2" {
		if c.Storage.R2Endpoint == "" && c.Storage.R2AccountID == "" {
			missing = append(missing, "R2_ACCOUNT_ID")
		}
		if c.Storage.R2AccessKeyID == "" {
			missing = append(missing, "R2_ACCESS_KEY_ID")
		}
		if c.Storage.R2SecretAccessKey == "" {
			missing = append(missing, "R2_SECRET_ACCESS_KEY")
		}
		if c.Storage.R2Bucket == "" {
			missing = append(missing, "R2_BUCKET")
		}
		if c.Storage.R2PublicDomain == "" {
			missing = append(missing, "R2_PUBLIC_DOMAIN")
		}
	}
	// CORS_ORIGINS 强制校验：必须显式配置，且不得包含 localhost（否则浏览器跨域会被拦截）
	if len(c.CORSOrigins) == 0 {
		missing = append(missing, "CORS_ORIGINS")
	} else {
		for _, o := range c.CORSOrigins {
			if strings.Contains(o, "localhost") {
				missing = append(missing, "CORS_ORIGINS(含localhost)")
				break
			}
		}
	}
	// 默认密码强制校验：生产环境不得使用开发默认值，防止弱口令被利用
	if c.DefaultPasswords.Admin == "admin123" {
		missing = append(missing, "ADMIN_DEFAULT_PASSWORD(仍为默认弱口令)")
	}
	if c.DefaultPasswords.Tutor == "tutor123" {
		missing = append(missing, "TUTOR_DEFAULT_PASSWORD(仍为默认弱口令)")
	}
	if c.DefaultPasswords.Student == "student123" {
		missing = append(missing, "STUDENT_DEFAULT_PASSWORD(仍为默认弱口令)")
	}
	// 子域名共享登录 Cookie 必须配置父域名（localhost 仅限开发环境）
	if c.AuthCookie.Domain == "" || c.AuthCookie.Domain == "localhost" {
		missing = append(missing, "AUTH_COOKIE_DOMAIN(必须为父域名，如 example.com)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("生产环境缺少必填配置: %s", strings.Join(missing, ", "))
	}
	return nil
}

// JWTExpiry 返回 access token 过期时长（JWT_EXPIRES_HOURS，默认 2h）。
func (c *Config) JWTExpiry() time.Duration {
	return time.Duration(c.JWTExpiresHours) * time.Hour
}

// JWTRefreshExpiry 返回 refresh token 过期时长（JWT_REFRESH_EXPIRES_DAYS，默认 7 天）。
func (c *Config) JWTRefreshExpiry() time.Duration {
	return time.Duration(c.JWTRefreshExpiresDays) * 24 * time.Hour
}

// IsProd 是否为生产环境。
func (c *Config) IsProd() bool { return c.AppEnv == "production" }

// envBoolOr 读取布尔环境变量；"true"→true、"false"→false、
// 未设置或空值→def（生产默认开启、开发默认关闭）。
func envBoolOr(key string, def bool) bool {
	v, set := os.LookupEnv(strings.ToUpper(key))
	if !set || v == "" {
		return def
	}
	return strings.EqualFold(v, "true")
}

// positiveInt 读取整数配置，非法或 <=0 时回退默认值。
func positiveInt(key string, def int) int {
	if v := viper.GetInt(key); v > 0 {
		return v
	}
	return def
}

// nonNegInt 读取非负整数配置，非法或 <0 时回退默认值。
func nonNegInt(key string, def int) int {
	if v := viper.GetInt(key); v >= 0 {
		return v
	}
	return def
}

// positiveFloat 读取浮点配置，非法或 <=0 时回退默认值。
func positiveFloat(key string, def float64) float64 {
	if v := viper.GetFloat64(key); v > 0 {
		return v
	}
	return def
}

// positiveDuration 读取时长配置，非法或 <=0 时回退默认值。
func positiveDuration(key string, def time.Duration) time.Duration {
	if v := viper.GetDuration(key); v > 0 {
		return v
	}
	return def
}

// wechatAppConfig 读取一组微信凭证；legacy 键对仅作向后兼容回退
// （旧 WECHAT_APP_ID/WECHAT_APP_SECRET 实际是小程序凭证，未区分前缀，已废弃）。
func wechatAppConfig(idKey, secretKey string, legacyKeys ...string) WechatAppConfig {
	c := WechatAppConfig{
		AppID:     viper.GetString(idKey),
		AppSecret: viper.GetString(secretKey),
	}
	if c.Configured() {
		return c
	}
	for i := 0; i+1 < len(legacyKeys); i += 2 {
		id, secret := viper.GetString(legacyKeys[i]), viper.GetString(legacyKeys[i+1])
		if id != "" && secret != "" {
			return WechatAppConfig{AppID: id, AppSecret: secret}
		}
	}
	return c
}

func splitOrigins(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
