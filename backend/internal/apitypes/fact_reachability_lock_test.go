// fact 投影位的**可达性**锁（ADR-0065 决策 8 的射程半边）。
//
// 为什么这一半住在 apitypes 而不是 internal/api：判据是「带 `fact:` tag 的字段所在的那个类型，
// 必须真的出现在某个 2xx 响应的闭包里」——而 2xx 闭包与全仓 AST 遍历在表态锁那一侧已经各有一份
// （responseDefinitions / sweptDirs / parsePackage），`nullability_lock_test.go` 的文件头明写
// 「闭包用 codegen 自己那套，**不另写第二份 $ref 遍历**」。在 api 包里再正则扫一遍 swagger 会同时
// 犯两件事：两份实现会漂，而且比那份窄——sweptDirs 还含 model 与 valuation/repository，
// 投影位完全可能长在那边的类型上。
package apitypes

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// factTagKey 与 internal/api/consumption_fact_lock_test.go 里的 `fact` 是同一个 tag。
// 两处各写一遍字面量而不是共享常量：那是测试内的常量，跨包共享得开一个非测试的导出符号，
// 为一枚 tag 名造一个生产 API 更糟。改 tag 名时两把锁都要红，那正是想要的连带。
const factTagKey = "fact"

func TestFactProjectionsAreInResponseClosure(t *testing.T) {
	closure := responseDefinitions(t)
	pkgs := map[string]parsedPackage{}
	for pkgName, dir := range sweptDirs {
		p, err := parsePackage(dir, false)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", dir, err)
		}
		if len(p.files) == 0 {
			t.Fatalf("扫描面 %s（%s）里一个非测试 .go 都没有——射程塌了不等于无违规", pkgName, dir)
		}
		pkgs[pkgName] = p
	}

	found := 0
	for pkgName, p := range pkgs {
		for _, f := range p.files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || st.Fields == nil {
						continue
					}
					for _, fld := range st.Fields.List {
						if len(fld.Names) == 0 || fld.Tag == nil {
							continue
						}
						tag := strings.Trim(fld.Tag.Value, "`")
						factVal, has := reflect.StructTag(tag).Lookup(factTagKey)
						if !has || factVal == "" {
							continue
						}
						found++
						defName := pkgName + "." + ts.Name.Name
						if !closure[defName] {
							t.Errorf("fact:%q 挂在 %s.%s 上，但 %s 不在任何 2xx 响应的类型闭包里："+
								"这一格对外没人读得到，它不是消费点。tag 登记的是「消费点对齐」，"+
								"挂在进不了契约的类型上就是一条幽灵登记。",
								factVal, defName, fld.Names[0].Name, defName)
						}
					}
				}
			}
		}
	}
	// 与 api 那一侧同一条防空转判据，但**理由不同**：那边怕的是「登记表成了唯一声明方」，
	// 这边怕的是「tag 写法变了，而这把射程锁一条都没扫到还报绿」。两条都留着，因为两者的
	// 失效方式不一样（一处 tag 名改错，那边可能仍扫到 api/service 的字段而这里四个目录都扫空）。
	if found == 0 {
		t.Fatal("sweptDirs 四个包里一处 fact tag 都没有——是 tag 改名了，还是投影位全被删了？" +
			"（扫不到时不得当作「无违规」）")
	}
}
