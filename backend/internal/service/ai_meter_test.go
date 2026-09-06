// Package service AI 计量闸门测试（ADR-0031，#619）：billed 两分支（注册表驱动 + 借键
// 逃费防护）、金额等价（同输入同输出）、幂等键透传与降级键、预检阻断短路、自动命名
// billed=false 不扣费，以及「计费金额 diff=0」迁移契约——迁移前 handler 编排（本文件
// legacyHandlerBillingPipeline 原样复刻）与迁移后 metered 端口对同一请求序列产生的
// 扣费流水与 usage 数据面逐行断言一致。
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// fakeAIMeter 第二个 meter adapter（seam 坐实，ADR-0031 决策 1）：记录事实、可编程结果。
// 互斥保护：StreamChat 的异步命名 goroutine 也可能进入闸门（CI -race 下验证）。
type fakeAIMeter struct {
	mu                 sync.Mutex
	preflightN         int
	deductN            int
	gotUserID          int
	gotRequestID       string
	gotPromptChars     int
	gotCompletionChars int
	preflightErr       error
	deductRes          *AITokensResult
	deductErr          error
}

func (f *fakeAIMeter) AIPreflight(userID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.preflightN++
	f.gotUserID = userID
	return f.preflightErr
}

func (f *fakeAIMeter) DeductAI(_ context.Context, userID int, requestID string, promptChars, completionChars int) (*AITokensResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deductN++
	f.gotUserID = userID
	f.gotRequestID = requestID
	f.gotPromptChars = promptChars
	f.gotCompletionChars = completionChars
	if f.deductErr != nil {
		return nil, f.deductErr
	}
	if f.deductRes != nil {
		return f.deductRes, nil
	}
	return &AITokensResult{Points: 10, TotalTokens: 100, Balance: 990, PromptTokens: 4, CompletionTokens: 3}, nil
}

func (f *fakeAIMeter) snapshot() (preflightN, deductN, promptChars, completionChars int, requestID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.preflightN, f.deductN, f.gotPromptChars, f.gotCompletionChars, f.gotRequestID
}

// newMeteredStack 构建测试闸门栈：fake inner port + fake meter（第二 adapter 组合）。
func newMeteredStack(content string, meter *fakeAIMeter) (AIModelPort, *fakeAIModelPort) {
	inner := &fakeAIModelPort{content: content}
	return NewMeteredAIModel(inner, meter, zap.NewNop()), inner
}

