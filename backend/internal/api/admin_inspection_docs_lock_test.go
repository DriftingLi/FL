// 巡检两条列表端点的注解 ↔ paging.ItemsPage 键序锁（ADR-0056 §3 / #1097、§11 / #1100）。
//
// 背景：CI 钉住的 swag v1.16.4 展不开泛型实例化，故 admin_inspection.go 的两条 @Success 用
// **内联 object{}** 手抄信封字段（items/page/page_size/total）。手抄就是第二份事实源：漏一个字段、
// 换一下顺序，注解与运行时输出（paging.ItemsPage 的键序，ADR-0009 §2 的字节契约）会静默分叉——
// 而 swagger 新鲜度锁与 apitypes 全等锁只看「注解 → 生成物」，看不见「注解 vs 运行时」。
//
// 本锁用 go/parser 读注释（照 valuation/handler/dictcrud_docs_lock_test.go 的做法）：
//   - 事实源 = ItemsPage 实际 marshal 出来的顶层键序（不手抄键表）；
//   - 断言每条内联 object 的字段**声明序**与它逐项相等；
//   - 负样本证明该判定真的会红（换序 / 少字段都必须不等）。
package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"forklift-training/internal/service"
	"forklift-training/pkg/paging"
)

// inlineObjectMarker @Success 内联信封字段表的起始标记（与 dictcrud_docs.go 同形）。
const inlineObjectMarker = "data=object{"

// inlineEnvelopeFields 取一条注解里内联 object{} 的字段名（声明序）。
// 非内联 object 形态（data=service.Foo 这类命名类型）返回 nil, nil —— 本锁只管内联手抄面。
func inlineEnvelopeFields(line string) ([]string, error) {
	i := strings.Index(line, inlineObjectMarker)
	if i < 0 {
		return nil, nil
	}
	rest := line[i+len(inlineObjectMarker):]
	depth := 1
	end := -1
	for j, r := range rest {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = j
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("object{} 括号不配平: %s", line)
	}
	body := strings.TrimSpace(rest[:end])
	if body == "" {
		return nil, fmt.Errorf("内联 object{} 为空: %s", line)
	}
	parts := strings.Split(body, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("字段形态异常: %s", p)
		}
		out = append(out, kv[0])
	}
	return out, nil
}

// TestAdminInspectionInlineEnvelopeKeyOrder 主锁：注解内联字段序 == ItemsPage 键序。
func TestAdminInspectionInlineEnvelopeKeyOrder(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "admin_inspection.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("解析 admin_inspection.go 失败: %v", err)
	}
	// 运行时事实源：与 envelope_registry_test.go 的形状锁同一条 marshal 路径。
	want := jsonKeyOrder(t, paging.ItemsPage[service.RecruitResumeViewDTO]{})
	if strings.Join(want, ",") != "items,page,page_size,total" {
		t.Fatalf("ItemsPage 键序 = %v（期望 items,page,page_size,total）—— 本锁的前提变了", want)
	}
	checked := 0
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Doc == nil {
			continue
		}
		for _, raw := range strings.Split(fd.Doc.Text(), "\n") {
			line := strings.TrimSpace(raw)
			if !strings.HasPrefix(line, "@Success ") {
				continue
			}
			names, err := inlineEnvelopeFields(line)
			if err != nil {
				t.Fatalf("%s 的 @Success 注解不可解析: %v", fd.Name.Name, err)
			}
			if names == nil {
				continue // 非内联信封（计数 / 积分流水用命名类型）
			}
			if !reflect.DeepEqual(names, want) {
				t.Fatalf("%s：@Success 内联信封字段序与 paging.ItemsPage 键序不一致（ADR-0009 §2）\n  注解      = %v\n  ItemsPage = %v",
					fd.Name.Name, names, want)
			}
			checked++
		}
	}
	if checked != 2 {
		t.Fatalf("内联信封注解条数 = %d，期望 2（ListRecruitViews / ListRecruitRequests）—— 注解形态变了？", checked)
	}
}

// TestAdminInspectionInlineEnvelopeKeyOrderProbe 判定面的正负样本（合成注解行）：
// 换序 / 少字段必须与事实源不等，逐字同形必须相等；命名类型注解不得被误当内联信封。
func TestAdminInspectionInlineEnvelopeKeyOrderProbe(t *testing.T) {
	want := jsonKeyOrder(t, paging.ItemsPage[service.RecruitResumeViewDTO]{})
	for _, bad := range []string{
		`@Success 200 {object} response.R{data=object{page=int,items=[]service.RecruitResumeViewDTO,page_size=int,total=int}} "success"`,
		`@Success 200 {object} response.R{data=object{items=[]service.RecruitResumeViewDTO,page=int,total=int}} "success"`,
	} {
		names, err := inlineEnvelopeFields(bad)
		if err != nil {
			t.Fatalf("合成注解不可解析: %v", err)
		}
		if reflect.DeepEqual(names, want) {
			t.Fatalf("负样本必须与 ItemsPage 键序不等（否则本锁空转）: %s", bad)
		}
	}
	good := `@Success 200 {object} response.R{data=object{items=[]service.RecruitResumeViewDTO,page=int,page_size=int,total=int}} "success"`
	names, err := inlineEnvelopeFields(good)
	if err != nil {
		t.Fatalf("正样本不可解析: %v", err)
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("正样本字段序必须等于 ItemsPage 键序\n  注解      = %v\n  ItemsPage = %v", names, want)
	}
	if named, err := inlineEnvelopeFields(`@Success 200 {object} response.R{data=service.PointsLedgerResult} "success"`); err != nil || named != nil {
		t.Fatalf("命名类型注解不得被判为内联信封，实得 %v / %v", named, err)
	}
}
