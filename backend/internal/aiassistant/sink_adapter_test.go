package aiassistant

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"forklift-training/internal/model"
)

// recordingSink 记录 adapter 发出的事件序列，验证 adapter 通过 StreamSink 产生事件。
type recordingSink struct {
	chunks   []string
	statuses []string
	sources  [][]DiagnosisSource
	metas    []DiagnosisMeta
}

func (r *recordingSink) OnChunk(content string)        { r.chunks = append(r.chunks, content) }
func (r *recordingSink) OnStatus(text string)          { r.statuses = append(r.statuses, text) }
func (r *recordingSink) OnSources(s []DiagnosisSource) { r.sources = append(r.sources, s) }
func (r *recordingSink) OnMeta(m DiagnosisMeta)        { r.metas = append(r.metas, m) }

func (r *recordingSink) Result() StreamResult {
	return StreamResult{
		Content: strings.Join(r.chunks, ""),
		Sources: lastOr(r.sources),
		Meta:    lastMetaOr(r.metas),
		HasMeta: len(r.metas) > 0,
	}
}

func lastOr(slices [][]DiagnosisSource) []DiagnosisSource {
	if len(slices) == 0 {
		return nil
	}
	return slices[len(slices)-1]
}

func lastMetaOr(metas []DiagnosisMeta) DiagnosisMeta {
	if len(metas) == 0 {
		return DiagnosisMeta{}
	}
	return metas[len(metas)-1]
}

// TestDiagnosisAdapterEventOrder 验证 adapter 产出事件的交错序：status 先于 chunk，
// chunk 先于 done 时的 meta（与上游 SSE 帧序一致）。
func TestDiagnosisAdapterEventOrder(t *testing.T) {
	fake := &fakeDiagnosisServer{t: t, frames: []string{
		`{"type":"status","text":"正在检索…"}`,
		`{"type":"delta","text":"答案"}`,
		`{"type":"done","answer":"答案","badges":[{"name":"RAG","emoji":"📚","cls":"cap-rag"}],"elapsed":1.0}`,
	}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	sink := &recordingSink{}
	_, _, err := newDiagnosisForTest(server).Stream(context.Background(),
		ModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgsSample("q", 0), sink)
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}

	// 交错序：status 在 chunk 之前，meta 在 chunk 之后
	if len(sink.statuses) != 1 {
		t.Fatalf("应 1 个 status: %v", sink.statuses)
	}
	if len(sink.chunks) != 1 {
		t.Fatalf("应 1 个 chunk: %v", sink.chunks)
	}
	if len(sink.metas) != 1 {
		t.Fatalf("应 1 个 meta: %v", sink.metas)
	}
	// 事件序的语义保证：status 先于所有 chunk（adapter 在 delta 前收到 status 帧就立即下沉）
	// meta 在所有 chunk 之后（done 帧在 delta 之后到达）
	result := sink.Result()
	if result.Content != "答案" {
		t.Errorf("result.Content 不符: %q", result.Content)
	}
	if !result.HasMeta {
		t.Error("result.HasMeta 应为 true")
	}
}

// TestStreamChatPersistsSourcesAndMeta 验证 service 持久化从 sink.Result() 读取
// sources/meta（而非 ctx 旁路），并写入消息行的 JSON 列。
func TestStreamChatPersistsSourcesAndMeta(t *testing.T) {
	_, assistant, _, db := newAIStack(t)
	ctx := context.Background()
	fake := &fakeAIModelPort{content: "诊断回复"}
	assistant.port = fake

	session, err := assistant.CreateSession(ctx, 7, "已命名会话", "", FeatureFaultDiagnosis)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}

	// 用带 sources/meta 的 sink 驱动持久化
	sink := NewSSEStreamSink(func(event string, data any) {})
	sink.OnChunk("诊断回复")
	sink.OnSources([]DiagnosisSource{{ID: "s1", Text: "SOP 文档"}})
	sink.OnMeta(DiagnosisMeta{Badges: []DiagnosisBadge{{Name: "RAG", Emoji: "📚", Cls: "cap-rag"}}, Elapsed: 2.0})

	_, _, err = assistant.StreamChat(ctx, 7, StreamChatReq{
		SessionID: session.ID,
		Messages:  []StreamMessage{{Role: "user", Content: "制动异响"}},
	}, sink)
	if err != nil {
		t.Fatalf("StreamChat 失败: %v", err)
	}

	// 验证持久化的助手消息行包含 sources 和 meta
	var msg model.AIChatMessage
	if err := db.Where("session_id = ? AND role = ?", session.ID, "assistant").First(&msg).Error; err != nil {
		t.Fatalf("查询助手消息失败: %v", err)
	}
	if !strings.Contains(msg.Sources, "SOP 文档") {
		t.Errorf("sources 列应包含来源: %q", msg.Sources)
	}
	if !strings.Contains(msg.Meta, "RAG") {
		t.Errorf("meta 列应包含徽章: %q", msg.Meta)
	}
}

// TestDiagnosisAdapterUsesStreamSink 验证 diagnosis adapter 通过 StreamSink 产生事件
// （而非 context 旁路），事件序与上游 SSE 帧一致。
func TestDiagnosisAdapterUsesStreamSink(t *testing.T) {
	answer := "检查制动系统"
	fake := &fakeDiagnosisServer{t: t, frames: []string{
		`{"type":"status","text":"正在检索…"}`,
		`{"type":"delta","text":"检查"}`,
		`{"type":"delta","text":"制动系统"}`,
		`{"type":"done","answer":"` + answer + `","badges":[{"name":"RAG","emoji":"📚","cls":"cap-rag"}],"elapsed":1.5}`,
	}}
	server := httptest.NewServer(fake.handler())
	defer server.Close()

	sink := &recordingSink{}

	content, _, err := newDiagnosisForTest(server).Stream(
		context.Background(),
		ModelSelector{FeatureKey: FeatureFaultDiagnosis},
		msgsSample("制动异响", 0),
		sink,
	)
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}

	if content != answer {
		t.Errorf("done.answer 应为权威全文: got=%q want=%q", content, answer)
	}

	// sink 应收到事件：1 status + 2 chunks（delta） + done 时的 meta
	if len(sink.statuses) != 1 || !strings.Contains(sink.statuses[0], "正在检索") {
		t.Errorf("sink 应收到 1 个 status 事件: %v", sink.statuses)
	}
	if len(sink.chunks) != 2 {
		t.Errorf("sink 应收到 2 个 chunk（delta 帧）: got %d %v", len(sink.chunks), sink.chunks)
	}
	if len(sink.metas) == 0 {
		t.Error("sink 应收到 OnMeta（done 帧的 badges+elapsed）")
	}
}
