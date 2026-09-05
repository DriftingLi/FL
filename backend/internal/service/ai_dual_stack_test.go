package service

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// TestAIDualStackSharedConfigFromBinding 双栈契约（#606）：两套绑定解析同一配置。
// 阻塞栈（ResolveFeatureSettings → ensureClient）与流式栈（ResolveChatSettings）
// 注入同一 resolver（*AIConfigService），解析结果与 ResolveConfig 单点一致。
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

	// 两栈注入同一 resolver 实例（调用侧不再各自解析的前提）
	if assistant.resolver == nil || aiSvc.resolver == nil {
		t.Fatal("两栈 resolver 均应注入")
	}
	if assistant.resolver != aiConfigSvc || aiSvc.resolver != aiConfigSvc {
		t.Fatalf("两栈应共用同一 resolver 实现: assistant=%T aiSvc=%T", assistant.resolver, aiSvc.resolver)
	}

	// 栈一：流式栈经注入 resolver 解析（与 StreamComplete 内部同一路径）
	mc, err := assistant.resolver.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "admin", ConfigID: cfgs[0].ID})
	if err != nil {
		t.Fatalf("ResolveChatSettings(admin) 失败: %v", err)
	}
	if mc.APIKey != resolved.APIKey || mc.BaseURL != resolved.BaseURL || mc.Model != resolved.Model {
		t.Errorf("流式栈解析结果与 ResolveConfig 不一致: 助手=%+v resolved=%+v", mc, resolved)
	}

	// 栈二：阻塞栈 featureKey 解析 → 从同一形状构建 client
	if err := aiSvc.ensureClient(ctx, FeatureAIAssistant); err != nil {
		t.Fatalf("ensureClient 失败: %v", err)
	}
	if aiSvc.apiKey != resolved.APIKey || aiSvc.baseURL != resolved.BaseURL || aiSvc.model != resolved.Model {
		t.Errorf("非流式栈 client 配置与 ResolveConfig 不一致: svc=(%q,%q,%q) resolved=(%q,%q,%q)",
			aiSvc.apiKey, aiSvc.baseURL, aiSvc.model, resolved.APIKey, resolved.BaseURL, resolved.Model)
	}

	// 阻塞栈流向的 resolver 方法与 ensureClient 同源
	feat, err := aiConfigSvc.ResolveFeatureSettings(ctx, FeatureAIAssistant)
	if err != nil || feat.APIKey != resolved.APIKey || feat.BaseURL != resolved.BaseURL || feat.Model != resolved.Model {
		t.Errorf("ResolveFeatureSettings 与 ResolveConfig 不一致: %+v err=%v", feat, err)
	}
}

