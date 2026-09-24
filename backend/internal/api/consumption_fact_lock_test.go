// 消费点对齐锁（ADR-0065 决策 8）：一个事实的**明文载体**与它的**投影位**必须说同一句话、
// 用同一个 key，且这件事要机器读得懂，不能靠注释互指。
//
// 判据原文写在 consumption_fact_registry.go 的文件头，那里每条都标了「由哪一行核」。
// 值得单独锁一句的原因：第十五波对齐 `company_disabled` 与 `ErrCompanyUnavailable` 用的是
// 两处人写的注释（「与联系面明文位置**同键同措辞**的那一格」）。注释不是锁——措辞改一半、
// key 改一处、投影位改名，都不会红，而代价由消费方付：它按 key 判断、按文案提示。
//
// 「投影位所在类型必须真的出现在 2xx 响应闭包里」这半条**不在这里**，在
// `internal/apitypes/fact_reachability_lock_test.go`——闭包与 AST 遍历的只有一份实现，
// 那是表态锁那一侧（它也扫 model 与 repository，射程比本包这两目录宽）。
package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// factTag 是投影位一侧的声明：响应字段上的 `fact:"<key>"`。
const factTag = "fact"

// factScanDirs 是 tag 的扫描面。射程取「响应 DTO 住的那些包」，不按目录便利取一个。
// 注意这**不是**全仓的响应射程：model 与 repository 也在 2xx 闭包里，那边的可达性由
// apitypes 那半边锁管（见文件头）。这里的 pkg 列同时是投影位键的前缀白名单，别再抄一份。
var factScanDirs = []struct{ dir, pkg string }{
	{".", "api"},
	{"../service", "service"},
}

// factFieldInfo 是一条扫描结果：投影位键 + 它挂在哪个文件上（报错时要说清来源）。
type factFieldInfo struct {
	key  string // 包名.类型名.json键
	file string
}

// taggedFactFields 扫出「带 fact tag 的响应字段」，按 fact 值分组。
//
// 一条都扫不到时**由调用方判 Fatal**（判据 (e) 的第二半）：tag 改名、写成注释、或者被
// go:generate 换掉写法，都会让这张表看起来「零违规」而其实是零覆盖。
func taggedFactFields(t *testing.T) map[string][]factFieldInfo {
	t.Helper()
	out := map[string][]factFieldInfo{}
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
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					// 只认**具名** struct 上的 tag：匿名内联 struct 拼不出「包名.类型名.json键」，
					// 硬凑一个占位键会让判据 (c) 报成「登记对不上号」，把真因（tag 挂错了地方）
					// 藏进一条看起来像笔误的红里。
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
							t.Errorf("%s/%s 的 %s.%s 上挂了 %s:%q 却没有 json 键：投影位是按键对外露出的，"+
								"没有键就不是消费点，别挂这个 tag。",
								src.dir, name, src.pkg, ts.Name.Name, factTag, factVal)
							continue
						}
						if !ts.Name.IsExported() || !fld.Names[0].IsExported() {
							t.Errorf("%s/%s 的 %s.%s.%s 挂了 fact:%q 但类型或字段是小写的——"+
								"它进不了 swagger 定义，不是对外投影位。",
								src.dir, name, src.pkg, ts.Name.Name, fld.Names[0].Name, factVal)
							continue
						}
						key := src.pkg + "." + ts.Name.Name + "." + jsonKey
						out[factVal] = append(out[factVal], factFieldInfo{key: key, file: name})
					}
				}
			}
		}
		if n == 0 {
			t.Fatalf("fact tag 扫描面 %s 里一个 .go 文件都没有——目录被改名还是搬走了？（扫不到不等于无违规）", src.dir)
		}
	}
	return out
}

