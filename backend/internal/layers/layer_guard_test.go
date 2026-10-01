package layers

import (
	"go/token"
	"strings"
	"testing"

	"forklift-training/internal/testutil"
)

// sourceFiles 把扫描结果转成判据输入（判据只吃路径与源码，不依赖测试脚手架）。
func sourceFiles(files []testutil.CodeFile) []SourceFile {
	out := make([]SourceFile, 0, len(files))
	for _, f := range files {
		out = append(out, SourceFile{Path: f.Path, Dir: f.Dir, Name: f.Name, Src: f.Src, Test: f.Test})
	}
	return out
}

// httpSurface 把「什么算 HTTP 面」接到 testutil 那唯一一处出处上（本包不重复定义，
// 否则判据又变双份；注入而不是 import，是为了避开 layers 包内测试的 import cycle）。
func httpSurface(f SourceFile) bool {
	return testutil.HTTPSurface(testutil.CodeFile{Path: f.Path, Dir: f.Dir, Name: f.Name, Src: f.Src, Test: f.Test})
}

// TestBackendDependencyDirection 单向依赖：今天实测 0 违例；判据的意义在拆包（P2）—— 那时新增的
// 每一条边都会被这两条规矩过一遍，而它们都不会让编译器报错。
func TestBackendDependencyDirection(t *testing.T) {
	t.Parallel()
	files := sourceFiles(testutil.ScanBackendCode(t))
	edges, err := importEdges(files)
	if err != nil {
		t.Fatalf("解析 import 边失败: %v", err)
	}
	if len(edges) < 500 {
		t.Fatalf("只解析出 %d 条自有包 import 边（实测基线 1000+）⇒ 判据射程塌了，这不是「无违规」", len(edges))
	}
	if v := directionViolations(edges); len(v) > 0 {
		t.Fatalf("分层方向被破坏：\n  %s", strings.Join(v, "\n  "))
	}
}

// TestBackendTestsLiveWithImplementation 测试同居：判据在「有人新建 tests/ 目录、或把测试挪出包」
// 时说话。
func TestBackendTestsLiveWithImplementation(t *testing.T) {
	t.Parallel()
	if v := colocationViolations(sourceFiles(testutil.ScanBackendCode(t))); len(v) > 0 {
		t.Fatalf("测试与实现被拆开：\n  %s", strings.Join(v, "\n  "))
	}
}

// TestBackendGinStaysOnHTTPSurface gin 的宿主面：非测试文件里 import gin 的，必须属于 HTTP 面
// 或在 ginHostAllowed 里登记过。今天实测 0 违例；判据的意义在拆包（P2）—— 域的 service.go
// 顺手写上 *gin.Context 不会让编译器或 go vet 说话，只会让「换骨架」从改一处变成全仓改。
func TestBackendGinStaysOnHTTPSurface(t *testing.T) {
	t.Parallel()
	files := sourceFiles(testutil.ScanBackendCode(t))
	// 防空转：判据只对「非测试且不在 HTTP 面」的文件说话，这个集合塌了就等于没判。
	checked := 0
	for _, f := range files {
		if !f.Test && !httpSurface(f) {
			checked++
		}
	}
	if checked < 100 {
		t.Fatalf("判定面只有 %d 个非测试且非 HTTP 面的文件（实测基线数百）⇒ 射程塌了，这不是「无违规」", checked)
	}
	v, err := ginImportViolations(files, httpSurface)
	if err != nil {
		t.Fatalf("解析 import 失败: %v", err)
	}
	if len(v) > 0 {
		t.Fatalf("gin 跑到 HTTP 面之外：\n  %s", strings.Join(v, "\n  "))
	}
}

