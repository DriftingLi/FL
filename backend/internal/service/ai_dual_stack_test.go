// Package service 单一 AIModelPort 契约测试（ADR-0029 T2，#607；前身：Blocking/Streaming
// 双栈契约测试）。覆盖：同一绑定两方法解析同配置、Complete 与 Stream 收集结果一致、
// client 签名缓存命中（阻塞/流式共享，client 不重建）、超时纪律单点分化断言（120s/300s）、
// resolver 分支覆盖（decrypt-failed/空 featureKey/custom 不完整/未知来源）经新 port 路径走通。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// recordingResolver 凭证 resolver 记录包装：转发到 *AIConfigService，同时记录
// 两流向的调用次数、解析结果与收到的 ctx 剩余时长（超时纪律断言通道）。
type recordingResolver struct {
	AIConfigResolver
	mu              sync.Mutex
	featureCalls    int
	chatCalls       int
	lastFeature     AISettings
	lastChat        AISettings
	featureDeadline time.Duration
	chatDeadline    time.Duration
}

func (r *recordingResolver) ResolveFeatureSettings(ctx context.Context, featureKey string) (AISettings, error) {
	s, err := r.AIConfigResolver.ResolveFeatureSettings(ctx, featureKey)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.featureCalls++
	if dl, ok := ctx.Deadline(); ok {
		r.featureDeadline = time.Until(dl)
	}
	if err == nil {
		r.lastFeature = s
	}
	return s, err
}

func (r *recordingResolver) ResolveChatSettings(ctx context.Context, sel AIModelSelector) (AISettings, error) {
	s, err := r.AIConfigResolver.ResolveChatSettings(ctx, sel)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chatCalls++
	if dl, ok := ctx.Deadline(); ok {
		r.chatDeadline = time.Until(dl)
	}
	if err == nil {
		r.lastChat = s
	}
	return s, err
}

// snapshot 便捷读取（契约测试用）。
func (r *recordingResolver) snapshot() (feat, chat AISettings, featDL, chatDL time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastFeature, r.lastChat, r.featureDeadline, r.chatDeadline
}

func (r *recordingResolver) chatSettings() AISettings {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastChat
}

func (r *recordingResolver) featureSettings() AISettings {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastFeature
}

func (r *recordingResolver) calls() (featCalls, chatCalls int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.featureCalls, r.chatCalls
}

// stubAIHandler OpenAI 兼容 stub：非流式请求返回完整回复（full），流式请求（body 含
// stream:true）按 SSE 分片（chunks）；failFirst>0 时前 N 次请求返回 500（重试验证），
// filter 时非流式返回空内容 + finish_reason=content_filter（审查截断分支验证）。
type stubAIHandler struct {
	mu        sync.Mutex
	calls     int
	failFirst int
	filter    bool
	full      string
	chunks    []string
}

func (h *stubAIHandler) callCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls
}

