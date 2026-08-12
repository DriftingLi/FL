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
	AppEnv           string
	Port             string
	SecretKey        string
	JWTSecretKey     string
	JWTExpiresHours  int
	DatabaseURL      string
	CORSOrigins      []string
	UploadFolder     string
	VolumeMountPath  string
	MaxContentLength int64
	// LibreOfficeSidecarURL LibreOffice sidecar HTTP 地址(如 http://libreoffice:8000)。
	// 为空时降级到本地 exec 调用(向后兼容)。
	LibreOfficeSidecarURL string
	// Storage 文件存储配置（local 本地磁盘 / r2 Cloudflare R2 对象存储）。
	Storage StorageConfig
	// AI 服务配置（OpenAI 兼容格式）。优先级：AI_* > DEEPSEEK_* > ZHIPU_* > OPENAI_API_KEY。
	AIAPIKey         string
	AIBaseURL        string
	AIModel          string
	Valuation        ValuationConfig
	Redis            RedisConfig
	RateLimit        RateLimitConfig
	DefaultPasswords DefaultPasswordsConfig
	// SMTP 邮件发送配置（邮箱验证码注册/登录）。
	SMTP SMTPConfig
	// EmailCodeTTL 邮箱验证码有效期（EMAIL_CODE_TTL_MINUTES，默认 5 分钟）。
	EmailCodeTTL time.Duration
	// Wechat 微信开放平台配置（扫码登录，授权信息待接入）。
	Wechat WechatConfig
	// AuthCookie 登录态 Cookie 配置（父域名共享登录）。
	AuthCookie AuthCookieConfig
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

// WechatConfig 微信开放平台配置（扫码登录框架占位，授权信息待接入）。
type WechatConfig struct {
	AppID     string // WECHAT_APP_ID
	AppSecret string // WECHAT_APP_SECRET
}

// AuthCookieConfig 登录态 Cookie 配置。
// Domain 需配置为父域名（如 localhost / example.com），各子域名才能共享登录态。
type AuthCookieConfig struct {
	Name   string // AUTH_COOKIE_NAME，默认 hrwai_token
	Domain string // AUTH_COOKIE_DOMAIN，默认 localhost
	Secure bool   // AUTH_COOKIE_SECURE，生产默认 true（HTTPS 才发送）
}

// StorageConfig 文件存储配置。
// Driver 为 "local" 时使用本地磁盘（UploadFolder），为 "r2" 时使用 Cloudflare R2。
type StorageConfig struct {
	Driver            string // STORAGE_DRIVER，默认 "local"
	R2AccountID       string // R2_ACCOUNT_ID
	R2AccessKeyID     string // R2_ACCESS_KEY_ID
	R2SecretAccessKey string // R2_SECRET_ACCESS_KEY
	R2Bucket          string // R2_BUCKET
	R2PublicDomain    string // R2_PUBLIC_DOMAIN，如 https://cdn.example.com
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

// setDefaults 集中定义全部配置默认值。
func setDefaults() {
	viper.SetDefault("app_env", "development")
	viper.SetDefault("port", "8080")
	viper.SetDefault("secret_key", "dev-secret-key")
	viper.SetDefault("jwt_secret_key", "jwt-secret-key")
	viper.SetDefault("jwt_expires_hours", 24)
	viper.SetDefault("max_content_length_mb", 250)
	viper.SetDefault("database_url", "")
	viper.SetDefault("cors_origins", "http://localhost:5173,http://localhost:5174,"+
		"http://training.localhost:5173,http://valuation.localhost:5173,"+
		"http://mentor.localhost:5173,http://manage.localhost:5173")
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
	viper.SetDefault("redis_pool_size", 10)
	viper.SetDefault("redis_min_idle_conns", 3)
	viper.SetDefault("redis_max_retries", 3)
	viper.SetDefault("redis_key_prefix", "fl:")
	viper.SetDefault("redis_dial_timeout", "5s")
	viper.SetDefault("redis_read_timeout", "3s")
	viper.SetDefault("redis_write_timeout", "3s")
	viper.SetDefault("redis_pool_timeout", "4s")
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
	viper.SetDefault("wechat_app_id", "")
	viper.SetDefault("wechat_app_secret", "")
	viper.SetDefault("auth_cookie_name", "hrwai_token")
	viper.SetDefault("auth_cookie_domain", "localhost")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("log_format", "console")
	viper.SetDefault("log_dir", "")
	viper.SetDefault("log_max_size_mb", 100)
	viper.SetDefault("log_max_backups", 7)
	viper.SetDefault("log_max_age_days", 30)
	viper.SetDefault("log_compress", true)
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

	cfg := &Config{
		AppEnv:          appEnv,
		Port:            viper.GetString("port"),
		SecretKey:       viper.GetString("secret_key"),
		JWTSecretKey:    viper.GetString("jwt_secret_key"),
		JWTExpiresHours: positiveInt("jwt_expires_hours", 24),
		DatabaseURL:     viper.GetString("database_url"),
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
			PoolSize:     positiveInt("redis_pool_size", 10),
			MinIdleConns: positiveInt("redis_min_idle_conns", 3),
			MaxRetries:   positiveInt("redis_max_retries", 3),
			Prefix:       viper.GetString("redis_key_prefix"),
			DialTimeout:  positiveDuration("redis_dial_timeout", 5*time.Second),
			ReadTimeout:  positiveDuration("redis_read_timeout", 3*time.Second),
			WriteTimeout: positiveDuration("redis_write_timeout", 3*time.Second),
			PoolTimeout:  positiveDuration("redis_pool_timeout", 4*time.Second),
			IdleTimeout:  positiveDuration("redis_idle_timeout", 5*time.Minute),
		},
		RateLimit: RateLimitConfig{
			Enabled: rateLimitEnabled,
			RPS:     positiveFloat("rate_limit_rps", 20),
			Burst:   positiveInt("rate_limit_burst", 40),
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
		EmailCodeTTL: time.Duration(positiveInt("email_code_ttl_minutes", 5)) * time.Minute,
		Wechat: WechatConfig{
			AppID:     viper.GetString("wechat_app_id"),
			AppSecret: viper.GetString("wechat_app_secret"),
		},
		AuthCookie: AuthCookieConfig{
			Name:   viper.GetString("auth_cookie_name"),
			Domain: viper.GetString("auth_cookie_domain"),
			Secure: authCookieSecure,
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
	if c.Storage.Driver == "r2" {
		if c.Storage.R2AccountID == "" {
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

// JWTExpiry 返回 JWT 过期时长。
func (c *Config) JWTExpiry() time.Duration {
	return time.Duration(c.JWTExpiresHours) * time.Hour
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
