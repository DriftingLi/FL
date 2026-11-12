package aiassistant

import "strings"

// StreamSink 是流式模型调用的事件输出 seam（ModelPort.Stream 的 onChunk 替代）。
// adapter 在流式过程中调用 On* 方法产生事件；具体实现同时负责 SSE 帧写出与
// 事件累积（供持久化层经 Result() 读取）。取代原先通过 context 旁路传递诊断
// 富事件（DiagnosisSources / DiagnosisMeta / DiagnosisStatusSink 三个 ctx key）。
type StreamSink interface {
	// OnChunk 产生内容增量，写出 message SSE 帧。
	OnChunk(content string)
	// OnStatus 产生状态文本（如"正在检索…"），写出 status SSE 帧。
	OnStatus(text string)
	// OnSources 产生诊断来源资料，非空时写出 sources SSE 帧并存入 result。
	OnSources(sources []DiagnosisSource)
	// OnMeta 产生诊断元数据（徽章 + 耗时），存入 result（meta 进 done 载荷，不单独写 SSE）。
	OnMeta(meta DiagnosisMeta)
}

// StreamResult 是流式调用的累积结果，由持久化层在流式完成后读取。
type StreamResult struct {
	// Content 是 OnChunk 累积的内容增量拼接（用于 done.content 的一致性比对）。
	Content string
	// Sources 是诊断来源资料（非诊断功能恒空）。
	Sources []DiagnosisSource
	// Meta 是诊断元数据（徽章 + 耗时）。
	Meta DiagnosisMeta
	// HasMeta 标记是否收到过 OnMeta（区分"没收到"与"收到空值"）。
	HasMeta bool
}

// StreamSinkWithResult 是持久化面对 StreamSink 的完整视图：既有事件产出（StreamSink）
// 又有累积结果读取（Result）。StreamChat 接受此接口以保证 sink 不会因缺 Result()
// 而静默丢弃 sources/meta——这正是原 ctx 旁路的失败模式。
type StreamSinkWithResult interface {
	StreamSink
	// Result 返回累积结果供持久化层与 handler 的 done 载荷消费。
	Result() StreamResult
}

// SSEStreamSink 实现 StreamSinkWithResult：将事件写为 SSE 帧并累积 result。
// 由 handler 创建并绑定 gin.Context 的 sendEvent 闭包。
type SSEStreamSink struct {
	sendEvent func(event string, data any)
	sb        strings.Builder
	sources   []DiagnosisSource
	meta      DiagnosisMeta
	hasMeta   bool
}

// NewSSEStreamSink 创建绑定 SSE 写出函数的 StreamSink。
func NewSSEStreamSink(sendEvent func(event string, data any)) *SSEStreamSink {
	return &SSEStreamSink{sendEvent: sendEvent}
}

var _ StreamSinkWithResult = (*SSEStreamSink)(nil)

// OnChunk 写出 message SSE 帧并累积 content。
func (s *SSEStreamSink) OnChunk(content string) {
	s.sb.WriteString(content)
	s.sendEvent("message", map[string]string{"content": content})
}

// OnStatus 写出 status SSE 帧。
func (s *SSEStreamSink) OnStatus(text string) {
	s.sendEvent("status", map[string]string{"text": text})
}

// OnSources 非空时写出 sources SSE 帧并存入 result。
func (s *SSEStreamSink) OnSources(sources []DiagnosisSource) {
	s.sources = sources
	if len(sources) > 0 {
		s.sendEvent("sources", map[string]any{"sources": sources})
	}
}

// OnMeta 存入 result（meta 进 done 载荷，不单独写 SSE 帧）。
func (s *SSEStreamSink) OnMeta(meta DiagnosisMeta) {
	s.meta = meta
	s.hasMeta = true
}

// Result 返回累积结果供持久化层与 handler 的 done 载荷消费。
func (s *SSEStreamSink) Result() StreamResult {
	return StreamResult{
		Content: s.sb.String(),
		Sources: s.sources,
		Meta:    s.meta,
		HasMeta: s.hasMeta,
	}
}
