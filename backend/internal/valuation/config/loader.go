// Package config 提供残值评估子模块的配置结构体与环境变量加载能力。
// 合并到 forklift-training 后已不读 yaml，统一走 .env / 环境变量。
package config

import (
	"os"
	"strconv"
)

// Config 是残值评估子模块的局部配置结构
// 仅供 valuation 包内部使用，主程序入口使用 forklift-training/internal/config.Config
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
	Storage  StorageConfig
}

// ServerConfig HTTP 服务相关配置
type ServerConfig struct {
	Port        int
	Mode        string
	CORSOrigins []string
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string
	Format string
	Output string
}

// StorageConfig 文件存储配置
type StorageConfig struct {
	PDFOutputDir string
}

// Load 从环境变量加载配置（保留兼容签名，main.go 不再调用）。
// configPath 参数仅作占位，不再读文件。
func Load(configPath string) (*Config, error) {
	return LoadFromEnv("", nil), nil
}

// LoadFromEnv 从环境变量构造残值评估配置。
// databaseURL 与 corsOrigins 由主程序注入，其余字段从 VALUATION_* 环境变量读取。
func LoadFromEnv(databaseURL string, corsOrigins []string) *Config {
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"*"}
	}
	return &Config{
		Server: ServerConfig{
			Port:        getenvInt("VALUATION_SERVER_PORT", 8080),
			Mode:        getenvStr("VALUATION_SERVER_MODE", "debug"),
			CORSOrigins: corsOrigins,
		},
		Database: DatabaseConfig{
			DSN:             databaseURL,
			MaxOpenConns:    getenvInt("VALUATION_DB_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getenvInt("VALUATION_DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getenvInt("VALUATION_DB_CONN_MAX_LIFETIME", 3600),
		},
		Log: LogConfig{
			Level:  getenvStr("VALUATION_LOG_LEVEL", "info"),
			Format: getenvStr("VALUATION_LOG_FORMAT", "console"),
			Output: getenvStr("VALUATION_LOG_OUTPUT", "stdout"),
		},
		Storage: StorageConfig{
			PDFOutputDir: getenvStr("VALUATION_PDF_OUTPUT_DIR", "storage/reports"),
		},
	}
}

func getenvStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
