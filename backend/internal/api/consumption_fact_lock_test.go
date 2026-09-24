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

// factScanDirs 是 tag 的扫描面。射程取「响应 DTO 可能住的那些包」，不按目录便利取一个。
//
// 这一份与 apitypes 那把锁的 sweptDirs 必须同步（两份都是测试内常量，共享不了）。漂移由
// `TestFactScanDirsCoverTheContractUniverse` 管：它从生成物里数出「定义键出现过哪几个包名」，
// 少一个就红——所以忘了加目录的后果不是悄悄漏扫，是一条指名道姓的红。
var factScanDirs = []struct{ dir, pkg string }{
	{".", "api"},
	{"../service", "service"},
	{"../model", "model"},
	// valuation/model 的 Go 包名也叫 model，swagger 定义键同样落在 `model.` 前缀下（两边类型名
	// 不重叠，swag 自己在重名时会报），所以 pkg 列必须同为 "model" 才对得上生成物。
	// 少这一条就是实打实的漏口：投影位长在残值侧的 DTO 上时，(a)(c)(d) 一条都碰不到它。
	{"../valuation/model", "model"},
	{"../valuation/repository", "repository"},
	{"../../pkg/response", "response"},
}

// TestFactScanDirsCoverTheContractUniverse 是上面那条「两份清单会漂」的绊线。
// 判据来源是 swagger 定义键的前缀集合，而不是再去抄一份目录清单——抄来的清单本身会漂，
// 生成物不会（它由 handler 注解生成，且 CI 有新鲜度锁）。
func TestFactScanDirsCoverTheContractUniverse(t *testing.T) {
	defs := factDescriptions(t)
	scanned := map[string]bool{}
	for _, src := range factScanDirs {
		scanned[src.pkg] = true
	}
	for defName := range defs {
		pkg, _, ok := strings.Cut(defName, ".")
		if !ok {
			t.Errorf("定义键 %q 不带包名前缀——投影位的键形状要重判", defName)
			continue
		}
		if !scanned[pkg] {
			t.Errorf("对外契约里出现了 %q 包的类型，但 factScanDirs 不扫它：那一类投影位会拿到 (f)"+
				"（可达性，那份射程是全的）却拿不到 (a)(c)(d)（对齐本身）。把目录补进 factScanDirs。", pkg)
		}
	}
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
			// 投影位键的前缀取**包子句**（那才是 swagger 定义键的前缀），不取清单里的标签列；
			// 两者必须相等，否则说明有一行目录贴错了前缀——标签写错会让绊线以为覆盖了、实际按
			// 另一个前缀去查契约，正是要防的那种「清单与生成物各说各话」。
			if f.Name.Name != src.pkg {
				t.Errorf("%s/%s 的包子句是 %q，factScanDirs 却把它标成 %q：投影位键前缀按包子句算，"+
					"这一行的 pkg 列要改。", src.dir, name, f.Name.Name, src.pkg)
			}
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
								src.dir, name, f.Name.Name, ts.Name.Name, factTag, factVal)
							continue
						}
						if !ts.Name.IsExported() || !fld.Names[0].IsExported() {
							t.Errorf("%s/%s 的 %s.%s.%s 挂了 fact:%q 但类型或字段是小写的——"+
								"它进不了 swagger 定义，不是对外投影位。",
								src.dir, name, f.Name.Name, ts.Name.Name, fld.Names[0].Name, factVal)
							continue
						}
						key := f.Name.Name + "." + ts.Name.Name + "." + jsonKey
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
	registry := ConsumptionFacts()

	// 判据 (b)(c)(a) 里「表自己说得通不通」的那几条不依赖代码，抽成纯函数并单独配正向例
	// （TestConsumptionFactRulesFire）——今天表里只有一行，重复 key / 投影位跨 key 复用 /
	// 一句错误挂两个 key 这三条**都还没有真实的行能触发**；不打夹具验过，它们就是三条
	// 「看起来有、从没跑过」的分支。
	problems := checkFactTable(registry)
	for _, p := range problems {
		t.Error(p)
	}

	// (a) 每个 tag 的 key 必须登记。
	for key, fields := range tagged {
		if !hasFactKey(registry, key) {
			names := make([]string, 0, len(fields))
			for _, fi := range fields {
				names = append(names, fi.key)
			}
			t.Errorf("字段 %v 挂了 fact:%q，登记表里没有这个 key——要么补登记，"+
				"要么这个事实只有单侧载体，那它就不该有 tag（单侧谈不上对齐）。", names, key)
		}
	}

	// (b) 的可达半边：登记的句子必须在 api 层的**非测试**代码里找得到宿主。
	// 没有这一条，「明文面可达」就只是一句约定——而本波推论明写「拿不出能让它变红的证据不是表态」。
	// 全程推导，不抄清单：句子 → service 里那句 `errors.New("…")`/`fmt.Errorf("…")` 的宿主标识符
	// → api 层引用它没有。两条规则本身抽成纯函数，正向例在 TestConsumptionFactRulesFire。
	refs := apiErrorFaceRefs(t)
	faces := checkSentinelFaces(registry, sentinelHosts(t), func(q string) bool {
		return identifierWiredIntoErrorFace(refs, q)
	})
	for _, p := range faces {
		t.Error(p)
	}
	for _, spec := range registry {
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

// sentinelHosts 建「对外句子 → service 里那枚 `errors.New("…")` 的宿主标识符」。
//
// 为什么推导而不抄清单：登记表里再存一份「标识符名」就是一条没人核对的第二源（改哨兵名不会
// 有人记得回来改它，于是那条引用查不到、锁红得没道理；或者更坏——查到错的）。句子是哨兵的
// **对外形状**，本就是要跨载体对齐的东西，用它当键最贴近这件事实的身份。
// 两句相同文案会塌成一格，那是**有意**的：同文案双载体本波已裁定保持分裂（决策 9），而判据 (b)
// 另有「一句错误只能是一件事实的明文面载体」那条独立断言管住登记表这一侧。
func sentinelHosts(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir("../service")
	if err != nil {
		t.Fatalf("读 internal/service 失败: %v", err)
	}
	fset := token.NewFileSet()
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		n++
		f, err := parser.ParseFile(fset, filepath.Join("../service", name), nil, 0)
		if err != nil {
			t.Fatalf("解析 ../service/%s 失败: %v", name, err)
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, nm := range vs.Names {
					if i >= len(vs.Values) || !nm.IsExported() {
						continue
					}
					if text, ok := errorsNewLiteral(vs.Values[i]); ok {
						out[text] = "service." + nm.Name
					}
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("../service 里一个非测试 .go 都没有——哨兵宿主表建在空集上，(b) 的可达半边就变成永远通过")
	}
	return out
}

// errorsNewLiteral 认 `errors.New("字面量")` 与 `fmt.Errorf("字面量")`（两者都被 errors.Is 直接比文本，
// 而本仓 91 处哨兵里两种写法都有），含被括号包住的形态。**带格式参数的 fmt.Errorf 不当宿主**：
// 拼出来的句子没有稳定身份，正是判据 (b) 要挡的形状。
func errorsNewLiteral(e ast.Expr) (string, bool) {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = p.X
	}
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" && sel.Sel.Name != "Errorf" {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "errors" && pkg.Name != "fmt" {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// checkSentinelFaces 验「登记的明文面载体确实接在 api 层的错误面上」（判据 (b) 的可达半边）。
//
// `hosts` 与 `used` 都作参数进来而不是在函数里现查：这两条规则今天没有第二行表能触发它们
// （表里只有一个事实），所以必须能拿夹具打红，否则它们与 checkFactTable 那四条一样是空架子。
func checkSentinelFaces(reg []FactSpec, hosts map[string]string, used func(string) bool) []string {
	var out []string
	for _, spec := range reg {
		for _, errv := range spec.Sentinels {
			if errv == nil {
				continue
			}
			name, ok := hosts[errv.Error()]
			if !ok {
				out = append(out, "事实 "+spec.Key+" 登记的句子 "+errv.Error()+
					" 在 internal/service 里找不到 `errors.New(\"…\")` / `fmt.Errorf(\"…\")` 的宿主标识符："+
					"明文面载体必须是一枚具名哨兵——藏在拼装出来的 error 里既没法被 errors.Is 分档，"+
					"也就谈不上「这一句是这件事实的身份」")
				continue
			}
			if !used(name) {
				out = append(out, "事实 "+spec.Key+" 的明文面载体 "+name+
					" 没有被接进 internal/api 任何端点的错误面（既不出现在某处 errors.Is 的实参里，"+
					"也不出现在某张 errStatusTable / errStatusEntry 里）：这句话发不出去，"+
					"登记它作「明文面载体」是虚账。要么把它接进某个端点的错误面，"+
					"要么它本来就不是这件事实的明文载体")
			}
		}
	}
	return out
}

// apiErrorFaceRefs 收集「被接进 api 层错误面」的标识符。两种接法都算，因为它们就是本仓错误面
// 仅有的两种写法：
//
//	`errors.Is(err, service.ErrX)`          —— 手写分支（raw handler 那一侧）
//	`errStatusTable{… {sentinel: service.ErrX, …}}` —— 声明式档位表（endpoint.go 那一侧）
//
// 判据问的是「这枚哨兵有没有被接到某个端点的错误出口」，不是「api 层有没有提到它」——第一版
// 按「提到」做，连踩两个假绿：① 注释里写了哨兵名；② **登记表自己** `Sentinels: []error{…}`
// 就是全仓对它的第 2 处引用，于是每登记一枚哨兵都自动满足自己的可达性判据。一条会自己点亮自己的
// 可达性判据比没有更坏：它把「登记」冒充成「接线」。
func apiErrorFaceRefs(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读 internal/api 失败: %v", err)
	}
	out := map[string]bool{}
	fset := token.NewFileSet()
	n := 0
	add := func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := x.X.(*ast.Ident); ok {
				out[pkg.Name+"."+x.Sel.Name] = true
				out[x.Sel.Name] = true // 同包裸名形态
			}
		case *ast.Ident:
			out[x.Name] = true
		}
	}
	for _, e := range entries {
		name := e.Name()
		// 登记表自己**不算**引用方：它写 `Sentinels: []error{service.ErrX}`，那是登记而不是接线，
		// 若把它算进来，每登记一枚哨兵都会自动满足自己的可达性判据（红证时实测到的假绿）。
		// 一条会自己点亮自己的可达性判据比没有更坏：它把「登记」冒充成「接线」。
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			name == "consumption_fact_registry.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", name, err)
		}
		n++
		ast.Inspect(f, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.CallExpr:
				// errors.Is 的**任一**实参都算接线：判据方向（Is(err, 哨兵)）与反写都不该成为漏口。
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Is" {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "errors" {
						for _, arg := range x.Args {
							add(arg)
						}
					}
				}
			case *ast.CompositeLit:
				// 整张表的**子树**都收下。第一版只遍历 `x.Elts`，实测数出 17 个标识符而按字面量
				// 数是 100+：哨兵实际写在 `entries: []errStatusEntry{{sentinel: service.ErrX, …}}`
				// 里，`x.Elts` 拿到的那个 KeyValueExpr 被 add 直接丢掉了。一条认法太窄的可达性判据
				// 会「报红得多」，看着像收紧，其实是把没接线的和接线了的混在一起靠运气分。
				if id, ok := x.Type.(*ast.Ident); ok && (id.Name == "errStatusTable" || id.Name == "errStatusEntry") {
					ast.Inspect(x, func(n ast.Node) bool {
						if sel, ok := n.(*ast.SelectorExpr); ok {
							add(sel)
						}
						return true
					})
				}
			}
			return true
		})
	}
	if n == 0 {
		t.Fatal("internal/api 里没有非测试 .go——错误面引用集建在空集上，(b) 的可达半边只会一条条报红，" +
			"看着像收紧其实是锁坏了")
	}
	if len(out) < 50 {
		t.Fatalf("整层只数出 %d 个接进错误面的标识符——两种接法的认法被改坏了？"+
			"（实测基线 100+：errStatusTable 子树里的选择器 99 种 + errors.Is 实参 5 种）", len(out))
	}
	return out
}

