package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// BackendCodeRoots 是静态锁的扫描根（**模块根相对**）。拆包不会改这里：新包在根之下自动进射程 ——
// 这正是这些锁要的「射程清单只留一个真源」，不必再逐处抄目录。
var BackendCodeRoots = []string{"internal", "pkg"}

// CodeFile 一个待静态扫描的后端源文件。Path 是**模块根相对**的斜杠路径（报告与白名单都拿它作键），
// 于是报告不随「测试文件住在哪一层」漂。
type CodeFile struct {
	Path string // 例：internal/service/forum_service.go
	Dir  string // 例：internal/service
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
// 今天 = internal/api 下的非测试源文件。拆包之后（P2）只改这**一处**：届时 HTTP 面 = 各域包里
// 承载端点与路由的那些文件。各锁一律调它，别再各写一份目录判断 —— 那是「射程清单改回双份」的老病。
func HTTPSurface(f CodeFile) bool {
	return !f.Test && f.Dir == "internal/api"
}

// packageClause 取源文件的 package 名（取不到返回空串，由调用方决定算不算失败）。
func packageClause(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "package "))
		}
	}
	return ""
}
