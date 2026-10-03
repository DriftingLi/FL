// 内容生成域的 nullable 行为例（波 2c 从 nullable_outlets_ai_test.go 拆出：AI 三格随域搬走）。
//
// 机制见 nullable_declaration_test.go 的 nullableOutletTables（分域表 + init 并进汇总，
// runner 逐条执行，不是给 AST 查名字的花名册）。
//
// 本格的形状：pending 阶段 async_task.result 列还没写过，GetTaskStatus 只在
// len(task.Result) > 0 时才赋值 ⇒ 轮询第一次就打到的那一发就是 null。
package core

import (
	"strconv"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nullableOutletsContent = map[string]func(t *testing.T) any{
	"core.GenTaskStatus.results": outletGenTaskPending,
}

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsContent)
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
