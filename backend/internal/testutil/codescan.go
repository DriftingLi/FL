package testutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// BackendCodeRoots 是静态锁的扫描根（**模块根相对**）。拆包不会改这里：新包在根之下自动进射程 ——
// 这正是这些锁要的「射程清单只留一个真源」，不必再逐处抄目录。
var BackendCodeRoots = []string{"internal", "pkg"}

// CodeFile 一个待静态扫描的后端源文件。Path 是**模块根相对**的斜杠路径（报告与白名单都拿它作键），
// 于是报告不随「测试文件住在哪一层」漂。
type CodeFile struct {
	Path string // 例：internal/forum/service.go
	Dir  string // 例：internal/core
	Name string // 例：forum_service.go
	Pkg  string // package 子句
	Src  string
	Test bool // 文件名以 _test.go 结尾
}

// ModuleRoot 返回 backend 模块根：按本文件位置现算，不受调用方所在目录影响。
func ModuleRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller 失败：定位不到模块根")
	}
	// 本文件在 <root>/internal/testutil/codescan.go
	return filepath.Dir(filepath.Dir(filepath.Dir(self)))
}

// SelfDir 返回**调用方**源文件所在目录（模块根相对斜杠路径）。
//
// 用途：射程写「生产代码全域 + 本包测试」的锁需要知道「本包」是哪个目录。硬编码目录会在拆包时
// 失效，现算则跟着文件一起走。
func SelfDir(t *testing.T) string {
	t.Helper()
	root := ModuleRoot(t)
	_, caller, _, ok := runtime.Caller(1)
	if !ok {
		t.Fatal("runtime.Caller 失败：定位不到调用方文件")
	}
	rel, err := filepath.Rel(root, filepath.Dir(caller))
	if err != nil {
		t.Fatalf("相对模块根定位调用方目录失败: %v", err)
	}
	return filepath.ToSlash(rel)
}

// ScanBackendCode 递归读入 BackendCodeRoots 下的全部 .go（含测试）。
//
// 防空转是硬要求：**一个文件都没扫到即 Fatal**。「读不到」与「无违规」在测试里长得一模一样，
// 而静态锁最容易静默失效的正是这一格（第十五波四条「绿色不等于跑了」之一）。
// 论域内的防空转下界（「扫到 N 个以下即 Fatal」）由调用方按自己的射程再加。
func ScanBackendCode(t *testing.T) []CodeFile {
	t.Helper()
	root := ModuleRoot(t)
	var files []CodeFile
	for _, r := range BackendCodeRoots {
		base := filepath.Join(root, filepath.FromSlash(r))
		err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") {
				return nil
			}
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				return relErr
			}
			src, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			f := CodeFile{
				Path: filepath.ToSlash(rel),
				Dir:  filepath.ToSlash(filepath.Dir(rel)),
				Name: d.Name(),
				Src:  string(src),
				Test: strings.HasSuffix(d.Name(), "_test.go"),
			}
			f.Pkg = packageClause(f.Src)
			files = append(files, f)
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败: %v", r, err)
		}
	}
	if len(files) == 0 {
		t.Fatalf("扫描面为空：%v 下一个 .go 都没读到 ⇒ 这不是「无违规」，是判据失效", BackendCodeRoots)
	}
	return files
}

// FindCode 按模块根相对路径取一个文件；找不到即 Fatal —— 判据宿主文件搬家时，必须回到锁里改路径，
// 而不是让锁静默指向零个文件。
func FindCode(t *testing.T, files []CodeFile, path string) CodeFile {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f
		}
	}
	t.Fatalf("扫描面里没有 %s（判据宿主文件搬走了？请改这里的路径）", path)
	return CodeFile{}
}

// Production 保留非测试文件。
func Production(f CodeFile) bool { return !f.Test }