// TestAIMeterBilledBranches billed 两分支：注册表对话功能过闸（预检 + 扣费，事实透传）；
// 游客与空回复不产生扣费；免费声明（WithAIMeterFree）跳过闸门；借免费功能键发起的
// 对话不逃费（回退通用对话计费）。
func TestAIMeterBilledBranches(t *testing.T) {
	ctx := context.Background()
	meter := &fakeAIMeter{}
	port, _ := newMeteredStack("回复内容", meter)
	prompt := "叉车启动困难怎么排查？"
	msgs := []*schema.Message{
		schema.SystemMessage(forkliftExpertSystemPrompt),
		schema.UserMessage(prompt),
	}

	// billed=true（专项聊天）：预检 + 扣费各一次，prompt/completion 事实与请求一致
	content, usage, err := port.Stream(WithAIRequestID(ctx, "req-billed"),
		AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: 7}, msgs, nil)
	if err != nil || content != "回复内容" {
		t.Fatalf("billed 调用异常: content=%q err=%v", content, err)
	}
	if usage == nil || usage.Res == nil || usage.Err != nil {
		t.Fatalf("billed 调用应回传 usage 数据面: %+v", usage)
	}
	if n1, n2, pc, cc, rid := meter.snapshot(); n1 != 1 || n2 != 1 || pc != len(prompt) || cc != len("回复内容") || rid != "req-billed" {
		t.Fatalf("闸门事实透传不符: preflight=%d deduct=%d prompt=%d completion=%d rid=%q", n1, n2, pc, cc, rid)
	}

	// 游客（未登录）：不预检、不扣费、usage=nil
	meter2 := &fakeAIMeter{}
	port2, _ := newMeteredStack("回复内容", meter2)
	_, usage2, err := port2.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult}, msgs, nil)
	if err != nil || usage2 != nil {
		t.Fatalf("游客调用不应计费: usage=%+v err=%v", usage2, err)
	}
	if n1, n2, _, _, _ := meter2.snapshot(); n1 != 0 || n2 != 0 {
		t.Fatalf("游客不得触发闸门: preflight=%d deduct=%d", n1, n2)
	}

	// 空回复：不扣费（与迁移前 handler 条件一致）
	meter3 := &fakeAIMeter{}
	port3, _ := newMeteredStack("", meter3)
	_, usage3, err := port3.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: 7}, msgs, nil)
	if err != nil || usage3 != nil {
		t.Fatalf("空回复不应扣费: usage=%+v err=%v", usage3, err)
	}

	// 显式免费声明：内部二次消费跳过闸门（自动命名路径）
	meter4 := &fakeAIMeter{}
	port4, _ := newMeteredStack("标题", meter4)
	_, usage4, err := port4.Stream(WithAIMeterFree(ctx),
		AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: 7}, msgs, nil)
	if err != nil || usage4 != nil {
		t.Fatalf("免费声明调用不应计费: usage=%+v err=%v", usage4, err)
	}
	if n1, n2, _, _, _ := meter4.snapshot(); n1 != 0 || n2 != 0 {
		t.Fatalf("免费声明不得触发闸门: preflight=%d deduct=%d", n1, n2)
	}

	// 借键逃费防护：billed=false 的阻塞功能键（评分）经对话阶梯回退为通用对话 → 仍计费
	meter5 := &fakeAIMeter{}
	port5, _ := newMeteredStack("回复内容", meter5)
	_, usage5, err := port5.Stream(ctx, AIModelSelector{FeatureKey: FeatureGradeShortAnswer, UserID: 7}, msgs, nil)
	if err != nil || usage5 == nil {
		t.Fatalf("借免费功能键对话应回退通用计费: usage=%+v err=%v", usage5, err)
	}
	if n1, n2, _, _, _ := meter5.snapshot(); n1 != 1 || n2 != 1 {
		t.Fatalf("借键对话应过闸: preflight=%d deduct=%d", n1, n2)
	}
}

// TestAIMeterRequestIDFallbackKey 请求标识降级键（迁移前 handler 生成策略内移，格式不变）。
func TestAIMeterRequestIDFallbackKey(t *testing.T) {
	ctx := context.Background()
	meter := &fakeAIMeter{}
	port, _ := newMeteredStack("回复", meter)
	msgs := []*schema.Message{schema.UserMessage("问")}

	if _, _, err := port.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: 42}, msgs, nil); err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if _, _, _, _, rid := meter.snapshot(); !regexp.MustCompile(`^ai-42-[0-9]+$`).MatchString(rid) {
		t.Fatalf("降级键格式应保持 ai-{uid}-{UnixNano}: %q", rid)
	}
}

// TestAIMeterPreflightBlocksBeforeTransport 预检阻断：不发起传输、不扣费，哨兵原样上抛
// （handler 据此映射迁移前文案「积分不足，请先去任务中心完成任务」）。
func TestAIMeterPreflightBlocksBeforeTransport(t *testing.T) {
	ctx := context.Background()
	meter := &fakeAIMeter{preflightErr: ErrInsufficientPoints}
	port, inner := newMeteredStack("不应到达", meter)
	msgs := []*schema.Message{schema.UserMessage("问")}

	content, usage, err := port.Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: 7}, msgs, nil)
	if !errors.Is(err, ErrInsufficientPoints) || content != "" || usage != nil {
		t.Fatalf("预检阻断应短路: content=%q usage=%+v err=%v", content, usage, err)
	}
	if inner.streamN != 0 || meter.deductN != 0 {
		t.Fatalf("预检失败不得触达传输与扣费: stream=%d deduct=%d", inner.streamN, meter.deductN)
	}
}

