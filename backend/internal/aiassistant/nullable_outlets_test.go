// AI / 诊断域的 nullable 行为例（域包版本；由 internal/service/nullable_outlets_ai_test.go 拆出，波 2c）。
//
// 机制见本文件下半段的 nullableOutletTables（分域表 + init 并进汇总，runner 逐条执行，
// 不是给 AST 查名字的花名册）。**域包不得 import internal/core 的测试文件**（ADR-0070 决策 9：
// 证据表可住域包），所以这里自带一份形状相同的 runner，照抄
// internal/core/nullable_declaration_test.go。键格式 = <包名>.<类型>.<json键>，
// 登记处是 internal/apitypes/nullability_lock_test.go 的 nullableEvidenceSources
// （它按**目录**收表：域包一行，见该锁 :115-176 的「一条事实一个证据」判据）。
//
// 本段三格的共同点：**集合那一格从来没有 make(...) 兜底**，所以「没有内容」那一档就是 Go 的
// nil 切片，marshal 出来是 `null` 而不是 `[]`：
//   - AIChatMessageDTO.images / .sources ⇒ 列是**字符串**存的 JSON，GetSessionMessages 只在
//     列非空串时才 Unmarshal，「没图 / 没来源」那两档留 nil。同一份历史回放，两格两种缺席形状。
//   - DiagnosisFaultCodePage.items ⇒ 代理把上游 body 直接 Decode 进结构体，一个 make 都没有。
//     这一格的可空性**由对端进程决定**：上游省略 items 键或发 `null`，这里就是 `null`。
//     出口用 httptest 假上游量的是「代理不兜底」这个事实（本仓改判不了对端发什么，
//     所以它也改判不成 nonnil——那句 nonnil 会替对端撒一句本仓没资格替它作的保）。
package aiassistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nullableOutlets 键 = 包名.类型名.json键（与判据 4 的账同一格式），值 = 走真实出口取到的结果。
//
// 新增一条 nullable 声明时必须同时在这里留一行，否则 apitypes 的判据 4 当场点名；
// 反过来，把某条改判成 nonnil 而不删这里的键，会被「表里有键、字段却不是 nullable」那半边抓到。
var nullableOutlets = map[string]func(t *testing.T) any{
	"aiassistant.AIChatMessageDTO.images":      outletAIChatMessageWithoutImages,
	"aiassistant.AIChatMessageDTO.sources":     outletAIChatMessageWithoutImages,
	"aiassistant.DiagnosisFaultCodePage.items": outletFaultCodesUpstreamOmitsItems,
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

// outletAIChatMessageWithoutImages 历史回放：一条用户消息（没带图）与一条助手消息（没来源）。
// 两条都是生产上最常见的形状——绝大多数对话既不带图也没有诊断来源。
func outletAIChatMessageWithoutImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := testutil.SeedStudent(t, db, "对话学员", "hash")
	session := model.AIChatSession{UserID: owner.ID, Title: "无图无来源会话", ModelName: "m", FeatureKey: "ai_assistant"}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("播会话失败: %v", err)
	}
	msg := model.AIChatMessage{SessionID: session.ID, Role: "user", Content: "液压泵压力多少正常", CreatedAt: testutil.Now()}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatalf("播消息失败: %v", err)
	}
	svc := NewService(db, nil, nil, "", zap.NewNop(), nil)
	out, err := svc.GetSessionMessages(context.Background(), owner.ID, session.ID)
	if err != nil {
		t.Fatalf("历史回放失败: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("回放取到 %d 条消息，期望 1 条——出口没落到要证的那一格", len(out))
	}
	return out[0]
}

// outletFaultCodesUpstreamOmitsItems 假上游只回 code=200 + data.total=0（省略 items 键，
// 对端「这一页没有条目」的一种常见写法）⇒ 代理 Decode 后 Items 仍是 nil。
func outletFaultCodesUpstreamOmitsItems(t *testing.T) any {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"total":0}}`))
	}))
	defer server.Close()
	page, err := newProxyForTest(server).ListFaultCodes(context.Background(), "", "", 1, 20)
	if err != nil {
		t.Fatalf("故障码分页失败: %v", err)
	}
	return page
}
