// Package service 实现业务服务层。
// 本文件：AI 助手模块（会话管理 + eino 流式对话）。
package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"io"
	"mime/multipart"
	"strings"
	"time"

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

// faultConsultSystemPrompt 故障咨询系统提示词。
const faultConsultSystemPrompt = `你是一名资深的叉车故障诊断专家，拥有 20 年以上一线维修经验。
你熟悉林德、丰田、杭叉、合力、永恒力、TCM 等主流品牌叉车的常见故障模式。

回答要求：
1. 用中文回答，专业、实用、可操作
2. 按"可能原因 → 排查步骤 → 处理方法"的结构组织回答
3. 按可能性从高到低排列原因，说明判断依据
4. 排查步骤具体到工具、测量位置、判断标准
5. 涉及安全的操作（制动、液压、电气高压部件）必须明确警示
6. 需要专业设备或资质的维修，明确提示联系专业维修人员
7. 信息不足时先列出需要确认的关键信息，再给出初步判断
8. 不确定时坦诚告知，不编造数据`

// faultCodeQuerySystemPrompt 故障代码查询系统提示词。
const faultCodeQuerySystemPrompt = `你是一名叉车故障代码专家，精通国内外主流品牌（林德、丰田、杭叉、合力、永恒力、TCM 等）叉车自诊断系统的故障代码体系。

回答要求：
1. 用中文回答，按"代码含义 → 严重程度 → 可能原因 → 处理建议"结构组织
2. 严重程度分为：紧急（立即停机）、重要（尽快处理）、一般（可短时继续作业）
3. 不同品牌的代码编号可能相同但含义不同；用户未提供品牌时，先询问品牌与车型，同时给出常见品牌下的典型含义参考
4. 处理建议具体到操作步骤与所需工具
5. 明确提示：最终诊断应以对应品牌官方维修手册为准
6. 不确定时坦诚告知，不编造代码含义`

// maintenanceKnowledgeSystemPrompt 维保知识系统提示词。
const maintenanceKnowledgeSystemPrompt = `你是一名叉车维保专家，熟悉各品牌电动叉车、内燃叉车的保养体系与行业标准。

回答要求：
1. 用中文回答，按"保养周期 → 保养项目 → 执行标准 → 注意事项"结构组织
2. 区分日常检查（班前班后）、周检、月度、季度、年度保养的项目差异
3. 给出可量化的标准（如液压油型号、轮胎磨损限度、电瓶电解液比重范围）
4. 电动叉车重点说明电瓶充放电与维护规范，内燃叉车说明机油滤芯与排放要求
5. 涉及安全的操作必须明确警示
6. 不确定时坦诚告知，不编造数据`

// drawingRecognitionSystemPrompt 图纸识别系统提示词。
const drawingRecognitionSystemPrompt = `你是一名叉车图纸分析专家，擅长识读叉车领域的机械结构图、装配图、电路原理图、液压原理图与气动回路图。

回答要求：
1. 用中文回答，先整体描述图纸类型与表达内容，再分项解读
2. 机械图纸：识别主要部件名称、装配关系、配合公差与关键尺寸
3. 电路图纸：识别电器元件符号、供电回路、控制逻辑与保护装置
4. 液压图纸：识别泵、阀、缸等元件，说明油路走向与工作原理
5. 图纸不清晰或无法辨认的部分，明确说明，不猜测编造
6. 结尾给出需要用户补充确认的信息（如图纸版本、部件编号）`

// exerciseSolvingSystemPrompt 习题解答系统提示词。
const exerciseSolvingSystemPrompt = `你是一名叉车维修培训教员，负责解答叉车操作、维保、安全规范相关的培训习题与考试题目。

回答要求：
1. 用中文回答，先给出最终答案，再给出解析
2. 解析按步骤推理，说明每一步的依据（法规、原理、操作规范）
3. 说明题目考查的知识点，便于举一反三
4. 若题目信息不完整或有歧义，列出需要补充的条件并按最常见理解作答
5. 涉及安全规范的题目，注明依据的标准或规范名称
6. 不确定时坦诚告知，不编造答案`

