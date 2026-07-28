// Package service 实现业务服务层。
// 本文件：AI 助手模块（会话管理 + eino 流式对话）。
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// forkliftExpertSystemPrompt 叉车维修专家系统提示词。
const forkliftExpertSystemPrompt = `你是一名资深的叉车维修专家，拥有 20 年以上叉车维保与故障诊断经验。
你熟悉国内外主流品牌（如林德、丰田、杭叉、合力、永恒力、TCM 等）的电动平衡重叉车、内燃叉车、前移式叉车、仓储叉车的结构、工作原理与常见故障。
你的职责是为用户提供专业的：
- 叉车选购建议（按工况、吨位、配置推荐合适车型）
- 维保周期与项目（日常检查、季度保养、年度大修）
- 故障诊断与排查（启动困难、液压异常、转向失灵、电池续航下降等）
- 操作规范与安全注意事项
- 配件更换与维修成本评估

回答要求：
1. 用中文回答，专业、实用、可操作
2. 涉及安全的操作必须明确警示
3. 复杂故障按"可能原因 → 排查步骤 → 处理方法"结构回答
4. 不确定时坦诚告知，不编造数据
5. 涉及维修必须由专业人员执行的，明确提示联系专业维修人员`

// UserModelDTO 用户自定义模型展示对象（api_key 脱敏）。
type UserModelDTO struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"` // 脱敏后的 API Key
	BaseURL   string    `json:"base_url"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaveUserModelReq 创建/更新用户自定义模型请求。
type SaveUserModelReq struct {
	ID      int    `json:"id"` // 0 表示新建，>0 表示更新
	Name    string `json:"name"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// AIChatSessionDTO 会话展示对象。
type AIChatSessionDTO struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	ModelName string    `json:"model_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AIChatMessageDTO 消息展示对象。
type AIChatMessageDTO struct {
	ID        int       `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// StreamChatReq 流式对话请求。
