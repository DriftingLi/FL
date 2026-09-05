// Package service 实现业务服务层。
package service

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	"gorm.io/gorm"

	"forklift-training/internal/cache"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
)

// AI 功能键（与前端展示一致）。新增功能时在此追加并同步前端。
const (
	FeatureGradeShortAnswer       = "grade_short_answer"
	FeatureGenerateChapterContent = "generate_chapter_content"
	FeatureAIAssistant            = "ai_assistant" // 遗留：多绑定，已由 normal/expert 双绑定替代，仅作兼容回退
	FeatureAIAssistantNormal      = "ai_assistant_normal"
	FeatureAIAssistantExpert      = "ai_assistant_expert"
	FeatureQuestionExplanation    = "ai_question_analysis"
	FeatureFaultConsult           = "fault_consult"
	FeatureFaultCodeQuery         = "fault_code_query"
	FeatureMaintenanceKnowledge   = "maintenance_knowledge"
	FeatureDrawingRecognition     = "drawing_recognition"
	FeatureExerciseSolving        = "exercise_solving"
)

// AllAIFeatures 全部 AI 功能键列表（用于绑定列表的全量展示）。
// AI 助手已由 multi 的 ai_assistant 拆为双绑定的 normal/expert 单绑定，其它功能保持单绑定。
var AllAIFeatures = []string{
	FeatureGradeShortAnswer,
	FeatureGenerateChapterContent,
	FeatureAIAssistantNormal,
	FeatureAIAssistantExpert,
	FeatureQuestionExplanation,
	FeatureFaultConsult,
	FeatureFaultCodeQuery,
	FeatureMaintenanceKnowledge,
	FeatureDrawingRecognition,
	FeatureExerciseSolving,
}

// FeatureLabel 功能键对应的中文名称。
var FeatureLabel = map[string]string{
	FeatureGradeShortAnswer:       "简答题 AI 评分",
	FeatureGenerateChapterContent: "课程内容生成",
	FeatureAIAssistantNormal:      "AI 助手 · 普通模式",
	FeatureAIAssistantExpert:      "AI 助手 · 专家模式",
	FeatureQuestionExplanation:    "题目 AI 解析",
	FeatureFaultConsult:           "故障咨询",
	FeatureFaultCodeQuery:         "故障代码查询",
	FeatureMaintenanceKnowledge:   "维保知识",
	FeatureDrawingRecognition:     "图纸识别",
	FeatureExerciseSolving:        "习题解答",
	// 遗留兼容
	FeatureAIAssistant: "AI 助手对话",
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

// FeatureBindingDTO 功能绑定展示对象（全部功能均为单绑定：ConfigID/ConfigName 字段有效）。
type FeatureBindingDTO struct {
	FeatureKey   string `json:"feature_key"`
	FeatureLabel string `json:"feature_label"`
	ConfigID     *int   `json:"config_id,omitempty"`
	ConfigName   string `json:"config_name,omitempty"`
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

	// aiResolveTTL 进程内热点解析缓存的 TTL：管理端改绑后最迟此延迟生效
	//（写路径同时主动失效，TTL 仅兜底多实例部署）。
	aiResolveTTL = 30 * time.Second
)

// aiResolveEntry 进程内热点解析缓存条目。
type aiResolveEntry struct {
	value   any
	expires time.Time
}

// aiResolveCache 进程内热点解析缓存（ResolveConfig/ListConfigsForFeature/ResolveAssistantPair）。
// 解密后的 API Key 只留在进程内、不落 Redis；写路径（建/改/删配置与绑定）主动清空。
type aiResolveCache struct {
	mu      sync.Mutex
	entries map[string]aiResolveEntry
}

func (c *aiResolveCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.value, true
}

func (c *aiResolveCache) set(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]aiResolveEntry)
	}
	c.entries[key] = aiResolveEntry{value: v, expires: time.Now().Add(aiResolveTTL)}
}

// invalidate 清空全部热点解析缓存（写路径调用）。
func (c *aiResolveCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}

// AIConfigService 多 AI 配置管理 + 功能绑定。
type AIConfigService struct {
	db           *gorm.DB
	secretKey    string // 用于加密 API Key 的主密钥（SECRET_KEY）
	logger       *zap.Logger
	resolveCache *aiResolveCache
}