// TestAIMeterCompleteBilledGuard 阻塞补全过闸：注册表免费行放行（评分/章节/解析）；
// billed 行因端口签名无计费主体显式报错，拒绝静默免费（ADR-0031 决策 2）。
func TestAIMeterCompleteBilledGuard(t *testing.T) {
	meter := &fakeAIMeter{}
	port, inner := newMeteredStack("ok", meter)

	got, err := port.Complete(FeatureGradeShortAnswer, []*schema.Message{schema.UserMessage("q")}, AICompleteOptions{MaxTokens: 8})
	if err != nil || got != "ok" {
		t.Fatalf("免费阻塞补全应放行: %q err=%v", got, err)
	}
	if _, n2, _, _, _ := meter.snapshot(); n2 != 0 {
		t.Fatalf("免费阻塞补全不得触发扣费: deduct=%d", n2)
	}

	if _, err := port.Complete(FeatureAIAssistantNormal, nil, AICompleteOptions{}); err == nil || !strings.Contains(err.Error(), "拒绝静默免费") {
		t.Fatalf("billed 阻塞补全应显式报错: %v", err)
	}
	if inner.completeN != 1 {
		t.Fatalf("billed 阻塞补全不得触达传输: complete=%d", inner.completeN)
	}
}

// TestAIMeterPromptChars 口径事实单点：只算最后一条用户消息文本，system/历史/图片不计。
func TestAIMeterPromptChars(t *testing.T) {
	b64 := "aGk="
	multimodal := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "图里的部件是什么？"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64}}},
	}}
	cases := []struct {
		name string
		msgs []*schema.Message
		want int
	}{
		{"空消息", nil, 0},
		{"无用户消息", []*schema.Message{schema.SystemMessage("sys")}, 0},
		{"只算最后一条用户消息", []*schema.Message{
			schema.UserMessage("历史消息不应计入"),
			schema.SystemMessage("sys"),
			schema.UserMessage("当前提问"),
		}, len("当前提问")},
		{"多模态取首个文本 part", []*schema.Message{
			schema.SystemMessage("sys"),
			schema.UserMessage("历史消息不应计入"),
			multimodal,
		}, len("图里的部件是什么？")},
		{"纯图片消息计0", []*schema.Message{
			{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
				{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64}}},
			}},
		}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := aiPromptChars(tc.msgs); got != tc.want {
				t.Fatalf("aiPromptChars = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAIMeterAutoTitleNoDoubleCharge 自动命名 billed=false 不扣费（端到端）：
// 主对话（占坑标题会话首次发消息）过闸扣费一次；service 内部追加的标题生成第二次
// port 调用经 WithAIMeterFree 显式免费——不预检、不扣费，无双扣。
func TestAIMeterAutoTitleNoDoubleCharge(t *testing.T) {
	db := testutil.NewFileDB(t)
	cfgSvc := NewAIConfigService(db, "test-master-key", zap.NewNop())
	meter := &fakeAIMeter{}
	inner := &fakeAIModelPort{content: "先查电瓶，再查起动机。"}
	port := NewMeteredAIModel(inner, meter, zap.NewNop())
	assistant := NewAIAssistantService(db, cfgSvc, NewFileStore("", nil, zap.NewNop()), "test-master-key", zap.NewNop(), port)

	ctx := context.Background()
	session, err := assistant.CreateSession(ctx, 7, "新会话", "", FeatureFaultConsult)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	_, usage, err := assistant.StreamChat(ctx, 7, StreamChatReq{
		SessionID:  session.ID,
		FeatureKey: FeatureFaultConsult,
		Messages: []struct {
			Role    string   `json:"role"`
			Content string   `json:"content"`
			Images  []string `json:"images"`
		}{{Role: "user", Content: "叉车启动困难怎么办"}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamChat 失败: %v", err)
	}
	if usage == nil || usage.Res == nil {
		t.Fatalf("主对话应回传 usage 数据面: %+v", usage)
	}
	if n1, n2, _, _, _ := meter.snapshot(); n1 != 1 || n2 != 1 {
		t.Fatalf("主对话应恰好预检+扣费各一次: preflight=%d deduct=%d", n1, n2)
	}

	// 等待异步自动命名发起第二次端口调用（fake inner 计数），再确认其未触发闸门
	deadline := time.Now().Add(5 * time.Second)
	for {
		inner.mu.Lock()
		n := inner.streamN
		inner.mu.Unlock()
		if n >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("自动命名应发起第二次端口调用，streamN=%d", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // 若自动命名误入闸门，此处足以暴露
	if n1, n2, _, _, _ := meter.snapshot(); n1 != 1 || n2 != 1 {
		t.Fatalf("自动命名不得预检/扣费（无双扣）: preflight=%d deduct=%d", n1, n2)
	}
}

// ---- 计费金额 diff=0 迁移契约 ----

// billingFacts 迁移前 handler 的计费输入事实。
type billingFacts struct {
	userID     int
	requestID  string // RequestID 中间件注入（空 = 现场降级）
	prompt     string // 最后一条用户消息原文
	content    string // 模型回复全文
	multimodal bool   // 原文以多模态消息首文本 part 承载（图片消息）
}

// legacyHandlerBillingPipeline 迁移前 api/ai_assistant.go 计费编排原样复刻（#619 对照面）：
// 预检先于传输 → 传输成功（harness 以已知回复模拟）且内容非空才进入扣费段 →
// 请求标识缺失时现场降级 → promptChars = len(最后一条用户消息原文) → DeductAI。
// 迁移后口径的任何调整只允许发生在 meter 单点；本函数与 metered 端口对同一请求
// 序列的扣费流水必须逐行一致（TestAIMeteringAmountDiffZero 钉住）。
func legacyHandlerBillingPipeline(ctx context.Context, points *PointsService, f billingFacts) (*AITokensResult, error) {
	if f.userID > 0 {
		if err := points.AIPreflight(f.userID); err != nil {
			return nil, err
		}
	}
	if f.userID > 0 && f.content != "" {
		requestID := f.requestID
		if requestID == "" {
			requestID = fmt.Sprintf("ai-%d-%d", f.userID, time.Now().UnixNano())
		}
		return points.DeductAI(ctx, f.userID, requestID, len(f.prompt), len(f.content))
	}
	return nil, nil
}

// meteredHandlerPipeline 迁移后路径：同一请求事实经 metered 端口（真实积分域 meter +
// fake 传输 adapter）——闸门在端口内单点，调用方不再编排。
func meteredHandlerPipeline(ctx context.Context, port AIModelPort, f billingFacts, ctxReqID string) (string, *AIUsage, error) {
	callCtx := ctx
	if ctxReqID != "" {
		callCtx = WithAIRequestID(callCtx, ctxReqID)
	}
	msgs := []*schema.Message{schema.SystemMessage(forkliftExpertSystemPrompt)}
	if f.multimodal {
		b64 := "aGk="
		msgs = append(msgs, &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeText, Text: f.prompt},
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64}}},
		}})
	} else {
		msgs = append(msgs, schema.UserMessage(f.prompt))
	}
	sel := AIModelSelector{FeatureKey: FeatureFaultConsult, UserID: f.userID}
	return port.Stream(callCtx, sel, msgs, nil)
}