// featureChatKeys 专项功能键集合（模型由管理端单绑定解析，用户无需选模型）。
var featureChatKeys = map[string]bool{
	FeatureFaultConsult:         true,
	FeatureFaultCodeQuery:       true,
	FeatureMaintenanceKnowledge: true,
	FeatureDrawingRecognition:   true,
	FeatureExerciseSolving:      true,
}

// featureSystemPrompt 返回功能对应的系统提示词；未注册的功能回退到通用叉车专家提示词。
func featureSystemPrompt(featureKey string) string {
	switch featureKey {
	case FeatureFaultConsult:
		return faultConsultSystemPrompt
	case FeatureFaultCodeQuery:
		return faultCodeQuerySystemPrompt
	case FeatureMaintenanceKnowledge:
		return maintenanceKnowledgeSystemPrompt
	case FeatureDrawingRecognition:
		return drawingRecognitionSystemPrompt
	case FeatureExerciseSolving:
		return exerciseSolvingSystemPrompt
	}
	return forkliftExpertSystemPrompt
}

// aiImageDirPrefix AI 助手图片存储子目录（URL/对象 key 前缀）。
const aiImageDirPrefix = "images/ai-assistant"

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
	ID         int       `json:"id"`
	Title      string    `json:"title"`
	ModelName  string    `json:"model_name"`
	FeatureKey string    `json:"feature_key"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AIChatMessageDTO 消息展示对象。
type AIChatMessageDTO struct {
	ID        int       `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Images    []string  `json:"images"` // 用户消息附带的图片 URL
	CreatedAt time.Time `json:"created_at"`
}

// AIAssistantMode AI 助手模式（隐藏底层模型，对用户仅暴露双模式）。
type AIAssistantMode string

const (
	ModeNormal AIAssistantMode = "normal"
	ModeExpert AIAssistantMode = "expert"
)

// AIAssistantModeModels 双模式可用模型（按普通/专家分别返回，null 表示未绑定）。
type AIAssistantModeModels struct {
	Normal *ModelOption `json:"normal"`
	Expert *ModelOption `json:"expert"`
}

// StreamChatReq 流式对话请求。
// 专项功能：FeatureKey=fault_consult 等（管理端单绑定模型）；
// 通用助手：Mode=normal|expert（隐藏底层模型，推荐）；兼容旧前端：ModelSource=admin|user|custom + ConfigID
type StreamChatReq struct {
	SessionID     int             `json:"session_id"`     // 可选，登录用户指定会话
	FeatureKey    string          `json:"feature_key"`    // 专项功能键（空/"ai_assistant"=通用对话；专项功能走单绑定模型）
	Mode          AIAssistantMode `json:"mode"`           // 通用助手：normal | expert
	ModelSource   string          `json:"model_source"`   // 兼容旧： "admin" | "user" | "custom"
	ConfigID      int             `json:"config_id"`      // 兼容旧：ModelSource="admin" 时引用管理员配置
	UserModelID   int             `json:"user_model_id"`  // 兼容旧：ModelSource="user" 时引用用户自定义模型
	CustomAPIKey  string          `json:"custom_api_key"` // 兼容旧：ModelSource="custom" 时临时输入
	CustomBaseURL string          `json:"custom_base_url"`
	CustomModel   string          `json:"custom_model"`
	Messages      []struct {
		Role    string   `json:"role"`
		Content string   `json:"content"`
		Images  []string `json:"images"` // 用户消息附带的图片 URL（仅最后一条用户消息生效）
	} `json:"messages"`
}

// AIAssistantService AI 助手模块服务。
type AIAssistantService struct {
	db          *gorm.DB
	aiConfigSvc *AIConfigService
	fileSvc     *FileStore // 图片上传/读取（多模态对话）
	secretKey   string     // 用于加密用户自定义 API Key 的主密钥（SECRET_KEY）
	logger      *zap.Logger
	streamer    AIStreamingTransport // 流式传输槽位（nil 时自实装；测试可注入 fake）
}

