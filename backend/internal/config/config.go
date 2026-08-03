// Package config 负责加载与校验应用配置。
// 所有配置通过环境变量注入。
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
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
	LogLevel          string
	LogFormat         string
	LogOutput         string
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

// Load 从环境变量加载配置。非 production 环境会自动加载 .env 文件。
func Load() (*Config, error) {
	appEnv := getenv("APP_ENV", "development")
	if appEnv != "production" {
		_ = godotenv.Load()
	}

	maxMB, _ := strconv.Atoi(getenv("MAX_CONTENT_LENGTH_MB", "250"))
	jwtHours, _ := strconv.Atoi(getenv("JWT_EXPIRES_HOURS", "24"))
	valuationDBMaxOpen, _ := strconv.Atoi(getenv("VALUATION_DB_MAX_OPEN_CONNS", "20"))
	valuationDBMaxIdle, _ := strconv.Atoi(getenv("VALUATION_DB_MAX_IDLE_CONNS", "5"))
	valuationDBLifetime, _ := strconv.Atoi(getenv("VALUATION_DB_CONN_MAX_LIFETIME", "3600"))
	redisPoolSize, _ := strconv.Atoi(getenv("REDIS_POOL_SIZE", "10"))
	redisDB, _ := strconv.Atoi(getenv("REDIS_DB", "0"))
	redisMinIdle, _ := strconv.Atoi(getenv("REDIS_MIN_IDLE_CONNS", "3"))
	redisMaxRetries, _ := strconv.Atoi(getenv("REDIS_MAX_RETRIES", "3"))
	redisDialTimeout := getDuration("REDIS_DIAL_TIMEOUT", 5*time.Second)
	redisReadTimeout := getDuration("REDIS_READ_TIMEOUT", 3*time.Second)
	redisWriteTimeout := getDuration("REDIS_WRITE_TIMEOUT", 3*time.Second)
	redisPoolTimeout := getDuration("REDIS_POOL_TIMEOUT", 4*time.Second)
	redisIdleTimeout := getDuration("REDIS_IDLE_TIMEOUT", 5*time.Minute)

	// 限流配置：production 默认开启，其他环境默认关闭
	rateLimitEnabled := getenv("RATE_LIMIT_ENABLED", "") == "true" ||
		(appEnv == "production" && getenv("RATE_LIMIT_ENABLED", "true") != "false")
	rateLimitRPS, _ := strconv.ParseFloat(getenv("RATE_LIMIT_RPS", "20"), 64)
	if rateLimitRPS <= 0 {
		rateLimitRPS = 20
	}
	rateLimitBurst, _ := strconv.Atoi(getenv("RATE_LIMIT_BURST", "40"))
	if rateLimitBurst <= 0 {
		rateLimitBurst = 40
	}

	cfg := &Config{
		AppEnv:          appEnv,
		Port:            getenv("PORT", "8080"),
		SecretKey:       getenv("SECRET_KEY", "dev-secret-key"),
		JWTSecretKey:    getenv("JWT_SECRET_KEY", "jwt-secret-key"),
		JWTExpiresHours: jwtHours,
		DatabaseURL:     getenv("DATABASE_URL", ""),
		// 本地开发默认允许所有子域名 origin（生产环境必须通过 CORS_ORIGINS 注入实际域名）
		CORSOrigins: splitOrigins(getenv("CORS_ORIGINS",
			"http://localhost:5173,http://localhost:5174,"+
				"http://training.localhost:5173,http://valuation.localhost:5173,"+
				"http://mentor.localhost:5173,http://manage.localhost:5173")),
		UploadFolder:     getenv("UPLOAD_FOLDER", ""),
		VolumeMountPath:  getenv("VOLUME_MOUNT_PATH", ""),
		MaxContentLength: int64(maxMB) * 1024 * 1024,
		// LibreOffice sidecar HTTP 地址;为空则降级到本地 exec(向后兼容)
		LibreOfficeSidecarURL: getenv("LIBREOFFICE_SIDECAR_URL", ""),
		Storage: StorageConfig{
			Driver:            getenv("STORAGE_DRIVER", "local"),
			R2AccountID:       getenv("R2_ACCOUNT_ID", ""),
			R2AccessKeyID:     getenv("R2_ACCESS_KEY_ID", ""),
			R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY", ""),
			R2Bucket:          getenv("R2_BUCKET", ""),
			R2PublicDomain:    getenv("R2_PUBLIC_DOMAIN", ""),
		},
		AIAPIKey:  getenvChainDef("", "AI_API_KEY", "DEEPSEEK_API_KEY", "ZHIPU_API_KEY", "OPENAI_API_KEY"),
		AIBaseURL: getenvChainDef("https://api.deepseek.com", "AI_BASE_URL", "DEEPSEEK_API_URL", "ZHIPU_BASE_URL"),
		AIModel:   getenvChainDef("deepseek-v4-flash", "AI_MODEL", "MODEL", "ZHIPU_MODEL"),
		Valuation: ValuationConfig{
			PDFOutputDir:      getenv("VALUATION_PDF_OUTPUT_DIR", "storage/reports"),
			LogLevel:          getenv("VALUATION_LOG_LEVEL", "info"),
			LogFormat:         getenv("VALUATION_LOG_FORMAT", "console"),
			LogOutput:         getenv("VALUATION_LOG_OUTPUT", "stdout"),
			DBMaxOpenConns:    valuationDBMaxOpen,
			DBMaxIdleConns:    valuationDBMaxIdle,
			DBConnMaxLifetime: valuationDBLifetime,
		},
		Redis: RedisConfig{
			Addr:         getenv("REDIS_ADDR", "localhost:6379"),
			Password:     getenv("REDIS_PASSWORD", ""),
			DB:           redisDB,
			PoolSize:     redisPoolSize,
			MinIdleConns: redisMinIdle,
			MaxRetries:   redisMaxRetries,
			Prefix:       getenv("REDIS_KEY_PREFIX", "fl:"),
			DialTimeout:  redisDialTimeout,
			ReadTimeout:  redisReadTimeout,
			WriteTimeout: redisWriteTimeout,
			PoolTimeout:  redisPoolTimeout,
			IdleTimeout:  redisIdleTimeout,
		},
		RateLimit: RateLimitConfig{
			Enabled: rateLimitEnabled,
			RPS:     rateLimitRPS,
			Burst:   rateLimitBurst,
		},
		DefaultPasswords: DefaultPasswordsConfig{
			Admin:   getenv("ADMIN_DEFAULT_PASSWORD", "admin123"),
			Tutor:   getenv("TUTOR_DEFAULT_PASSWORD", "tutor123"),
			Student: getenv("STUDENT_DEFAULT_PASSWORD", "student123"),
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

	// 生产环境 CORS 配置自检：若仍使用本地开发默认值或为空，
	// 前端跨域请求会被浏览器拦截。提前在日志告警，便于定位。
	if cfg.IsProd() {
		if len(cfg.CORSOrigins) == 0 {
			slog.Warn("CORS_ORIGINS 为空，生产环境前端跨域请求将被浏览器拦截；请在部署环境变量中设置前端页面源")
		}
		for _, o := range cfg.CORSOrigins {
			if strings.Contains(o, "localhost") {
				slog.Warn("CORS_ORIGINS 仍包含本地开发地址，生产环境前端跨域可能被拦截", "origins", cfg.CORSOrigins)
			}
		}
	}

	return cfg, nil
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

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvChainDef 按顺序返回第一个非空环境变量，全部为空时返回 def。
// 用于 AI 配置字段的多名称兼容回退（如 AI_API_KEY > DEEPSEEK_API_KEY > ZHIPU_API_KEY）。
func getenvChainDef(def string, keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return def
}

// getDuration 从环境变量读取 time.Duration，支持 "5s"/"5000ms"/"5m" 等格式。
// 解析失败或未设置时返回 def。
func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
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
