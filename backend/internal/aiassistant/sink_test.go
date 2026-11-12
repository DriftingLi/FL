package aiassistant

import (
	"testing"
)

// recordingSendEvent records SSE events for assertions.
type recordingSendEvent struct {
	events []string
}

func (r *recordingSendEvent) send(event string, data any) {
	r.events = append(r.events, event)
}

// TestSSEStreamSinkWritesMessageEvents 验证 OnChunk 写出 message SSE 帧并累积 content。
func TestSSEStreamSinkWritesMessageEvents(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	sink.OnChunk("hello ")
	sink.OnChunk("world")

	if len(rec.events) != 2 {
		t.Fatalf("expected 2 SSE events, got %d: %v", len(rec.events), rec.events)
	}
	if rec.events[0] != "message" || rec.events[1] != "message" {
		t.Errorf("expected both events to be 'message', got %v", rec.events)
	}

	result := sink.Result()
	if result.Content != "hello world" {
		t.Errorf("expected content %q, got %q", "hello world", result.Content)
	}
}

// TestSSEStreamSinkWritesStatusEvents 验证 OnStatus 写出 status SSE 帧。
func TestSSEStreamSinkWritesStatusEvents(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	sink.OnStatus("正在检索...")

	if len(rec.events) != 1 || rec.events[0] != "status" {
		t.Errorf("expected 1 'status' event, got %v", rec.events)
	}
}

// TestSSEStreamSinkWritesSourcesEvents 验证 OnSources 写出 sources SSE 帧（仅非空时）。
func TestSSEStreamSinkWritesSourcesEvents(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	sink.OnSources([]DiagnosisSource{{ID: "s1", Text: "manual"}})

	if len(rec.events) != 1 || rec.events[0] != "sources" {
		t.Errorf("expected 1 'sources' event, got %v", rec.events)
	}

	result := sink.Result()
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source in result, got %d", len(result.Sources))
	}
}

// TestSSEStreamSinkSkipsEmptySources 验证空 sources 不写 SSE 帧但仍存入 result。
func TestSSEStreamSinkSkipsEmptySources(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	sink.OnSources(nil)

	if len(rec.events) != 0 {
		t.Errorf("expected 0 SSE events for empty sources, got %v", rec.events)
	}

	result := sink.Result()
	if result.Sources != nil {
		t.Errorf("expected nil sources in result, got %v", result.Sources)
	}
}

// TestSSEStreamSinkStoresMetaWithoutSSE 验证 OnMeta 只存 result 不写 SSE（meta 进 done 载荷）。
func TestSSEStreamSinkStoresMetaWithoutSSE(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	meta := DiagnosisMeta{
		Badges:  []DiagnosisBadge{{Name: "知识检索", Emoji: "📚", Cls: "cap-rag"}},
		Elapsed: 1.5,
	}
	sink.OnMeta(meta)

	if len(rec.events) != 0 {
		t.Errorf("expected 0 SSE events for meta, got %v", rec.events)
	}

	result := sink.Result()
	if !result.HasMeta {
		t.Error("expected HasMeta=true")
	}
	if result.Meta.Elapsed != 1.5 {
		t.Errorf("expected elapsed 1.5, got %v", result.Meta.Elapsed)
	}
}

// TestSSEStreamSinkAccumulatesAllEvents 验证混合事件流的完整累积。
func TestSSEStreamSinkAccumulatesAllEvents(t *testing.T) {
	rec := &recordingSendEvent{}
	sink := NewSSEStreamSink(rec.send)

	sink.OnStatus("正在检索...")
	sink.OnChunk("答案：")
	sink.OnChunk("检查制动系统")
	sink.OnSources([]DiagnosisSource{{ID: "s1", Text: "SOP"}})
	sink.OnMeta(DiagnosisMeta{Badges: []DiagnosisBadge{{Name: "RAG"}}, Elapsed: 2.0})

	// status + 2 message + sources = 4 SSE events（meta 不写 SSE）
	if len(rec.events) != 4 {
		t.Fatalf("expected 4 SSE events, got %d: %v", len(rec.events), rec.events)
	}
	if rec.events[0] != "status" {
		t.Errorf("first event should be 'status', got %q", rec.events[0])
	}
	if rec.events[1] != "message" || rec.events[2] != "message" {
		t.Errorf("events 2-3 should be 'message', got %v", rec.events[1:3])
	}
	if rec.events[3] != "sources" {
		t.Errorf("last event should be 'sources', got %q", rec.events[3])
	}

	result := sink.Result()
	if result.Content != "答案：检查制动系统" {
		t.Errorf("content mismatch: %q", result.Content)
	}
	if len(result.Sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(result.Sources))
	}
	if !result.HasMeta {
		t.Error("expected HasMeta=true")
	}
}