// ProductionOrSelfTests 保留「生产代码全域」与「本包测试」（selfDir 由 SelfDir(t) 现算）。
//
// 为什么不收「全部文件」：这些锁的原语义是「本包（含其测试）不许出现某形态」。宽化后若把别的包的
// 测试夹具也收进来，会把 HTTP 查询串、契约测试的 db.Where 这类**不在论域内**的文本判红
// （实测：证件分区谓词在 internal/ 全域有 33 处命中，全部落在 internal/api 的契约夹具里）。
func ProductionOrSelfTests(f CodeFile, selfDir string) bool {
	return !f.Test || f.Dir == selfDir
}

// HTTPSurface 报告一个非测试文件是否属于 HTTP 面（端点声明 / 渲染 / 路由所在那一层）。
//
// 两种命中方式：
//   - 整目录：internal/api 与 internal/valuation/handler（残值模块自带 handler 与路由，同样受
//     「端点守卫只用能力常量」「端点不得持 *gorm.DB」「端点不得构造站内信」这些判据管）；
//   - 文件名前缀 `handler`：域包（internal/<域>，ADR-0070）把 HTTP 出口与域实现放进同一个包，
//     按目录圈射程就不成立了，约定改由**文件名**承载 —— 域包里 handler*.go 是 HTTP 面，
//     service.go / dto.go / errors.go 是域实现。这样 P2 逐域迁移时这条射程零清单维护。
//     先例：scripts/check-catalog-sort.mjs 的 GUARDED_PATH_SEGMENT 同样按文件名判定。
//
// 各锁一律调它，别再各写一份目录判断 —— 那是「射程清单改回双份」的老病。
func HTTPSurface(f CodeFile) bool {
	if f.Test {
		return false
	}
	return f.Dir == "internal/api" || f.Dir == "internal/valuation/handler" || strings.HasPrefix(f.Name, "handler")
}

// ScanDir 一个「对外契约类型可能住的包」：模块根相对路径 + 该目录里源文件应有的 package 名。
type ScanDir struct {
	Dir string
	Pkg string
}

// responseStaticPackages 是「响应面」里**不由域声明表派生**的那一半（模块根相对）。
//
// 三类：装配根与共享层（internal/api、internal/core、internal/model、pkg/response）、残值模块
// 按技术分层留下的两个包（valuation/model 与 valuation/repository —— 它们的定义键前缀是
// `model.` / `repository.`，不叫 `valuation.`），以及 audit —— 它虽已出包（P2 波 4f），却
// **不在域声明表**里（审计面没有 Web 消费方、不进 codegen），派生器看不到它，只能在这里登记。
var responseStaticPackages = []ScanDir{
	{"internal/api", "api"},
	{"internal/core", "core"},
	{"internal/model", "model"},
	// valuation/model 的 Go 包名也叫 model，swagger 定义键同样落在 `model.` 前缀下（两边类型名
	// 不重叠，swag 自己在重名时会报），所以 Pkg 列必须同为 "model" 才对得上生成物。
	{"internal/valuation/model", "model"},
	{"internal/valuation/repository", "repository"},
	{"internal/audit", "audit"},
	{"pkg/response", "response"},
}

// ResponsePackages 返回「响应 DTO / 对外契约类型可能住的包」（模块根相对）。
//
// 这是两把 fact 锁共用的**目录宇宙**：api 侧的 tag 扫描面与 apitypes 侧的可达性射程论域不同
// （过滤条件不同、另有一把表态锁射程更窄），但「哪些包可能承载响应类型」是同一件事 ——
// 各抄一份的结果是搬一次包要改两处，且两处会漂。
//
// #1445 P3-B 起，「域包」那一半**现读域声明表**（internal/apitypes/domains.go）而不是手抄：
// 域名的唯一出处本来就是声明表（ADR-0070 决策 3），手抄那份的代价是每加一个域要改三处
// （本清单 + 两把 fact 锁的计数）。现在新增域只动声明表；两把锁的 `n != 35` 会照常变红，那是
// **有意的**：射程变大要有人签字，不是漏扫。
func ResponsePackages(t *testing.T) []ScanDir {
	t.Helper()
	domains := domainPackages(t)
	out := make([]ScanDir, 0, len(responseStaticPackages)+len(domains))
	out = append(out, responseStaticPackages...)
	for _, d := range domains {
		for _, s := range responseStaticPackages {
			if s.Dir == d.Dir {
				t.Fatalf("域 %q 的目录 %s 已能从声明表现算，却又列在 responseStaticPackages 里 —— "+
					"同一目录只留一个真源，把静态那一枚删掉。", d.Pkg, d.Dir)
			}
		}
		out = append(out, d)
	}
	if len(out) < 30 {
		t.Fatalf("响应面包目录只派生出 %d 个（下界 30）—— 射程塌了不等于无违规", len(out))
	}
	return out
}

