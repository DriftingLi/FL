// Package service AI 配置解析与传输收敛回归（#397）：
// 降级阶梯三档（专项单绑定/双模式/遗留回退）、热点缓存失效、
// Blocking/Streaming 双 slot 端口注入与各栈端到端（评分/解析/对话）。
package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newAIStack(t *testing.T) (*AIConfigService, *AIAssistantService, *AIService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	cfgSvc := NewAIConfigService(db, "test-master-key", zap.NewNop())
	assistant := NewAIAssistantService(db, cfgSvc, NewFileStore("", nil, zap.NewNop()), "test-master-key", zap.NewNop())
	aiSvc := NewAIService(db, cfgSvc, zap.NewNop())
	return cfgSvc, assistant, aiSvc, db
}

// TestResolveAssistantLadderThreeTiers 降级阶梯三档：
// ① 专项单绑定 → ② normal/expert 双模式 → ③ 遗留 ai_assistant 回退。
func TestResolveAssistantLadderThreeTiers(t *testing.T) {
	cfgSvc, assistant, _, db := newAIStack(t)
	ctx := context.Background()

	mkConfig := func(name string) int {
		if err := cfgSvc.CreateConfig(ctx, name, "sk-"+name, "https://api.example.com/v1", "m-"+name, ""); err != nil {
			t.Fatalf("CreateConfig(%s) 失败: %v", name, err)
		}
		cfgs, err := cfgSvc.ListConfigs(ctx)
		if err != nil {
			t.Fatalf("ListConfigs 失败: %v", err)
		}
		for _, c := range cfgs {
			if c.Name == name {
				return c.ID
			}
		}
		t.Fatal("未找到新建配置")
		return 0
	}

	// ③ 遗留回退：仅遗留 ai_assistant 两条绑定（多绑定改造前的存量数据形态）
	if err := db.Create(&model.AIFeatureBinding{FeatureKey: FeatureAIAssistant, ConfigID: mkConfig("legacy0")}).Error; err != nil {
		t.Fatalf("建遗留绑定失败: %v", err)
	}
	if err := db.Create(&model.AIFeatureBinding{FeatureKey: FeatureAIAssistant, ConfigID: mkConfig("legacy1")}).Error; err != nil {
		t.Fatalf("建遗留绑定失败: %v", err)
	}
	modes, err := assistant.ListAssistantModes(ctx)
	if err != nil {
		t.Fatalf("ListAssistantModes 失败: %v", err)
	}
	if modes.Normal == nil || modes.Normal.Name != "legacy0" || modes.Expert == nil || modes.Expert.Name != "legacy1" {
		t.Fatalf("遗留回退映射异常: %+v", modes)
	}
	mc, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{Mode: ModeExpert})
	if err != nil {
		t.Fatalf("ResolveChatSettings(expert) 失败: %v", err)
	}
	if !strings.Contains(mc.APIKey, "legacy1") {
		t.Fatalf("expert 模式应解析到遗留第二条配置: %+v", mc)
	}

	// ② 双模式：normal 单绑定后，expert 未绑定 → 报错（展示与解析同一阶梯）
	normalID := mkConfig("normal-bind")
	if err := cfgSvc.SetBinding(ctx, FeatureAIAssistantNormal, normalID); err != nil {
		t.Fatalf("SetBinding(normal) 失败: %v", err)
	}
	modes, err = assistant.ListAssistantModes(ctx)
	if err != nil {
		t.Fatalf("ListAssistantModes 失败: %v", err)
	}
	if modes.Normal == nil || modes.Normal.Name != "normal-bind" || modes.Expert != nil {
		t.Fatalf("双模式解析异常: %+v", modes)
	}
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{Mode: ModeExpert}); err == nil {
		t.Fatal("expert 未绑定应报错")
	}
	mc, err = cfgSvc.ResolveChatSettings(ctx, AIModelSelector{Mode: ModeNormal})
	if err != nil || !strings.Contains(mc.APIKey, "normal-bind") {
		t.Fatalf("normal 模式解析异常: %+v err=%v", mc, err)
	}

	// ① 专项单绑定：FeatureKey 单绑定优先，忽略请求模型来源字段（防绕过）
	faultID := mkConfig("fault-bind")
	if err := cfgSvc.SetBinding(ctx, FeatureFaultConsult, faultID); err != nil {
		t.Fatalf("SetBinding(fault) 失败: %v", err)
	}
	mc, err = cfgSvc.ResolveChatSettings(ctx, AIModelSelector{
		FeatureKey: FeatureFaultConsult, ModelSource: "custom",
		CustomAPIKey: "sk-bypass", CustomBaseURL: "https://evil.example.com", CustomModel: "evil",
	})
	if err != nil || !strings.Contains(mc.APIKey, "fault-bind") {
		t.Fatalf("专项单绑定应忽略请求侧模型来源: %+v err=%v", mc, err)
	}

	// 专项未绑定 → 报错
	if _, err := cfgSvc.ResolveChatSettings(ctx, AIModelSelector{FeatureKey: FeatureExerciseSolving}); err == nil {
		t.Fatal("专项未绑定应报错")
	}
}

