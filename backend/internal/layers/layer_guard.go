// Package layers 只有一层判据：把「谁可以依赖谁」与「测试与实现同居」写成可执行的规矩。
//
// 为什么单独成包：这两条判据的对象是**整棵后端源码树**，不属于任何一层 —— 挂在某一层里，
// 「谁守着这条规矩」就变得要看目录才猜得到。
//
// 只收**今天还不成立、且拆包后仍需成立**的那几条；已有守卫管着的不重复登记（authz 的向上依赖锁
// 在 internal/api/authz_lock_test.go、api 不得持 *gorm.DB 在 internal/api/gorm_seam_guard_test.go）
// —— 判据改回双份正是本仓反复点名要避开的老病。
//
// 三条规矩：① 单向依赖（pkg/httpx 是叶子、internal/api 只许 cmd/ 依赖、
// internal/middleware 不得 import internal/service）；
// ② 测试与实现同居；③ gin 只许出现在 HTTP 面与登记的 HTTP 基建里。
// 「哪些文件算 HTTP 面」这件事**不在本包定义**：判据由调用方以 isHTTP 注入（测试里传
// testutil.HTTPSurface —— 全仓唯一出处）。本包不 import testutil，否则 layers 的包内测试
// （它要 import testutil 拿扫描结果）会撞 Go 的 import cycle in test。
//
// 判据本身不依赖任何测试脚手架（只吃路径与源码），于是它既能在测试里跑，也不会把 gorm/sqlite
// 这类测试依赖拖进生产包。
package layers

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// SourceFile 一条判据输入：模块根相对路径、所在目录、文件名、源码、是否测试文件。
type SourceFile struct {
	Path string
	Dir  string
	Name string
	Src  string
	Test bool
}

// Edge 一条包级 import 边：From 依赖 To（模块根相对包路径）。
type Edge struct {
	From string
	To   string
}

// importPaths 解析出一份源码的包子句与全部 import 路径（含第三方）。
func importPaths(fset *token.FileSet, f SourceFile) (string, []string, error) {
	parsed, err := parser.ParseFile(fset, f.Path, f.Src, parser.ImportsOnly)
	if err != nil {
		return "", nil, err
	}
	out := make([]string, 0, len(parsed.Imports))
	for _, imp := range parsed.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		out = append(out, p)
	}
	return parsed.Name.Name, out, nil
}

// importEdges 解析出全部包级 import 边（只收 forklift-training/ 前缀的自有包）。
func importEdges(files []SourceFile) ([]Edge, error) {
	fset := token.NewFileSet()
	var out []Edge
	for _, f := range files {
		pkgName, paths, err := importPaths(fset, f)
		if err != nil {
			return nil, err
		}
		// 外部测试包（`package foo_test`）不进依赖图：它住在被测包之外，两条边都能拿，不构成生产图里的
		// 三角。middleware 的外部测试包正是靠这一点继续用**真实** audit.Service 落库举证
		// （见 internal/middleware/audit_ip_test.go 的文件头）。内部测试包（`package foo`）仍要判：
		// 它反向 import 会在 test 构建里成环——编译器会报，这里判是为了给出一条指名的错而不是指到无关的包。
		if strings.HasSuffix(pkgName, "_test") {
			continue
		}
		for _, p := range paths {
			if !strings.HasPrefix(p, "forklift-training/") {
				continue
			}
			out = append(out, Edge{From: f.Dir, To: strings.TrimPrefix(p, "forklift-training/")})
		}
	}
	return out, nil
}

// directionViolations 单向依赖的三条硬规矩：
//
//  1. `pkg/httpx` 是 HTTP 骨架叶子：**不得** import 任何 `internal/...`。域包与 internal/api 都要
//     import 它 —— 它反向依赖内部层就把叶子变成中继站，拆包时立刻成环。
//  2. `internal/api` 是装配面：只允许 `cmd/...`（装配根）依赖它。域包与 service 一旦 import 它，
//     「handler → service」的单向就断了；而这条断法在编译器那里不报错（只要不成环），只能靠判据。
//  3. `internal/middleware` 是 HTTP 基建，**不得** import `internal/service`：service 要 import 域包
//     （事件构造器），域包的 handler*.go 要 import middleware（JWTAuth / CapabilityRequired）
//     ——「middleware → service → 域包 → middleware」是个三角，这条边把三角闭合成 import cycle
//     （ADR-0070 之后第一次拆 notification 时实测撞上，且编译器把错报在无关的 cmd 上）。
//     审计写依赖用 AuditWriter 接口反转：接口声明在 middleware（消费方），实现仍是
//     audit.Service 单点，装配点注入前判 nil（typed nil 装进接口不等于 nil 接口）。
//
// 射程里的文件由 importEdges 决定（外部测试包不进图，理由见那里）。
func directionViolations(edges []Edge) []string {
	var out []string
	for _, e := range edges {
		if e.From == "pkg/httpx" && strings.HasPrefix(e.To, "internal/") {
			out = append(out, e.From+" 不得 import "+e.To+"（HTTP 骨架必须是最底层叶子）")
		}
		if e.To == "internal/api" && !strings.HasPrefix(e.From, "cmd/") {
			out = append(out, e.From+" 不得 import internal/api（装配面只允许 cmd/ 依赖）")
		}
		if e.From == "internal/middleware" && e.To == "internal/service" {
			out = append(out, e.From+" 不得 import "+e.To+
				"（三角：service → 域包 → middleware，中间件反向依赖业务层会成环；审计写走 AuditWriter 接口反转）")
		}
	}
	return out
}

