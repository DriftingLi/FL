package apitypes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// 前端契约类型生成器（spec #940 片五③）。
//
// 渲染是**纯函数**（swagger 产物 + 域声明 → 输出字符串）：无时间戳、无随机序、无 map 迭代序
// ——「字节级全等」契约测试（codegen_test.go）的前提。生成物过期由该测试直接变红暴露。

// tsHeaderTemplate 生成物头部。%s₁ = 域标题，%s₂ = 端点清单，%s₃ = 本次生成覆盖的 Go 类型。
const tsHeaderTemplate = `// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：%s
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
%s//
// 覆盖的 Go 类型：%s
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

`

// Schema swagger 定义的最小可用子集（只解析生成 TS 所需的字段）。
type Schema struct {
	Ref       string            `json:"$ref"`
	Type      string            `json:"type"`
	Items     *Schema           `json:"items"`
	Props     map[string]Schema `json:"properties"`
	AddProps  json.RawMessage   `json:"additionalProperties"`
	AllOf     []Schema          `json:"allOf"`
	XNullable bool              `json:"x-nullable"`
	XOptional bool              `json:"x-optional"`
}

// normalize 折叠 swag 的 allOf 包装：字段带 @Description 时，swag 会把 $ref 包成
// {"allOf":[{"$ref":…}],"description":…,"x-nullable":…} —— 语义与直接 $ref 等价的单元素形态。
func normalize(s Schema) Schema {
	if len(s.AllOf) == 1 {
		merged := s.AllOf[0]
		merged.XNullable = merged.XNullable || s.XNullable
		merged.XOptional = merged.XOptional || s.XOptional
		return merged
	}
	return s
}

// Spec swagger.json 的最小投影。
type Spec struct {
	Definitions map[string]Schema `json:"definitions"`
	Paths       map[string]any    `json:"paths"`
}

// LoadSpec 读取并解析 swagger 产物。
func LoadSpec(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 swagger 产物失败（先 cd backend && make swagger）: %w", err)
	}
	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("解析 swagger 产物失败: %w", err)
	}
	if len(spec.Definitions) == 0 {
		return nil, errors.New("swagger 产物没有 definitions（注解里没有可生成的类型）")
	}
	return &spec, nil
}

// refName 从 $ref 取类型名（#/definitions/service.CheckInResult → service.CheckInResult）。
func refName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// tsName 把 Go 侧类型名映射为生成物里的 TS 名（去掉包前缀）。
func tsName(goName string) string {
	if i := strings.LastIndex(goName, "."); i >= 0 {
		return goName[i+1:]
	}
	return goName
}

