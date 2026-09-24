// ADR-0065 批⑤ 的第二把锁：路径整型 id 的解析点必须只有一个。
//
// 为什么需要它：`pathInt` 补齐 `<= 0` 只修好了「走 helper 的那批」端点。同一件事实此前还在
// 37 处自定义实现里各成立一次（36 处裸 `strconv.(Atoi|ParseInt)` 读 `c.Param`，分布在 10 个文件；
// 另 1 处把**路径**参数喂给查询侧守卫 `requiredPositiveID`，`real_exam.go` 的 parsePaperAction）。
// 本批把 `internal/api` 里的这些全部收进 pathInt/pathInt64；这道锁负责让「第二处实现」长不回来。
//
// 扫描器走 AST 而不是文本匹配，理由与第④批的泄漏断言（ADR-0064 决策 10）同一条：
// `idStr := c.Param("id")` 存进变量再转换的写法（`contact.go`/`recruit.go`/`resume_pdf.go` 三处就是这样）
// 用字面量正则扫不出来 —— 只按字面量锁会得到一次假绿。
//
// 判定范围与**已知盲区**都写在下面（不写成「凡是…即红」）：本锁按「单个函数体内」收集
// 由 `c.Param` 喂进来的变量，因此**跨函数中转**（`raw := rawID(c)` 之后再 `strconv.Atoi(raw)`）
// 不在射程内。这条缺口由 TestPathParseDetectorKnownBlindSpot 钉成显式声明 —— 哪天有人把
// 检测器做深，那条夹具会红，就会被迫回来改这里的措辞，而不是留一句读起来比实际更硬的注释。
package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// allowedPathParseFuncs 是本包允许的两处实现（都在 endpoint.go）。加第三个名字等于把这件事重新分散。
var allowedPathParseFuncs = map[string]bool{"pathInt": true, "pathInt64": true}

// intParseFuncs 是整数转换的本名；queryParseHelpers 是**查询参数**侧的单点守卫——
// 它们被用在路径参数上就是第二份实现（`real_exam.go` 曾经那样）。
var (
	intParseFuncs = map[string]bool{"Atoi": true, "ParseInt": true, "ParseUint": true}
	queryHelpers  = map[string]bool{"requiredPositiveID": true, "queryIDPtr": true, "atoiDefault": true, "queryIntPtr": true}
)

// pathParseSite 一次「从路径参数取整数却没走 pathInt/pathInt64」。
type pathParseSite struct {
	file string
	line int
	fn   string
	via  string // inline（直接套 c.Param）| var（经由同函数内的中间变量）
}

func (s pathParseSite) String() string {
	return fmt.Sprintf("%s:%d %s(%s)", s.file, s.line, s.fn, s.via)
}

func isParamCall(e ast.Expr) bool {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	se, ok := ce.Fun.(*ast.SelectorExpr)
	return ok && se.Sel.Name == "Param"
}

// callName 返回被调用者的限定名与短名（strconv.Atoi → ("strconv","Atoi")；queryIDPtr → ("","queryIDPtr")）。
func callName(ce *ast.CallExpr) (pkg, name string) {
	switch fn := ce.Fun.(type) {
	case *ast.SelectorExpr:
		p, ok := fn.X.(*ast.Ident)
		if !ok {
			return "", ""
		}
		return p.Name, fn.Sel.Name
	case *ast.Ident:
		return "", fn.Name
	}
	return "", ""
}

// strconvAlias 解析本文件里 strconv 的导入名（`import s "strconv"` 也要被抓到，
// 否则换个别名就是第二个漏口 —— 与「只认一种漏法的锁等于没有锁」同条判据）。
func strconvAlias(file *ast.File) string {
	for _, im := range file.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		if path != "strconv" {
			continue
		}
		if im.Name != nil {
			return im.Name.Name
		}
		return "strconv"
	}
	return ""
}

