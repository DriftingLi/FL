// Package layers 只有一层判据：把「谁可以依赖谁」与「测试与实现同居」写成可执行的规矩。
//
// 为什么单独成包：这两条判据的对象是**整棵后端源码树**，不属于任何一层 —— 挂在某一层里，
// 「谁守着这条规矩」就变得要看目录才猜得到。
//
// 只收**今天还不成立、且拆包后仍需成立**的那几条；已有守卫管着的不重复登记（authz 的向上依赖锁
// 在 internal/api/authz_lock_test.go、api 不得持 *gorm.DB 在 internal/api/gorm_seam_guard_test.go）
// —— 判据改回双份正是本仓反复点名要避开的老病。
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

// importEdges 解析出全部包级 import 边（只收 forklift-training/ 前缀的自有包）。
func importEdges(files []SourceFile) ([]Edge, error) {
	fset := token.NewFileSet()
	var out []Edge
	for _, f := range files {
		parsed, err := parser.ParseFile(fset, f.Path, f.Src, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, imp := range parsed.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil || !strings.HasPrefix(p, "forklift-training/") {
				continue
			}
			out = append(out, Edge{From: f.Dir, To: strings.TrimPrefix(p, "forklift-training/")})
		}
	}
	return out, nil
}

// directionViolations 单向依赖的两条硬规矩：
//
//  1. `pkg/httpx` 是 HTTP 骨架叶子：**不得** import 任何 `internal/...`。域包与 internal/api 都要
//     import 它 —— 它反向依赖内部层就把叶子变成中继站，拆包时立刻成环。
//  2. `internal/api` 是装配面：只允许 `cmd/...`（装配根）依赖它。域包与 service 一旦 import 它，
//     「handler → service」的单向就断了；而这条断法在编译器那里不报错（只要不成环），只能靠判据。
func directionViolations(edges []Edge) []string {
	var out []string
	for _, e := range edges {
		if e.From == "pkg/httpx" && strings.HasPrefix(e.To, "internal/") {
			out = append(out, e.From+" 不得 import "+e.To+"（HTTP 骨架必须是最底层叶子）")
		}
		if e.To == "internal/api" && !strings.HasPrefix(e.From, "cmd/") {
			out = append(out, e.From+" 不得 import internal/api（装配面只允许 cmd/ 依赖）")
		}
	}
	return out
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
