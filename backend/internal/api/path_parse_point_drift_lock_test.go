// ADR-0065 批⑤ 的第二把锁：路径整型 id 的解析点必须只有一个。
//
// 为什么需要它：`pathInt` 补齐 `<= 0` 只修好了「走 helper 的那批」端点；同一件事实此前还在
// 36 处裸 `strconv.Atoi(c.Param(...))` 里各写一遍（`question_bank.go` 6 处、`training_catalog.go` 12 处、
// `question_interaction.go` 7 处直接丢弃 err ⇒ 非数字 id 被当成 0 去查库，对外答 **200 + 空列表**），
// 另有 1 处把路径参数喂给查询参数侧的 `requiredPositiveID`（`real_exam.go` 的 parsePaperAction）。
// 本批把它们全部收进 pathInt/pathInt64；这道锁负责让「第三处实现」再也长不出来。
//
// 扫描器走 AST 而不是文本匹配，理由与第④批的泄漏断言（决策 10）同一条：
// `idStr := c.Param("id")` 存进变量再转换的写法（`contact.go`/`recruit.go`/`resume_pdf.go` 三处就是这样）
// 用字面量正则扫不出来，只按字面量锁会得到一次假绿。
//
// 判据刻意**偏严**：变量名在整个文件范围内共享（同文件另一个函数里的同名变量也会被判违规）。
// 宁可误红也不能漏 —— 误红看得见，漏了就是假绿。
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

// 允许的两处实现（都在 endpoint.go）。加第三个名字等于把这件事重新分散开。
var allowedPathParseFuncs = map[string]bool{"pathInt": true, "pathInt64": true}

// 整数转换的本名 + 查询参数侧的单点守卫（后者被用在路径参数上就是第二份实现）。
var intParseFuncs = map[string]bool{
	"Atoi": true, "ParseInt": true, "ParseUint": true,
}
var queryParseHelpers = map[string]bool{
	"requiredPositiveID": true, "queryIDPtr": true, "atoiDefault": true, "queryIntPtr": true,
}

// pathParseSite 一次违规：从路径参数取整数，却没走 pathInt/pathInt64。
type pathParseSite struct {
	file string
	line int
	fn   string // 被调用的转换函数名
	via  string // inline（直接套 c.Param）| var（经由中间变量）
}

func (s pathParseSite) String() string {
	return fmt.Sprintf("%s:%d 用 %s(%s) 解析路径参数", s.file, s.line, s.fn, s.via)
}

// isParamCall 认 `X.Param("k")`（gin 的路径参数读取）。
func isParamCall(e ast.Expr) bool {
	ce, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	se, ok := ce.Fun.(*ast.SelectorExpr)
	return ok && se.Sel.Name == "Param"
}

// callName 返回被调用者的短名（strconv.Atoi → "Atoi"；queryIDPtr → "queryIDPtr"）。
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

// findPathParseSites 在一个已解析的文件里找「把路径参数转成整数」的自定义实现。
// 跳过 pathInt/pathInt64 自己的函数体（那是允许的那一处）。
func findPathParseSites(fset *token.FileSet, file *ast.File) []pathParseSite {
	var out []pathParseSite
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || allowedPathParseFuncs[fd.Name.Name] {
			continue
		}
		if fd.Body == nil {
			continue
		}
		// 第一遍：哪些变量是被 c.Param(...) 喂进来的。
		paramVars := map[string]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || !isParamCall(as.Rhs[0]) {
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
			isStrconv := pkg == "strconv" && intParseFuncs[name]
			isQueryHelper := pkg == "" && queryParseHelpers[name]
			if !isStrconv && !isQueryHelper {
				return true
			}
			via := ""
			for _, a := range ce.Args {
				if isParamCall(a) {
					via = "inline"
				} else if id, ok := a.(*ast.Ident); ok && paramVars[id.Name] {
					via = "var"
				}
			}
			if via != "" {
				out = append(out, pathParseSite{
					line: fset.Position(ce.Pos()).Line, fn: name, via: via,
				})
			}
			return true
		})
	}
	return out
}

// scanPackagePathParseSites 扫一个目录下的非测试 Go 文件；返回违规点与扫过的文件数。
func scanPackagePathParseSites(t *testing.T, dir string) (sites []pathParseSite, files int) {
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

// TestPathIntHasASingleParsePoint 锁本体：路径整型 id 只能由 pathInt/pathInt64 解析。
func TestPathIntHasASingleParsePoint(t *testing.T) {
	sites, files := scanPackagePathParseSites(t, ".")

	// 空转判据（第十五波教训：找不到输入的护栏要 Fatal，不许安静通过）。
	if files < 30 {
		t.Fatalf("只扫到 %d 个源文件，说明目录或判据变了（本包实际有 50+ 个）⇒ 这条锁现在不构成保护", files)
	}
	if n := countPathHelpers(t, "."); n != len(allowedPathParseFuncs) {
		t.Fatalf("本包里名为 pathInt* 的 helper 应有 %d 个（pathInt/pathInt64），实得 %d ⇒ 有人加了第三枚"+
			"或改了名：同一件事实一旦有两个同名族出口，改一处忘另一处就回来了",
			len(allowedPathParseFuncs), n)
	}

	if len(sites) > 0 {
		t.Errorf("发现 %d 处自定义的路径整数解析点（ADR-0065 批⑤ 已清零，只能保持 0）：\n  %s\n"+
			"⇒ 同一件「路径 id 不是正整数」的事实有了第二份实现：它少挡一档、换一套文案，"+
			"且不会随 pathInt 一起改。正解是调 pathInt/pathInt64 并把本端点那句文案传进去。",
			len(sites), joinSites(sites))
	}
	t.Logf("扫描面：%d 个源文件，自定义解析点 %d 处", files, len(sites))
}

// TestPathParseDetectorFiresOnPlantedSources 自证扫描器不是空转：
// 三种坏形状必须各自被抓到，一种好形状必须不被抓到。
func TestPathParseDetectorFiresOnPlantedSources(t *testing.T) {
	src := `package probe

import "github.com/gin-gonic/gin"

func badInline(c *gin.Context) int {
	v, _ := strconv.Atoi(c.Param("id"))
	return v
}

func badViaVar(c *gin.Context) int {
	idStr := c.Param("id")
	v, _ := strconv.Atoi(idStr)
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
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "probe.go", src, 0)
	if err != nil {
		t.Fatalf("夹具解析失败：%v", err)
	}
	got := findPathParseSites(fset, parsed)
	want := map[string]int{
		"Atoi/inline":               1, // v, _ := strconv.Atoi(c.Param("id"))
		"Atoi/var":                  1, // idStr := c.Param("id"); strconv.Atoi(idStr)
		"requiredPositiveID/inline": 1, // 路径参数喂给查询侧 helper
	}
	gotKeys := map[string]int{}
	for _, s := range got {
		gotKeys[s.fn+"/"+s.via]++
	}
	if len(got) != 3 || !sameCounts(want, gotKeys) {
		t.Fatalf("夹具应恰好抓到 3 处并按形状归因：\n  want %v\n  got  %v（%s）\n"+
			"⇒ 扫描器与坏形状失联，生产代码那条判据就是假绿", want, gotKeys, joinSites(got))
	}
}

// countPathHelpers 数整个包里名为 pathInt* 的函数定义（不限文件——第三枚可能藏在别处）。
// 它与上面的扫描互补：扫描按「实现形状」抓，这里按「名字族」抓，二者任一条红都算拦住。
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

// sameCounts 比较两张「形状 → 次数」表（夹具断言用，避免依赖顺序）。
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
