// 反馈域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 两格：单条反馈的 images（无图时必须发 []）+ 列表的 items（空列表时必须发 []）。
// 断言本体只有一份：testutil.AssertNonNilOutlets。
package feedback

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// nonnilOutletsFeedback 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
var nonnilOutletsFeedback = map[string]func(t *testing.T) any{
	"feedback.FeedbackDTO.images":       outletFeedbackNoImages,
	"feedback.FeedbackListResult.items": outletFeedbackListEmpty,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsFeedback})
}

// outletFeedbackNoImages 提交一条无图反馈：images 是空集而不是 null。
// 走真实写面（Submit）而不是手搓 DTO——零值 DTO 证明不了出口。
func outletFeedbackNoImages(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	dto, err := svc.Submit(7, &FeedbackInput{Content: "课程第三节视频打不开"})
	if err != nil {
		t.Fatalf("提交反馈失败: %v", err)
	}
	return dto
}

// outletFeedbackListEmpty 本人历史为空：items 是空集而不是 null。
func outletFeedbackListEmpty(t *testing.T) any {
	t.Helper()
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	res, err := svc.ListMine(7, 1, 20)
	if err != nil {
		t.Fatalf("本人历史失败: %v", err)
	}
	return res
}
