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
	OpenAIAPIKey     string
	ZhipuAPIKey      string
	ZhipuBaseURL     string
	ZhipuModel       string
	Coze             CozeConfig
	Valuation        ValuationConfig
	Redis            RedisConfig
	DefaultPasswords DefaultPasswordsConfig
}

// DefaultPasswordsConfig 默认账号密码配置，生产环境必须覆盖开发默认值。
type DefaultPasswordsConfig struct {
	Admin   string
	Tutor   string
	Student string
}

// CozeConfig 扣子智能体 OAuth 配置。
type CozeConfig struct {
	ProjectID       string
	OAuthAppID      string
	OAuthKID        string
	OAuthPrivateKey string
	OAuthKeyPath    string
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
	JWTSecretKey      string // 估值模块独立 JWT 签名密钥
	JWTExpiresHours   int    // 估值模块独立 JWT 过期时长（小时）
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
	valuationJWTHours, _ := strconv.Atoi(getenv("VALUATION_JWT_EXPIRES_HOURS", "168")) // 默认 7 天
	redisPoolSize, _ := strconv.Atoi(getenv("REDIS_POOL_SIZE", "10"))
	redisDB, _ := strconv.Atoi(getenv("REDIS_DB", "0"))
	redisMinIdle, _ := strconv.Atoi(getenv("REDIS_MIN_IDLE_CONNS", "3"))
	redisMaxRetries, _ := strconv.Atoi(getenv("REDIS_MAX_RETRIES", "3"))
	redisDialTimeout := getDuration("REDIS_DIAL_TIMEOUT", 5*time.Second)
	redisReadTimeout := getDuration("REDIS_READ_TIMEOUT", 3*time.Second)
	redisWriteTimeout := getDuration("REDIS_WRITE_TIMEOUT", 3*time.Second)
	redisPoolTimeout := getDuration("REDIS_POOL_TIMEOUT", 4*time.Second)
	redisIdleTimeout := getDuration("REDIS_IDLE_TIMEOUT", 5*time.Minute)

	cfg := &Config{
		AppEnv:           appEnv,
		Port:             getenv("PORT", "8080"),
		SecretKey:        getenv("SECRET_KEY", "dev-secret-key"),
		JWTSecretKey:     getenv("JWT_SECRET_KEY", "jwt-secret-key"),
		JWTExpiresHours:  jwtHours,
		DatabaseURL:      getenv("DATABASE_URL", ""),
		CORSOrigins:      splitOrigins(getenv("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174")),
		UploadFolder:     getenv("UPLOAD_FOLDER", ""),
		VolumeMountPath:  getenv("VOLUME_MOUNT_PATH", ""),
		MaxContentLength: int64(maxMB) * 1024 * 1024,
		OpenAIAPIKey:     getenv("OPENAI_API_KEY", ""),
		ZhipuAPIKey:      getenv("ZHIPU_API_KEY", ""),
		ZhipuBaseURL:     getenv("ZHIPU_BASE_URL", "https://open.bigmodel.cn/api/paas/v4"),
		ZhipuModel:       getenv("ZHIPU_MODEL", "glm-4.7-flash"),
		Coze: CozeConfig{
			ProjectID:       getenv("COZE_PROJECT_ID", ""),
			OAuthAppID:      getenv("COZE_OAUTH_APP_ID", ""),
			OAuthKID:        getenv("COZE_OAUTH_KID", ""),
			OAuthPrivateKey: getenv("COZE_OAUTH_PRIVATE_KEY", ""),
			OAuthKeyPath:    getenv("COZE_OAUTH_PRIVATE_KEY_PATH", ""),
		},
		Valuation: ValuationConfig{
			PDFOutputDir:      getenv("VALUATION_PDF_OUTPUT_DIR", "storage/reports"),
			LogLevel:          getenv("VALUATION_LOG_LEVEL", "info"),
			LogFormat:         getenv("VALUATION_LOG_FORMAT", "console"),
			LogOutput:         getenv("VALUATION_LOG_OUTPUT", "stdout"),
			DBMaxOpenConns:    valuationDBMaxOpen,
			DBMaxIdleConns:    valuationDBMaxIdle,
			DBConnMaxLifetime: valuationDBLifetime,
			JWTSecretKey:      getenv("VALUATION_JWT_SECRET_KEY", "valuation-jwt-secret-key"),
			JWTExpiresHours:   valuationJWTHours,
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
	if c.Valuation.JWTSecretKey == "" || c.Valuation.JWTSecretKey == "valuation-jwt-secret-key" {
		missing = append(missing, "VALUATION_JWT_SECRET_KEY")
	}
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.Redis.Addr == "" {
		missing = append(missing, "REDIS_ADDR")
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

// ValuationJWTExpiry 返回估值模块独立 JWT 过期时长。
func (c *Config) ValuationJWTExpiry() time.Duration {
	return time.Duration(c.Valuation.JWTExpiresHours) * time.Hour
}

// IsProd 是否为生产环境。
func (c *Config) IsProd() bool { return c.AppEnv == "production" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
