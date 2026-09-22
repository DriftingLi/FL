// Package service 诊断助手**线上传真值**夹具测试（ADR-0063）：夹具是 2026-09-22 在
// lxc101 实包抓下来的原始响应体（20260904 / 20260921 两版，长文本与数组按原样键裁剪、
// 不新增键），经 fakeDiagnosisServer.raw **原样回写**后驱动真实 adapter —— 与既有
// 「用 Go 类型回编码」的 fake 互补：后者厂商一改键名就恒绿，本文件不会。
package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// wireNew20260921 新版：正文只剩 [IMG:image_id] 裸令牌，URL 全部另在 data.answer_images[]。
// file_path 是交付方构建机的 Windows 绝对路径（实测泄漏 E:\temp\…），本仓一律不读。
const wireNew20260921 = `{"code":200,"data":{
"answer_images":[
 {"image_id":"img_ee752a0879d6","url":"/assistant/static/fault_images/制动系统/1721219449286.png","file_path":"E:\\temp\\assistant_delivery_20260921\\knowledge\\static\\fault_images\\制动系统\\1721219449286.png","caption":"","source_type":"markdown_case","kind":"operation_photo","anchor_type":"strong_step","display":"inline_large","click_action":"preview","step_no":1,"doc_title":"B04_制动蓄能器压低","system":"制动系统","source_file":"B04_制动蓄能器压低.md"},
 {"image_id":"img_0fd6a1f4d3ab","url":"/assistant/static/manual/ep_linde_e_1286_01_manual/page_448_10052.png","file_path":"","caption":"液压原理图局部","source_type":"pdf_manual","kind":"diagram_page","anchor_type":"global_diagram","display":"inline_large","click_action":"open_pdf","step_no":null,"doc_title":"林德E1286","system":"","source_file":"ep_linde_e_1286_01_manual.pdf"}],
"answer_sources":[
 {"id":"fault-136","text":"检查蓄能器压力。<<IMAGE:/assistant/static/manual/ep_linde_e_1286_01_manual/page_448_10052.png>> [IMG:img_ee752a0879d6]","metadata":{"source_url":"/assistant/static/manual/ep_linde_e_1286_01_manual/ep_linde_e_1286_01_manual.pdf#page=448","page_start":448,"page_end":448,"doc_title":"林德E1286","image_refs":[{"image_id":"img_0fd6a1f4d3ab","url":"/assistant/static/manual/ep_linde_e_1286_01_manual/page_448_10052.png"},{"image_id":"img_ee752a0879d6","url":"/assistant/static/fault_images/制动系统/1721219449286.png"}]},"score":0.81,"dense_score":0.79,"bm25_score":12.4},
 {"id":"case-31","text":"更换制动片后复测。","metadata":{},"score":0.6,"dense_score":0.6,"bm25_score":3.1}],
"delivery_contract":{},"delivery_mode":"full","evidence":[],"evidence_count":2,"exact_faults":[],
"intent":"diagnose","is_llm_generated":true,"llm_error":null,"maintenance_intervals":[],
"maintenance_rows":[],"rag_stage":"symptom_retrieve_then_generate","retrieval":{},
"retrieved_chunks":[{"text":"…<<IMAGE:/app/static/manual/ep_test/page_2.png>>","metadata":{}}],
"slots":{"brand":"all","fault_code":null},
"sop_text":"# 制动系统异响 SOP\n\n## 一、前置安全准备\n1. 停机确认。\n\n[IMG:img_ee752a0879d6]\n\n2. 断开主电源插头。\n\n[IMG:img_0fd6a1f4d3ab]\n\n## 二、验收\n恢复供电后试运行。",
"task_context":{"action":"continue"},"visual_error":null,"visual_evidence":[],"visual_info":{}
}}`