// factDescriptions 读 swagger 生成物里每个定义的属性描述。
//
// 为什么不走 apitypes.LoadSpec：那里的 Schema 是「生成 TS 所需的最小投影」，**故意不解析
// description**。本判据要读的恰恰是 description，所以只能自己按属性取一次——这只做一次
// 直接查表（定义名 → properties → description），不涉及 $ref 遍历（遍历在 apitypes 那一侧，
// 一份实现）。
func factDescriptions(t *testing.T) map[string]map[string]string {
	t.Helper()
	// 生成物不在就 Fatal，**不 Skip**：一把因为找不到输入就静默通过的锁等于没有锁。
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "swagger.json"))
	if err != nil {
		t.Fatalf("读 swagger.json 失败（先跑 make swagger）: %v", err)
	}
	var doc struct {
		Definitions map[string]struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("解析 swagger.json 失败: %v", err)
	}
	if len(doc.Definitions) == 0 {
		t.Fatalf("swagger.json 的 definitions 是空的（%d 条）——生成物没跑还是跑坏了？", len(doc.Definitions))
	}
	out := map[string]map[string]string{}
	for name, def := range doc.Definitions {
		m := map[string]string{}
		for prop, v := range def.Properties {
			m[prop] = v.Description
		}
		out[name] = m
	}
	return out
}

func TestConsumptionFactsAreAligned(t *testing.T) {
	tagged := taggedFactFields(t)
	if len(tagged) == 0 {
		t.Fatal("整个扫描面里没有一处 fact tag：登记表成了唯一的声明方，那它就不是一把锁。" +
			"检查 tag 写法（`fact:\"key\"`）与 factScanDirs。")
	}
	descs := factDescriptions(t)

	// 判据 (b)(c)(a) 里「表自己说得通不通」的那几条不依赖代码，抽成纯函数并单独配正向例
	// （TestConsumptionFactRulesFire）——今天表里只有一行，重复 key / 投影位跨 key 复用 /
	// 一句错误挂两个 key 这三条**都还没有真实的行能触发**；不打夹具验过，它们就是三条
	// 「看起来有、从没跑过」的分支。
	problems := checkFactTable(ConsumptionFacts())
	for _, p := range problems {
		t.Error(p)
	}

	// (a) 每个 tag 的 key 必须登记。
	for key, fields := range tagged {
		if !hasFactKey(ConsumptionFacts(), key) {
			names := make([]string, 0, len(fields))
			for _, fi := range fields {
				names = append(names, fi.key)
			}
			t.Errorf("字段 %v 挂了 fact:%q，登记表里没有这个 key——要么补登记，"+
				"要么这个事实只有单侧载体，那它就不该有 tag（单侧谈不上对齐）。", names, key)
		}
	}

	// (c)(d) 逐个登记的 key 验。
	for _, spec := range ConsumptionFacts() {
		// (c) 表里的投影位 ⇔ 代码里带该 tag 的字段，双向逐字相等。
		got := map[string]factFieldInfo{}
		for _, fi := range tagged[spec.Key] {
			got[fi.key] = fi
		}
		want := map[string]bool{}
		for _, p := range spec.Projections {
			want[p] = true
			if _, ok := got[p]; !ok {
				t.Errorf("登记表说 %q 的投影位有 %s，但代码里没有任何带 fact:%q 的字段叫这个名字"+
					"（改名了？删了？还是 tag 挂在别的类型上了？）：登记要跟着事实走。", spec.Key, p, spec.Key)
			}
		}
		for k, fi := range got {
			if !want[k] {
				t.Errorf("代码里 %s（%s）挂了 fact:%q，登记表却没有这一行：漏登记的那个载体不受任何锁保护。",
					fi.key, fi.file, spec.Key)
			}
		}

		// (d) 每个错误载体那句对外句子，必须逐字出现在**每一个**同 key 投影位的契约描述里。
		for _, p := range spec.Projections {
			defName, jsonKey, ok := splitProjection(p)
			if !ok {
				t.Errorf("投影位 %q 的形状不对（应「包名.类型名.json键」恰好三段）", p)
				continue
			}
			props, has := descs[defName]
			if !has {
				t.Errorf("投影位 %s 所在的类型在 swagger 定义表里找不到：这一格根本没进契约。", p)
				continue
			}
			desc, has := props[jsonKey]
			if !has {
				t.Errorf("投影位 %s 在 swagger 里没有这个属性：字段被改名了，还是那条字段的注释没进契约？", p)
				continue
			}
			for _, errv := range spec.Sentinels {
				if errv == nil {
					continue
				}
				if !strings.Contains(desc, errv.Error()) {
					t.Errorf("消费点对齐锁: 投影位 %s 的契约描述里没有明文载体那句话 %q（实际描述：%q）。"+
						"决策 5 要求同一件事在两个位置**同一句措辞**——消费方按 key 判断、按文案提示，"+
						"两边句子不同它就认不出这是同一件事。", p, errv.Error(), desc)
				}
			}
		}
	}
}

