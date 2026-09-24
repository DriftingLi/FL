// 消费点对齐锁（ADR-0065 决策 8）：一个事实的**明文载体**与它的**投影位**必须说同一句话、
// 用同一个 key，且这件事是机器读得懂的，不是注释里互指。
//
// 五条判据的原文写在 consumption_fact_registry.go 的文件头。这里只补一句为什么值得单独一把：
// 第十五波把 `company_disabled` 与 `ErrCompanyUnavailable` 对齐靠的是两处人写的注释
// （「与联系面明文位置同键同措辞的那一格」）。注释不是锁——措辞改一半、key 改一处、
// 或者第三个载体悄悄长出来而没人登记，都不会红。而这三件事的代价都由消费方付。
package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// factTag 是投影位一侧的声明：响应字段上的 `fact:"<key>"`。
const factTag = "fact"

// factScanDirs 是 tag 的扫描面。**与表态锁的 sweptDirs 同理由**：射程取「消费点所在的那些包」，
// 不按目录便利取一个——投影位可能长在 service 的 DTO 上，也可能长在 api 自己拼的响应类型上。
var factScanDirs = []struct{ dir, pkg string }{
	{".", "api"},
	{"../service", "service"},
}

// defRefRE 抓 swagger 里的 $ref。定义名就是 `#/definitions/` 后面那一段（本仓全用点分全名）。
var defRefRE = regexp.MustCompile(`#/definitions/([A-Za-z0-9_.]+)`)

// taggedFactFields 扫出「带 fact tag 的响应字段」，键 = fact 值，值 = `包名.类型名.json键` 列表。
//
// 一条都扫不到时**由调用方判 Fatal**（见判据 (e)）：tag 改名、写成注释、或者被 go:generate
// 换掉写法，都会让这张表看起来「零违规」而其实是零覆盖。
func taggedFactFields(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, src := range factScanDirs {
		entries, err := os.ReadDir(src.dir)
		if err != nil {
			t.Fatalf("读 fact tag 扫描目录 %s 失败: %v", src.dir, err)
		}
		fset := token.NewFileSet()
		n := 0
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(src.dir, name), nil, parser.ParseComments)
			if err != nil {
				t.Fatalf("解析 %s/%s 失败: %v", src.dir, name, err)
			}
			n++
			ast.Inspect(f, func(node ast.Node) bool {
				st, ok := node.(*ast.StructType)
				if !ok || st.Fields == nil {
					return true
				}
				// 类型名从包住它的那条 type 声明拿——ast.Inspect 给不到父节点，故自己再走一层。
				for _, fld := range st.Fields.List {
					if len(fld.Names) == 0 || fld.Tag == nil {
						continue
					}
					tag := strings.Trim(fld.Tag.Value, "`")
					factVal, has := reflect.StructTag(tag).Lookup(factTag)
					if !has || factVal == "" {
						continue
					}
					jsonKey := strings.Split(reflect.StructTag(tag).Get("json"), ",")[0]
					if jsonKey == "" || jsonKey == "-" {
						t.Errorf("%s.%s 上的 %s:%q 没有对应的 json 键：投影位是按键对外露出的，"+
							"没有键就不是消费点，别挂这个 tag。", src.pkg, name, factTag, factVal)
						continue
					}
					out[factVal] = append(out[factVal], typeOfStruct(st, f)+"."+jsonKey)
				}
				return true
			})
		}
		if n == 0 {
			t.Fatalf("fact tag 扫描面 %s 里一个 .go 文件都没有——目录被改名还是搬走了？（扫不到不等于无违规）", src.dir)
		}
	}
	return out
}

// typeOfStruct 找 StructType 所属的 `type X struct` 名字。找不到（匿名内联的 struct）返回 "?"，
// 由判据 (c) 的「逐字相等」当场红掉——那正是想要的：登记一个对不上号的投影位就是幽灵登记。
func typeOfStruct(st *ast.StructType, f *ast.File) string {
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if cand, ok := ts.Type.(*ast.StructType); ok && cand == st {
				return f.Name.Name + "." + ts.Name.Name
			}
		}
	}
	return "?"
}

// swaggerDoc 是一份 swagger.json：定义表 + 「哪些定义出现在 2xx 响应里」。
type swaggerDoc struct {
	// closure 是 2xx 响应的**传递** $ref 闭包。投影位必须落在这里面，否则那个 tag
	// 说的「消费点」根本不存在于对外契约里。
	closure map[string]bool
	// raw 是每个定义的原始 JSON，判据 (d) 从里面取某个属性的 description。
	raw map[string]json.RawMessage
}

