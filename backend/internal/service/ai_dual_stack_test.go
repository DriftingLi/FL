package service

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestAIDualStackSharedConfigFromBinding(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMemoryDB(t)
	if err := db.AutoMigrate(&model.AIConfig{}, &model.AIFeatureBinding{}, &model.AIUserModel{}); err != nil {
		t.Fatalf("AutoMigrate AI 表失败: %v", err)
	}
	secretKey := "test-master-key"
	aiConfigSvc := NewAIConfigService(db, secretKey, zap.NewNop())
	assistant := NewAIAssistantService(db, aiConfigSvc, NewFileStore("", nil, zap.NewNop()), secretKey, zap.NewNop())
	aiSvc := NewAIService(db, aiConfigSvc, zap.NewNop())

	const (
		apiKey  = "sk-test-1234567890"
		baseURL = "https://api.example.com/v1"
		model   = "deepseek-chat"
	)
	if err := aiConfigSvc.CreateConfig(ctx, "测试模型", apiKey, baseURL, model, ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, err := aiConfigSvc.ListConfigs(ctx)
	if err != nil || len(cfgs) != 1 {
		t.Fatalf("ListConfigs 失败: %v, %v", cfgs, err)
	}
	if err := aiConfigSvc.SetBinding(ctx, FeatureAIAssistant, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}

	// 唯一 resolved 配置来源
	resolved := aiConfigSvc.ResolveConfig(ctx, FeatureAIAssistant)
	if resolved.APIKey != apiKey || resolved.BaseURL != baseURL || resolved.Model != model {
		t.Fatalf("ResolveConfig 解析异常: %+v", resolved)
	}

	// 栈一：eino 流式助手从同一形状解析（admin 绑定路径）
	mc, err := assistant.resolveModelConfig(ctx, 0, StreamChatReq{ModelSource: "admin", ConfigID: cfgs[0].ID})
	if err != nil {
		t.Fatalf("resolveModelConfig(admin) 失败: %v", err)
	}
	if mc.APIKey != resolved.APIKey || mc.BaseURL != resolved.BaseURL || mc.Model != resolved.Model {
		t.Errorf("流式栈解析结果与 ResolveConfig 不一致: 助手=%+v resolved=%+v", mc, resolved)
	}

	// 栈二：go-openai 非流式栈从同一形状构建 client
	if err := aiSvc.ensureClient(ctx, FeatureAIAssistant); err != nil {
		t.Fatalf("ensureClient 失败: %v", err)
	}
	if aiSvc.apiKey != resolved.APIKey || aiSvc.baseURL != resolved.BaseURL || aiSvc.model != resolved.Model {
		t.Errorf("非流式栈 client 配置与 ResolveConfig 不一致: svc=(%q,%q,%q) resolved=(%q,%q,%q)",
			aiSvc.apiKey, aiSvc.baseURL, aiSvc.model, resolved.APIKey, resolved.BaseURL, resolved.Model)
	}
}

func TestAIAssistantResolveModelConfig_Sources(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMemoryDB(t)
	if err := db.AutoMigrate(&model.AIConfig{}, &model.AIFeatureBinding{}, &model.AIUserModel{}); err != nil {
		t.Fatalf("AutoMigrate AI 表失败: %v", err)
	}
	secretKey := "test-master-key"
	aiConfigSvc := NewAIConfigService(db, secretKey, zap.NewNop())
	assistant := NewAIAssistantService(db, aiConfigSvc, NewFileStore("", nil, zap.NewNop()), secretKey, zap.NewNop())

	// user 来源：解密后的 API Key 与 DB 字段一一映射
	encKey, err := security.EncryptSecret("sk-user-secret", secretKey)
	if err != nil {
		t.Fatalf("EncryptSecret 失败: %v", err)
	}
	if err := db.Create(&model.AIUserModel{
		UserID: 7, Name: "我的模型", APIKey: encKey,
		BaseURL: "https://user.example.com/v1", Model: "qwen-plus",
	}).Error; err != nil {
		t.Fatalf("插入用户模型失败: %v", err)
	}
	mc, err := assistant.resolveModelConfig(ctx, 7, StreamChatReq{ModelSource: "user", UserModelID: 1})
	if err != nil {
		t.Fatalf("resolveModelConfig(user) 失败: %v", err)
	}
	if mc.APIKey != "sk-user-secret" || mc.BaseURL != "https://user.example.com/v1" || mc.Model != "qwen-plus" {
		t.Errorf("user 来源映射异常: %+v", mc)
	}

	// custom 来源：请求字段直接透传
	mc, err = assistant.resolveModelConfig(ctx, 0, StreamChatReq{
		ModelSource:  "custom",
		CustomAPIKey: "sk-custom", CustomBaseURL: "https://custom.example.com/v1", CustomModel: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("resolveModelConfig(custom) 失败: %v", err)
	}
	if mc.APIKey != "sk-custom" || mc.BaseURL != "https://custom.example.com/v1" || mc.Model != "gpt-4o" {
		t.Errorf("custom 来源映射异常: %+v", mc)
	}

	// 未知来源报错
	if _, err := assistant.resolveModelConfig(ctx, 0, StreamChatReq{ModelSource: "unknown"}); err == nil {
		t.Error("未知 model_source 应报错")
	}

	// 未绑定功能：go-openai 栈报错而非降级
	aiSvc := NewAIService(db, aiConfigSvc, zap.NewNop())
	if err := aiSvc.ensureClient(ctx, FeatureGradeShortAnswer); err == nil {
		t.Error("未绑定功能 ensureClient 应报错")
	}
}
