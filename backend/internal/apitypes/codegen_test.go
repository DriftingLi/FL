package apitypes

import (
	"os"
	"path/filepath"
	"testing"
)

// spec #940 片五③：前端契约类型的生成物同步契约。
//
// 手改生成物、或改了注解却忘记再生成（也不跑 swagger 新鲜度锁），本测试即红。
// prior art：internal/authz/codegen_test.go 与 internal/deploy/topology_test.go。

func specPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "backend", "docs", "swagger.json")
}

func outPath(t *testing.T, domain string) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "frontend", "src", "api", "generated", domain+".ts")
}

func TestFrontendAPITypesInSync(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败（先 cd backend && make swagger）: %v", err)
	}
	rendered, err := RenderAll(spec)
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if len(rendered) == 0 {
		t.Fatal("声明表为空，拒绝静默通过")
	}
	for _, d := range Domains {
		path := outPath(t, d.Name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取生成物 %s 失败（应先运行 cd backend && go run ./cmd/gen-apitypes）: %v", path, err)
		}
		if string(got) != rendered[d.Name] {
			t.Fatalf("生成物与注解不同步：请 cd backend && go run ./cmd/gen-apitypes（域 %s）", d.Name)
		}
	}
}

// TestDomainRootsExist 声明表里的根类型必须在 swagger definitions 里存在 ——
// 类型改名（如 DTO 更名）时给出「是声明表过期」而不是「渲染结果为空」的明确信号。
func TestDomainRootsExist(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败: %v", err)
	}
	for _, d := range Domains {
		if len(d.Roots) == 0 {
			t.Fatalf("域 %s 未声明根类型", d.Name)
		}
		for _, r := range d.Roots {
			if _, ok := spec.Definitions[r]; !ok {
				t.Fatalf("域 %s 的根类型 %q 不在 swagger definitions 里（注解改名或声明表过期）", d.Name, r)
			}
		}
	}
}

// TestRenderDomainDeterministic 渲染是纯函数：两次渲染必须逐字节一致
// （否则「字节全等」契约会在 CI 上随机红 —— map 迭代序泄漏是这里的经典成因）。
func TestRenderDomainDeterministic(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败: %v", err)
	}
	for _, d := range Domains {
		first, err := RenderDomain(spec, d)
		if err != nil {
			t.Fatalf("首次渲染失败: %v", err)
		}
		for i := 0; i < 5; i++ {
			again, err := RenderDomain(spec, d)
			if err != nil {
				t.Fatalf("再次渲染失败: %v", err)
			}
			if again != first {
				t.Fatalf("域 %s 渲染不确定（第 %d 次与首次不同）", d.Name, i+2)
			}
		}
	}
}