func loadSwagger(t *testing.T) swaggerDoc {
	t.Helper()
	// 与表态锁同一件依赖：锁的射程来自**生成物**，不是再抄一份清单。生成物不在就 Fatal——
	// 找不到输入时跳过的锁，在 `go test ./...` 里照样报 ok。
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("读 swagger.json 失败（先跑 make swagger）: %v", err)
	}
	var doc struct {
		Definitions map[string]json.RawMessage            `json:"definitions"`
		Paths       map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("解析 swagger.json 失败: %v", err)
	}
	if len(doc.Definitions) == 0 || len(doc.Paths) == 0 {
		t.Fatalf("swagger.json 是空的（definitions=%d paths=%d）——生成物没跑还是跑坏了？", len(doc.Definitions), len(doc.Paths))
	}
	// 种子 = 所有 2xx 响应里出现的 $ref；再沿定义表传递展开。
	seen := map[string]bool{}
	var queue []string
	for _, ops := range doc.Paths {
		for method, op := range ops {
			// swagger 的 path item 里 `parameters` 之类不是操作，不能当 2xx 种子。
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var opDoc struct {
				Responses map[string]json.RawMessage `json:"responses"`
			}
			if err := json.Unmarshal(op, &opDoc); err != nil {
				continue
			}
			for code, body := range opDoc.Responses {
				if !strings.HasPrefix(code, "2") {
					continue
				}
				for _, m := range defRefRE.FindAllStringSubmatch(string(body), -1) {
					if !seen[m[1]] {
						seen[m[1]] = true
						queue = append(queue, m[1])
					}
				}
			}
		}
	}
	if len(queue) == 0 {
		t.Fatalf("swagger.json 的 2xx 响应里一个 $ref 都没有——射程为空即本锁零覆盖，别让它假装绿")
	}
	for len(queue) > 0 {
		name := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		raw, ok := doc.Definitions[name]
		if !ok {
			continue
		}
		for _, m := range defRefRE.FindAllStringSubmatch(string(raw), -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				queue = append(queue, m[1])
			}
		}
	}
	return swaggerDoc{closure: seen, raw: doc.Definitions}
}

// propertyDescription 取某个定义里某个 json 属性的 description。第二个返回值 = 属性在不在。
func (s swaggerDoc) propertyDescription(defName, jsonKey string) (string, bool) {
	raw, ok := s.raw[defName]
	if !ok {
		return "", false
	}
	var def struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return "", false
	}
	p, ok := def.Properties[jsonKey]
	return p.Description, ok
}