// NewAIConfigService 构造 AIConfigService。
func NewAIConfigService(db *gorm.DB, secretKey string, logger *zap.Logger) *AIConfigService {
	return &AIConfigService{db: db, secretKey: secretKey, logger: logger, resolveCache: &aiResolveCache{}}
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
			ID: r.ID, Name: r.Name, APIKey: s.maskKeyForDisplay(&r),
			BaseURL: r.BaseURL, Model: r.Model,
			Description: r.Description, IsActive: r.IsActive,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	return out, nil
}

// GetConfigByID 按 ID 查询配置（含完整 api_key，仅供后端内部使用）。
func (s *AIConfigService) GetConfigByID(ctx context.Context, id int) (*model.AIConfig, error) {
	var cfg model.AIConfig
	if err := s.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	key, err := s.decryptAPIKey(&cfg)
	if err != nil {
		return nil, err
	}
	cfg.APIKey = key
	return &cfg, nil
}

// decryptAPIKey 解密配置的 API Key（解密切点单点，内部消费路径统一走此）。
func (s *AIConfigService) decryptAPIKey(cfg *model.AIConfig) (string, error) {
	key, err := security.DecryptSecret(cfg.APIKey, s.secretKey)
	if err != nil {
		return "", fmt.Errorf("解密配置 %d 的 API Key 失败: %w", cfg.ID, err)
	}
	return key, nil
}

// maskKeyForDisplay 脱敏展示用解密：解密失败记日志并按原样脱敏（历史脏数据容忍，
// 仅列表展示路径使用；内部消费路径用 decryptAPIKey 直接报错）。
func (s *AIConfigService) maskKeyForDisplay(cfg *model.AIConfig) string {
	key, err := security.DecryptSecret(cfg.APIKey, s.secretKey)
	if err != nil {
		s.logger.Warn("解密 API Key 失败，按原样脱敏展示", zap.Int("id", cfg.ID), zap.Error(err))
		key = cfg.APIKey
	}
	return MaskKey(key)
}