// TestResolveHotCacheInvalidation 热路径缓存：改绑后立即生效（写路径主动失效），
// 未命中 DB 时同键二次解析命中缓存（结果一致）。
func TestResolveHotCacheInvalidation(t *testing.T) {
	cfgSvc, _, _, _ := newAIStack(t)
	ctx := context.Background()

	if err := cfgSvc.CreateConfig(ctx, "cfg-a", "sk-aaa", "https://a.example.com/v1", "m-a", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ := cfgSvc.ListConfigs(ctx)
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}
	if got := cfgSvc.ResolveConfig(ctx, FeatureGradeShortAnswer); got.APIKey != "sk-aaa" {
		t.Fatalf("首次解析异常: %+v", got)
	}

	// 改绑 → 缓存失效，立即解析到新配置
	if err := cfgSvc.CreateConfig(ctx, "cfg-b", "sk-bbb", "https://b.example.com/v1", "m-b", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ = cfgSvc.ListConfigs(ctx)
	if len(cfgs) != 2 {
		t.Fatalf("应有两条配置: %d", len(cfgs))
	}
	var cfgBID int
	for _, c := range cfgs {
		if c.Name == "cfg-b" {
			cfgBID = c.ID
		}
	}
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, cfgBID); err != nil {
		t.Fatalf("改绑失败: %v", err)
	}
	if got := cfgSvc.ResolveConfig(ctx, FeatureGradeShortAnswer); got.APIKey != "sk-bbb" {
		t.Fatalf("改绑后应立即生效: %+v", got)
	}
	// 展示缓存（Redis 不可用时降级直查）与热点缓存一致
	bindings, err := cfgSvc.ListBindings(ctx)
	if err != nil {
		t.Fatalf("ListBindings 失败: %v", err)
	}
	for _, b := range bindings {
		if b.FeatureKey == FeatureGradeShortAnswer && b.ConfigID != nil && *b.ConfigID != cfgBID {
			t.Fatalf("绑定展示应指向新配置: %+v", b)
		}
	}
}

// fakeStreamingTransport 流式槽位 fake（端口注入范本验证）。
// 互斥保护：StreamChat 的异步命名 goroutine 也可能进入槽位（CI -race 下验证）。
type fakeStreamingTransport struct {
	mu      sync.Mutex
	content string
	gotSel  AIModelSelector
	gotMsgs []*schema.Message
	chunks  []string
}

func (f *fakeStreamingTransport) StreamComplete(_ context.Context, sel AIModelSelector, msgs []*schema.Message, onChunk func(string)) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotSel = sel
	f.gotMsgs = msgs
	if onChunk != nil {
		onChunk(f.content)
		f.chunks = append(f.chunks, f.content)
	}
	return f.content, nil
}

// snapshot 读取 fake 记录（与后台 goroutine 同步）。
func (f *fakeStreamingTransport) snapshot() (AIModelSelector, []*schema.Message, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotSel, f.gotMsgs, f.chunks
}

// fakeBlockingTransport 阻塞槽位 fake。
type fakeBlockingTransport struct {
	content string
}

