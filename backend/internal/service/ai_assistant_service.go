// Package service 实现业务服务层。
// 本文件：AI 助手模块（会话管理 + eino 流式对话）。
package service

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
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

// AIAssistantService AI 助手模块服务。
type AIAssistantService struct {
	db          *gorm.DB
	aiConfigSvc *AIConfigService
	secretKey   string // 用于加密用户自定义 API Key 的主密钥（SECRET_KEY）
	logger      *zap.Logger
}

// NewAIAssistantService 构造 AIAssistantService。
func NewAIAssistantService(db *gorm.DB, aiConfigSvc *AIConfigService, secretKey string, logger *zap.Logger) *AIAssistantService {
	return &AIAssistantService{db: db, aiConfigSvc: aiConfigSvc, secretKey: secretKey, logger: logger}
}

// ListPublicModels 返回管理员绑定到 AI 助手功能的可用配置列表（不含 api_key）。
// 仅返回 ai_feature_bindings 中 feature_key='ai_assistant' 绑定且 is_active=true 的配置。
// 若管理员未绑定任何配置，返回空列表（前端将提示用户选择自定义模型）。
func (s *AIAssistantService) ListPublicModels(ctx context.Context) ([]ModelOption, error) {
	cfgs, err := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistant)
	if err != nil {
		return nil, err
	}
	out := make([]ModelOption, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, ModelOption{ID: c.ID, Name: c.Name, Model: c.Model, BaseURL: c.BaseURL})
	}
	return out, nil
}

