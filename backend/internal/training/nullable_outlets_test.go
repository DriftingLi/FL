// 培训域的 nullable 行为例（域包版本；由 internal/service/nullable_outlets_practice_test.go 拆出，波 3b-2）。
//
// 机制见本文件下半段的 nullableOutletTables（分域表 + init 并进汇总，runner 逐条执行，
// 不是给 AST 查名字的花名册）。**域包不得 import internal/core 的测试文件**（ADR-0070 决策 9：
// 证据表可住域包），所以这里自带一份形状相同的 runner，照抄
// internal/core/nullable_declaration_test.go。键格式 = <包名>.<类型>.<json键>，
// 登记处是 internal/apitypes/nullability_lock_test.go 的 nullableEvidenceSources（它按**目录**收表）。
//
// 这一格为什么 nullable（原判词，随键一起搬来）：
//   - QuestionTagsResultDTO.tag_ids ⇒ **请求体回显**：该键由 handler 直接回写客户端发来的切片
//     （本包 handler_admin.go 的 training.QuestionTagsResultDTO{TagIDs: req.TagIDs}），
//     客户端发 `"tag_ids": null` 或干脆不发这个键，拿到的就是 `null`。
//     这一格**不能**改判 nonnil：那等于把「入参什么形状」谎报成「出参恒非 null」。
//     出口按 handler 那一行逐字复现包装（切片仍取自同一条服务方法），先例见
//     internal/service/nonnil_outlets_people_test.go 末尾那三格同口径的信封（该文件 P2 波 4e 随域删除）。
package training

import (
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nullableOutlets 键 = 包名.类型名.json键（与判据 4 的账同一格式），值 = 走真实出口取到的结果。
//
// 新增一条 nullable 声明时必须同时在这里留一行，否则 apitypes 的判据 4 当场点名；
// 反过来，把某条改判成 nonnil 而不删这里的键，会被「表里有键、字段却不是 nullable」那半边抓到。
var nullableOutlets = map[string]func(t *testing.T) any{
	"training.QuestionTagsResultDTO.tag_ids": outletQuestionTagsEchoNil,
}

// nullableOutletTables 是域包这张 nullable 表的汇总点；runner 逐条执行它（理由同留驻侧
// internal/core/nullable_declaration_test.go：一张只有 AST 在看的表就是花名册）。
var nullableOutletTables []map[string]func(t *testing.T) any

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutlets)
}

// allNullableOutlets 展开所有分表。同名键出现在两张表里即判红：一条事实两处举证，改一处另一处还在过。
func allNullableOutlets(t *testing.T) map[string]func(t *testing.T) any {
	t.Helper()
	out := map[string]func(t *testing.T) any{}
	owner := map[string]int{}
	for i, tbl := range nullableOutletTables {
		for k, v := range tbl {
			if prev, dup := owner[k]; dup {
				t.Fatalf("%s 被第 %d 张与第 %d 张表各自举证——一条事实一个证据，留一处", k, prev, i)
			}
			owner[k] = i
			out[k] = v
		}
	}
	return out
}

// TestNullableDeclaredOutletsEmitNull 每条登记过的出口都必须真的发出 null。
//
// 键名反推 json 键：表键最后一段就是要看的键；对不上（字段被改名、被 omitempty 掉、
// 或整个类型不在射程里）一律判红，不留「找不到就当通过」的分支。
func TestNullableDeclaredOutletsEmitNull(t *testing.T) {
	outlets := allNullableOutlets(t)
	if len(outlets) == 0 {
		t.Fatal("证据表是空的：判据 4 会因此空转，这里必须同步红")
	}
	for key, build := range outlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			got := testutil.MarshalKey(t, build(t), jsonKey)
			if got != "null" {
				t.Fatalf("声明 nullable 的 %s 实际发出 %s——这句表态没有出口证明，"+
					"要么找一条真发 null 的出口，要么按实测改判 nonnil", key, got)
			}
		})
	}
}

// outletQuestionTagsEchoNil 打标面：客户端发 `{"tag_ids":null}`（或省略该键）时清空标签并回显，
// 回显的就是那个 nil 切片。服务侧真跑一次（题目必须存在，否则 400 而不是这一发响应）。
func outletQuestionTagsEchoNil(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "打标回显题", "A")
	// handler 的 Decode 阶段（c.ShouldBindJSON 同一件事）：body {"tag_ids":null} 解进 []int
	// 得到的是 nil —— encoding/json 对 JSON null 不清空也不分配目标切片。
	var req struct {
		TagIDs []int `json:"tag_ids"`
	}
	if err := json.Unmarshal([]byte(`{"tag_ids":null}`), &req); err != nil {
		t.Fatalf("复现 handler 的请求解码失败: %v", err)
	}
	if err := NewService(db, zap.NewNop()).SetQuestionTags(q.ID, req.TagIDs); err != nil {
		t.Fatalf("清空题目标签失败: %v", err)
	}
	return &QuestionTagsResultDTO{TagIDs: req.TagIDs}
}