func (h *stubAIHandler) serve(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.calls++
	fail := h.failFirst > 0
	if fail {
		h.failFirst--
	}
	filter := h.filter
	full := h.full
	chunks := h.chunks
	h.mu.Unlock()

	if fail {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"stub down","type":"server_error"}}`))
		return
	}
	var body struct {
		Stream bool `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, c := range chunks {
			_, _ = fmt.Fprintf(w, "data: {\"id\":\"stub\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", c)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}
	content, finishReason := full, "stop"
	if filter {
		content, finishReason = "", aiFinishReasonContentFilter
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"id":"stub","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":%q}]}`, content, finishReason)
}

// newStubOpenAIServer 注册 stub 服务的生命周期。
func newStubOpenAIServer(t *testing.T, h *stubAIHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)
	return srv
}

// newPortStack 构建经 recordingResolver 注入的 eino 生产 adapter（契约测试骨架）。
func newPortStack(t *testing.T) (*AIConfigService, *recordingResolver, AIModelPort, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	if err := db.AutoMigrate(&model.AIConfig{}, &model.AIFeatureBinding{}, &model.AIUserModel{}); err != nil {
		t.Fatalf("AutoMigrate AI 表失败: %v", err)
	}
	cfgSvc := NewAIConfigService(db, "test-master-key", zap.NewNop())
	rec := &recordingResolver{AIConfigResolver: cfgSvc}
	adapter := NewEinoAIModel(rec, zap.NewNop())
	return cfgSvc, rec, adapter, db
}

// TestAIModelPortSharedConfigFromBinding 单 port 契约：同一绑定（配置）两方法解析同配置、
// Complete 与 Stream 收集结果一致、签名缓存命中（client 不重建）、超时上下文单点分化。
func TestAIModelPortSharedConfigFromBinding(t *testing.T) {
	ctx := context.Background()
	cfgSvc, rec, adapter, db := newPortStack(t)

	const (
		apiKey    = "sk-port-test"
		modelName = "stub-model"
		full      = "你好，叉车"
	)
	h := &stubAIHandler{full: full, chunks: []string{"你好", "，叉车"}}
	baseURL := newStubOpenAIServer(t, h).URL

	if err := cfgSvc.CreateConfig(ctx, "端口测试模型", apiKey, baseURL, modelName, ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, err := cfgSvc.ListConfigs(ctx)
	if err != nil || len(cfgs) != 1 {
		t.Fatalf("ListConfigs 失败: %v, %v", cfgs, err)
	}
	// 同一配置同时绑定阻塞与流式两个功能键：两方法解析同配置的前提
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding(评分) 失败: %v", err)
	}
	if err := cfgSvc.SetBinding(ctx, FeatureFaultConsult, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding(故障咨询) 失败: %v", err)
	}

	// 阻塞与流式消费方共享同一端口实例（client 签名缓存跨方法复用的前提；
	// 承接 T1 双栈测试「两栈共用同一 resolver 实例」的断言）
	aiSvc := NewAIService(db, adapter, zap.NewNop())
	assistant := NewAIAssistantService(db, cfgSvc, NewFileStore("", nil, zap.NewNop()), "test-master-key", zap.NewNop(), adapter)
	if aiSvc.port != adapter || assistant.port != adapter {
		t.Fatal("阻塞与流式消费方应共享同一模型端口实例")
	}

	// 阻塞补全：resolver 解析 → 签名缓存建 client → stub 完整回复
	gotComplete, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{
		schema.UserMessage("叉车液压异常"),
	}, AICompleteOptions{MaxTokens: 16, Temperature: 0.2})
	if err != nil || gotComplete != full {
		t.Fatalf("Complete 异常: %q err=%v", gotComplete, err)
	}

	// 流式：同一配置解析 → Stream → 分片回调 + 累积结果
	var gotChunks []string
	gotStream, _, err := adapter.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult},
		[]*schema.Message{schema.UserMessage("叉车液压异常")},
		func(c string) { gotChunks = append(gotChunks, c) })
	if err != nil || gotStream != full {
		t.Fatalf("Stream 异常: %q err=%v", gotStream, err)
	}
	if len(gotChunks) != 2 || gotChunks[0] != "你好" || gotChunks[1] != "，叉车" {
		t.Fatalf("流式分片回调不符: %v", gotChunks)
	}

	// 同一绑定两方法解析同配置（与 ResolveConfig 单点一致）
	resolved := cfgSvc.ResolveConfig(ctx, FeatureGradeShortAnswer)
	feat, chat, _, _ := rec.snapshot()
	if feat.APIKey != apiKey || feat.BaseURL != baseURL || feat.Model != modelName {
		t.Errorf("阻塞流向解析异常: %+v", feat)
	}
	if chat.APIKey != feat.APIKey || chat.BaseURL != feat.BaseURL || chat.Model != feat.Model {
		t.Errorf("两方法解析结果不一致: complete=%+v stream=%+v", feat, chat)
	}
	if resolved.APIKey != apiKey || resolved.BaseURL != baseURL || resolved.Model != modelName {
		t.Errorf("ResolveConfig 单点不一致: %+v", resolved)
	}

	// 超时纪律单点分化（resolver 收到的 ctx 剩余时长）：阻塞 120s / 流式 300s
	_, _, featDL, chatDL := rec.snapshot()
	if featDL <= 0 || featDL > aiBlockingTimeout || featDL < aiBlockingTimeout-10*time.Second {
		t.Errorf("阻塞补全超时纪律异常: %v（期望约 %v）", featDL, aiBlockingTimeout)
	}
	if chatDL <= 0 || chatDL > aiStreamTimeout || chatDL < aiStreamTimeout-10*time.Second {
		t.Errorf("流式超时纪律异常: %v（期望约 %v）", chatDL, aiStreamTimeout)
	}

	// 签名缓存命中：Complete+Stream（及二次解析）同一签名 → client 仅重建一次
	gotComplete2, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{
		schema.UserMessage("再次提问"),
	}, AICompleteOptions{MaxTokens: 16, Temperature: 0.2})
	if err != nil || gotComplete2 != full {
		t.Fatalf("二次 Complete 异常: %q err=%v", gotComplete2, err)
	}
	if got := h.callCount(); got != 3 {
		t.Fatalf("stub 应收到 3 次调用（2 阻塞 + 1 流式）: %d", got)
	}
}

// TestAIModelPortClientCacheAcrossMethods 签名缓存命中断言：client 不重建
// （observer 计数「AI client 已重建」日志，流式复用阻塞侧已建的 client）。
func TestAIModelPortClientCacheAcrossMethods(t *testing.T) {
	ctx := context.Background()
	cfgSvc, rec, _, _ := newPortStack(t)
	h := &stubAIHandler{full: "ok", chunks: []string{"ok"}}
	baseURL := newStubOpenAIServer(t, h).URL

	if err := cfgSvc.CreateConfig(ctx, "缓存模型", "sk-cache", baseURL, "m-cache", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ := cfgSvc.ListConfigs(ctx)
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}
	if err := cfgSvc.SetBinding(ctx, FeatureExerciseSolving, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding(习题) 失败: %v", err)
	}

	// 重建日志经构造期注入的 observer logger 计数（adapter 经接口返回，不摸具体字段）
	core, logs := observer.New(zapcore.InfoLevel)
	adapter := NewEinoAIModel(rec, zap.New(core))

	if _, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8}); err != nil {
		t.Fatalf("首次 Complete 失败: %v", err)
	}
	if _, _, err := adapter.Stream(ctx, AIModelSelector{FeatureKey: FeatureExerciseSolving}, []*schema.Message{schema.UserMessage("q")}, nil); err != nil {
		t.Fatalf("Stream 失败: %v", err)
	}
	if _, _, err := adapter.Stream(ctx, AIModelSelector{FeatureKey: FeatureExerciseSolving}, []*schema.Message{schema.UserMessage("q")}, nil); err != nil {
		t.Fatalf("二次 Stream 失败: %v", err)
	}
	if _, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8}); err != nil {
		t.Fatalf("二次 Complete 失败: %v", err)
	}

	rebuilds := 0
	for _, e := range logs.TakeAll() {
		if e.Message == "AI client 已重建" {
			rebuilds++
		}
	}
	if rebuilds != 1 {
		t.Fatalf("同一签名四次调用（阻塞+流式交替）client 应仅重建 1 次: %d", rebuilds)
	}
	if featCalls, chatCalls := rec.calls(); featCalls != 2 || chatCalls != 2 {
		t.Fatalf("两流向 resolver 调用次数异常: feature=%d chat=%d", featCalls, chatCalls)
	}
}

// TestAIModelPortCompleteRetryOnServerError 阻塞补全重试语义（与原阻塞栈一致）：
// 首次 500 → 间隔重试成功；持续失败 → 错误透传。
func TestAIModelPortCompleteRetryOnServerError(t *testing.T) {
	ctx := context.Background()
	cfgSvc, _, adapter, _ := newPortStack(t)

	h := &stubAIHandler{full: "重试成功", failFirst: 1}
	baseURL := newStubOpenAIServer(t, h).URL
	if err := cfgSvc.CreateConfig(ctx, "重试模型", "sk-retry", baseURL, "m-retry", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ := cfgSvc.ListConfigs(ctx)
	if err := cfgSvc.SetBinding(ctx, FeatureGradeShortAnswer, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}

	got, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8})
	if err != nil || got != "重试成功" {
		t.Fatalf("重试后应成功: %q err=%v", got, err)
	}
	if got := h.callCount(); got != 2 {
		t.Fatalf("应恰好重试一次（2 次调用）: %d", got)
	}

	// 持续失败：第 2 次尝试仍失败 → 错误透传给调用方
	h2 := &stubAIHandler{failFirst: 2}
	srv2 := httptest.NewServer(http.HandlerFunc(h2.serve))
	defer srv2.Close()
	if err := cfgSvc.CreateConfig(ctx, "重试模型2", "sk-retry2", srv2.URL, "m-retry2", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ = cfgSvc.ListConfigs(ctx)
	var cfg2ID int
	for _, c := range cfgs {
		if c.Name == "重试模型2" {
			cfg2ID = c.ID
		}
	}
	if err := cfgSvc.SetBinding(ctx, FeatureQuestionExplanation, cfg2ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}
	if _, err := adapter.Complete(FeatureQuestionExplanation, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8}); err == nil {
		t.Fatal("持续失败应返回错误")
	}
	if got := h2.callCount(); got != 2 {
		t.Fatalf("持续失败应打满 2 次尝试: %d", got)
	}
}

// TestAIModelPortCompleteContentFilter 空内容 + finish_reason=content_filter：
// 不重试直接返回空内容（与原阻塞栈逐字语义一致）。
func TestAIModelPortCompleteContentFilter(t *testing.T) {
	ctx := context.Background()
	cfgSvc, _, adapter, _ := newPortStack(t)
	h := &stubAIHandler{filter: true}
	baseURL := newStubOpenAIServer(t, h).URL

	if err := cfgSvc.CreateConfig(ctx, "审查模型", "sk-filter", baseURL, "m-filter", ""); err != nil {
		t.Fatalf("CreateConfig 失败: %v", err)
	}
	cfgs, _ := cfgSvc.ListConfigs(ctx)
	if err := cfgSvc.SetBinding(ctx, FeatureGenerateChapterContent, cfgs[0].ID); err != nil {
		t.Fatalf("SetBinding 失败: %v", err)
	}

	got, err := adapter.Complete(FeatureGenerateChapterContent, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8})
	if err != nil || got != "" {
		t.Fatalf("content_filter 应返回空内容无错误: %q err=%v", got, err)
	}
	if got := h.callCount(); got != 1 {
		t.Fatalf("content_filter 不应重试: %d", got)
	}
}

// TestAIConfigResolverBranchesViaPort resolver 分支覆盖（#606 起，T2 起经新 port 路径走通）：
// featureKey/选择子 → AISettings 的旧 ModelSource 兼容与解析失败分支
// （专项单绑定/双模式/遗留回退三档阶梯见 ai_config_ladder_test.go）。
func TestAIConfigResolverBranchesViaPort(t *testing.T) {
	ctx := context.Background()
	cfgSvc, rec, adapter, db := newPortStack(t)
	h := &stubAIHandler{full: "分支回复", chunks: []string{"分支", "回复"}}
	baseURL := newStubOpenAIServer(t, h).URL

	// 旧 ModelSource=user：解密后的 API Key 与 DB 字段一一映射（解析成功后进入传输，stub 回复）
	encKey, err := security.EncryptSecret("sk-user-secret", "test-master-key")
	if err != nil {
		t.Fatalf("EncryptSecret 失败: %v", err)
	}
	if err := db.Create(&model.AIUserModel{
		UserID: 7, Name: "我的模型", APIKey: encKey,
		BaseURL: baseURL, Model: "qwen-plus",
	}).Error; err != nil {
		t.Fatalf("插入用户模型失败: %v", err)
	}
	gotStream, _, err := adapter.Stream(ctx, AIModelSelector{ModelSource: "user", UserID: 7, UserModelID: 1},
		[]*schema.Message{schema.UserMessage("q")}, nil)
	if err != nil || gotStream != "分支回复" {
		t.Fatalf("user 来源流式调用异常: %q err=%v", gotStream, err)
	}
	if chat := rec.chatSettings(); chat.APIKey != "sk-user-secret" || chat.BaseURL != baseURL || chat.Model != "qwen-plus" {
		t.Errorf("user 来源映射异常: %+v", chat)
	}

	// 未登录不能使用用户自定义模型
	if _, _, err := adapter.Stream(ctx, AIModelSelector{ModelSource: "user", UserModelID: 1}, nil, nil); err == nil || err.Error() != "未登录不能使用用户自定义模型" {
		t.Errorf("未登录使用 user 来源应报原文案: %v", err)
	}

	// 旧 ModelSource=custom：选择子字段直接透传
	gotStream, _, err = adapter.Stream(ctx, AIModelSelector{
		ModelSource:  "custom",
		CustomAPIKey: "sk-custom", CustomBaseURL: baseURL, CustomModel: "gpt-4o",
	}, []*schema.Message{schema.UserMessage("q")}, nil)
	if err != nil || gotStream != "分支回复" {
		t.Fatalf("custom 来源流式调用异常: %q err=%v", gotStream, err)
	}
	if chat := rec.chatSettings(); chat.APIKey != "sk-custom" || chat.BaseURL != baseURL || chat.Model != "gpt-4o" {
		t.Errorf("custom 来源映射异常: %+v", chat)
	}

	// custom 来源字段不完整 → 报错（文案逐字保留）
	if _, _, err := adapter.Stream(ctx, AIModelSelector{ModelSource: "custom", CustomAPIKey: "sk-custom"}, nil, nil); err == nil || err.Error() != "自定义模型配置不完整" {
		t.Errorf("custom 配置不完整应报原文案: %v", err)
	}

	// 未知来源报错（文案逐字保留）
	if _, _, err := adapter.Stream(ctx, AIModelSelector{ModelSource: "unknown"}, nil, nil); err == nil || err.Error() != "未知的 model_source: unknown" {
		t.Errorf("未知 model_source 应报原文案: %v", err)
	}

	// 阻塞流向：featureKey 单绑定解析 → Complete（stub 回复）
	if err := cfgSvc.CreateConfig(ctx, "评分模型", "sk-grading", baseURL, "grading-model", ""); err != nil {
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
	gotComplete, err := adapter.Complete(FeatureGradeShortAnswer, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8})
	if err != nil || gotComplete != "分支回复" {
		t.Fatalf("阻塞流向 Complete 异常: %q err=%v", gotComplete, err)
	}
	if feat := rec.featureSettings(); feat.APIKey != "sk-grading" || feat.BaseURL != baseURL || feat.Model != "grading-model" {
		t.Errorf("阻塞栈 featureKey 解析异常: %+v", feat)
	}

	// 未绑定功能 / 空 featureKey：报错而非降级（文案逐字保留）
	unboundMsg := fmt.Sprintf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", FeatureQuestionExplanation)
	if _, err := adapter.Complete(FeatureQuestionExplanation, nil, AICompleteOptions{}); err == nil || err.Error() != unboundMsg {
		t.Errorf("未绑定功能应报原文案: %v", err)
	}
	emptyKeyMsg := fmt.Sprintf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", "")
	if _, err := adapter.Complete("", nil, AICompleteOptions{}); err == nil || err.Error() != emptyKeyMsg {
		t.Errorf("空 featureKey 应报原文案: %v", err)
	}

	// 解析失败分支：API Key 解密失败 → ResolveConfig 标记 decrypt-failed，port 路径报错
	if err := cfgSvc.SetBinding(ctx, FeatureGenerateChapterContent, dirtyID); err != nil {
		t.Fatalf("SetBinding(脏数据) 失败: %v", err)
	}
	if got := cfgSvc.ResolveConfig(ctx, FeatureGenerateChapterContent); got.Source != "decrypt-failed" {
		t.Fatalf("解密失败应标记 decrypt-failed: %+v", got)
	}
	dirtyMsg := fmt.Sprintf("AI 功能 %q 未绑定配置，请在管理员后台 AI 配置页面绑定", FeatureGenerateChapterContent)
	if _, err := adapter.Complete(FeatureGenerateChapterContent, nil, AICompleteOptions{}); err == nil || err.Error() != dirtyMsg {
		t.Errorf("解密失败时 port 路径应报未绑定文案: %v", err)
	}

	// 专项功能未绑定应报错（防绕过：custom 字段不得兜底；文案逐字保留）
	if _, _, err := adapter.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult, ModelSource: "custom", CustomAPIKey: "sk-bypass"}, nil, nil); err == nil || err.Error() != "管理员未配置该功能的模型，请联系管理员" {
		t.Errorf("专项功能未绑定应报原文案: %v", err)
	}

	// user 来源解密失败 → 报错
	if err := db.Create(&model.AIUserModel{
		UserID: 8, Name: "脏模型", APIKey: security.EncryptedPrefix + "not-valid-ciphertext",
		BaseURL: baseURL, Model: "dirty-user-model",
	}).Error; err != nil {
		t.Fatalf("插入脏用户模型失败: %v", err)
	}
	if _, _, err := adapter.Stream(ctx, AIModelSelector{ModelSource: "user", UserID: 8, UserModelID: 2}, nil, nil); err == nil || !strings.Contains(err.Error(), "解密用户自定义模型 API Key 失败") {
		t.Errorf("用户模型解密失败应报错: %v", err)
	}
}