// ListUserModels 返回登录用户的自定义模型列表（api_key 脱敏）。
func (s *AIAssistantService) ListUserModels(ctx context.Context, userID int) ([]UserModelDTO, error) {
	var rows []model.AIUserModel
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]UserModelDTO, len(rows))
	for i, r := range rows {
		key, err := security.DecryptSecret(r.APIKey, s.secretKey)
		if err != nil {
			s.logger.Warn("ListUserModels 解密 API Key 失败，按原样脱敏展示", zap.Int("id", r.ID), zap.Error(err))
			key = r.APIKey
		}
		out[i] = UserModelDTO{
			ID: r.ID, Name: r.Name, APIKey: MaskKey(key),
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

	encKey, err := security.EncryptSecret(req.APIKey, s.secretKey)
	if err != nil {
		return fmt.Errorf("加密 API Key 失败: %w", err)
	}

	if req.ID > 0 {
		// 更新
		updates := map[string]any{
			"name": req.Name, "api_key": encKey,
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
		UserID: userID, Name: req.Name, APIKey: encKey,
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

// RenameSession 修改会话标题（校验归属）。
// 标题长度限制 100 字符；非空校验。
func (s *AIAssistantService) RenameSession(ctx context.Context, userID, sessionID int, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return errors.New("标题不能为空")
	}
	if r := []rune(title); len(r) > 100 {
		return errors.New("标题不能超过 100 字符")
	}
	res := s.db.WithContext(ctx).Model(&model.AIChatSession{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Update("title", title)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// autoTitleTriggerThreshold 自动命名的标题触发值。
// 仅当会话标题等于该值时（即从未被 AI 命名或用户重命名过），首次对话后才会自动生成标题。
const autoTitlePlaceholder = "新会话"

// maybeGenerateSessionTitle 异步生成会话标题。
// 仅当会话标题为占位符 "新会话" 时才生成；已被 AI 命名或用户手动改名后不再覆盖。
// 失败仅记录日志，不影响主流程。
func (s *AIAssistantService) maybeGenerateSessionTitle(ctx context.Context, userID, sessionID int, mc AISettings) {
	// 查询会话，校验归属和标题
	var session model.AIChatSession
	if err := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Limit(1).Find(&session).Error; err != nil {
		s.logger.Warn("自动命名：查询会话失败", zap.Int("session_id", sessionID), zap.Error(err))
		return
	}
	if session.ID == 0 {
		return
	}
	// 仅当标题为占位符时才生成（已被命名或用户手动改名后不再覆盖）
	if session.Title != autoTitlePlaceholder {
		return
	}

	// 拉取首条用户消息作为生成依据
	var userMsg model.AIChatMessage
	if err := s.db.WithContext(ctx).
		Where("session_id = ? AND role = ?", sessionID, "user").
		Order("created_at ASC").Limit(1).Find(&userMsg).Error; err != nil {
		s.logger.Warn("自动命名：查询用户消息失败", zap.Int("session_id", sessionID), zap.Error(err))
		return
	}
	if userMsg.ID == 0 || strings.TrimSpace(userMsg.Content) == "" {
		return
	}

	title, err := s.generateTitleWithModel(ctx, mc, userMsg.Content)
	if err != nil {
		s.logger.Warn("自动命名：调用模型失败", zap.Int("session_id", sessionID), zap.Error(err))
		return
	}
	title = sanitizeTitle(title)
	if title == "" || title == autoTitlePlaceholder {
		return
	}

	// 双重保险：再次校验标题仍为占位符（防止并发用户已手动改名）
	res := s.db.WithContext(ctx).Model(&model.AIChatSession{}).
		Where("id = ? AND user_id = ? AND title = ?", sessionID, userID, autoTitlePlaceholder).
		Update("title", title)
	if res.Error != nil {
		s.logger.Warn("自动命名：更新失败", zap.Int("session_id", sessionID), zap.Error(res.Error))
		return
	}
	if res.RowsAffected == 0 {
		// 会话不存在 / 不属于该用户 / 标题已不再是占位符（用户已手动改名）
		return
	}
	s.logger.Info("自动命名完成", zap.Int("session_id", sessionID), zap.String("title", title))
}

// generateTitleWithModel 调用同模型根据用户首条消息生成简短标题。
func (s *AIAssistantService) generateTitleWithModel(ctx context.Context, mc AISettings, userMessage string) (string, error) {
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
		Model:   mc.Model,
	})
	if err != nil {
		return "", err
	}

	const titlePrompt = `请根据用户的问题，生成一个简短的中文会话标题。
要求：
1. 不超过 20 个字
2. 直接输出标题文字，不要加任何前缀（如"标题："、"会话："等）
3. 不要使用引号、书名号、冒号等符号
4. 不要以句号、问号等标点符号结尾
5. 概括用户问题的核心主题

用户问题：%s`

	msgs := []*schema.Message{
		schema.SystemMessage("你是一个会话标题生成助手，根据用户消息生成简短的中文标题。"),
		schema.UserMessage(fmt.Sprintf(titlePrompt, userMessage)),
	}

	reader, err := chatModel.Stream(ctx, msgs)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var sb strings.Builder
	for {
		msg, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return sb.String(), err
		}
		if msg.Content != "" {
			sb.WriteString(msg.Content)
		}
	}
	return sb.String(), nil
}

// titleTrimRunes 需要从标题首尾去除的字符集合（使用 map 保证唯一性，避免 SA1024）。
var titleTrimRunes = map[rune]bool{
	'"':  true,            // ASCII 双引号
	'\'': true,            // ASCII 单引号
	'“':  true,            // 中文左双引号
	'”':  true,            // 中文右双引号
	'‘':  true,            // 中文左单引号
	'’':  true,            // 中文右单引号
	'「':  true, '」': true, // 日式直角引号
	'『': true, '』': true, // 日式双直角引号
	'【': true, '】': true, // 中文方头括号
	'《': true, '》': true, // 中文书名号
	'<': true, '>': true, // ASCII 尖括号
	':': true, '：': true, // 冒号（ASCII + 中文）
	'，': true, '。': true, // 中文逗号 + 中文句号
	'.': true,            // ASCII 句点
	'？': true, '?': true, // 问号（中文 + ASCII）
	'!': true, '！': true, // 感叹号（ASCII + 中文）
	';': true, '；': true, // 分号（ASCII + 中文）
}

// sanitizeTitle 清理模型生成的标题：去除空白、引号、首尾标点；截断到 30 字符。
func sanitizeTitle(raw string) string {
	t := strings.TrimSpace(raw)
	t = strings.TrimFunc(t, func(r rune) bool {
		return titleTrimRunes[r]
	})
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if r := []rune(t); len(r) > 30 {
		t = string(r[:30])
	}
	return t
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

// resolveModelConfig 根据 ModelSource 解析模型配置（与 AIService 消费同一 AISettings 形状）。
func (s *AIAssistantService) resolveModelConfig(ctx context.Context, userID int, req StreamChatReq) (AISettings, error) {
	switch req.ModelSource {
	case "admin":
		// 校验该配置是否被管理员绑定到 AI 助手功能（防止用户绕过前端传任意 config_id）
		boundCfgs, err := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistant)
		if err != nil {
			return AISettings{}, fmt.Errorf("校验可用模型失败: %w", err)
		}
		var cfg *model.AIConfig
		for i := range boundCfgs {
			if boundCfgs[i].ID == req.ConfigID {
				cfg = &boundCfgs[i]
				break
			}
		}
		if cfg == nil {
			return AISettings{}, errors.New("该模型未绑定到 AI 助手，请联系管理员或选择自定义模型")
		}
		return AISettings{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Source: "binding:" + cfg.Name}, nil
	case "user":
		if userID == 0 {
			return AISettings{}, errors.New("未登录不能使用用户自定义模型")
		}
		var m model.AIUserModel
		if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", req.UserModelID, userID).
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
		if req.CustomAPIKey == "" || req.CustomBaseURL == "" || req.CustomModel == "" {
			return AISettings{}, errors.New("自定义模型配置不完整")
		}
		return AISettings{APIKey: req.CustomAPIKey, BaseURL: req.CustomBaseURL, Model: req.CustomModel, Source: "custom"}, nil
	}
	return AISettings{}, fmt.Errorf("未知的 model_source: %s", req.ModelSource)
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

			// 异步生成会话标题：仅当标题为占位符"新会话"时（首次对话）
			// 使用独立 context 避免请求结束后被取消；recover 防止 panic 影响主流程
			mcCopy := mc
			sessionID := req.SessionID
			uid := userID
			go func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Warn("自动命名 panic", zap.Int("session_id", sessionID), zap.Any("panic", r))
					}
				}()
				bgCtx := context.Background()
				s.maybeGenerateSessionTitle(bgCtx, uid, sessionID, mcCopy)
			}()
		}
	}

	return fullContent.String(), nil
}