func TestConsumptionFactsAreAligned(t *testing.T) {
	tagged := taggedFactFields(t)
	if len(tagged) == 0 {
		t.Fatal("整个扫描面里没有一处 fact tag：登记表成了唯一的声明方，那它就不是一把锁。" +
			"检查 tag 写法（`fact:\"key\"`）与扫描目录。")
	}
	doc := loadSwagger(t)
	registry := ConsumptionFacts()

	// 判据 (e) 的反向半边之外，先要表本身自洽：key 唯一、投影位不跨 key 复用。
	byKey := map[string]FactSpec{}
	owner := map[string]string{}
	for _, spec := range registry {
		if spec.Key == "" {
			t.Fatal("登记表里有一条 key 为空的行")
		}
		if dup, seen := byKey[spec.Key]; seen {
			t.Errorf("事实 key %q 登记了两行（%v / %v）：一个事实只有一个身份，两行就是两个事实", spec.Key, dup.Projections, spec.Projections)
		}
		byKey[spec.Key] = spec
		for _, p := range spec.Projections {
			if prev, taken := owner[p]; taken {
				t.Errorf("投影位 %s 同时挂在 %q 与 %q 上：一个 key 说两件事实，消费方就没有判据了", p, prev, spec.Key)
			}
			owner[p] = spec.Key
		}
	}

	// (a) 每个 tag 的 key 必须登记。
	for key, fields := range tagged {
		if _, ok := byKey[key]; !ok {
			t.Errorf("字段 %v 挂了 fact:%q，登记表里没有这个 key——要么补登记，要么这个事实只有单侧载体，"+
				"那它就不该有 tag（单侧谈不上对齐）。", fields, key)
		}
	}

	// (b)(c)(d) 逐个登记的 key 验。
	sentinelText := map[string]string{}
	for _, spec := range registry {
		// (b) 两侧都要有，总数 ≥2。
		if len(spec.Sentinels) == 0 {
			t.Errorf("事实 %q 没有任何错误面载体：这张表锁的是「明文面与投影位对齐」，只有一侧就不是对齐。", spec.Key)
		}
		if len(spec.Projections) == 0 {
			t.Errorf("事实 %q 没有任何投影位：同上。", spec.Key)
		}
		if len(spec.Sentinels)+len(spec.Projections) < 2 {
			t.Errorf("事实 %q 的载体只有 %d 个：一个事实只在一个位置成立，不需要对齐锁。",
				spec.Key, len(spec.Sentinels)+len(spec.Projections))
		}
		for i, errv := range spec.Sentinels {
			if errv == nil {
				t.Errorf("事实 %q 的第 %d 个错误载体是 nil", spec.Key, i)
				continue
			}
			text := errv.Error()
			if text == "" {
				t.Errorf("事实 %q 的第 %d 个错误载体句子是空的", spec.Key, i)
				continue
			}
			if prev, seen := sentinelText[text]; seen && prev != spec.Key {
				t.Errorf("错误载体 %q 同时被登记成 %q 与 %q 的明文面：一句错误只能是一件事实的载体", text, prev, spec.Key)
			}
			sentinelText[text] = spec.Key
		}

		// (c) 表里的投影位 ⇔ 代码里带该 tag 的字段，双向逐字相等。
		got := map[string]bool{}
		for _, f := range tagged[spec.Key] {
			got[f] = true
		}
		want := map[string]bool{}
		for _, p := range spec.Projections {
			want[p] = true
			if !got[p] {
				t.Errorf("登记表说 %q 的投影位有 %s，但代码里没有任何带 fact:%q 的字段叫这个名字"+
					"（改名了？删了？还是 tag 挂在别的类型上了？）：登记要跟着事实走。", spec.Key, p, spec.Key)
			}
		}
		for f := range got {
			if !want[f] {
				t.Errorf("代码里 %s 挂了 fact:%q，登记表却没有这一行：漏登记的那个载体不受任何锁保护。", f, spec.Key)
			}
			if !strings.HasPrefix(f, "api.") && !strings.HasPrefix(f, "service.") {
				t.Errorf("投影位 %s 的形状不是「包名.类型名.json键」——锁与它对不上账", f)
			}
		}

		// (c) 的后半边 + (d)：投影位必须真的在 2xx 响应闭包里，且每个载体的句子里都要
		// 逐字含着同一件事实的那句话。
		for _, p := range spec.Projections {
			defName, jsonKey, ok := splitProjection(p)
			if !ok {
				t.Errorf("投影位 %q 的形状不对（应包名.类型名.json键）", p)
				continue
			}
			if !doc.closure[defName] {
				t.Errorf("投影位 %s 所在的类型不在任何 2xx 响应的闭包里：这一格对外没人读得到，"+
					"它不是消费点，tag 挂在这里就是登记了一个不存在的对齐对象。", p)
				continue
			}
			desc, has := doc.propertyDescription(defName, jsonKey)
			if !has {
				// swagger 的定义键带包名前缀（service.RecruitResumeCard），json 键是 snake_case。
				t.Errorf("投影位 %s 在 swagger 里没有这个属性：字段被改名、被 omitempty 吃掉，"+
					"还是那条字段的注释没进契约？", p)
				continue
			}
			for _, errv := range spec.Sentinels {
				if errv == nil {
					continue
				}
				if !strings.Contains(desc, errv.Error()) {
					t.Errorf("消费点对齐锁: 投影位 %s 的契约描述里没有明载体那句话 %q（实际描述：%q）。"+
						"决策 5 要求同一件事在两个位置**同一句措辞**——消费方按 key 判断、按文案提示，"+
						"两边句子不同它就认不出这是同一件事。", p, errv.Error(), desc)
				}
			}
		}
	}
}

// splitProjection 拆 `包名.类型名.json键`，恰好三段。前半段就是 swagger 的定义键
// （本仓的定义键带包名前缀，如 `service.RecruitResumeCard`），故直接拿它查表。
func splitProjection(p string) (defName, jsonKey string, ok bool) {
	if strings.Count(p, ".") != 2 {
		return "", "", false
	}
	i := strings.LastIndex(p, ".")
	if i <= 0 || i == len(p)-1 {
		return "", "", false
	}
	return p[:i], p[i+1:], true
}

// httpMethods 是 path item 里算「一个操作」的那批键。swagger 同一层还可能放 parameters 等，
// 不排掉就会把非响应的 $ref 当成 2xx 种子。
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true,
}