type StreamChatReq struct {
	SessionID     int    `json:"session_id"`     // 可选，登录用户指定会话
	ModelSource   string `json:"model_source"`   // "admin" | "user" | "custom"
	ConfigID      int    `json:"config_id"`      // ModelSource="admin" 时引用管理员配置
	UserModelID   int    `json:"user_model_id"`  // ModelSource="user" 时引用用户自定义模型
	CustomAPIKey  string `json:"custom_api_key"` // ModelSource="custom" 时临时输入
	CustomBaseURL string `json:"custom_base_url"`
	CustomModel   string `json:"custom_model"`
	Messages      []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// modelConfig 解析后的模型配置（内部使用）。
type modelConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

// AIAssistantService AI 助手模块服务。
type AIAssistantService struct {
	db          *gorm.DB
	aiConfigSvc *AIConfigService
}

// NewAIAssistantService 构造 AIAssistantService。
func NewAIAssistantService(db *gorm.DB, aiConfigSvc *AIConfigService) *AIAssistantService {
	return &AIAssistantService{db: db, aiConfigSvc: aiConfigSvc}
}

// ListPublicModels 返回管理员配置的 is_active=true 模型列表（不含 api_key）。
func (s *AIAssistantService) ListPublicModels(ctx context.Context) ([]ModelOption, error) {
	return s.aiConfigSvc.ListPublicModels(ctx)
}

// ListUserModels 返回登录用户的自定义模型列表（api_key 脱敏）。
func (s *AIAssistantService) ListUserModels(ctx context.Context, userID int) ([]UserModelDTO, error) {
	var rows []model.AIUserModel
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]UserModelDTO, len(rows))
	for i, r := range rows {
		out[i] = UserModelDTO{
			ID: r.ID, Name: r.Name, APIKey: MaskKey(r.APIKey),
			BaseURL: r.BaseURL, Model: r.Model,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	return out, nil
}

// SaveUserModel UPSERT 用户自定义模型（同用户同 model 唯一）。
func (s *AIAssistantService) SaveUserModel(ctx context.Context, userID int, req SaveUserModelReq) error {
	// 唯一性校验：同用户同 model 只能有一个
	var count int64
	q := s.db.WithContext(ctx).Model(&model.AIUserModel{}).
		Where("user_id = ? AND model = ?", userID, req.Model)
	if req.ID > 0 {
		q = q.Where("id <> ?", req.ID)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("已存在使用模型 %q 的自定义配置", req.Model)
	}

	if req.ID > 0 {
		// 更新
		updates := map[string]any{
			"name": req.Name, "api_key": req.APIKey,
			"base_url": req.BaseURL, "model": req.Model,
			"updated_at": time.Now(),
		}
		res := s.db.WithContext(ctx).Model(&model.AIUserModel{}).
			Where("id = ? AND user_id = ?", req.ID, userID).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	// 新建
	row := model.AIUserModel{
		UserID: userID, Name: req.Name, APIKey: req.APIKey,
		BaseURL: req.BaseURL, Model: req.Model,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// DeleteUserModel 删除用户自定义模型（校验归属）。
func (s *AIAssistantService) DeleteUserModel(ctx context.Context, userID, modelID int) error {
	res := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", modelID, userID).
		Delete(&model.AIUserModel{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CreateSession 创建会话（需登录）。
func (s *AIAssistantService) CreateSession(ctx context.Context, userID int, title, modelName string) (*AIChatSessionDTO, error) {
	if title == "" {
		title = "新会话"
	}
	row := model.AIChatSession{UserID: userID, Title: title, ModelName: modelName}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &AIChatSessionDTO{
		ID: row.ID, Title: row.Title, ModelName: row.ModelName,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// DeleteSession 删除会话及其消息（校验归属，ON DELETE CASCADE 自动删消息）。
func (s *AIAssistantService) DeleteSession(ctx context.Context, userID, sessionID int) error {
	var session model.AIChatSession
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).
		Limit(1).Find(&session).Error; err != nil {
		return err
	}
	if session.ID == 0 {
		return gorm.ErrRecordNotFound
	}
	return s.db.WithContext(ctx).Delete(&model.AIChatSession{}, sessionID).Error
}

// ListSessions 返回登录用户的会话列表（按创建时间倒序）。
func (s *AIAssistantService) ListSessions(ctx context.Context, userID int) ([]AIChatSessionDTO, error) {
	var rows []model.AIChatSession
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AIChatSessionDTO, len(rows))
	for i, r := range rows {
		out[i] = AIChatSessionDTO{
			ID: r.ID, Title: r.Title, ModelName: r.ModelName,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	return out, nil
}

// GetSessionMessages 返回指定会话的消息列表（按时间升序，校验归属）。
func (s *AIAssistantService) GetSessionMessages(ctx context.Context, userID, sessionID int) ([]AIChatMessageDTO, error) {
	var session model.AIChatSession
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).
		Limit(1).Find(&session).Error; err != nil {
		return nil, err
	}
	if session.ID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var rows []model.AIChatMessage
	if err := s.db.WithContext(ctx).Where("session_id = ?", sessionID).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AIChatMessageDTO, len(rows))
	for i, r := range rows {
		out[i] = AIChatMessageDTO{
			ID: r.ID, Role: r.Role, Content: r.Content, CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

// resolveModelConfig 根据 ModelSource 解析模型配置。
func (s *AIAssistantService) resolveModelConfig(ctx context.Context, userID int, req StreamChatReq) (*modelConfig, error) {
	switch req.ModelSource {
	case "admin":
		cfg, err := s.aiConfigSvc.GetConfigByID(ctx, req.ConfigID)
		if err != nil {
			return nil, err
		}
		if !cfg.IsActive {
			return nil, errors.New("该模型配置已停用")
		}
		return &modelConfig{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model}, nil
	case "user":
		if userID == 0 {
			return nil, errors.New("未登录不能使用用户自定义模型")
		}
		var m model.AIUserModel
		if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", req.UserModelID, userID).
			Limit(1).Find(&m).Error; err != nil {
			return nil, err
		}
		if m.ID == 0 {
			return nil, gorm.ErrRecordNotFound
		}
		return &modelConfig{APIKey: m.APIKey, BaseURL: m.BaseURL, Model: m.Model}, nil
	case "custom":
		if req.CustomAPIKey == "" || req.CustomBaseURL == "" || req.CustomModel == "" {
			return nil, errors.New("自定义模型配置不完整")
		}
		return &modelConfig{APIKey: req.CustomAPIKey, BaseURL: req.CustomBaseURL, Model: req.CustomModel}, nil
	}
	return nil, fmt.Errorf("未知的 model_source: %s", req.ModelSource)
}

// StreamChat 流式对话。
// onChunk 回调用于推送增量内容；返回完整回复内容。
func (s *AIAssistantService) StreamChat(ctx context.Context, userID int, req StreamChatReq, onChunk func(content string)) (string, error) {
	mc, err := s.resolveModelConfig(ctx, userID, req)
	if err != nil {
		return "", err
	}

	// 构建 eino ChatModel（每次新建，避免配置变化后旧实例残留）
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
		Model:   mc.Model,
	})
	if err != nil {
		return "", fmt.Errorf("构建模型失败: %w", err)
	}

	// 拼装消息：系统提示词 + 历史消息
	msgs := []*schema.Message{
		schema.SystemMessage(forkliftExpertSystemPrompt),
	}
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			msgs = append(msgs, schema.UserMessage(m.Content))
		case "assistant":
			msgs = append(msgs, &schema.Message{Role: schema.Assistant, Content: m.Content})
		}
	}

	// 流式调用
	reader, err := chatModel.Stream(ctx, msgs)
	if err != nil {
		return "", fmt.Errorf("调用模型失败: %w", err)
	}
	defer reader.Close()

	var fullContent strings.Builder
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fullContent.String(), fmt.Errorf("流式接收失败: %w", err)
		}
		if msg.Content != "" {
			fullContent.WriteString(msg.Content)
			if onChunk != nil {
				onChunk(msg.Content)
			}
		}
	}

	// 持久化（仅登录用户且指定了 SessionID）
	if userID > 0 && req.SessionID > 0 {
		var lastUserMsg string
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				break
			}
		}
		if lastUserMsg != "" {
			now := time.Now()
			if err := s.db.WithContext(ctx).Create(&model.AIChatMessage{
				SessionID: req.SessionID, Role: "user", Content: lastUserMsg,
			}).Error; err != nil {
				return fullContent.String(), fmt.Errorf("保存用户消息失败: %w", err)
			}
			if err := s.db.WithContext(ctx).Create(&model.AIChatMessage{
				SessionID: req.SessionID, Role: "assistant", Content: fullContent.String(),
			}).Error; err != nil {
				return fullContent.String(), fmt.Errorf("保存助手消息失败: %w", err)
			}
			if err := s.db.WithContext(ctx).Model(&model.AIChatSession{}).
				Where("id = ?", req.SessionID).
				Updates(map[string]any{"updated_at": now}).Error; err != nil {
				return fullContent.String(), fmt.Errorf("更新会话时间失败: %w", err)
			}
		}
	}

	return fullContent.String(), nil
}