// checkFactTable 只读表本身就能判的那几条（判据 (b) 与 (c) 的键一致性）。
// 返回人话描述的违规清单，空集 = 通过。与测试分离成纯函数是为了能拿夹具打红。
func checkFactTable(reg []FactSpec) []string {
	var out []string
	byKey := map[string]bool{}
	owner := map[string]string{}
	sentinelText := map[string]string{}
	for _, spec := range reg {
		if spec.Key == "" {
			out = append(out, "登记表里有一条 key 为空的行")
			continue
		}
		if byKey[spec.Key] {
			out = append(out, "事实 key "+spec.Key+" 登记了两行：一个事实只有一个身份，两行就是两个事实")
		}
		byKey[spec.Key] = true

		// (b) 两侧都要有载体。这两条合起来就蕴含「≥2 个载体」，不再单写一条永远不可达的计数。
		if len(spec.Sentinels) == 0 {
			out = append(out, "事实 "+spec.Key+" 没有任何明文面载体：这张表锁的是「明文面与投影位对齐」，"+
				"只有一侧就不是对齐。")
		}
		if len(spec.Projections) == 0 {
			out = append(out, "事实 "+spec.Key+" 没有任何投影位：同上。")
		}
		for i, errv := range spec.Sentinels {
			if errv == nil {
				out = append(out, "事实 "+spec.Key+" 的第 "+strconv.Itoa(i)+" 个明文面载体是 nil")
				continue
			}
			text := errv.Error()
			if text == "" {
				out = append(out, "事实 "+spec.Key+" 的第 "+strconv.Itoa(i)+" 个明文面载体句子是空的")
				continue
			}
			if prev, seen := sentinelText[text]; seen && prev != spec.Key {
				out = append(out, "错误句子 "+text+" 同时被登记成 "+prev+" 与 "+spec.Key+
					" 的明文面载体：一句错误只能是一件事实的身份")
			}
			sentinelText[text] = spec.Key
		}

		// (c) 的键一致性：同一件事实的投影位**必须同名**——「同键」是决策 5 那半句的字面意思，
		// 而它不是 (d) 那条字符串包含所能蕴含的（两句描述可以各含同一句而两个键名不同）。
		seenKey := ""
		for _, p := range spec.Projections {
			defName, jsonKey, ok := splitProjection(p)
			if !ok {
				out = append(out, "投影位 "+p+" 的形状不对（应「包名.类型名.json键」恰好三段）")
				continue
			}
			if prev, taken := owner[p]; taken {
				out = append(out, "投影位 "+p+" 同时挂在 "+prev+" 与 "+spec.Key+
					" 上：一个载体说两件事实，消费方就没有判据了")
			}
			owner[p] = spec.Key
			if seenKey == "" {
				seenKey = jsonKey
			} else if jsonKey != seenKey {
				out = append(out, "事实 "+spec.Key+" 的投影位不同名（"+seenKey+" vs "+jsonKey+
					"，"+defName+"）：同一件事在两个位置各起一名，正是本锁要拦的形状")
			}
		}
	}
	return out
}