// CreateConfig 新建配置。modelName 对应数据库 model 字段。
// 允许创建同 model 的多个配置（用户可能用不同 api_key/base_url 重复配置同款模型）。
func (s *AIConfigService) CreateConfig(ctx context.Context, name, apiKey, baseURL, modelName, description string) error {
	encKey, err := security.EncryptSecret(apiKey, s.secretKey)
	if err != nil {
		return fmt.Errorf("加密 API Key 失败: %w", err)
	}
	row := model.AIConfig{
		Name: name, APIKey: encKey, BaseURL: baseURL, Model: modelName,
		Description: description, IsActive: true,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	s.resolveCache.invalidate()
	return nil
}

// UpdateConfig 更新配置。apiKey 为空表示不修改。modelName 对应数据库 model 字段。
// 允许配置更新成与其它配置相同的 model（用户可能用不同 api_key/base_url 重复配置同款模型）。
func (s *AIConfigService) UpdateConfig(ctx context.Context, id int, name, apiKey, baseURL, modelName, description string, isActive *bool) error {
	updates := map[string]any{
		"name":        name,
		"base_url":    baseURL,
		"model":       modelName,
		"description": description,
		"updated_at":  time.Now(),
	}
	if apiKey != "" {
		encKey, err := security.EncryptSecret(apiKey, s.secretKey)
		if err != nil {
			return fmt.Errorf("加密 API Key 失败: %w", err)
		}
		updates["api_key"] = encKey
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
	s.resolveCache.invalidate()
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
	s.resolveCache.invalidate()
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
		s.logger.Warn("ListBindings 缓存读取异常，降级直查 DB", zap.Error(err))
	}
	return s.loadBindingsFromDB(ctx)
}

func (s *AIConfigService) loadBindingsFromDB(ctx context.Context) ([]FeatureBindingDTO, error) {
	// 一次性查出所有绑定 + 配置信息
	type joinRow struct {
		FeatureKey string  `gorm:"column:feature_key"`
		ConfigID   *int    `gorm:"column:config_id"`
		ConfigName *string `gorm:"column:config_name"`
	}
	var rows []joinRow
	err := s.db.WithContext(ctx).Table("ai_feature_bindings AS b").
		Select("b.feature_key, b.config_id, c.name AS config_name").
		Joins("LEFT JOIN ai_configs AS c ON c.id = b.config_id").
		Order("b.feature_key ASC, b.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	// 按 feature_key 分组（全部单绑定，仅保留第一条）
	single := make(map[string]*joinRow, len(rows))
	for i := range rows {
		fk := rows[i].FeatureKey
		if single[fk] == nil {
			single[fk] = &rows[i]
		}
	}

	out := make([]FeatureBindingDTO, 0, len(AllAIFeatures))
	for _, fk := range AllAIFeatures {
		dto := FeatureBindingDTO{
			FeatureKey:   fk,
			FeatureLabel: FeatureLabel[fk],
		}
		if r := single[fk]; r != nil {
			dto.ConfigID = r.ConfigID
			if r.ConfigName != nil {
				dto.ConfigName = *r.ConfigName
			}
		}
		out = append(out, dto)
	}
	return out, nil
}

// ListConfigsForFeature 查询指定功能绑定的配置（含解密后 api_key；进程内热点缓存）。
// 全部功能均为单绑定，至多返回 1 条；保留切片形状便于回退列表消费。
func (s *AIConfigService) ListConfigsForFeature(ctx context.Context, featureKey string) ([]model.AIConfig, error) {
	if !isValidFeature(featureKey) {
		return nil, fmt.Errorf("无效的功能键: %s", featureKey)
	}
	cacheKey := "feature-cfg:" + featureKey
	if v, ok := s.resolveCache.get(cacheKey); ok {
		return v.([]model.AIConfig), nil
	}
	var cfgs []model.AIConfig
	err := s.db.WithContext(ctx).
		Joins("JOIN ai_feature_bindings AS b ON b.config_id = ai_configs.id").
		Where("b.feature_key = ? AND ai_configs.is_active = ?", featureKey, true).
		Order("b.id ASC").
		Find(&cfgs).Error
	if err != nil {
		return nil, err
	}
	for i := range cfgs {
		key, err := s.decryptAPIKey(&cfgs[i])
		if err != nil {
			return nil, err
		}
		cfgs[i].APIKey = key
	}
	s.resolveCache.set(cacheKey, cfgs)
	return cfgs, nil
}

// UnbindConfig 解除指定功能的配置绑定（等效于 SetBinding(featureKey, 0)）。
func (s *AIConfigService) UnbindConfig(ctx context.Context, featureKey string, configID int) error {
	if !isValidFeature(featureKey) {
		return fmt.Errorf("无效的功能键: %s", featureKey)
	}
	if configID <= 0 {
		return errors.New("config_id 必须大于 0")
	}
	res := s.db.WithContext(ctx).
		Where("feature_key = ? AND config_id = ?", featureKey, configID).
		Delete(&model.AIFeatureBinding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	s.invalidateBindings(ctx)
	return nil
}

// SetBinding 绑定功能到指定配置。
// configID=0 表示解除绑定（清空该功能的所有绑定），configID>0 表示 UPSERT（覆盖原绑定）。
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
		// 解除绑定：清空该功能的所有绑定
		res := s.db.WithContext(ctx).Where("feature_key = ?", featureKey).
			Delete(&model.AIFeatureBinding{})
		if res.Error != nil {
			return res.Error
		}
	} else {
		// 单绑定：UPSERT
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

	s.invalidateBindings(ctx)
	return nil
}

// invalidateBindings 绑定写路径的缓存失效：Redis 展示缓存 + 进程内热点解析缓存。
func (s *AIConfigService) invalidateBindings(ctx context.Context) {
	if err := cache.Del(ctx, bindingsCacheKey); err != nil {
		s.logger.Warn("失效绑定缓存失败（不影响 DB 写入结果）", zap.Error(err))
	}
	s.resolveCache.invalidate()
}

// ResolveConfig 根据 feature_key 解析绑定的配置（进程内热点缓存，对话热路径直查 DB 的替代）。
// 仅查找 ai_feature_bindings → ai_configs（is_active=true），未绑定时返回空配置（APIKey 为空，调用方据此报错）。
func (s *AIConfigService) ResolveConfig(ctx context.Context, featureKey string) AISettings {
	cacheKey := "resolve:" + featureKey
	if v, ok := s.resolveCache.get(cacheKey); ok {
		return v.(AISettings)
	}
	resolved := s.loadResolveConfig(ctx, featureKey)
	s.resolveCache.set(cacheKey, resolved)
	return resolved
}

// loadResolveConfig ResolveConfig 的 DB 路径。
func (s *AIConfigService) loadResolveConfig(ctx context.Context, featureKey string) AISettings {
	var b model.AIFeatureBinding
	err := s.db.WithContext(ctx).Where("feature_key = ?", featureKey).Limit(1).Find(&b).Error
	if err == nil && b.ConfigID > 0 {
		var cfg model.AIConfig
		if err := s.db.WithContext(ctx).First(&cfg, b.ConfigID).Error; err == nil && cfg.IsActive {
			key, err := s.decryptAPIKey(&cfg)
			if err != nil {
				s.logger.Warn("ResolveConfig 解密 API Key 失败，配置不可用", zap.Int("config_id", b.ConfigID), zap.Error(err))
				return AISettings{Source: "decrypt-failed"}
			}
			return AISettings{
				APIKey: key, BaseURL: cfg.BaseURL, Model: cfg.Model,
				Source: "binding:" + cfg.Name,
			}
		}
	}
	// 未绑定或绑定失效，返回空配置（APIKey 为空，调用方据此报错）
	return AISettings{Source: "unbound"}
}

// ResolveAssistantPair AI 助手双模式绑定解析（降级阶梯单点，#397）：
// ① normal/expert 各自单绑定 → ② 双双未绑定时回退遗留 ai_assistant 多绑定
// （第一条→normal，第二条→expert，仅一条时 expert 复用 normal）。
// 展示（ListAssistantModes）与对话凭证解析（ResolveChatSettings）共用，不再各写一份阶梯。
func (s *AIConfigService) ResolveAssistantPair(ctx context.Context) (normal, expert *model.AIConfig, err error) {
	normalCfgs, err := s.ListConfigsForFeature(ctx, FeatureAIAssistantNormal)
	if err != nil {
		return nil, nil, err
	}
	expertCfgs, err := s.ListConfigsForFeature(ctx, FeatureAIAssistantExpert)
	if err != nil {
		return nil, nil, err
	}
	if len(normalCfgs) == 0 && len(expertCfgs) == 0 {
		// ② 回退遗留 ai_assistant 多绑定（兼容存量部署）
		legacyCfgs, err := s.ListConfigsForFeature(ctx, FeatureAIAssistant)
		if err != nil {
			return nil, nil, err
		}
		if len(legacyCfgs) > 0 {
			normal = &legacyCfgs[0]
		}
		if len(legacyCfgs) > 1 {
			expert = &legacyCfgs[1]
		} else if len(legacyCfgs) == 1 {
			// 单条时 expert 复用同一配置，避免前端无可用
			expert = normal
		}
		return normal, expert, nil
	}
	if len(normalCfgs) > 0 {
		normal = &normalCfgs[0]
	}
	if len(expertCfgs) > 0 {
		expert = &expertCfgs[0]
	}
	return normal, expert, nil
}

// AIConfigResolver 由 *AIConfigService 提供唯一实现（ADR-0029 决策 2：解析知识不泄出配置 service）。
var _ AIConfigResolver = (*AIConfigService)(nil)

// featureChatKeys 专项对话功能键集合（模型由管理端单绑定解析，用户无需选模型）。
// （自 ai_assistant_service.go 迁入：唯一消费方是对话凭证解析。）
var featureChatKeys = map[string]bool{
	FeatureFaultConsult:         true,
	FeatureFaultCodeQuery:       true,
	FeatureMaintenanceKnowledge: true,
	FeatureDrawingRecognition:   true,
	FeatureExerciseSolving:      true,
}

// ResolveFeatureSettings 阻塞栈凭证解析（AIConfigResolver 实现；自 AIService.ensureClient
// 的解析段迁入）：featureKey → 管理端单绑定；空键/未绑定报错（不再降级到环境变量）。
func (s *AIConfigService) ResolveFeatureSettings(ctx context.Context, featureKey string) (AISettings, error) {
	if featureKey == "" {
		return AISettings{}, fmt.Errorf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", featureKey)
	}
	cur := s.ResolveConfig(ctx, featureKey)
	if cur.APIKey == "" {
		return AISettings{}, fmt.Errorf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", featureKey)
	}
	return cur, nil
}

// ResolveChatSettings 对话凭证解析（AIConfigResolver 实现；自 AIAssistantService.resolveModelConfig
// 迁入，三重 switch 的知识内聚在配置 service，#606）。
// 优先级：专项功能（FeatureKey，管理端单绑定）→ Mode 双模式（normal/expert）→ 兼容旧 ModelSource。
func (s *AIConfigService) ResolveChatSettings(ctx context.Context, sel AIModelSelector) (AISettings, error) {
	// 专项功能：由管理端单绑定解析，忽略选择子中的模型来源字段（防绕过）
	if featureChatKeys[sel.FeatureKey] {
		mc := s.ResolveConfig(ctx, sel.FeatureKey)
		if mc.APIKey == "" {
			return AISettings{}, errors.New("管理员未配置该功能的模型，请联系管理员")
		}
		return mc, nil
	}
	// 通用助手：Mode 双模式（隐藏底层模型）——降级阶梯在 ResolveAssistantPair 单点
	if sel.Mode == ModeNormal || sel.Mode == ModeExpert {
		normal, expert, err := s.ResolveAssistantPair(ctx)
		if err != nil {
			return AISettings{}, fmt.Errorf("校验可用模型失败: %w", err)
		}
		cfg := normal
		if sel.Mode == ModeExpert {
			cfg = expert
		}
		if cfg == nil {
			return AISettings{}, errors.New("该模式未绑定模型，请联系管理员配置")
		}
		return AISettings{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Source: "binding:" + cfg.Name}, nil
	}
	switch sel.ModelSource {
	case "admin":
		// 校验该配置是否被管理员绑定到 AI 助手功能（兼容旧前端：同时校验新双绑定的两个 Feature）
		boundCfgsNormal, _ := s.ListConfigsForFeature(ctx, FeatureAIAssistantNormal)
		boundCfgsExpert, _ := s.ListConfigsForFeature(ctx, FeatureAIAssistantExpert)
		boundCfgsLegacy, _ := s.ListConfigsForFeature(ctx, FeatureAIAssistant)
		allBound := append(append(boundCfgsNormal, boundCfgsExpert...), boundCfgsLegacy...)
		var cfg *model.AIConfig
		for i := range allBound {
			if allBound[i].ID == sel.ConfigID {
				cfg = &allBound[i]
				break
			}
		}
		if cfg == nil {
			return AISettings{}, errors.New("该模型未绑定到 AI 助手，请联系管理员或选择自定义模型")
		}
		return AISettings{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Source: "binding:" + cfg.Name}, nil
	case "user":
		if sel.UserID == 0 {
			return AISettings{}, errors.New("未登录不能使用用户自定义模型")
		}
		var m model.AIUserModel
		if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", sel.UserModelID, sel.UserID).
			Limit(1).Find(&m).Error; err != nil {
			return AISettings{}, err
		}
		if m.ID == 0 {
			return AISettings{}, gorm.ErrRecordNotFound
		}
		key, err := security.DecryptSecret(m.APIKey, s.secretKey)
		if err != nil {
			return AISettings{}, fmt.Errorf("解密用户自定义模型 API Key 失败: %w", err)
		}
		return AISettings{APIKey: key, BaseURL: m.BaseURL, Model: m.Model, Source: "user:" + m.Name}, nil
	case "custom":
		if sel.CustomAPIKey == "" || sel.CustomBaseURL == "" || sel.CustomModel == "" {
			return AISettings{}, errors.New("自定义模型配置不完整")
		}
		return AISettings{APIKey: sel.CustomAPIKey, BaseURL: sel.CustomBaseURL, Model: sel.CustomModel, Source: "custom"}, nil
	}
	return AISettings{}, fmt.Errorf("未知的 model_source: %s", sel.ModelSource)
}

// TestConfig 管理端配置连通性测试：解密配置后建 go-openai client 发最小补全请求
// （30s 超时纪律单点在 service；此前为 handler 内联建 client）。
func (s *AIConfigService) TestConfig(ctx context.Context, id int) error {
	row, err := s.GetConfigByID(ctx, id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client := newOpenAIClient(row.APIKey, row.BaseURL)
	_, err = client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: row.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "请回复 'OK'"},
		},
		MaxTokens: 10,
	})
	return err
}

// HasActiveConfigs 是否存在已启用的 AI 配置。
// 服务启动时用于告警：若未配置任何模型，简答题评分等 AI 功能将走导师人工评分。
func (s *AIConfigService) HasActiveConfigs(ctx context.Context) bool {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.AIConfig{}).
		Where("is_active = ?", true).Count(&count).Error; err != nil {
		s.logger.Warn("查询 AI 配置失败", zap.Error(err))
		return false
	}
	return count > 0
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
	// 遗留兼容：ai_assistant 仍视为有效以便旧数据回退/迁移
	if key == FeatureAIAssistant {
		return true
	}
	return false
}