// NewAIAssistantService 构造 AIAssistantService。
func NewAIAssistantService(db *gorm.DB, aiConfigSvc *AIConfigService, fileSvc *FileStore, secretKey string, logger *zap.Logger) *AIAssistantService {
	return &AIAssistantService{db: db, aiConfigSvc: aiConfigSvc, fileSvc: fileSvc, secretKey: secretKey, logger: logger}
}

// ListPublicModels 返回管理员绑定到 AI 助手功能的可用配置列表（不含 api_key）。
// 兼容旧前端：优先返回 normal/expert 双绑定的配置；若未配置则回退到遗留 ai_assistant 多绑定。
func (s *AIAssistantService) ListPublicModels(ctx context.Context) ([]ModelOption, error) {
	modes, err := s.ListAssistantModes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ModelOption, 0, 2)
	if modes.Normal != nil {
		out = append(out, *modes.Normal)
	}
	if modes.Expert != nil {
		out = append(out, *modes.Expert)
	}
	if len(out) > 0 {
		return out, nil
	}
	// 回退：旧 ai_assistant 多绑定
	cfgs, err := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistant)
	if err != nil {
		return nil, err
	}
	for _, c := range cfgs {
		out = append(out, ModelOption{ID: c.ID, Name: c.Name, Model: c.Model, BaseURL: c.BaseURL})
	}
	return out, nil
}

