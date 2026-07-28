// Package service 实现业务服务层。
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
)

// AI 功能键（与前端展示一致）。新增功能时在此追加并同步前端。
const (
	FeatureGradeShortAnswer       = "grade_short_answer"
	FeatureGenerateChapterContent = "generate_chapter_content"
)

// AllAIFeatures 全部 AI 功能键列表（用于绑定列表的全量展示）。
var AllAIFeatures = []string{
	FeatureGradeShortAnswer,
	FeatureGenerateChapterContent,
}

// FeatureLabel 功能键对应的中文名称。
var FeatureLabel = map[string]string{
	FeatureGradeShortAnswer:       "简答题 AI 评分",
	FeatureGenerateChapterContent: "课程内容生成",
}

// AIConfigDTO 返回给前端的配置对象（API Key 脱敏）。
type AIConfigDTO struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	APIKey      string    `json:"api_key"` // 脱敏后的 API Key（如 sk-da...9b3）
	BaseURL     string    `json:"base_url"`
	Model       string    `json:"model"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FeatureBindingDTO 功能绑定展示对象。
type FeatureBindingDTO struct {
	FeatureKey   string `json:"feature_key"`
	FeatureLabel string `json:"feature_label"`
	ConfigID     *int   `json:"config_id"`   // nil 表示未绑定
	ConfigName   string `json:"config_name"` // 未绑定为空
}

// ModelOption 供 AI 助手用户选择的模型选项（不含 api_key）。
type ModelOption struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Model   string `json:"model"`
	BaseURL string `json:"base_url"`
}

const (
	bindingsCacheKey = "ai:bindings"
	bindingsCacheTTL = 5 * time.Minute
)

// AIConfigService 多 AI 配置管理 + 功能绑定。
type AIConfigService struct {
	db *gorm.DB
}

// NewAIConfigService 构造 AIConfigService。
func NewAIConfigService(db *gorm.DB) *AIConfigService {
	return &AIConfigService{db: db}
}

// ListConfigs 返回所有 AI 配置（API Key 脱敏）。
func (s *AIConfigService) ListConfigs(ctx context.Context) ([]AIConfigDTO, error) {
	var rows []model.AIConfig
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AIConfigDTO, len(rows))
	for i, r := range rows {
		out[i] = AIConfigDTO{
			ID: r.ID, Name: r.Name, APIKey: MaskKey(r.APIKey),
			BaseURL: r.BaseURL, Model: r.Model,
			Description: r.Description, IsActive: r.IsActive,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	return out, nil
}

// ListPublicModels 返回启用的 AI 配置（不含 api_key），供 AI 助手用户选择模型。
func (s *AIConfigService) ListPublicModels(ctx context.Context) ([]ModelOption, error) {
	var rows []model.AIConfig
	if err := s.db.WithContext(ctx).Where("is_active = ?", true).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ModelOption, len(rows))
	for i, r := range rows {
		out[i] = ModelOption{ID: r.ID, Name: r.Name, Model: r.Model, BaseURL: r.BaseURL}
	}
	return out, nil
}

// GetConfigByID 按 ID 查询配置（含完整 api_key，仅供后端内部使用）。
func (s *AIConfigService) GetConfigByID(ctx context.Context, id int) (*model.AIConfig, error) {
	var cfg model.AIConfig
	if err := s.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// checkModelUnique 校验同 model 名的启用配置唯一性（排除 excludeID）。
func (s *AIConfigService) checkModelUnique(ctx context.Context, modelName string, excludeID int) error {
	var count int64
	q := s.db.WithContext(ctx).Model(&model.AIConfig{}).
		Where("model = ? AND is_active = ?", modelName, true)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("已存在使用模型 %q 的启用配置，同类型模型只能有一个", modelName)
	}
	return nil
}

// CreateConfig 新建配置。modelName 对应数据库 model 字段。
// 同 model 名的启用配置只能有一个（应用层校验）。
func (s *AIConfigService) CreateConfig(ctx context.Context, name, apiKey, baseURL, modelName, description string) error {
	if err := s.checkModelUnique(ctx, modelName, 0); err != nil {
		return err
	}
	row := model.AIConfig{
		Name: name, APIKey: apiKey, BaseURL: baseURL, Model: modelName,
		Description: description, IsActive: true,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// UpdateConfig 更新配置。apiKey 为空表示不修改。modelName 对应数据库 model 字段。
// 同 model 名的启用配置只能有一个（应用层校验）。
func (s *AIConfigService) UpdateConfig(ctx context.Context, id int, name, apiKey, baseURL, modelName, description string, isActive *bool) error {
	if err := s.checkModelUnique(ctx, modelName, id); err != nil {
		return err
	}
	updates := map[string]any{
		"name":        name,
		"base_url":    baseURL,
		"model":       modelName,
		"description": description,
		"updated_at":  time.Now(),
	}
	if apiKey != "" {
		updates["api_key"] = apiKey
	}
	if isActive != nil {
		updates["is_active"] = *isActive
	}
	res := s.db.WithContext(ctx).Model(&model.AIConfig{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteConfig 删除配置。若被功能绑定则拒绝（需先解除绑定）。
func (s *AIConfigService) DeleteConfig(ctx context.Context, id int) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AIFeatureBinding{}).
		Where("config_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该配置已被功能绑定，请先解除绑定后再删除")
	}
	res := s.db.WithContext(ctx).Delete(&model.AIConfig{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListBindings 返回所有 AI 功能的绑定情况（未绑定功能也展示，config_id 为 null）。
func (s *AIConfigService) ListBindings(ctx context.Context) ([]FeatureBindingDTO, error) {
	// 优先查 Redis 缓存
	var cached []FeatureBindingDTO
	err := cache.GetOrSetJSON(ctx, bindingsCacheKey, bindingsCacheTTL, &cached, func() (any, error) {
		return s.loadBindingsFromDB(ctx)
	})
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		slog.Warn("ListBindings 缓存读取异常，降级直查 DB", "error", err)
	}
	return s.loadBindingsFromDB(ctx)
}

func (s *AIConfigService) loadBindingsFromDB(ctx context.Context) ([]FeatureBindingDTO, error) {
	// 一次性查出所有绑定 + 配置名
	type joinRow struct {
		FeatureKey string  `gorm:"column:feature_key"`
		ConfigID   *int    `gorm:"column:config_id"`
		ConfigName *string `gorm:"column:config_name"`
	}
	var rows []joinRow
	err := s.db.WithContext(ctx).Table("ai_feature_bindings AS b").
		Select("b.feature_key, b.config_id, c.name AS config_name").
		Joins("LEFT JOIN ai_configs AS c ON c.id = b.config_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	bindMap := make(map[string]*joinRow, len(rows))
	for i := range rows {
		bindMap[rows[i].FeatureKey] = &rows[i]
	}

	out := make([]FeatureBindingDTO, 0, len(AllAIFeatures))
	for _, fk := range AllAIFeatures {
		dto := FeatureBindingDTO{
			FeatureKey:   fk,
			FeatureLabel: FeatureLabel[fk],
		}
		if b, ok := bindMap[fk]; ok {
			dto.ConfigID = b.ConfigID
			if b.ConfigName != nil {
				dto.ConfigName = *b.ConfigName
			}
		}
		out = append(out, dto)
	}
	return out, nil
}

// SetBinding 绑定功能到指定配置（configID 为 0 表示解除绑定）。
func (s *AIConfigService) SetBinding(ctx context.Context, featureKey string, configID int) error {
	if !isValidFeature(featureKey) {
		return fmt.Errorf("无效的功能键: %s", featureKey)
	}
	// 若指定了 configID，校验配置存在且 is_active=true
	if configID > 0 {
		var cfg model.AIConfig
		err := s.db.WithContext(ctx).Select("id, is_active").First(&cfg, configID).Error
		if err != nil {
			return fmt.Errorf("配置不存在: %w", err)
		}
		if !cfg.IsActive {
			return errors.New("该配置已停用，请先启用或选择其他配置")
		}
	}

	now := time.Now()
	if configID == 0 {
		// 解除绑定
		res := s.db.WithContext(ctx).Where("feature_key = ?", featureKey).
			Delete(&model.AIFeatureBinding{})
		if res.Error != nil {
			return res.Error
		}
	} else {
		// UPSERT：先尝试 Update，RowsAffected=0 则 Create
		row := model.AIFeatureBinding{FeatureKey: featureKey, ConfigID: configID, UpdatedAt: now}
		res := s.db.WithContext(ctx).Model(&model.AIFeatureBinding{}).
			Where("feature_key = ?", featureKey).
			Updates(map[string]any{"config_id": configID, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
		}
	}

	// 失效绑定缓存
	if err := cache.Del(ctx, bindingsCacheKey); err != nil {
		slog.Warn("SetBinding 失效缓存失败（不影响 DB 写入结果）", "error", err)
	}
	return nil
}

// ResolveConfig 根据 feature_key 解析绑定的配置。
// 仅查找 ai_feature_bindings → ai_configs（is_active=true），未绑定时返回空配置（APIKey 为空，调用方据此报错）。
func (s *AIConfigService) ResolveConfig(ctx context.Context, featureKey string) AISettings {
	var b model.AIFeatureBinding
	err := s.db.WithContext(ctx).Where("feature_key = ?", featureKey).Limit(1).Find(&b).Error
	if err == nil && b.ConfigID > 0 {
		var cfg model.AIConfig
		if err := s.db.WithContext(ctx).First(&cfg, b.ConfigID).Error; err == nil && cfg.IsActive {
			return AISettings{
				APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model,
				Source: "binding:" + cfg.Name,
			}
		}
	}
	// 未绑定或绑定失效，返回空配置（APIKey 为空，ensureClient 会据此报错）
	return AISettings{Source: "unbound"}
}

// MaskKey 脱敏 API Key，保留前 6 位与后 4 位。
func MaskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 10 {
		masked := make([]byte, len(k))
		for i := range masked {
			masked[i] = '*'
		}
		return string(masked)
	}
	return k[:6] + "..." + k[len(k)-4:]
}

func isValidFeature(key string) bool {
	for _, f := range AllAIFeatures {
		if f == key {
			return true
		}
	}
	return false
}
