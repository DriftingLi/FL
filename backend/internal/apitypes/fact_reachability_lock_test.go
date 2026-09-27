// fact 投影位的**可达性**锁（ADR-0065 决策 8 的射程半边）。
//
// 为什么这一半住在 apitypes 而不是 internal/api：判据是「带 `fact:` tag 的字段所在的那个类型，
// 必须真的出现在某个 2xx 响应的类型闭包里」——而 2xx 闭包与全仓 AST 遍历在表态锁那一侧已经各有一份
// （responseDefinitions / parsePackage），`nullability_lock_test.go` 的文件头明写「闭包用 codegen
// 自己那套，**不另写第二份 $ref 遍历**」。在 api 包里再拿正则扫一遍 swagger 会同时犯两件事：
// 两份实现会漂，而且那份比这份窄。
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

// factScopeDirs 是这把锁的扫描目录集。**定义键前缀取解析出来的包子句**，不取这里的名义，
// 所以两个目录同前缀是允许的（下面 model 那两行就是实情）。
//
// 比 sweptDirs 多两枚：
//   - `../valuation/model`：与 internal/model 同包子句 model，实测 17 个 `model.*` 定义里 13 个
//     来自它、3 个来自前者。不加就是实打实的漏扫——残值侧的 DTO 上挂个 tag，这条锁一条也碰不到。
//   - `../../pkg/response`：信封 `response.R`。它是 2xx 响应的外壳本身，属于「对外契约里的类型」。
//
// 为什么不干脆改 sweptDirs 把这两枚加进去：那份是**表态锁**的射程，加类型进去会改动一把已上生产
// 的锁的判据（信封三个字段全是标量，对表态锁没有意义；残值 model 会新增一批要求表态的字段，
// 那是另一件事、该另立一条决策）。两把锁的论域不同，就别共用一张清单——但两边各自加了什么，
// 下面那条论域自检会把「闭包里有、清单里没扫」变成一条指名道姓的红。
var factScopeDirs = []string{
	"../api",
	"../service",
	"../model",
	"../valuation/model",
	"../valuation/repository",
	"../../pkg/response",
}

func TestFactProjectionsAreInResponseClosure(t *testing.T) {
	// 数一遍目录：论域自检比的是**前缀**，删掉 `../valuation/model` 它看不出来（下一段写这条盲区），
	// 所以这里补一条「清单本身有几枚」的断言——与批⑤ 那条「pathInt* 名字族必须恰好两枚」同形。
	// 加一枚目录时这条会红，逼着回来一起改数并想清楚为什么加。
	if n := len(factScopeDirs); n != 6 {
		t.Fatalf("fact 射程应是 6 个包目录，实际 %d 个：%v —— 少了就是漏扫（前缀绊线抓不到同前缀的两个目录），"+
			"多了就回来把这条数和上面的注释一起改。", n, factScopeDirs)
	}
	closure := responseDefinitions(t)
	scanned := map[string]bool{}
	found := 0
	for _, dir := range factScopeDirs {
		p, err := parsePackage(dir, false)
		if err != nil {
			t.Fatalf("读 fact 可达性扫描面 %s 失败: %v", dir, err)
		}
		if len(p.files) == 0 {
			t.Fatalf("扫描面 %s 里一个非测试 .go 都没有——射程塌了不等于无违规", dir)
		}
		for _, f := range p.files {
			scanned[f.Name.Name] = true
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
						defName := f.Name.Name + "." + ts.Name.Name
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
	// 论域自检：2xx 闭包里的定义键出现过哪些包名前缀，每一个都必须真的被扫到。判据来源是生成物
	// 而不是这份目录清单——清单会漂，生成物不会（CI 对它还有新鲜度锁）。与 api 侧那条绊线同判据。
	//
	// **它有一条与生俱来的盲区，写下来而不是假装没有**：它比的是**前缀**，不是目录。
	// `internal/model` 与 `internal/valuation/model` 共用 `model` 前缀（因为 swagger 定义键本身就
	// 按包子句命名），所以把后者从清单里删掉，这条自检看不出来。这不是实现偷懒——判据只能读到
	// 生成物，而生成物自己也分不开这两个包（swag 用同一个命名空间）。⇒ 那两个目录都得靠人守，
	// 真要锁住得换一种键身份（比如把包路径编进投影位键），那会把投影位与 swagger 定义键的对应关系拆散。
	for defName := range closure {
		pkg, _, ok := strings.Cut(defName, ".")
		if !ok {
			t.Errorf("闭包里的定义键 %q 不带包名前缀——投影位的键形状要重判", defName)
			continue
		}
		if !scanned[pkg] {
			t.Errorf("2xx 闭包出现了 %q 包的类型，但 factScopeDirs 没扫它：把目录补进去，"+
				"否则那一类投影位这条锁一条也碰不到。", pkg)
		}
	}
	// 与 api 那一侧同一条防空转判据，但**理由不同**：那边怕「登记表成了唯一的声明方」，
	// 这边怕「tag 写法变了，而这把射程锁一条都没扫到还报绿」。两条都留，因为失效方式不一样。
	if found == 0 {
		t.Fatal("factScopeDirs 里一处 fact tag 都没有——是 tag 改名了，还是投影位全被删了？" +
			"（扫不到时不得当作「无违规」）")
	}
}