// ginImportPath 是 HTTP 骨架依赖的 Web 框架包。
const ginImportPath = "github.com/gin-gonic/gin"

// ginHostAllowed 逐条登记的「不是 HTTP 面、但名正言顺 import gin」的包（目录 → 理由）。
//
// 表要**活的**：每个条目都必须真的扫到一个 gin 宿主，否则 TestGinHostWhitelistIsLive 报红
// （同 internal/api/gorm_seam_guard_test.go 的 TestGormDBWhitelistIsLive）。
// 除此之外任何地方 import gin 都是违规：域的 service.go 若开始直接读 *gin.Context，
// 「handler → service」的单向就断了，「换/升 Web 框架」也从改一处变成全仓改。
var ginHostAllowed = map[string]string{
	"internal/middleware": "gin 中间件基建（鉴权 / 能力位 / 限流）：HTTP 面的一部分，但不写端点",
	"internal/logger":     "请求日志与 panic 恢复中间件，同属 HTTP 基建",
	"pkg/httpx":           "HTTP 骨架本体（Endpoint / 渲染 / 解析出口），gin 的第一宿主",
	"pkg/response":        "响应信封渲染，与骨架同一层",
}

// ginImported 报告一份源码是否 import 了 gin（白名单活性自测也用它）。
func ginImported(fset *token.FileSet, f SourceFile) (bool, error) {
	_, paths, err := importPaths(fset, f)
	if err != nil {
		return false, err
	}
	for _, p := range paths {
		if p == ginImportPath {
			return true, nil
		}
	}
	return false, nil
}

// ginImportViolations 报告「非测试文件 import 了 gin，却不属于 HTTP 面」。
//
// isHTTP 由调用方注入（测试里传 testutil.HTTPSurface）：本包不重复定义「什么算 HTTP 面」——
// 判据改回双份正是本仓反复点名要避开的老病。今天 HTTP 面 = internal/api 整目录，
// 或域包里的 handler*.go（ADR-0070 的域包约定）。
func ginImportViolations(files []SourceFile, isHTTP func(SourceFile) bool) ([]string, error) {
	fset := token.NewFileSet()
	var out []string
	for _, f := range files {
		if f.Test || isHTTP(f) {
			continue
		}
		if _, ok := ginHostAllowed[f.Dir]; ok {
			continue
		}
		imported, err := ginImported(fset, f)
		if err != nil {
			return nil, err
		}
		if imported {
			out = append(out, f.Path+" 不是 HTTP 面却 import 了 gin：HTTP 出口只住在 internal/api "+
				"或域包的 handler*.go 里（ADR-0070）：域实现要的是解析好的入参，不是 *gin.Context")
		}
	}
	return out, nil
}

// colocationViolations 测试与实现同居：
//
//  1. 每个 `_test.go` 必须与同目录的非测试 `.go` 同住 —— Go 的包内测试就是「与实现同住」的形态；
//     测试被挪进独立目录时，它测的那份实现通常也一起被挪出包边界。
//  2. `internal/` 与 `pkg/` 下不得出现名为 test/tests 的目录（独立测试目录的入口）。
func colocationViolations(files []SourceFile) []string {
	var out []string
	hasProd := map[string]bool{}
	for _, f := range files {
		if !f.Test {
			hasProd[f.Dir] = true
		}
	}
	for _, f := range files {
		if f.Test && !hasProd[f.Dir] {
			out = append(out, f.Path+" 所在目录没有非测试源文件（测试与实现被拆开了）")
		}
		if strings.HasSuffix(f.Dir, "/test") || strings.HasSuffix(f.Dir, "/tests") {
			out = append(out, f.Path+" 住在独立测试目录里（本仓约定：测试与实现同居）")
		}
	}
	return out
}