// wireOld20260904 旧版基线：无 answer_images 键（20 vs 21），正文是内联 markdown 图，
// 指向公网不可达的助手内网路径（ADR-0032 已下线 assistant 子域）⇒ 学员端本就是坏图。
const wireOld20260904 = `{"code":200,"data":{
"answer_sources":[{"id":12,"text":"页图。<<IMAGE:/assistant/static/manual/ep_test/page_1.png>>","metadata":{"source_url":"/assistant/static/manual/ep_test/ep_test.pdf#page=1","page_start":1,"page_end":1}}],
"delivery_contract":{},"delivery_mode":"full","evidence":[],"evidence_count":0,"exact_faults":[],
"intent":"diagnose","is_llm_generated":true,"llm_error":null,"maintenance_intervals":[],
"maintenance_rows":[],"rag_stage":"generate","retrieval":{},"retrieved_chunks":[],"slots":{},
"sop_text":"## 步骤\n![手册图片](/assistant/static/manual/ep_test/page_1.png)\n完成。",
"task_context":{},"visual_error":null,"visual_evidence":[],"visual_info":{}
}}`

func wireDataKeys(t *testing.T, raw string) []string {
	t.Helper()
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("夹具本身不是合法 JSON: %v", err)
	}
	keys := make([]string, 0, len(env.Data))
	for k := range env.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func wantDataKeys(withAnswerImages bool) []string {
	keys := []string{"answer_sources", "delivery_contract", "delivery_mode", "evidence",
		"evidence_count", "exact_faults", "intent", "is_llm_generated", "llm_error",
		"maintenance_intervals", "maintenance_rows", "rag_stage", "retrieval",
		"retrieved_chunks", "slots", "sop_text", "task_context", "visual_error",
		"visual_evidence", "visual_info"}
	if withAnswerImages {
		return append([]string{"answer_images"}, keys...)
	}
	return keys
}