// findPathParseSites 扫一个文件里自定义的路径整数解析点；跳过 pathInt/pathInt64 自己的函数体。
func findPathParseSites(fset *token.FileSet, file *ast.File) []pathParseSite {
	strconvName := strconvAlias(file)
	var out []pathParseSite
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil || allowedPathParseFuncs[fd.Name.Name] {
			continue
		}
		// 第一遍：本函数内哪些变量是被 c.Param(...) 喂进来的（**仅同函数内**，见文件头盲区声明）。
		paramVars := map[string]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Rhs) == 0 || !isParamCall(as.Rhs[0]) {
				return true
			}
			for _, lhs := range as.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
					paramVars[id.Name] = true
				}
			}
			return true
		})
		// 第二遍：谁拿路径参数（或那些变量）去做整数转换。
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			ce, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pkg, name := callName(ce)
			isStrconv := pkg == strconvName && intParseFuncs[name]
			isQueryHelper := pkg == "" && queryHelpers[name]
			if !isStrconv && !isQueryHelper {
				return true
			}
			via := ""
			for _, a := range ce.Args {
				if isParamCall(a) {
					via = "inline"
				} else if id, ok := a.(*ast.Ident); ok && paramVars[id.Name] && via == "" {
					via = "var"
				}
			}
			if via != "" {
				out = append(out, pathParseSite{line: fset.Position(ce.Pos()).Line, fn: name, via: via})
			}
			return true
		})
	}
	return out
}

// scanDir 扫一个目录下的非测试 Go 文件，返回违规点与扫过的文件数。读不到目录/解析失败一律 Fatal：
// 定位不到输入时安静通过，就是把护栏换成一次自我确认（第十五波四条「绿色不等于跑了」之一）。
func scanDir(t *testing.T, dir string) (sites []pathParseSite, files int) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不到扫描目录 %s：%v ⇒ 扫描器无从验证，不许当成「无违规」", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", name, err)
		}
		files++
		for _, s := range findPathParseSites(fset, parsed) {
			s.file = name
			sites = append(sites, s)
		}
	}
	return sites, files
}

// countPathHelpers 数一个目录里名为 pathInt* 的函数定义（第三枚可能藏在别的文件）。
func countPathHelpers(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不到 %s：%v", dir, err)
	}
	fset := token.NewFileSet()
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败：%v", name, err)
		}
		for _, d := range parsed.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && strings.HasPrefix(fd.Name.Name, "pathInt") {
				n++
			}
		}
	}
	return n
}

// valuationPathParseDebt 是**尚未**收进 helper 的跨包债务实测数（internal/valuation/handler，
// 6 处 `strconv.ParseInt(c.Param("id"))`，同样只看 err ⇒ 0/负数继续往下走 repo）。
// 本批不同接：那 6 处要收，得先有一枚跨包共享的解析出口，而「现造一个没有先例可校的形状」
// 正是 ADR-0064 决策 7 否掉的做法。⇒ 记成只能减的债，登记在 ADR-0065 决策 1 末段，与 ② 的
// 跨包 seam 一起定形。这个数字变大即红；变小说明有人先收了，请把本行与 ADR 一起改小。
const valuationPathParseDebt = 6

// TestPathIntHasASingleParsePoint 锁本体：internal/api 内路径整型 id 只能由 pathInt/pathInt64 解析。
func TestPathIntHasASingleParsePoint(t *testing.T) {
	sites, files := scanDir(t, ".")

	// 空转判据：扫到的文件数远低于本包实际数量 ⇒ 目录或判据变了，不许当成「无违规」。
	if files < 30 {
		t.Fatalf("只扫到 %d 个源文件，说明目录或判据变了（本包实际有 50+ 个）⇒ 这条锁现在不构成保护", files)
	}
	if n := countPathHelpers(t, "."); n != len(allowedPathParseFuncs) {
		t.Fatalf("本包里名为 pathInt* 的 helper 应有 %d 个（pathInt/pathInt64），实得 %d ⇒ 有人加了第三枚"+
			"或改了名：同一件事实一旦有两个同名族出口，改一处忘另一处就回来了",
			len(allowedPathParseFuncs), n)
	}
	if len(sites) > 0 {
		t.Errorf("internal/api 里发现 %d 处自定义的路径整数解析点（ADR-0065 批⑤ 已清零，只能保持 0）：\n  %s\n"+
			"⇒ 同一件「路径 id 不是正整数」的事实有了第二份实现：它少挡一档、换一套文案，"+
			"且不会随 pathInt 一起改。正解是调 pathInt/pathInt64 并把本端点那句文案传进去。",
			len(sites), joinSites(sites))
	}

	vSites, vFiles := scanDir(t, "../valuation/handler")
	if vFiles == 0 {
		t.Fatalf("valuation/handler 一个源文件都没扫到 ⇒ 跨包债务这条断言是空的")
	}
	if len(vSites) > valuationPathParseDebt {
		t.Errorf("valuation/handler 的路径整数解析点从 %d 涨到 %d ⇒ 债务只能减。新增处请改走一枚共享出口，"+
			"而不是再加一份实现：\n  %s", valuationPathParseDebt, len(vSites), joinSites(vSites))
	}
	if len(vSites) < valuationPathParseDebt {
		t.Errorf("valuation/handler 实测只剩 %d 处（登记常量是 %d）⇒ 有人先收了这批债，"+
			"请同步把常量改小并在 ADR-0065 决策 1 末段记一笔", len(vSites), valuationPathParseDebt)
	}
	t.Logf("扫描面：internal/api %d 个文件 / 自定义解析点 %d 处；valuation/handler %d 个文件 / 债务 %d 处（登记 %d）",
		files, len(sites), vFiles, len(vSites), valuationPathParseDebt)
}

