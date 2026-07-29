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
	// AI 服务配置（OpenAI 兼容格式）。优先级：AI_* > DEEPSEEK_* > ZHIPU_* > OPENAI_API_KEY。
	AIAPIKey         string
	AIBaseURL        string
	AIModel          string
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

// ValuationConfig 残值评估模块配置。
type ValuationConfig struct {
	PDFOutputDir      string
	LogLevel          string
	LogFormat         string
	LogOutput         string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime int
	// Deprecated: 估值模块已并入主体系 JWT 鉴权,统一使用 JWT_SECRET_KEY。
	// 此字段保留仅为向后兼容,实际不再使用。
	JWTSecretKey string
	// Deprecated: 同上,已统一使用主体系 JWTExpiry。
	JWTExpiresHours int
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
		AIAPIKey:              getenvChainDef("", "AI_API_KEY", "DEEPSEEK_API_KEY", "ZHIPU_API_KEY", "OPENAI_API_KEY"),
		AIBaseURL:             getenvChainDef("https://api.deepseek.com", "AI_BASE_URL", "DEEPSEEK_API_URL", "ZHIPU_BASE_URL"),
		AIModel:               getenvChainDef("deepseek-v4-flash", "AI_MODEL", "MODEL", "ZHIPU_MODEL"),
		Valuation: ValuationConfig{
			PDFOutputDir:      getenv("VALUATION_PDF_OUTPUT_DIR", "storage/reports"),
			LogLevel:          getenv("VALUATION_LOG_LEVEL", "info"),
			LogFormat:         getenv("VALUATION_LOG_FORMAT", "console"),
			LogOutput:         getenv("VALUATION_LOG_OUTPUT", "stdout"),
			DBMaxOpenConns:    valuationDBMaxOpen,
			DBMaxIdleConns:    valuationDBMaxIdle,
			DBConnMaxLifetime: valuationDBLifetime,
			// Deprecated: 估值模块已统一使用主体系 JWT_SECRET_KEY,此字段不再使用。
			JWTSecretKey:    getenv("VALUATION_JWT_SECRET_KEY", "valuation-jwt-secret-key"),
			JWTExpiresHours: valuationJWTHours,
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
	// Deprecated: VALUATION_JWT_SECRET_KEY 已废弃,估值模块统一使用 JWT_SECRET_KEY。
	// 不再校验此环境变量。
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