// TestAIConfigResolverBranches resolver 分支覆盖（#606）：
// featureKey/选择子 → AISettings 的旧 ModelSource 兼容与解析失败分支
// （专项单绑定/双模式/遗留回退三档阶梯见 ai_config_ladder_test.go）。
func TestAIConfigResolverBranches(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMemoryDB(t)
	if err := db.AutoMigrate(&model.AIConfig{}, &model.AIFeatureBinding{}, &model.AIUserModel{}); err != nil {
		t.Fatalf("AutoMigrate AI 表失败: %v", err)
	}
	secretKey := "test-master-key"
	cfgSvc := NewAIConfigService(db, secretKey, zap.NewNop())

	// 旧 ModelSource=user：解密后的 API Key 与 DB 字段一一映射
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
	mc, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "user", UserID: 7, UserModelID: 1})
	if err != nil {
		t.Fatalf("ResolveChatSettings(user) 失败: %v", err)
	}
	if mc.APIKey != "sk-user-secret" || mc.BaseURL != "https://user.example.com/v1" || mc.Model != "qwen-plus" {
		t.Errorf("user 来源映射异常: %+v", mc)
	}

	// 未登录不能使用用户自定义模型
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "user", UserModelID: 1}); err == nil {
		t.Error("未登录使用 user 来源应报错")
	}

	// 旧 ModelSource=custom：选择子字段直接透传
	mc, err = cfgSvc.ResolveChatSettings(ctx, AIModelSelector{
		ModelSource:  "custom",
		CustomAPIKey: "sk-custom", CustomBaseURL: "https://custom.example.com/v1", CustomModel: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("ResolveChatSettings(custom) 失败: %v", err)
	}
	if mc.APIKey != "sk-custom" || mc.BaseURL != "https://custom.example.com/v1" || mc.Model != "gpt-4o" {
		t.Errorf("custom 来源映射异常: %+v", mc)
	}

	// custom 来源字段不完整 → 报错
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "custom", CustomAPIKey: "sk-custom"}); err == nil {
		t.Error("custom 配置不完整应报错")
	}

	// 未知来源报错
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "unknown"}); err == nil {
		t.Error("未知 model_source 应报错")
	}

	// 阻塞栈流向：featureKey 单绑定解析
	if err := cfgSvc.CreateConfig(ctx, "评分模型", "sk-grading", "https://grading.example.com/v1", "grading-model", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	// 解密失败分支的脏数据配置（提前入库，与评分配置一并按名取 ID）：
	// 带加密前缀但密文非法；不带前缀的历史明文按原样返回，不会触发解密失败
	if err := db.Create(&model.AIConfig{
		Name: "脏数据", APIKey: security.EncryptedPrefix + "not-valid-ciphertext", BaseURL: "https://dirty.example.com", Model: "dirty-model", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("插入脏配置失败: %v", err)
	}
	cfgs, err := cfgSvc.ListConfigs(ctx)
	if err != nil {
		t.Fatalf("ListConfigs 失败: %v", err)
	}
	var gradingID, dirtyID int
	for _, c := range cfgs {
		switch c.Name {
		case "评分模型":
			gradingID = c.ID
		case "脏数据":
			dirtyID = c.ID
		}
	}
	if gradingID == 0 || dirtyID == 0 {
		t.Fatalf("配置查询异常: gradingID=%d dirtyID=%d", gradingID, dirtyID)
	}
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, gradingID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}
	feat, err := cfgSvc.ResolveFeatureSettings(ctx, FeatureGradeShortAnswer)
	if err != nil || feat.APIKey != "sk-grading" || feat.BaseURL != "https://grading.example.com/v1" || feat.Model != "grading-model" {
		t.Errorf("阻塞栈 featureKey 解析异常: %+v err=%v", feat, err)
	}

	// 未绑定功能 / 空 featureKey：报错而非降级
	if _, err := cfgSvc.ResolveFeatureSettings(ctx, FeatureQuestionExplanation); err == nil {
		t.Error("未绑定功能应报错")
	}
	if _, err := cfgSvc.ResolveFeatureSettings(ctx, ""); err == nil {
		t.Error("空 featureKey 应报错")
	}

	// 解析失败分支：API Key 解密失败 → ResolveConfig 标记 decrypt-failed，两栈解析均报错
	if err := cfgSvc.SetBinding(ctx, FeatureGenerateChapterContent, dirtyID); err != nil {
		t.Fatalf("SetBinding(脏数据) 失败: %v", err)
	}
	if got := cfgSvc.ResolveConfig(ctx, FeatureGenerateChapterContent); got.Source != "decrypt-failed" {
		t.Fatalf("解密失败应标记 decrypt-failed: %+v", got)
	}
	if _, err := cfgSvc.ResolveFeatureSettings(ctx, FeatureGenerateChapterContent); err == nil {
		t.Error("解密失败时阻塞栈解析应报错")
	}
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult, ModelSource: "custom", CustomAPIKey: "sk-bypass"}); err == nil {
		t.Error("专项功能未绑定应报错（防绕过：custom 字段不得兜底）")
	}

	// user 来源解密失败 → 报错
	if err := db.Create(&model.AIUserModel{
		UserID: 8, Name: "脏模型", APIKey: security.EncryptedPrefix + "not-valid-ciphertext",
		BaseURL: "https://dirty.example.com", Model: "dirty-user-model",
	}).Error; err != nil {
		t.Fatalf("插入脏用户模型失败: %v", err)
	}
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{ModelSource: "user", UserID: 8, UserModelID: 2}); err == nil {
		t.Error("用户模型解密失败应报错")
	}
}
