// 笔记域的 nonnil 行为例（ADR-0065 决策 5 判据 5）：跑真实出口、看发出的是 [] 还是 null。
//
// 一格：条目里的 images（#2069）——无图笔记读出来必须是 []，不是 null。
// `testutil.MarshalKey` 只看**顶层**键，所以出口返回那条条目本身（与 forum 的 images 三格同款）。
// 分页信封那格（note.NotePageDTO.items）的证据留在 internal/api 的 nonnilOutlets 表：
// 它走的是整条真实路由，组装点在 handler 那一层。
package note

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/questionbank"
	"forklift-training/internal/testutil"
)

// nonnilOutletsNote 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
var nonnilOutletsNote = map[string]func(t *testing.T) any{
	"note.NoteDTO.images":         outletNoteNoImages,
	"note.QuestionNoteDTO.images": outletQuestionNoteNoImages,
}

// TestNonNilDeclaredOutletsNeverEmitNull 本域的举证入口（判据本体在 testutil）。
func TestNonNilDeclaredOutletsNeverEmitNull(t *testing.T) {
	testutil.AssertNonNilOutlets(t, []map[string]func(t *testing.T) any{nonnilOutletsNote})
}

// outletQuestionNoteNoImages 题目笔记那一格（/api/questions/{id}/note 的返回形状，#2069）：
// 无图时同样是 []。它与 NoteDTO 是两个 DTO——题目面原本直接回 model.Note，加图后换了显式 DTO。
func outletQuestionNoteNoImages(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	// 题目维度写面要求这道题对本学员可读（ADR-0062 决策 4），故先播一道池内题。
	q := testutil.SeedQuestion(t, db, "single_choice", "题干", "A")
	svc := NewService(db, nil, zap.NewNop())
	n, err := svc.UpsertForQuestion(q.ID, 7, "题目笔记无图", nil, questionbank.NewQuestionReadScope(nil))
	if err != nil {
		t.Fatalf("保存题目笔记失败: %v", err)
	}
	return QuestionNoteToDTO(n)
}

// outletNoteNoImages 无图笔记：images 是空集而不是 null（历史行 images 列为 '[]' 也走同一条读面）。
func outletNoteNoImages(t *testing.T) any {
	t.Helper()
	// 无图即不碰存储 ⇒ fileSvc 传 nil（图片校验只在有图时才会问存储）。
	svc := NewService(testutil.NewMemoryDB(t), nil, zap.NewNop())
	n, err := svc.Create(7, "无图笔记", nil)
	if err != nil {
		t.Fatalf("建笔记失败: %v", err)
	}
	return NoteToDTO(n, "")
}
