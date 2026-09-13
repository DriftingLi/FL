package apitypes

import (
	"encoding/json"
	"strings"
	"testing"
)

// spec #952 片一：渲染器的**纯函数 seam**。
//
// 与 codegen_test.go 的分工：那里用真实 swagger 产物钉「生成物与注解同步」；
// 这里用合成 Schema 钉渲染规则本身（表达力边界：缺省态 / 可空 / 未定型 any），
// 不依赖 swagger.json，故能在注解层还没补齐时先把规则定下来（TDD 的红灯在此）。

func probeSpec(defs map[string]Schema) *Spec {
	return &Spec{Definitions: defs, Paths: map[string]any{}}
}

func renderProbe(t *testing.T, defs map[string]Schema, roots ...string) string {
	t.Helper()
	out, err := RenderDomain(probeSpec(defs), Domain{
		Name:      "probe",
		Title:     "探针",
		Roots:     roots,
		Endpoints: []Endpoint{{Method: "GET", Path: "/probe"}},
	})
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	return out
}

func assertLine(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("生成物里找不到 %q\n实际内容:\n%s", want, got)
	}
}

// 缺省态：Go 侧 omitempty 的字段在响应里**可能整个键不存在**，注解层用 x-optional 表达，
// 生成物必须渲染可选属性（T | undefined），否则前端 hand-written 的 ? 会被静默降级成必填。
func TestRenderOptionalField(t *testing.T) {
	got := renderProbe(t, map[string]Schema{
		"service.Probe": {Type: "object", Props: map[string]Schema{
			"name": {Type: "string", XOptional: true},
			"id":   {Type: "integer"},
		}},
	}, "service.Probe")

	assertLine(t, got, "  name?: string\n")
	assertLine(t, got, "  id: number\n")
}

// 可空态：Go 侧指针字段在响应里是 null（键一定存在），注解层用 x-nullable 表达。
// 规则是「可空作用于字段本身」，与字段是引用/标量/数组无关 —— 打卡域试点只覆盖了引用形态。
func TestRenderNullableField(t *testing.T) {
	got := renderProbe(t, map[string]Schema{
		"service.A": {Type: "object", Props: map[string]Schema{
			"name": {Type: "string", XNullable: true},
			"tags": {Type: "array", Items: &Schema{Type: "string"}, XNullable: true},
			"ref":  {AllOf: []Schema{{Ref: "#/definitions/service.A"}}, XNullable: true, XOptional: true},
			"id":   {Type: "integer"},
		}},
	}, "service.A")

	assertLine(t, got, "  name: string | null\n")
	assertLine(t, got, "  tags: string[] | null\n")
	assertLine(t, got, "  ref?: A | null\n")
	assertLine(t, got, "  id: number\n")
}

// 未定型：Go 侧 any 字段在 swagger 里是空 schema {}，语义是「任意值」。
// 渲染成 Record<string, unknown> 是错的（运行时可能是 string / number / 数组），
// 只有显式 type:object 才是记录类型。
func TestRenderUntypedFieldIsUnknown(t *testing.T) {
	got := renderProbe(t, map[string]Schema{
		"service.A": {Type: "object", Props: map[string]Schema{
			"user_answer":   {},
			"answers":       {XNullable: true},
			"options":       {Type: "object"},
			"answers_state": {Type: "object", AddProps: json.RawMessage("{}")},
			"typed_map":     {Type: "object", AddProps: json.RawMessage("{\"type\":\"string\"}")},
		}},
	}, "service.A")

	assertLine(t, got, "  user_answer: unknown\n")
	assertLine(t, got, "  answers: unknown\n") // unknown 已含 null，不渲染 unknown | null
	assertLine(t, got, "  options: Record<string, unknown>\n")
	assertLine(t, got, "  answers_state: Record<string, unknown>\n")
	assertLine(t, got, "  typed_map: Record<string, string>\n")
}