// ledgerSnap 流水快照（金额等价断言的比对单元）。
type ledgerSnap struct {
	Delta   int
	Reason  string
	RefType string
	RefID   string
}

func fetchLedger(t *testing.T, db *gorm.DB, userID int) []ledgerSnap {
	t.Helper()
	var rows []model.PointsLedger
	if err := db.Where("user_id = ? AND reason = ?", userID, "ai_tokens").Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("查询流水失败: %v", err)
	}
	out := make([]ledgerSnap, 0, len(rows))
	for _, r := range rows {
		out = append(out, ledgerSnap{Delta: r.Delta, Reason: r.Reason, RefType: r.RefType, RefID: r.RefID})
	}
	return out
}

// TestAIMeteringAmountDiffZero 计费金额 diff=0（#619 验收）：迁移前 handler 编排与
// 迁移后 metered 端口对同一请求序列（相同输入事实、相同幂等键、相同余额轨迹）产生的
// 扣费流水与 usage 数据面逐行一致。降级键行仅比对格式（时间戳键天然不同）。
func TestAIMeteringAmountDiffZero(t *testing.T) {
	ctx := context.Background()

	steps := []struct {
		name        string
		balance     int // 每步前两侧余额重置为该值
		userID      int
		requestID   string
		prompt      string
		content     string
		multimodal  bool
		wantErr     error
		fallbackKey bool // requestID 缺失 → 两侧各自现场降级，仅比对键格式
	}{
		{name: "常规对话", balance: 1000, userID: 42, requestID: "req-1", prompt: "叉车启动困难怎么排查？", content: "先查电瓶与起动机，再查油路与喷油器。"},
		{name: "同请求重放幂等", balance: 1000, userID: 42, requestID: "req-1", prompt: "叉车启动困难怎么排查？", content: "先查电瓶与起动机，再查油路与喷油器。"},
		{name: "缺失请求标识走降级键", balance: 1000, userID: 42, requestID: "", prompt: "叉车液压升降缓慢的原因？", content: "可能原因包括液压油不足、液压泵磨损与溢流阀卡滞。", fallbackKey: true},
		{name: "极短输入走兜底100tokens", balance: 1000, userID: 42, requestID: "req-2", prompt: "好", content: "好"},
		{name: "大额扣费封顶100", balance: 1000, userID: 42, requestID: "req-3", prompt: strings.Repeat("叉", 8000), content: strings.Repeat("答", 40000)},
		{name: "游客不计费", balance: 1000, userID: 0, requestID: "req-4", prompt: "游客提问", content: "游客回复"},
		{name: "空回复不扣费", balance: 1000, userID: 42, requestID: "req-5", prompt: "空回复提问", content: ""},
		{name: "余额不足预检阻断", balance: 0, userID: 42, requestID: "req-6", prompt: "余额不足提问", content: "不应到达", wantErr: ErrInsufficientPoints},
		{name: "多模态图文消息同口径", balance: 1000, userID: 42, requestID: "req-7", prompt: "这张液压图里的部件是什么？", content: "图中是多路阀与先导阀组。", multimodal: true},
	}

	// 两侧同构环境：各自独立 DB + 积分域实现；迁移侧以真实 metered 端口 + fake 传输。
	ptsOld, dbOld := newPointsSvc(t)
	uidOld := seedUserWithBalance(t, dbOld, 1000)
	ptsNew, dbNew := newPointsSvc(t)
	uidNew := seedUserWithBalance(t, dbNew, 1000)
	innerNew := &fakeAIModelPort{}
	portNew := NewMeteredAIModel(innerNew, ptsNew, zap.NewNop())
	if _, ok := any(ptsOld).(AIMetering); !ok {
		t.Fatal("*PointsService 应原样满足 AIMetering（生产 adapter = 积分域实现）")
	}

	// 记录会话级降级键，验证「同侧重试拿到新键」之外，两侧账目仍等价
	for _, st := range steps {
		// userID 0 = 游客（两侧均无计费主体）；否则为各侧种子用户
		effOld, effNew := uidOld, uidNew
		if st.userID == 0 {
			effOld, effNew = 0, 0
		}
		// 余额轨迹对齐
		for _, pair := range []struct {
			db  *gorm.DB
			uid int
		}{{dbOld, uidOld}, {dbNew, uidNew}} {
			if err := pair.db.Model(&model.HrwaiUser{}).Where("id = ?", pair.uid).
				UpdateColumn("points_balance", st.balance).Error; err != nil {
				t.Fatalf("重置余额失败: %v", err)
			}
		}
		oldBefore := fetchLedger(t, dbOld, uidOld)
		newBefore := fetchLedger(t, dbNew, uidNew)

		// 迁移前：handler 编排（事实选取按原样：promptChars = len(最后一条用户消息原文)）
		resOld, errOld := legacyHandlerBillingPipeline(ctx, ptsOld, billingFacts{
			userID: effOld, requestID: st.requestID, prompt: st.prompt, content: st.content, multimodal: st.multimodal,
		})
		// 迁移后：metered 端口（闸门内移）；fake 传输返回与迁移前相同的回复内容
		innerNew.mu.Lock()
		innerNew.content = st.content
		innerNew.mu.Unlock()
		contentNew, usageNew, errNew := meteredHandlerPipeline(ctx, portNew, billingFacts{
			userID: effNew, requestID: st.requestID, prompt: st.prompt, content: st.content, multimodal: st.multimodal,
		}, st.requestID)

		// 错误面等价
		if st.wantErr != nil {
			if !errors.Is(errOld, st.wantErr) || !errors.Is(errNew, st.wantErr) {
				t.Fatalf("[%s] 两侧错误应一致: old=%v new=%v", st.name, errOld, errNew)
			}
		} else if errOld != nil || errNew != nil {
			t.Fatalf("[%s] 两侧不应报错: old=%v new=%v", st.name, errOld, errNew)
		}
		if (resOld == nil) != (usageNew == nil || usageNew.Res == nil) {
			t.Fatalf("[%s] usage 有无不一致: old=%+v new=%+v", st.name, resOld, usageNew)
		}

		// usage 数据面等价（SSE usage 事件负载逐字段一致）
		if resOld != nil && usageNew != nil && usageNew.Res != nil {
			if resOld.Points != usageNew.Res.Points || resOld.TotalTokens != usageNew.Res.TotalTokens ||
				resOld.PromptTokens != usageNew.Res.PromptTokens || resOld.CompletionTokens != usageNew.Res.CompletionTokens ||
				resOld.Balance != usageNew.Res.Balance {
				t.Fatalf("[%s] usage 数据面 diff≠0:\n old=%+v\n new=%+v", st.name, resOld, usageNew.Res)
			}
		}
		if st.wantErr == nil && st.content != "" && contentNew != st.content {
			t.Fatalf("[%s] 传输内容应原样透传: %q", st.name, contentNew)
		}

		// 扣费流水等价（新增行逐行比对）
		oldAfter := fetchLedger(t, dbOld, uidOld)
		newAfter := fetchLedger(t, dbNew, uidNew)
		oldNew, newNew := oldAfter[len(oldBefore):], newAfter[len(newBefore):]
		if len(oldNew) != len(newNew) {
			t.Fatalf("[%s] 新增流水行数 diff≠0: old=%d new=%d\n old=%+v\n new=%+v", st.name, len(oldNew), len(newNew), oldNew, newNew)
		}
		for i := range oldNew {
			o, n := oldNew[i], newNew[i]
			if o.Delta != n.Delta || o.Reason != n.Reason || o.RefType != n.RefType {
				t.Fatalf("[%s] 扣费流水 diff≠0:\n old=%+v\n new=%+v", st.name, o, n)
			}
			if st.fallbackKey {
				pat := regexp.MustCompile(`^ai-[0-9]+-[0-9]+$`)
				if !pat.MatchString(o.RefID) || !pat.MatchString(n.RefID) {
					t.Fatalf("[%s] 降级键格式异常: old=%q new=%q", st.name, o.RefID, n.RefID)
				}
			} else if o.RefID != n.RefID {
				t.Fatalf("[%s] 幂等键应透传一致: old=%q new=%q", st.name, o.RefID, n.RefID)
			}
		}
	}

	// 终态：两侧全量流水（金额序列）一致；重放步不产生第二行
	oldAll := fetchLedger(t, dbOld, uidOld)
	newAll := fetchLedger(t, dbNew, uidNew)
	if len(oldAll) != len(newAll) {
		t.Fatalf("全量流水行数 diff≠0: old=%d new=%d", len(oldAll), len(newAll))
	}
	for i := range oldAll {
		if oldAll[i].Delta != newAll[i].Delta {
			t.Fatalf("第 %d 行金额 diff≠0: old=%+v new=%+v", i, oldAll[i], newAll[i])
		}
	}
	var deltas []int
	for _, r := range oldAll {
		deltas = append(deltas, r.Delta)
	}
	// 预期金额序列：常规(-10) + 降级键(-10) + 兜底(-10) + 大额封顶(-100) + 多模态(-10)；
	// 重放幂等无新行、游客/空回复/预检阻断无行
	wantDeltas := []int{-10, -10, -10, -100, -10}
	if len(deltas) != len(wantDeltas) {
		t.Fatalf("扣费次数不符（重放/游客/空回复/阻断均不得扣费）: %v", deltas)
	}
	for i := range wantDeltas {
		if deltas[i] != wantDeltas[i] {
			t.Fatalf("金额序列不符: got=%v want=%v", deltas, wantDeltas)
		}
	}
}
