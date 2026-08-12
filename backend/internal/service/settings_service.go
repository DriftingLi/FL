// Package service 实现业务服务层。
package service

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// AISettings AI 配置快照。Source 标识配置来源，便于前端展示与诊断。
type AISettings struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
	Source  string `json:"source"` // "db" | "env" | "default"
}

const (
	aiSettingsCacheKey = "settings:ai"
	aiSettingsCacheTTL = 5 * time.Minute
)

// SettingsService 提供系统设置读写，含 Redis 缓存。
type SettingsService struct {
	db       *gorm.DB
	envKey   string // 环境变量读取的 API key（兜底）
	envURL   string
	envModel string
	logger   *zap.Logger
}

// NewSettingsService 构造 SettingsService。
// envKey/envURL/envModel 为环境变量读取的兜底配置（来自 cfg.AIAPIKey 等）。
func NewSettingsService(db *gorm.DB, envKey, envURL, envModel string, logger *zap.Logger) *SettingsService {
	return &SettingsService{db: db, envKey: envKey, envURL: envURL, envModel: envModel, logger: logger}
}

// GetAISettings 获取当前生效的 AI 配置。
// 优先级：DB.ai_api_key 非空 → 使用 DB 配置（Source="db"）；
// DB.ai_api_key 为空 → 降级到环境变量（Source="env" 或 "default"）。
// 通过 Redis 缓存 5 分钟减少 DB 压力。
func (s *SettingsService) GetAISettings(ctx context.Context) AISettings {
	var cached AISettings
	err := cache.GetOrSetJSON(ctx, aiSettingsCacheKey, aiSettingsCacheTTL, &cached, func() (any, error) {
		return s.loadAISettingsFromDB(ctx)
	})
	if err == nil {
		return cached
	}
	// Redis 异常或 loader 失败时降级
	if !errors.Is(err, redis.Nil) {
		s.logger.Warn("GetAISettings 缓存读取异常，降级直查 DB", zap.Error(err))
	}
	direct, _ := s.loadAISettingsFromDB(ctx)
	return direct
}

// loadAISettingsFromDB 从 DB 读取 AI 配置；DB 中 ai_api_key 为空时降级到环境变量。
func (s *SettingsService) loadAISettingsFromDB(ctx context.Context) (AISettings, error) {
	var rows []model.SystemSetting
	err := s.db.WithContext(ctx).
		Where("key IN ?", []string{"ai_api_key", "ai_base_url", "ai_model"}).
		Find(&rows).Error
	if err != nil {
		// DB 异常时仍降级返回环境变量，避免完全不可用
		s.logger.Warn("loadAISettingsFromDB 查询失败，降级环境变量", zap.Error(err))
		return s.envSettings("env"), err
	}

	m := make(map[string]string, 3)
	for _, r := range rows {
		m[r.Key] = r.Value
	}

	if m["ai_api_key"] != "" {
		return AISettings{
			APIKey:  m["ai_api_key"],
			BaseURL: m["ai_base_url"],
			Model:   m["ai_model"],
			Source:  "db",
		}, nil
	}
	// DB 未配置 → 降级到环境变量
	return s.envSettings("env"), nil
}

// envSettings 返回环境变量配置快照。source 由调用方决定（"env" 或 "default"）。
func (s *SettingsService) envSettings(source string) AISettings {
	return AISettings{
		APIKey:  s.envKey,
		BaseURL: s.envURL,
		Model:   s.envModel,
		Source:  source,
	}
}

// SetAISettings 写入 DB 并失效缓存。仅更新 value 与 updated_at，保留 description。
func (s *SettingsService) SetAISettings(ctx context.Context, apiKey, baseURL, modelName string) error {
	pairs := map[string]string{
		"ai_api_key":  apiKey,
		"ai_base_url": baseURL,
		"ai_model":    modelName,
	}
	now := time.Now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for k, v := range pairs {
			// UPSERT：key 存在则更新 value+updated_at，不存在则插入（description 留空）
			res := tx.Model(&model.SystemSetting{}).
				Where("key = ?", k).
				Updates(map[string]any{"value": v, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// 记录不存在，插入新行
				if err := tx.Create(&model.SystemSetting{
					Key: k, Value: v, UpdatedAt: now,
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 失效缓存，下次读取自动重建
	if err := cache.Del(ctx, aiSettingsCacheKey); err != nil {
		s.logger.Warn("SetAISettings 失效缓存失败（不影响 DB 写入结果）", zap.Error(err))
	}
	return nil
}
