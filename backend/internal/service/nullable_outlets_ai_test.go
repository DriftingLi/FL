// AI / 诊断 / 生成任务域的 nullable 行为例（批①-B 第②段）。
//
// 机制见 nullable_declaration_test.go 的 nullableOutletTables（分域表 + init 并进汇总，
// runner 逐条执行，不是给 AST 查名字的花名册）。
//
// 本段四格的共同点：**集合那一格从来没有 make(...) 兜底**，所以「没有内容」那一档就是 Go 的
// nil 切片，marshal 出来是 `null` 而不是 `[]`。四格各自的「没有内容」是哪一种，写在各自出口上：
//   - AIChatMessageDTO.images / .sources ⇒ 列是**字符串**存的 JSON，`GetSessionMessages` 只在
//     列非空串时才 Unmarshal，「没图 / 没来源」那两档留 nil。同一份历史回放，两格两种缺席形状。
//   - GenTaskStatus.results ⇒ pending 阶段 async_task.result 列还没写过，GetTaskStatus 只在
//     `len(task.Result) > 0` 时才赋值 ⇒ 轮询第一次就打到的那一发就是 null。
//   - DiagnosisFaultCodePage.items ⇒ 代理把上游 body 直接 Decode 进结构体，一个 make 都没有。
//     这一格的可空性**由对端进程决定**：上游省略 items 键或发 `null`，这里就是 `null`。
//     出口用 httptest 假上游量的是「代理不兜底」这个事实（本仓改判不了对端发什么，
//     所以它也改判不成 nonnil——那句 nonnil 会替对端撒一句本仓没资格替它作的保）。
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nullableOutletsAI = map[string]func(t *testing.T) any{
	"service.AIChatMessageDTO.images":      outletAIChatMessageWithoutImages,
	"service.AIChatMessageDTO.sources":     outletAIChatMessageWithoutImages,
	"service.GenTaskStatus.results":        outletGenTaskPending,
	"service.DiagnosisFaultCodePage.items": outletFaultCodesUpstreamOmitsItems,
}

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsAI)
}

// outletAIChatMessageWithoutImages 历史回放：一条用户消息（没带图）与一条助手消息（没来源）。
// 两条都是生产上最常见的形状——绝大多数对话既不带图也没有诊断来源。
func outletAIChatMessageWithoutImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	owner := seedForumUser(t, db, "对话学员")
	session := model.AIChatSession{UserID: owner.ID, Title: "无图无来源会话", ModelName: "m", FeatureKey: "ai_assistant"}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("播会话失败: %v", err)
	}
	msg := model.AIChatMessage{SessionID: session.ID, Role: "user", Content: "液压泵压力多少正常", CreatedAt: testutil.Now()}
	if err := db.Create(&msg).Error; err != nil {
		t.Fatalf("播消息失败: %v", err)
	}
	svc := NewAIAssistantService(db, nil, nil, "", zap.NewNop(), nil)
	out, err := svc.GetSessionMessages(context.Background(), owner.ID, session.ID)
	if err != nil {
		t.Fatalf("历史回放失败: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("回放取到 %d 条消息，期望 1 条——出口没落到要证的那一格", len(out))
	}
	return out[0]
}

// outletGenTaskPending pending 阶段的轮询：result 列尚未写过（异步任务刚建那一刻的真实形状）。
func outletGenTaskPending(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	task := model.AsyncTask{
		ID: 9201, TaskType: "course_content_generate", Status: "pending",
		Payload:   model.JSONB([]byte(`{"course_id":1,"chapter_ids":[2,3],"user_id":1}`)),
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("播 pending 任务失败: %v", err)
	}
	svc := NewContentGenerateService(db, nil, zap.NewNop())
	status, err := svc.GetTaskStatus(strconv.Itoa(task.ID))
	if err != nil {
		t.Fatalf("查任务状态失败: %v", err)
	}
	if status.Status != "pending" {
		t.Fatalf("状态 = %s, 期望 pending（这条证据量的是「还没写过 result」那一档）", status.Status)
	}
	return status
}

// outletFaultCodesUpstreamOmitsItems 假上游只回 `{"code":200,"data":{"total":0}}`：
// 省略 items 键（对端「这一页没有条目」的一种常见写法）⇒ 代理 Decode 后 Items 仍是 nil。
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
