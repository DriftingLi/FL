// AI 域（CONTEXT.md「AI 助手」「AI 计费」）。
package model

import "time"

// ===== 22. AI 多配置 =====

// AIConfig AI 服务配置（多套命名配置，可绑定到不同 AI 功能）。
type AIConfig struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;uniqueIndex" json:"name"`
	APIKey      string    `gorm:"column:api_key" json:"api_key"`
	BaseURL     string    `gorm:"column:base_url" json:"base_url"`
	Model       string    `gorm:"column:model" json:"model"`
	Description string    `gorm:"column:description" json:"description"`
	IsActive    bool      `gorm:"column:is_active" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIConfig) TableName() string { return "ai_configs" }

// AIFeatureBinding AI 功能-配置绑定。
// 单绑定功能（如简答题评分）在应用层限制每个 feature_key 只能有一条记录；
// 多绑定功能（如 AI 助手）允许一个 feature_key 绑定多个 config_id（DB 通过 (feature_key, config_id) 复合唯一约束保证不重复）。
type AIFeatureBinding struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FeatureKey string    `gorm:"column:feature_key" json:"feature_key"`
	ConfigID   int       `gorm:"column:config_id" json:"config_id"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIFeatureBinding) TableName() string { return "ai_feature_bindings" }

// ===== 23. AI 助手模块 =====

// AIChatSession AI 助手会话（归属 valuation_users）。
type AIChatSession struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Title      string    `gorm:"column:title" json:"title"`
	ModelName  string    `gorm:"column:model_name" json:"model_name"`
	FeatureKey string    `gorm:"column:feature_key" json:"feature_key"` // 所属功能（旧数据为 ai_assistant）
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIChatSession) TableName() string { return "ai_chat_sessions" }

// AIChatMessage AI 助手会话消息。
type AIChatMessage struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionID int       `gorm:"column:session_id" json:"session_id"`
	Role      string    `gorm:"column:role" json:"role"` // 'user' | 'assistant' | 'system'
	Content   string    `gorm:"column:content" json:"content"`
	Images    string    `gorm:"column:images" json:"-"` // JSON 数组字符串，用户消息附带的图片 URL
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

// AIUserModel 用户自定义 AI 模型配置（openai 兼容格式，归属 valuation_users）。
type AIUserModel struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Name      string    `gorm:"column:name" json:"name"`
	APIKey    string    `gorm:"column:api_key" json:"api_key"`
	BaseURL   string    `gorm:"column:base_url" json:"base_url"`
	Model     string    `gorm:"column:model" json:"model"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIUserModel) TableName() string { return "ai_user_models" }

// ===== 9. AI 生成记录 =====

type AIGenerationLog struct {
	LogID          int       `gorm:"column:log_id;primaryKey" json:"log_id"`
	UserID         int       `gorm:"column:user_id" json:"user_id"`
	UserType       string    `gorm:"column:user_type" json:"user_type"`
	GenerationType string    `gorm:"column:generation_type" json:"generation_type"`
	InputParams    JSONB     `gorm:"column:input_params;type:jsonb" json:"input_params,omitempty"`
	OutputResult   string    `gorm:"column:output_result" json:"output_result"`
	Status         int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIGenerationLog) TableName() string { return "ai_generation_log" }
