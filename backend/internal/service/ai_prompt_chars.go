// 本文件：「什么算 prompt」的唯一实现（ADR-0053 §5）。
//
// 词表口径：**只算最后一条用户消息的文本长度**——system 提示词、历史消息与图片一律不计。
// 生产里有两条输入路径，它们的**输入类型不同**（请求体 DTO 消息 vs 传输层 eino 消息），
// 所以这里定义包内中立形态（角色 + 文本），由两个 adapter 各自投影过来，口径函数只写一遍。
//
// 两个 adapter 不是重复实现——它们是两种真实存在的输入：
//   - `aiPromptMessagesFromDTO`：服务层请求体。多模态原文在 `Content`，图片加载失败时注入的
//     注记文本只存在于传输层消息，**不参与计费**（这正是必须走这条路径的理由）；
//   - `aiPromptMessagesFromPort`：计量闸门的传输层消息。它已被重组（注入 system / 历史 /
//     图片注记），故取文本时要滤角色，且多模态消息的原文在首个文本 part（Content 为空）。
//
// 在此之前，字符数由调用方算好后经 ctx 声明、meter 未取到时再自己推导一遍——两份实现只靠
// 注释说明「为什么不同」，而测试断言的是测试文件里自建的镜像，分叉永远测不红。
package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// aiPromptRoleUser 计费口径里唯一被计入的角色。
const aiPromptRoleUser = "user"

// aiPromptMessage 计费口径的中立输入形态（角色 + 文本）。
type aiPromptMessage struct {
	Role string
	Text string
}

// aiPromptCharsOf **唯一实现**：从后往前取第一条用户消息的文本长度；没有用户消息计 0。
// 多模态（纯图片）消息的 Text 为空串，故自然计 0。
func aiPromptCharsOf(msgs []aiPromptMessage) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != aiPromptRoleUser {
			continue
		}
		return len(msgs[i].Text)
	}
	return 0
}

// aiPromptMessagesFromDTO 请求体消息列表 → 中立形态。
// 文本取 DTO 的 Content（多模态原文也在这里；图片本身不计）。
func aiPromptMessagesFromDTO(msgs []AIStreamMessage) []aiPromptMessage {
	out := make([]aiPromptMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, aiPromptMessage{Role: m.Role, Text: m.Content})
	}
	return out
}

// aiPromptMessagesFromPort 传输层消息列表 → 中立形态。
// 多模态消息的请求原文在首个文本 part（Content 为空）；没有文本 part 则计 0。
func aiPromptMessagesFromPort(msgs []*schema.Message) []aiPromptMessage {
	out := make([]aiPromptMessage, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		text := m.Content
		if text == "" {
			for _, part := range m.UserInputMultiContent {
				if part.Type == schema.ChatMessagePartTypeText {
					text = part.Text
					break
				}
			}
		}
		out = append(out, aiPromptMessage{Role: roleOfPortMessage(m.Role), Text: text})
	}
	return out
}

// roleOfPortMessage 传输层角色 → 中立形态的角色字符串。
func roleOfPortMessage(role schema.RoleType) string {
	if role == schema.User {
		return aiPromptRoleUser
	}
	return string(role)
}

// aiPromptMessagesCtxKey ctx 键：调用方把**消息列表**交给计量闸门的通道。
type aiPromptMessagesCtxKey struct{}

// withAIPromptMessages 声明本次计费的 prompt 事实来源（与 withAIMeterFree 同机制、同样不导出）。
//
// 刻意传**消息列表**而不是算好的字符数：口径（取哪一条、怎么取文本）只允许有一份实现，
// 声明数字等于把口径复制到调用方。
func withAIPromptMessages(ctx context.Context, msgs []aiPromptMessage) context.Context {
	return context.WithValue(ctx, aiPromptMessagesCtxKey{}, msgs)
}

// aiPromptMessagesDeclared 读取声明的 prompt 消息列表（DTO 层事实）。
func aiPromptMessagesDeclared(ctx context.Context) ([]aiPromptMessage, bool) {
	v, ok := ctx.Value(aiPromptMessagesCtxKey{}).([]aiPromptMessage)
	return v, ok
}
