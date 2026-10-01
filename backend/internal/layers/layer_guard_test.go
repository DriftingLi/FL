package layers

import (
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
}
