// AI / 诊断域的 nonnil 行为例（票 2242 起；判据本体仍在 testutil，见 testutil/nonnil.go）。
//
// 为什么这个包此前没有这张表：2237 之前诊断域只有 v1 的只读代理，返回的是裸数组与分页信封，
// 集合字段要么在 api 层组装、要么根本不在生成面。2242 把 v2 的引导面搬进来之后，这里新长出
// 五格「集合恒非 null」的表态——空集而不是 null，前端才敢直接 .length / v-for，不必写 ?? []。
//
// 五格各自的「空」从哪来：
//   - DiagnosisIntents.brands / .faq、DiagnosisCatalogItems.items、DiagnosisCodeHint.samples
//     ⇒ 上游回 null 或缺字段时，service 的归一（Intents/Systems/Faults/CodeHint 末尾那几行）
//     把它们换成空集。举证用的假上游**故意只回 `{}`**，走的就是这条归一。
//   - AIChatMessageMeta.badges ⇒ 列里存了 meta 但没命中徽章（badges 被省略）；读面
//     GetSessionMessages 把 nil 归一成 []DiagnosisBadge{}。
package aiassistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nonnilOutletsAIAssistant 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
var nonnilOutletsAIAssistant = map[string]func(t *testing.T) any{
	"aiassistant.DiagnosisIntents.brands":     outletIntentsWithoutBrands,
	"aiassistant.DiagnosisIntents.faq":        outletIntentsWithoutBrands,
	"aiassistant.DiagnosisCatalogItems.items": outletCatalogItemsEmpty,
	"aiassistant.DiagnosisCodeHint.samples":   outletCodeHintWithoutSamples,
	"aiassistant.AIChatMessageMeta.badges":    outletChatMessageMetaWithoutBadges,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsAIAssistant})
}

// newEmptyCatalogService 造一个「上游什么都回 {}」的引导面代理：服务端活着、鉴权过了、
// 但响应里一个集合字段都没有——正是归一逻辑要兜住的那一档。
func newEmptyCatalogService(t *testing.T) *DiagnosisCatalogService {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return NewDiagnosisCatalogService(server.URL, "test-key", zap.NewNop())
}

// outletIntentsWithoutBrands 引导第一层：上游没给 brands / faq，出口仍是两个空集。
func outletIntentsWithoutBrands(t *testing.T) any {
	t.Helper()
	out, err := newEmptyCatalogService(t).Intents(context.Background())
	if err != nil {
		t.Fatalf("拉引导面失败: %v", err)
	}
	return out
}

// outletCatalogItemsEmpty 第二/三层：上游没给 items（或给了 null），出口是 {"items":[]}。
func outletCatalogItemsEmpty(t *testing.T) any {
	t.Helper()
	out, err := newEmptyCatalogService(t).Faults(context.Background(), "ep", "")
	if err != nil {
		t.Fatalf("拉故障现象失败: %v", err)
	}
	return out
}

// outletCodeHintWithoutSamples 故障码体检：非 format 状态不带 samples，出口是空集。
func outletCodeHintWithoutSamples(t *testing.T) any {
	t.Helper()
	out, err := newEmptyCatalogService(t).CodeHint(context.Background(), "E01")
	if err != nil {
		t.Fatalf("故障码体检失败: %v", err)
	}
	return out
}

// outletChatMessageMetaWithoutBadges 历史回放：meta 列有 elapsed 但没有 badges。
// 这一格与上面四格不同源——它走的是「列 → DTO」的读面，不是上游归一。
func outletChatMessageMetaWithoutBadges(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := testutil.SeedStudent(t, db, "徽章缺席学员", "hash")
	session := model.AIChatSession{UserID: owner.ID, Title: "无徽章会话", ModelName: "m", FeatureKey: "fault_diagnosis"}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("播会话失败: %v", err)
	}
	msg := model.AIChatMessage{
		SessionID: session.ID,
		Role:      "assistant",
		Content:   "液压系统压力不足，先查滤芯。",
		// 只有 elapsed 的 meta：上游那次 done 帧没命中任何徽章。
		Meta:      `{"elapsed":12.4}`,
		CreatedAt: testutil.Now(),
	}
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
	if out[0].Meta == nil {
		t.Fatal("meta 列非空却没解出元数据——出口没落到要证的那一格")
	}
	return *out[0].Meta
}