func (f *fakeBlockingTransport) CallModel(_ []openai.ChatCompletionMessage, _ int, _ float32, _ string) (string, error) {
	return f.content, nil
}

// TestStreamingSlotInjectedEndToEnd 对话端到端（fake 流式槽位）：
// prompt 组装（功能系统提示词在首位）与持久化真语义不回归。
// 会话标题用非占位符：阻断异步命名 goroutine 走真实生成路径（其仅做一次 DB 读即退出）。
func TestStreamingSlotInjectedEndToEnd(t *testing.T) {
	_, assistant, _, db := newAIStack(t)
	ctx := context.Background()
	fake := &fakeStreamingTransport{content: "模拟回复"}
	assistant.streamer = fake

	session, err := assistant.CreateSession(ctx, 7, "已命名会话", "", FeatureFaultConsult)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	var chunks []string
	full, err := assistant.StreamChat(ctx, 7, StreamChatReq{
		SessionID:    session.ID,
		ModelSource:  "custom",
		CustomAPIKey: "sk-custom", CustomBaseURL: "https://custom.example.com/v1", CustomModel: "gpt-4o",
		Messages: []struct {
			Role    string   `json:"role"`
			Content string   `json:"content"`
			Images  []string `json:"images"`
		}{{Role: "user", Content: "叉车启动困难怎么办"}},
	}, func(c string) { chunks = append(chunks, c) })
	if err != nil {
		t.Fatalf("StreamChat 失败: %v", err)
	}
	if full != "模拟回复" || len(chunks) != 1 {
		t.Fatalf("流式回调/完整回复不符: full=%q chunks=%v", full, chunks)
	}

	// prompt 组装：首位为通用专家系统提示词（FeatureKey 为空），末位为用户消息
	gotSel, gotMsgs, gotChunks := fake.snapshot()
	if len(gotChunks) != 1 {
		t.Fatalf("槽位应仅被主对话调用一次, got %d", len(gotChunks))
	}
	// 选择子投影：请求的模型来源字段应原样透传给槽位（解析在槽位内完成）
	if gotSel.ModelSource != "custom" || gotSel.CustomAPIKey != "sk-custom" ||
		gotSel.CustomBaseURL != "https://custom.example.com/v1" || gotSel.CustomModel != "gpt-4o" || gotSel.UserID != 7 {
		t.Fatalf("选择子投影异常: %+v", gotSel)
	}
	if len(gotMsgs) != 2 || gotMsgs[0].Role != schema.System || gotMsgs[0].Content != forkliftExpertSystemPrompt {
		t.Fatalf("系统提示词组装异常: %+v", gotMsgs)
	}
	if gotMsgs[1].Content != "叉车启动困难怎么办" {
		t.Fatalf("用户消息组装异常: %+v", gotMsgs[1])
	}

	// 持久化：用户/助手消息各一行，会话时间被刷新
	var msgs []model.AIChatMessage
	if err := db.Where("session_id = ?", session.ID).Order("id ASC").Find(&msgs).Error; err != nil {
		t.Fatalf("查询消息失败: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" || msgs[1].Content != "模拟回复" {
		t.Fatalf("持久化消息不符: %+v", msgs)
	}
}

// TestBlockingSlotInjectedEndToEnd 评分/解析端到端（fake 阻塞槽位）：
// 端口注入生效、评分 JSON 解析与解析文本裁剪真语义不回归。
func TestBlockingSlotInjectedEndToEnd(t *testing.T) {
	_, _, aiSvc, _ := newAIStack(t)
	aiSvc.blocking = &fakeBlockingTransport{content: `{"score": 8, "comment": "要点齐全"}`}

	res := aiSvc.GradeShortAnswer("题干", "参考答案", "评分标准", "学员作答", 10, nil)
	if res == nil || res.Score != 8 || res.Comment != "要点齐全" {
		t.Fatalf("评分端到端结果异常: %+v", res)
	}

	aiSvc.blocking = &fakeBlockingTransport{content: "  解析正文  "}
	expl, err := aiSvc.GenerateQuestionExplanation("题干", "答案", "参考解析")
	if err != nil || expl != "解析正文" {
		t.Fatalf("解析端到端结果异常: %q err=%v", expl, err)
	}
}