// collect 取根类型的传递闭包（引用到的定义一并收集），返回按名排序的切片。
func collect(spec *Spec, roots []string) ([]string, error) {
	seen := map[string]bool{}
	var visit func(name string) error
	visit = func(name string) error {
		if seen[name] {
			return nil
		}
		def, ok := spec.Definitions[name]
		if !ok {
			return fmt.Errorf("swagger definitions 里没有 %q（注解改名？声明表未同步？）", name)
		}
		seen[name] = true
		refs := []string{}
		collectRefs(def, &refs)
		for _, r := range refs {
			if err := visit(r); err != nil {
				return err
			}
		}
		return nil
	}
	for _, r := range roots {
		if err := visit(r); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func collectRefs(s Schema, out *[]string) {
	s = normalize(s)
	if s.Ref != "" {
		*out = append(*out, refName(s.Ref))
	}
	for _, part := range s.AllOf {
		collectRefs(part, out)
	}
	if s.Items != nil {
		collectRefs(*s.Items, out)
	}
	for _, p := range s.Props {
		collectRefs(p, out)
	}
	if sub, ok := additionalSchema(s); ok {
		collectRefs(sub, out)
	}
}

// additionalSchema 取 additionalProperties 的对象 schema —— swagger 里它可能是 true/false
// 字面量，也可能是对象；只有对象形态才有可渲染的值类型。解析写在一处，避免两个调用点各抄一遍。
func additionalSchema(s Schema) (Schema, bool) {
	if len(s.AddProps) == 0 || !strings.HasPrefix(strings.TrimSpace(string(s.AddProps)), "{") {
		return Schema{}, false
	}
	var sub Schema
	if err := json.Unmarshal(s.AddProps, &sub); err != nil {
		return Schema{}, false
	}
	return sub, true
}

// tsType 把 swagger 类型渲染为 TS 类型表达式。
// 可空性（x-nullable）作用于**字段本身**：数组字段是「数组可空」，元素可空写在 items 上。
// 故先渲染基底类型，再统一追加 `| null` —— 与基底是引用 / 标量 / 数组无关。
func tsType(s Schema) string {
	s = normalize(s)
	base := tsBaseType(s)
	if s.XNullable && base != "unknown" {
		// unknown 已包含 null，渲染 unknown | null 只是噪音（Go 侧 any 字段的常见形态）。
		return base + " | null"
	}
	return base
}

// tsBaseType 渲染不含可空性的基底类型。
func tsBaseType(s Schema) string {
	if s.Ref != "" {
		return tsName(refName(s.Ref))
	}
	switch s.Type {
	case "array":
		if s.Items == nil {
			return "unknown[]"
		}
		inner := tsType(*s.Items)
		if strings.ContainsAny(inner, "|&") {
			return "(" + inner + ")[]"
		}
		return inner + "[]"
	case "integer", "number":
		return "number"
	case "boolean":
		return "boolean"
	case "string":
		return "string"
	case "object":
		if len(s.Props) > 0 {
			return "{ [key: string]: unknown }"
		}
		if sub, ok := additionalSchema(s); ok {
			return "Record<string, " + tsType(sub) + ">"
		}
		return "Record<string, unknown>"
	default:
		// 空 schema（Go 侧 any 在 swagger 里没有 type）语义是**任意值**：渲染 unknown，
		// 消费方必须先收窄。此前落到 object 分支渲染 Record<string, unknown>，
		// 对实际是 string / number / 数组的取值会撒谎。
		return "unknown"
	}
}

// RenderDomain 渲染一个域的 TS 生成物（纯函数，确定性输出）。
func RenderDomain(spec *Spec, d Domain) (string, error) {
	if len(d.Roots) == 0 {
		return "", fmt.Errorf("域 %q 没声明任何根类型，拒绝生成空文件", d.Name)
	}
	names, err := collect(spec, d.Roots)
	if err != nil {
		return "", err
	}
	short := make([]string, 0, len(names))
	for _, n := range names {
		short = append(short, tsName(n))
	}
	var ep strings.Builder
	for _, e := range d.Endpoints {
		fmt.Fprintf(&ep, "//   %-4s %s\n", e.Method, e.Path)
	}
	var body strings.Builder
	for i, n := range names {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(renderInterface(short[i], spec.Definitions[n]))
	}
	return fmt.Sprintf(tsHeaderTemplate, d.Title, ep.String(), strings.Join(short, " / ")) + body.String(), nil
}

func renderInterface(name string, s Schema) string {
	var b strings.Builder
	fmt.Fprintf(&b, "export interface %s {\n", name)
	keys := make([]string, 0, len(s.Props))
	for k := range s.Props {
		keys = append(keys, k)
	}
	sort.Strings(keys) // 显式排序：不依赖 swag 的输出顺序，保证渲染确定性
	for _, k := range keys {
		field := s.Props[k]
		opt := ""
		if normalize(field).XOptional {
			opt = "?"
		}
		fmt.Fprintf(&b, "  %s%s: %s\n", tsKey(k), opt, tsType(field))
	}
	b.WriteString("}\n")
	return b.String()
}

// tsKey 需要引号的 JSON 键（含非标识符字符时）加引号。
func tsKey(k string) string {
	for i, r := range k {
		ok := r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9')
		if !ok {
			return "'" + k + "'"
		}
	}
	return k
}

// RenderAll 渲染全部登记域（域顺序 = 声明表顺序，稳定）。
func RenderAll(spec *Spec) (map[string]string, error) {
	out := make(map[string]string, len(Domains))
	for _, d := range Domains {
		content, err := RenderDomain(spec, d)
		if err != nil {
			return nil, err
		}
		out[d.Name] = content
	}
	return out, nil
}