// TestGinHostWhitelistIsLive 白名单必须是活的：每条登记的目录都得真的扫到一个 gin 宿主，
// 否则死条目会一直替一个已经不存在的包开门（同 gorm seam 那条 TestGormDBWhitelistIsLive）。
func TestGinHostWhitelistIsLive(t *testing.T) {
	t.Parallel()
	files := sourceFiles(testutil.ScanBackendCode(t))
	fset := token.NewFileSet()
	hosts := map[string]int{}
	for _, f := range files {
		if f.Test {
			continue
		}
		imported, err := ginImported(fset, f)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", f.Path, err)
		}
		if imported {
			hosts[f.Dir]++
		}
	}
	if len(hosts) < 5 {
		t.Fatalf("全仓只扫到 %d 个 gin 宿主（实测基线 6+）⇒ 判据射程塌了", len(hosts))
	}
	for dir, why := range ginHostAllowed {
		if hosts[dir] == 0 {
			t.Errorf("白名单条目 %q（%s）今天一个 gin 宿主都没有 ⇒ 死条目，删掉它", dir, why)
		}
	}
}

// TestLayerGuardDetectsPlantedViolations 判定面自测：合成违例必须被报出 —— 没有这一条，上面两条
// 「0 违例」可能只是判据没在跑（本仓对静态锁的一贯要求）。
func TestLayerGuardDetectsPlantedViolations(t *testing.T) {
	t.Parallel()
	edges := []Edge{
		{From: "pkg/httpx", To: "internal/service"},    // 叶子反向依赖 ⇒ 违规
		{From: "internal/service", To: "internal/api"}, // 装配面被反向依赖 ⇒ 违规
		{From: "internal/api", To: "pkg/httpx"},        // 允许：装配面依赖叶子
		{From: "cmd/server", To: "internal/api"},       // 允许：装配根依赖装配面
	}
	if got := directionViolations(edges); len(got) != 2 {
		t.Fatalf("合成违例应恰好报 2 条（httpx→internal 与 service→api），实得 %d：%v", len(got), got)
	}
	files := []SourceFile{
		{Path: "internal/service/only_test_test.go", Dir: "internal/service", Test: true}, // 孤立测试 ⇒ 违规
		{Path: "internal/forum/service.go", Dir: "internal/forum"},
		{Path: "internal/forum/handler_test.go", Dir: "internal/forum", Test: true}, // 同居 ⇒ 合规
		{Path: "pkg/httpx/tests/x_test.go", Dir: "pkg/httpx/tests", Test: true},     // 独立 tests 目录 ⇒ 两条都中
		{Path: "pkg/httpx/parse.go", Dir: "pkg/httpx"},
	}
	if got := colocationViolations(files); len(got) != 3 {
		t.Fatalf("合成违例应恰好报 3 条（孤立测试 1 条 + 独立目录 2 条），实得 %d：%v", len(got), got)
	}
	// gin 宿主面：域实现里 import gin ⇒ 违规；域 handler 与登记基建 ⇒ 合规；测试文件不进判据。
	ginFiles := []SourceFile{
		{Path: "internal/forum/service.go", Dir: "internal/forum", Src: "package forum\n\nimport \"github.com/gin-gonic/gin\"\n"},
		{Path: "internal/forum/handler.go", Dir: "internal/forum", Name: "handler.go", Src: "package forum\n\nimport \"github.com/gin-gonic/gin\"\n"},
		{Path: "pkg/response/response.go", Dir: "pkg/response", Src: "package response\n\nimport \"github.com/gin-gonic/gin\"\n"},
		{Path: "internal/forum/service_test.go", Dir: "internal/forum", Test: true, Src: "package forum\n\nimport \"github.com/gin-gonic/gin\"\n"},
		{Path: "internal/forum/dto.go", Dir: "internal/forum", Src: "package forum\n"},
	}
	got, err := ginImportViolations(ginFiles, func(f SourceFile) bool {
		return strings.HasPrefix(f.Name, "handler") || f.Dir == "pkg/httpx" || f.Dir == "pkg/response"
	})
	if err != nil {
		t.Fatalf("合成集合解析失败: %v", err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "internal/forum/service.go") {
		t.Fatalf("合成违例应恰好报 1 条（域实现里 import gin），实得 %d：%v", len(got), got)
	}
}