// identifierWiredIntoErrorFace 问「这枚哨兵是不是某个端点错误面的一部分」。
func identifierWiredIntoErrorFace(refs map[string]bool, qualified string) bool {
	if refs[qualified] {
		return true
	}
	i := strings.LastIndex(qualified, ".")
	return i >= 0 && refs[qualified[i+1:]]
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

// TestSentinelFaceRulesFire 给「明文面可达」那两条配正向例：它们在今天那张一行的表上
// 都不会触发（唯一的载体既查得到宿主、也被 contact.go 引用着），不打夹具就是两条空架子。
func TestSentinelFaceRulesFire(t *testing.T) {
	hosts := map[string]string{"句子甲": "service.ErrFaceA", "句子乙": "service.ErrFaceB"}
	used := func(q string) bool { return q == "service.ErrFaceA" }
	reg := []FactSpec{
		{Key: "no_host", Sentinels: []error{errFact("句子丙")}, Projections: []string{"service.A.k"}},
		{Key: "not_wired", Sentinels: []error{errFact("句子乙")}, Projections: []string{"service.A.k"}},
		{Key: "ok", Sentinels: []error{errFact("句子甲")}, Projections: []string{"service.A.k"}},
		{Key: "nil_carrier", Sentinels: []error{nil}, Projections: []string{"service.A.k"}},
	}
	got := checkSentinelFaces(reg, hosts, used)
	if len(got) != 2 {
		t.Fatalf("应恰好两条违规（找不到宿主 / 未被 api 引用），实际 %d 条：%v", len(got), got)
	}
	if !strings.Contains(got[0], "找不到") || !strings.Contains(got[1], "没有被接进") {
		t.Fatalf("报的不是那两件事：%v", got)
	}
	// 反向半边：合规的一行与「载体是 nil」（由 checkFactTable 管，这里不重复报）都不得被报红。
	if n := len(checkSentinelFaces([]FactSpec{reg[2], reg[3]}, hosts, used)); n != 0 {
		t.Fatalf("合规的行被报红了：%v", checkSentinelFaces([]FactSpec{reg[2], reg[3]}, hosts, used))
	}
}

// errFact 是夹具用的具名错误：只为一句话，不需要 errors.New 的那层包装。
type errFact string

func (e errFact) Error() string { return string(e) }