// hasFactKey 只回答「登记没登记」，把 key 一致性那半交给 checkFactTable。
func hasFactKey(reg []FactSpec, key string) bool {
	for _, spec := range reg {
		if spec.Key == key {
			return true
		}
	}
	return false
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

// TestConsumptionFactRulesFire 给 checkFactTable 配正向例：每条规则都必须真的能红。
// 今天登记表只有一行，「重复 key」「投影位跨 key 复用」「一句错误挂两个 key」「投影位不同名」
// 四条**都没有真实的行能触发**——不拿夹具打一次，它们就只是四段看起来像断言的代码。
func TestConsumptionFactRulesFire(t *testing.T) {
	okSpec := FactSpec{
		Key:         "f_ok",
		Sentinels:   []error{errFact("句子甲")},
		Projections: []string{"service.A.same_key"},
	}
	// 与 okSpec 同 key 但换一个投影位：夹具要「恰好一条违规」，所以每一例只能踩中那一条规则。
	// 第一版这里复用同一个 spec，结果「重复 key」与「投影位跨 key 复用」同时红——那不是断言不严格，
	// 是夹具写错了（两条规则本来就该各管一件事）。
	dupKeySpec := FactSpec{
		Key:         "f_ok",
		Sentinels:   []error{errFact("句子乙")},
		Projections: []string{"service.A.another_key"},
	}
	cases := []struct {
		name    string
		reg     []FactSpec
		wantSub string
	}{
		{"合规的一行不得被报", []FactSpec{okSpec}, ""},
		{"key 为空", []FactSpec{{Key: "", Sentinels: []error{errFact("x")}, Projections: []string{"service.A.k"}}}, "key 为空"},
		{"同一 key 登记两行", []FactSpec{okSpec, dupKeySpec}, "登记了两行"},
		{"缺明文面载体", []FactSpec{{Key: "f", Projections: []string{"service.A.k"}}}, "没有任何明文面载体"},
		{"缺投影位", []FactSpec{{Key: "f", Sentinels: []error{errFact("x")}}}, "没有任何投影位"},
		{"载体是 nil", []FactSpec{{Key: "f", Sentinels: []error{nil}, Projections: []string{"service.A.k"}}}, "是 nil"},
		{"载体句子为空", []FactSpec{{Key: "f", Sentinels: []error{errFact("")}, Projections: []string{"service.A.k"}}}, "句子是空的"},
		{"一句错误挂两个 key", []FactSpec{
			{Key: "f1", Sentinels: []error{errFact("同一句")}, Projections: []string{"service.A.k1"}},
			{Key: "f2", Sentinels: []error{errFact("同一句")}, Projections: []string{"service.A.k2"}},
		}, "一句错误只能是一件事实的身份"},
		{"投影位跨 key 复用", []FactSpec{
			{Key: "f1", Sentinels: []error{errFact("甲")}, Projections: []string{"service.A.shared"}},
			{Key: "f2", Sentinels: []error{errFact("乙")}, Projections: []string{"service.A.shared"}},
		}, "同时挂在"},
		{"同一事实的投影位不同名", []FactSpec{{Key: "f", Sentinels: []error{errFact("甲")},
			Projections: []string{"service.A.one_key", "service.B.other_key"}}}, "投影位不同名"},
		{"投影位形状少一段", []FactSpec{{Key: "f", Sentinels: []error{errFact("甲")},
			Projections: []string{"service.A"}}}, "形状不对"},
	}
	for _, c := range cases {
		got := checkFactTable(c.reg)
		if c.wantSub == "" {
			if len(got) != 0 {
				t.Fatalf("%s：合规的行被报红了 %v", c.name, got)
			}
			continue
		}
		if len(got) != 1 {
			t.Fatalf("%s：应恰好一条违规，实际 %d 条 %v", c.name, len(got), got)
		}
		if !strings.Contains(got[0], c.wantSub) {
			t.Fatalf("%s：报的不是那件事。\n want 含 %q\n got  %s", c.name, c.wantSub, got[0])
		}
	}
}

// errFact 是夹具用的具名错误：只为一句话，不需要 errors.New 的那层包装。
type errFact string

func (e errFact) Error() string { return string(e) }