// TestDiagnosisWire_KeySetLock 键集锁：厂商下一次改/加/删键时这里必须红（生产解码仍保持
// 宽容，只锁「已知形状」）。answer_images 就是 20260921 靠这类锁发现的。
func TestDiagnosisWire_KeySetLock(t *testing.T) {
	newKeys, oldKeys := wireDataKeys(t, wireNew20260921), wireDataKeys(t, wireOld20260904)
	if got, want := strings.Join(newKeys, "|"), strings.Join(wantDataKeys(true), "|"); got != want {
		t.Fatalf("data 键集漂移:\n got=%s\nwant=%s", got, want)
	}
	if got, want := strings.Join(oldKeys, "|"), strings.Join(wantDataKeys(false), "|"); got != want {
		t.Fatalf("旧版 data 键集漂移:\n got=%s\nwant=%s", got, want)
	}
	// 20260921 相对旧版只增 answer_images 一个键，其余零差异（升级兼容性的机检陈述）
	if len(newKeys)-len(oldKeys) != 1 {
		t.Fatalf("新旧键集差应恰为 answer_images 一键: new=%d old=%d", len(newKeys), len(oldKeys))
	}
	var probe struct {
		Data struct {
			AnswerImages []map[string]json.RawMessage `json:"answer_images"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(wireNew20260921), &probe); err != nil {
		t.Fatalf("解析 answer_images 失败: %v", err)
	}
	if len(probe.Data.AnswerImages) == 0 {
		t.Fatal("夹具应含 answer_images 条目")
	}
	imgKeys := probe.Data.AnswerImages[0]
	got := make([]string, 0, len(imgKeys))
	for k := range imgKeys {
		got = append(got, k)
	}
	sort.Strings(got)
	want := []string{"anchor_type", "caption", "click_action", "display", "doc_title",
		"file_path", "image_id", "kind", "source_file", "source_type", "step_no", "system", "url"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("answer_images 条目键集漂移:\n got=%s\nwant=%s", strings.Join(got, "|"), strings.Join(want, "|"))
	}
}

// streamWithWire 用原始响应字节驱动真实 adapter，返回归一后正文、切块与来源。
func streamWithWire(t *testing.T, raw string) (content string, chunks []string, sources []DiagnosisSource) {
	t.Helper()
	fake := &fakeDiagnosisServer{t: t, raw: raw}
	server := httptest.NewServer(fake.handler())
	defer server.Close()
	ctx := WithDiagnosisSources(context.Background())
	got, _, err := newDiagnosisForTest(server).Stream(ctx, AIModelSelector{FeatureKey: FeatureFaultDiagnosis},
		[]*schema.Message{schema.UserMessage("制动系统异响怎么排查")}, func(c string) { chunks = append(chunks, c) })
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}
	return got, chunks, DiagnosisSourcesFrom(ctx)
}

// TestDiagnosisWire_NewVersionTokensNormalized 新版真值：令牌换成本站代理图（中文案例
// 目录逐段百分号转义），正文不再出现内部标识；切块拼接恒等于归一后全文。
func TestDiagnosisWire_NewVersionTokensNormalized(t *testing.T) {
	content, chunks, sources := streamWithWire(t, wireNew20260921)
	for _, want := range []string{
		"![诊断配图](/api/ai-assistant/diagnosis/manual/fault_images/%E5%88%B6%E5%8A%A8%E7%B3%BB%E7%BB%9F/1721219449286.png)",
		"![液压原理图局部](/api/ai-assistant/diagnosis/manual/manual/ep_linde_e_1286_01_manual/page_448_10052.png)",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("正文应含归一后的代理图:\nwant=%s\ngot=%s", want, content)
		}
	}
	for _, leak := range []string{"[IMG:", "img_ee752a0879d6", "img_0fd6a1f4d3ab", "/assistant/static/fault_images", "E:\\temp"} {
		if strings.Contains(content, leak) {
			t.Fatalf("正文泄漏内部标识/内网路径 %q:\n%s", leak, content)
		}
	}
	if strings.Join(chunks, "") != content {
		t.Fatalf("切块拼接 != 归一后全文:\njoined=%q\ncontent=%q", strings.Join(chunks, ""), content)
	}
	// 来源面：历史标记透传，且**结构化来源正文里的 [IMG:] 令牌换成两端认得的 <<IMAGE:>> 形状**
	// （实测 fault-* 来源 text 里带令牌，不换形学员会在来源卡片里看到内部标识）
	if len(sources) != 2 {
		t.Fatalf("sources 条数不符: %d", len(sources))
	}
	if !strings.Contains(sources[0].Text, "<<IMAGE:/assistant/static/manual/") {
		t.Fatalf("既有 manual 标记应透传: %q", sources[0].Text)
	}
	if want := "<<IMAGE:/assistant/static/fault_images/制动系统/1721219449286.png>>"; !strings.Contains(sources[0].Text, want) {
		t.Fatalf("来源令牌应换成 %s，got %q", want, sources[0].Text)
	}
	if strings.Contains(sources[0].Text, "[IMG:") {
		t.Fatalf("来源正文不应残留裸令牌: %q", sources[0].Text)
	}
	if !strings.HasSuffix(sources[0].Metadata.SourceURL, ".pdf#page=448") {
		t.Fatalf("source_url 的 #page 锚点应保留: %s", sources[0].Metadata.SourceURL)
	}
}

// TestDiagnosisWire_OldVersionStillWorks 回滚格（新代码 + 旧知识）：旧版响应无
// answer_images，归一层对它是「把本就打不开的内网图改成本站代理图」，不产令牌、不报错。
func TestDiagnosisWire_OldVersionStillWorks(t *testing.T) {
	content, chunks, sources := streamWithWire(t, wireOld20260904)
	want := "![手册图片](/api/ai-assistant/diagnosis/manual/manual/ep_test/page_1.png)"
	if !strings.Contains(content, want) {
		t.Fatalf("旧版内联图应改指本站代理:\nwant=%s\ngot=%s", want, content)
	}
	if strings.Contains(content, "[IMG:") || strings.Contains(content, "/assistant/static/") {
		t.Fatalf("旧版正文不应残留令牌或内网绝对路径:\n%s", content)
	}
	if strings.Join(chunks, "") != content {
		t.Fatalf("切块拼接 != 全文")
	}
	if len(sources) != 1 || string(sources[0].ID) != "12" {
		t.Fatalf("旧版数字 id 应透出为字符串: %+v", sources)
	}
}

// TestNormalizeDiagnosisImages 纯函数面：未知令牌、空索引、带描述后缀、越界 URL 形态。
func TestNormalizeDiagnosisImages(t *testing.T) {
	idx := map[string]diagnosisImageRef{
		"img_a1": {path: "/api/ai-assistant/diagnosis/manual/fault_images/x/1.png", caption: "步骤图"},
	}
	cases := []struct {
		name, in string
		index    map[string]diagnosisImageRef
		want     string
		drop     bool
	}{
		{"已知令牌换图", "前\n[IMG:img_a1]\n后", idx, "![步骤图](/api/ai-assistant/diagnosis/manual/fault_images/x/1.png)", false},
		{"未知令牌丢弃", "A[IMG:img_missing]B", idx, "AB", true},
		{"空索引丢全部", "A[IMG:img_a1]B", nil, "AB", true},
		{"令牌含空格", "A[IMG: img_a1 ]B", idx, "![步骤图](/api/ai-assistant/diagnosis/manual/fault_images/x/1.png)B", false},
		{"描述后缀参与 alt", "<<IMAGE:/assistant/static/fault_images/x/2.png | 描述:接线图>>", idx, "![接线图](/api/ai-assistant/diagnosis/manual/fault_images/x/2.png)", false},
		{"alt 里的方括号与反引号被清洗", "A[IMG:img_a1]B",
			map[string]diagnosisImageRef{"img_a1": {path: "/api/ai-assistant/diagnosis/manual/fault_images/x/1.png", caption: "a]b`c"}},
			"![abc](/api/ai-assistant/diagnosis/manual/fault_images/x/1.png)B", false},
		{"本站代理图二次归一不动", "![x](/api/ai-assistant/diagnosis/manual/fault_images/x/1.png)", idx,
			"![x](/api/ai-assistant/diagnosis/manual/fault_images/x/1.png)", false},
		{"绝对 http URL 不透传", "![x](http://evil.example.com/a.png)", idx, "", true},
		{"穿越路径丢弃", "![x](/assistant/static/manual/../../etc/passwd.png)", idx, "", true},
		{"无扩展名丢弃", "![x](/assistant/static/manual/ep)", idx, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeDiagnosisImages(tc.in, tc.index)
			if tc.drop {
				if strings.Contains(got, "evil") || strings.Contains(got, "passwd") || strings.Contains(got, "img_missing") ||
					strings.Contains(got, "[IMG:") || strings.Contains(got, "<<IMAGE") {
					t.Fatalf("应丢弃而未丢弃: %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("归一结果不符:\nwant=%s\ngot =%s", tc.want, got)
			}
		})
	}
}

// TestCanonicalizeDiagnosisSources 来源面两件事：/app/static/ 前缀归一、[IMG:id] 换回
// <<IMAGE:>> 形状（未知 id 丢令牌）。
func TestCanonicalizeDiagnosisSources(t *testing.T) {
	idx := map[string]diagnosisImageRef{
		"img_a1": {path: "/api/ai-assistant/diagnosis/manual/fault_images/x/1.png", raw: "/assistant/static/fault_images/x/1.png"},
	}
	sources := []DiagnosisSource{
		{Text: "见 <<IMAGE:/app/static/manual/ep/page_1.png>> 与 [IMG:img_a1] 以及 [IMG:img_missing]", Metadata: DiagnosisSourceMetadata{
			SourceURL: "/app/static/manual/ep/ep.pdf#page=1",
		}},
		{Text: "已正确 <<IMAGE:/assistant/static/manual/ep/page_2.png>>"},
	}
	got := canonicalizeDiagnosisSources(sources, idx)
	if !strings.Contains(got[0].Text, "/assistant/static/manual/ep/page_1.png") ||
		strings.Contains(got[0].Text, "/app/static/") {
		t.Fatalf("Text 前缀未归一: %q", got[0].Text)
	}
	if !strings.Contains(got[0].Text, "<<IMAGE:/assistant/static/fault_images/x/1.png>>") {
		t.Fatalf("已知令牌未换成来源标记: %q", got[0].Text)
	}
	if strings.Contains(got[0].Text, "[IMG:") || strings.Contains(got[0].Text, "img_missing") {
		t.Fatalf("令牌（含未知的）不应残留: %q", got[0].Text)
	}
	if got[0].Metadata.SourceURL != "/assistant/static/manual/ep/ep.pdf#page=1" {
		t.Fatalf("SourceURL 前缀未归一: %q", got[0].Metadata.SourceURL)
	}
	if got[1].Text != sources[1].Text {
		t.Fatalf("正确形状不应被改写: %q", got[1].Text)
	}
}