// TestPathParseDetectorFiresOnPlantedSources 自证扫描器不是空转：三种坏形状各被抓到、
// 一种好形状不被抓到；且 strconv 是以**别名**导入的 —— 不解析导入别名的实现会当场少抓两处。
// 生产代码那条判据的可信度全部押在这上面。
func TestPathParseDetectorFiresOnPlantedSources(t *testing.T) {
	src := `package probe

import (
	st "strconv"

	"github.com/gin-gonic/gin"
)

func badInline(c *gin.Context) int {
	v, _ := st.Atoi(c.Param("id"))
	return v
}

func badViaVar(c *gin.Context) int {
	idStr := c.Param("id")
	v, _ := st.Atoi(idStr)
	return v
}

func badQueryHelper(c *gin.Context) int {
	v, _ := requiredPositiveID(c.Param("paper_id"))
	return v
}

func good(c *gin.Context) int {
	v, _ := pathInt(c, "id", "ID无效")
	return v
}
`
	got := parseAndScan(t, "probe.go", src)
	want := map[string]int{
		"Atoi/inline":               1, // 直接套 c.Param（且 strconv 走别名导入）
		"Atoi/var":                  1, // 经由同函数内的中间变量
		"requiredPositiveID/inline": 1, // 路径参数喂给查询侧 helper
	}
	keys := map[string]int{}
	for _, s := range got {
		keys[s.fn+"/"+s.via]++
	}
	if len(got) != 3 || !sameCounts(want, keys) {
		t.Fatalf("夹具应恰好抓到 3 处并按形状归因：\n  want %v\n  got  %v（%s）\n"+
			"⇒ 扫描器与坏形状失联，生产代码那条判据就是假绿", want, keys, joinSites(got))
	}
}

// TestPathParseDetectorKnownBlindSpot 把**盲区**也钉成断言：跨函数中转今天抓不到，
// 这里断言它确实抓不到。检测器被做深时这条会红，逼着改措辞与 ADR —— 而不是留一句
// 读起来比实际更硬的「凡是…即红」。
func TestPathParseDetectorKnownBlindSpot(t *testing.T) {
	src := `package probe

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func rawID(c *gin.Context) string { return c.Param("id") }

func twoHop(c *gin.Context) int {
	raw := rawID(c)
	v, _ := strconv.Atoi(raw)
	return v
}
`
	if got := parseAndScan(t, "blind.go", src); len(got) != 0 {
		t.Fatalf("跨函数中转现在**能**被抓到了（%s）⇒ 检测器变强了，请删掉这条盲区夹具，"+
			"并把文件头与 ADR-0065 里「仅同函数内」的措辞改掉", joinSites(got))
	}
}

func parseAndScan(t *testing.T, name, src string) []pathParseSite {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("夹具解析失败：%v", err)
	}
	return findPathParseSites(fset, parsed)
}

func sameCounts(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func joinSites(sites []pathParseSite) string {
	out := make([]string, 0, len(sites))
	for _, s := range sites {
		out = append(out, s.String())
	}
	return strings.Join(out, "\n  ")
}