// ListAssistantModes 返回双模式（普通/专家）分别绑定的配置（不含 api_key），新前端专用。
// 降级阶梯在 AIConfigService.ResolveAssistantPair 单点。
func (s *AIAssistantService) ListAssistantModes(ctx context.Context) (AIAssistantModeModels, error) {
	var res AIAssistantModeModels
	normal, expert, err := s.aiConfigSvc.ResolveAssistantPair(ctx)
	if err != nil {
		return res, err
	}
	if normal != nil {
		res.Normal = &ModelOption{ID: normal.ID, Name: normal.Name, Model: normal.Model, BaseURL: normal.BaseURL}
	}
	if expert != nil {
		res.Expert = &ModelOption{ID: expert.ID, Name: expert.Name, Model: expert.Model, BaseURL: expert.BaseURL}
	}
	return res, nil
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

// CreateSession 创建会话（需登录）。featureKey 为空时归入通用 AI 助手。
func (s *AIAssistantService) CreateSession(ctx context.Context, userID int, title, modelName, featureKey string) (*AIChatSessionDTO, error) {
	if title == "" {
		title = "新会话"
	}
	if featureKey == "" {
		featureKey = FeatureAIAssistant
	}
	row := model.AIChatSession{UserID: userID, Title: title, ModelName: modelName, FeatureKey: featureKey}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &AIChatSessionDTO{
		ID: row.ID, Title: row.Title, ModelName: row.ModelName, FeatureKey: row.FeatureKey,
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

// generateTitleWithModel 调用同模型根据用户首条消息生成简短标题
// （client 构建/超时/收集循环全部在流式槽位单点）。
func (s *AIAssistantService) generateTitleWithModel(ctx context.Context, mc AISettings, userMessage string) (string, error) {
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
	return s.streamingSlot().StreamComplete(ctx, mc, msgs, nil)
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

// ListSessions 返回登录用户指定功能的会话列表（按创建时间倒序）。
// featureKey 为空时归入通用 AI 助手。
func (s *AIAssistantService) ListSessions(ctx context.Context, userID int, featureKey string) ([]AIChatSessionDTO, error) {
	if featureKey == "" {
		featureKey = FeatureAIAssistant
	}
	var rows []model.AIChatSession
	if err := s.db.WithContext(ctx).Where("user_id = ? AND feature_key = ?", userID, featureKey).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AIChatSessionDTO, len(rows))
	for i, r := range rows {
		out[i] = AIChatSessionDTO{
			ID: r.ID, Title: r.Title, ModelName: r.ModelName, FeatureKey: r.FeatureKey,
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
		var imgs []string
		if r.Images != "" {
			// 解析失败按无图处理，不阻断消息列表
			_ = json.Unmarshal([]byte(r.Images), &imgs)
		}
		out[i] = AIChatMessageDTO{
			ID: r.ID, Role: r.Role, Content: r.Content, Images: imgs, CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

// resolveModelConfig 解析模型配置（与 AIService 消费同一 AISettings 形状）。
// 优先级：专项功能（FeatureKey，管理端单绑定）→ Mode 双模式（normal/expert）→ 兼容旧 ModelSource。
func (s *AIAssistantService) resolveModelConfig(ctx context.Context, userID int, req StreamChatReq) (AISettings, error) {
	// 专项功能：由管理端单绑定解析，忽略请求中的模型来源字段（防绕过）
	if featureChatKeys[req.FeatureKey] {
		mc := s.aiConfigSvc.ResolveConfig(ctx, req.FeatureKey)
		if mc.APIKey == "" {
			return AISettings{}, errors.New("管理员未配置该功能的模型，请联系管理员")
		}
		return mc, nil
	}
	// 通用助手：Mode 双模式（隐藏底层模型）——降级阶梯在 ResolveAssistantPair 单点
	if req.Mode == ModeNormal || req.Mode == ModeExpert {
		normal, expert, err := s.aiConfigSvc.ResolveAssistantPair(ctx)
		if err != nil {
			return AISettings{}, fmt.Errorf("校验可用模型失败: %w", err)
		}
		cfg := normal
		if req.Mode == ModeExpert {
			cfg = expert
		}
		if cfg == nil {
			return AISettings{}, errors.New("该模式未绑定模型，请联系管理员配置")
		}
		return AISettings{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model, Source: "binding:" + cfg.Name}, nil
	}
	switch req.ModelSource {
	case "admin":
		// 校验该配置是否被管理员绑定到 AI 助手功能（兼容旧前端：同时校验新双绑定的两个 Feature）
		boundCfgsNormal, _ := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistantNormal)
		boundCfgsExpert, _ := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistantExpert)
		boundCfgsLegacy, _ := s.aiConfigSvc.ListConfigsForFeature(ctx, FeatureAIAssistant)
		allBound := append(append(boundCfgsNormal, boundCfgsExpert...), boundCfgsLegacy...)
		var cfg *model.AIConfig
		for i := range allBound {
			if allBound[i].ID == req.ConfigID {
				cfg = &allBound[i]
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
// 传输（建 client/超时/Recv 收集）在流式槽位 StreamComplete 单点；此处留 prompt 组装与持久化真语义。
func (s *AIAssistantService) StreamChat(ctx context.Context, userID int, req StreamChatReq, onChunk func(content string)) (string, error) {
	mc, err := s.resolveModelConfig(ctx, userID, req)
	if err != nil {
		return "", err
	}

	// 拼装消息：功能系统提示词 + 历史消息
	msgs := []*schema.Message{
		schema.SystemMessage(featureSystemPrompt(req.FeatureKey)),
	}
	// 仅最后一条用户消息附带图片（多模态）；历史图片不重发（控制 token，规避多图限制）
	lastUserIdx := -1
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	for i, m := range req.Messages {
		switch m.Role {
		case "user":
			if i == lastUserIdx && len(m.Images) > 0 {
				userMsg, err := s.buildImageUserMessage(ctx, m.Content, m.Images)
				if err != nil {
					return "", err
				}
				msgs = append(msgs, userMsg)
				continue
			}
			msgs = append(msgs, schema.UserMessage(m.Content))
		case "assistant":
			msgs = append(msgs, &schema.Message{Role: schema.Assistant, Content: m.Content})
		}
	}

	fullContent, err := s.streamingSlot().StreamComplete(ctx, mc, msgs, onChunk)
	if err != nil {
		return fullContent, err
	}

	// 持久化（仅登录用户且指定了 SessionID）
	if userID > 0 && req.SessionID > 0 {
		var lastUserMsg string
		var lastUserImages []string
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastUserMsg = req.Messages[i].Content
				lastUserImages = req.Messages[i].Images
				break
			}
		}
		if lastUserMsg != "" || len(lastUserImages) > 0 {
			// 纯图片消息（无文本）也允许持久化
			imagesJSON := ""
			if len(lastUserImages) > 0 {
				if b, err := json.Marshal(lastUserImages); err == nil {
					imagesJSON = string(b)
				}
			}
			now := time.Now()
			if err := s.db.WithContext(ctx).Create(&model.AIChatMessage{
				SessionID: req.SessionID, Role: "user", Content: lastUserMsg, Images: imagesJSON,
			}).Error; err != nil {
				return fullContent, fmt.Errorf("保存用户消息失败: %w", err)
			}
			if err := s.db.WithContext(ctx).Create(&model.AIChatMessage{
				SessionID: req.SessionID, Role: "assistant", Content: fullContent,
			}).Error; err != nil {
				return fullContent, fmt.Errorf("保存助手消息失败: %w", err)
			}
			if err := s.db.WithContext(ctx).Model(&model.AIChatSession{}).
				Where("id = ?", req.SessionID).
				Updates(map[string]any{"updated_at": now}).Error; err != nil {
				return fullContent, fmt.Errorf("更新会话时间失败: %w", err)
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

	return fullContent, nil
}

// buildImageUserMessage 构建带图片的多模态用户消息。
// 仅接受本站 images/ai-assistant/ 前缀的 URL（防 SSRF）；单张读取失败时跳过并在文本中注明，不中断对话。
func (s *AIAssistantService) buildImageUserMessage(ctx context.Context, content string, images []string) (*schema.Message, error) {
	parts := make([]schema.MessageInputPart, 0, len(images)+2)
	if content != "" {
		parts = append(parts, schema.MessageInputPart{Type: schema.ChatMessagePartTypeText, Text: content})
	}
	var loadFails []string
	for _, u := range images {
		if !isAIImageURL(u) {
			loadFails = append(loadFails, u)
			continue
		}
		data, mime, err := s.fileSvc.Read(u)
		if err != nil || len(data) == 0 {
			s.logger.Warn("读取图片失败，跳过", zap.String("url", u), zap.Error(err))
			loadFails = append(loadFails, u)
			continue
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					Base64Data: &b64,
					MIMEType:   mime,
				},
			},
		})
	}
	if len(loadFails) > 0 {
		parts = append(parts, schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeText,
			Text: "[部分图片加载失败: " + strings.Join(loadFails, ", ") + "]",
		})
	}
	if len(parts) == 0 {
		return nil, errors.New("消息内容为空")
	}
	return &schema.Message{Role: schema.User, UserInputMultiContent: parts}, nil
}

// UploadImage 上传 AI 助手对话图片：校验格式/大小后保存到 images/ai-assistant/ 子目录，返回可访问 URL。
func (s *AIAssistantService) UploadImage(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Filename == "" {
		return "", errors.New("未选择文件")
	}
	if ok, msg := s.fileSvc.ValidateImage(fileHeader.Filename, fileHeader.Size); !ok {
		return "", errors.New(msg)
	}
	src, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("图片上传失败")
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		return "", errors.New("图片上传失败")
	}
	url, err := s.fileSvc.Save(content, fileHeader.Filename, aiImageDirPrefix)
	if err != nil {
		return "", errors.New("图片上传失败: " + err.Error())
	}
	return url, nil
}

// isAIImageURL 判断 URL 是否指向本站 images/ai-assistant/ 子目录。
// local：/static/uploads/images/ai-assistant/xxx；R2：https://<域名>/images/ai-assistant/xxx。
func isAIImageURL(u string) bool {
	u = strings.TrimSpace(u)
	if u == "" {
		return false
	}
	if strings.HasPrefix(u, "/static/uploads/images/ai-assistant/") {
		return true
	}
	idx := strings.Index(u, "/images/ai-assistant/")
	return idx > 0 && (strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://"))
}