// domainPackages 从域声明表现算域包目录（模块根相对，顺序 = 声明表顺序）。
//
// 为什么现读而不是抄一份：域名的唯一出处是 internal/apitypes/domains.go（ADR-0070 决策 3）。
// 为什么用 go/parser 而不是 import：apitypes 的包内测试 import testutil，testutil 再 import
// apitypes 就是 Go 的 `import cycle not allowed in test`。
// 声明了却没有 internal/<域> 目录的域（credential 无目录、valuation 只有子包）自动略过 —— 它们
// 由 responseStaticPackages 兜住；目录在而包名不符则判红（那是声明表与目录漂了，不是「还没搬」）。
func domainPackages(t *testing.T) []ScanDir {
	t.Helper()
	root := ModuleRoot(t)
	srcPath := filepath.Join(root, "internal", "apitypes", "domains.go")
	file, err := parser.ParseFile(token.NewFileSet(), srcPath, nil, 0)
	if err != nil {
		t.Fatalf("解析域声明表 %s 失败: %v", srcPath, err)
	}
	names := domainNames(file)
	if len(names) < 25 {
		t.Fatalf("域声明表只解析出 %d 个域名（下界 25）⇒ 现读器失效，不是「域变少了」", len(names))
	}
	out := make([]ScanDir, 0, len(names))
	for _, name := range names {
		pkgName := strings.ToLower(name)
		dir := "internal/" + pkgName
		if !hasPackage(t, dir, filepath.Join(root, filepath.FromSlash(dir)), pkgName) {
			continue
		}
		out = append(out, ScanDir{Dir: dir, Pkg: pkgName})
	}
	if len(out) < 25 {
		t.Fatalf("域声明表有 %d 个域名，却只落到 %d 个域包目录（下界 25）—— 目录集体搬家了？", len(names), len(out))
	}
	return out
}

// domainNames 取 `var Domains = []Domain{…}` 里每条的 Name 字面量。
func domainNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "Domains" || len(vs.Values) != 1 {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range lit.Elts {
				cl, ok := elt.(*ast.CompositeLit)
				if !ok {
					continue
				}
				for _, field := range cl.Elts {
					kv, ok := field.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					val, ok2 := kv.Value.(*ast.BasicLit)
					if !ok || key.Name != "Name" || !ok2 || val.Kind != token.STRING {
						continue
					}
					s, err := strconv.Unquote(val.Value)
					if err == nil && s != "" {
						names = append(names, s)
					}
				}
			}
		}
	}
	return names
}

// hasPackage 报告 dir 目录里是否至少有一个「非测试 .go 且 package 子句 = pkgName」的文件。
func hasPackage(t *testing.T, dir, abs, pkgName string) bool {
	t.Helper()
	if _, err := os.Stat(abs); err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("定位域目录 %s 失败: %v", dir, err)
		}
		return false
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		t.Fatalf("读域目录 %s 失败: %v", dir, err)
	}
	anyGo := false
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		anyGo = true
		src, readErr := os.ReadFile(filepath.Join(abs, e.Name()))
		if readErr != nil {
			t.Fatalf("读 %s/%s 失败: %v", dir, e.Name(), readErr)
		}
		if packageClause(string(src)) == pkgName {
			return true
		}
	}
	if anyGo {
		t.Fatalf("域目录 %s 里有 .go 却无一 package %s —— 域声明表与目录漂了", dir, pkgName)
	}
	return false
}

func packageClause(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package "))
		}
	}
	return ""
}
