package apitypes

import (
	"strings"
	"testing"

	"forklift-training/internal/codegen"
)

// spec #940 片五③：前端契约类型的生成物同步契约。
//
// 手改生成物、或改了注解却忘记再生成（也不跑 swagger 新鲜度锁），本测试即红。
// prior art：internal/authz/codegen_test.go 与 internal/deploy/topology_test.go。
// 定位与比对自 ADR-0053 §9 起走 codegen（不再手写 `../../../`）。

func specPath(t *testing.T) string {
	t.Helper()
	path, err := codegen.Resolve("", LocateSwaggerSpec, NotFoundSwaggerSpec)
	if err != nil {
		t.Fatalf("定位 swagger 产物失败: %v", err)
	}
	return path
}

func TestFrontendAPITypesInSync(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败（先 cd backend && make swagger）: %v", err)
	}
	if len(Domains) == 0 {
		t.Fatal("声明表为空，拒绝静默通过")
	}
	for _, d := range Domains {
		domain := d
		codegen.AssertInSync(t, GeneratedTSGen(domain.Name, func() (string, error) {
			return RenderDomain(spec, domain)
		}))
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

// TestDomainEndpointsExist 声明表里登记的端点必须在 swagger paths 里存在 ——
// 生成物头部会把这些端点写给读者看，靠人工维护必然会漂；这条锁把它钉在注解产物上。
func TestDomainEndpointsExist(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败: %v", err)
	}
	for _, d := range Domains {
		if len(d.Endpoints) == 0 {
			t.Fatalf("域 %s 未声明任何端点", d.Name)
		}
		for _, e := range d.Endpoints {
			raw, ok := spec.Paths[e.Path]
			if !ok {
				t.Fatalf("域 %s 声明的端点 %s %s 不在 swagger paths 里（注解改了路径？声明表过期？）", d.Name, e.Method, e.Path)
			}
			ops, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("域 %s 的端点 %s 在 swagger 里不是对象", d.Name, e.Path)
			}
			if _, ok := ops[strings.ToLower(e.Method)]; !ok {
				t.Fatalf("域 %s 的端点 %s 缺 %s 方法", d.Name, e.Path, e.Method)
			}
		}
	}
}

// TestDomainEndpointsDeclareData 域内每个端点的 data 指认要么存在、要么在声明表里显式登记为
// 「有意无 data」（spec #952 片一）。这条锁把「注解补全」从人工清点变成可执行断言：
// 新增端点忘了指认 data、或注解被误删，都在这里直接红，而不是等生成物接到前端才发现。
func TestDomainEndpointsDeclareData(t *testing.T) {
	spec, err := LoadSpec(specPath(t))
	if err != nil {
		t.Fatalf("读取 swagger 产物失败: %v", err)
	}
	for _, d := range Domains {
		// 该域生成物实际覆盖的类型（根 + 传递闭包）：端点的 data 必须落在里面，
		// 否则「响应类型由生成物提供」这句话对该端点不成立。
		covered := map[string]bool{}
		names, err := collect(spec, d.Roots)
		if err != nil {
			t.Fatalf("域 %s 取传递闭包失败: %v", d.Name, err)
		}
		for _, n := range names {
			covered[n] = true
		}
		for _, e := range d.Endpoints {
			ref, hasData := spec.DataRef(e.Method, e.Path)
			switch {
			case e.NoData && hasData:
				t.Fatalf("域 %s 的 %s %s 登记为有意无 data，但注解指认了 %s —— 二者必须一致", d.Name, e.Method, e.Path, ref)
			case !e.NoData && !hasData:
				t.Fatalf("域 %s 的 %s %s 没有 data 指认：补 @Success 200 {object} response.R{data=service.Xxx}，或把声明表的 NoData 置 true（确实无载荷）", d.Name, e.Method, e.Path)
			case hasData && ref != "" && !covered[ref]:
				t.Fatalf("域 %s 的 %s %s 指认了 %s，但它不在该域生成物覆盖的类型里（声明表 Roots 漏了？）", d.Name, e.Method, e.Path, ref)
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
