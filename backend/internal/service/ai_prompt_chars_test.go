// 「什么算 prompt」口径的唯一实现测试（ADR-0053 §5）。
//
// 三条锁：
//   - 口径函数本身的边界（末尾非用户消息 / 无用户消息 / 空列表）
//   - 两个 adapter 各自的投影规则（DTO 取 Content；端口消息多模态取首个文本 part）
//   - **行为变更**：尾消息为非用户角色时，扣费字符数按「最后一条用户消息」而不是最后一条消息
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// TestAIPromptCharsOf_NeutralForm 口径函数边界（中立形态，不依赖任何 adapter）。
func TestAIPromptCharsOf_NeutralForm(t *testing.T) {
	cases := []struct {
		name string
		msgs []aiPromptMessage
		want int
	}{
		{"空列表", nil, 0},
		{"无用户消息", []aiPromptMessage{{Role: "system", Text: "系统提示"}, {Role: "assistant", Text: "回复"}}, 0},
		{"末尾用户消息", []aiPromptMessage{{Role: "system", Text: "s"}, {Role: "user", Text: "12345"}}, 5},
		{"末尾非用户：退到前一条用户消息", []aiPromptMessage{
			{Role: "user", Text: "1234567890"},
			{Role: "assistant", Text: strings.Repeat("x", 999)},
		}, 10},
		{"取最后一条用户消息（不是第一条）", []aiPromptMessage{
			{Role: "user", Text: "1"},
			{Role: "assistant", Text: "2"},
			{Role: "user", Text: "33"},
			{Role: "assistant", Text: "4"},
		}, 2},
		{"纯图片用户消息（文本为空）计 0", []aiPromptMessage{{Role: "assistant", Text: "a"}, {Role: "user", Text: ""}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := aiPromptCharsOf(tc.msgs); got != tc.want {
				t.Fatalf("口径函数 = %d, 期望 %d", got, tc.want)
			}
		})
	}
}

// TestAIPromptMessagesFromDTO 请求体 adapter：角色与 Content 直接映射，Images 不参与口径。
func TestAIPromptMessagesFromDTO(t *testing.T) {
	got := aiPromptMessagesFromDTO([]AIStreamMessage{
		{Role: "system", Content: "s"},
		{Role: "user", Content: "你好"},
		{Role: "user", Content: "", Images: []string{"a.png", "b.png"}}, // 纯图片
	})
	if len(got) != 3 {
		t.Fatalf("投影条数 = %d, 期望 3", len(got))
	}
	if got[1].Role != "user" || got[1].Text != "你好" {
		t.Fatalf("投影漂移: %+v", got[1])
	}
	if got[2].Text != "" {
		t.Fatalf("纯图片消息的文本应为空串（图片不计 prompt），得到 %q", got[2].Text)
	}
	if aiPromptCharsOf(got) != 0 {
		t.Fatal("末尾为纯图片用户消息时应计 0")
	}
}

// TestAIPromptMessagesFromPort 传输层 adapter：多模态取首个文本 part；nil 消息跳过；角色归一。
func TestAIPromptMessagesFromPort(t *testing.T) {
	b64 := "aGk="
	msgs := []*schema.Message{
		schema.SystemMessage("系统提示"),
		nil,
		{Role: schema.User, Content: "纯文本"},
		{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64}}},
			{Type: schema.ChatMessagePartTypeText, Text: "首个文本 part"},
			{Type: schema.ChatMessagePartTypeText, Text: "第二个文本 part 不取"},
		}},
		{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
			{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{Base64Data: &b64}}},
		}},
		{Role: schema.Assistant, Content: "回复"},
	}
	got := aiPromptMessagesFromPort(msgs)
	if len(got) != 5 {
		t.Fatalf("投影条数 = %d, 期望 5（nil 消息跳过）", len(got))
	}
	if got[0].Role != "system" {
		t.Fatalf("系统消息角色 = %q", got[0].Role)
	}
	if got[1].Text != "纯文本" {
		t.Fatalf("纯文本消息 = %q", got[1].Text)
	}
	if got[2].Text != "首个文本 part" {
		t.Fatalf("多模态应取首个文本 part，得到 %q", got[2].Text)
	}
	if got[3].Text != "" {
		t.Fatalf("无文本 part 的多模态消息应投影为空串，得到 %q", got[3].Text)
	}
	if got[4].Role != "assistant" {
		t.Fatalf("助手消息角色 = %q", got[4].Role)
	}
	// 末尾为助手消息 → 退到最近一条用户消息（多模态首个文本 part）
	if n := aiPromptCharsOf(got); n != 0 {
		t.Fatalf("末尾用户消息无文本 part 时应计 0，得到 %d", n)
	}
}

// TestAIPromptChars_TailNotUser_Billed_BehaviorChange 行为变更断言（端到端过闸）：
// 请求体以非用户消息结尾时，扣费字符数按**最后一条用户消息**，不再按最后一条消息。
//
// 这条路径在公开 API 上不可达——handler 已拒绝「最后一条不是 user」的请求
// （api/ai_assistant.go 的「消息不能为空」），所以它是纵深防御：口径与词表一致，
// 且直连端口的内部消费方也不会算错。
func TestAIPromptChars_TailNotUser_Billed_BehaviorChange(t *testing.T) {
	ctx := context.Background()

	// 声明路径（请求 DTO）：末尾是超长助手消息
	meter := &fakeAIMeter{}
	port, _ := newMeteredStack("回复", meter)
	dtoMsgs := []AIStreamMessage{
		{Role: "user", Content: "1234567890"},
		{Role: "assistant", Content: strings.Repeat("x", 999)},
	}
	callCtx := withAIPromptMessages(WithAIRequestID(ctx, "req-tail"), aiPromptMessagesFromDTO(dtoMsgs))
	msgs := []*schema.Message{schema.UserMessage("端口侧文本很长很长很长很长很长很长")}
	if _, _, err := port.Stream(callCtx, AIModelSelector{FeatureKey: FeatureMaintenanceKnowledge, UserID: 7}, msgs, nil); err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if _, _, pc, _, _ := meter.snapshot(); pc != 10 {
		t.Fatalf("声明路径扣费字符数 = %d, 期望 10（最后一条用户消息，而非末尾 999 的助手消息）", pc)
	}

	// 回退路径（端口消息）：末尾是助手消息，同样退到最近一条用户消息
	meter2 := &fakeAIMeter{}
	port2, _ := newMeteredStack("回复", meter2)
	portMsgs := []*schema.Message{
		schema.UserMessage("用户问题"),
		{Role: schema.Assistant, Content: strings.Repeat("y", 500)},
	}
	if _, _, err := port2.Stream(WithAIRequestID(ctx, "req-tail-2"),
		AIModelSelector{FeatureKey: FeatureMaintenanceKnowledge, UserID: 7}, portMsgs, nil); err != nil {
		t.Fatalf("调用失败: %v", err)
	}
	if _, _, pc, _, _ := meter2.snapshot(); pc != len("用户问题") {
		t.Fatalf("回退路径扣费字符数 = %d, 期望 %d（最后一条用户消息）", pc, len("用户问题"))
	}
}
