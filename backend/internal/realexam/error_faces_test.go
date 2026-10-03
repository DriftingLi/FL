// 真题域的**事实→错误**档位锁（ADR-0064 决策 8）：卷不可用 / 未兑换 / 卷内无题三件事必须能被区分。
// 由 internal/service/practice_paper_face_test.go 按接缝拆来（波 4a，随域包搬走）。
package realexam

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/testutil"
)

// TestRealPaperThreeFacts 按卷开考链路上的三件事必须能被区分：卷不可用 / 未兑换 / 卷内无题。
// 此前它们挤在同一格 errStatusAll(404)，A 批在端点注释里把「升哨兵再换表」登记为正解。
func TestRealPaperThreeFacts(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, points.NewService(db, zap.NewNop(), nil, notification.NewService(db, zap.NewNop())), zap.NewNop())
	paper := model.RealExamPaper{Title: "2026 叉车真题", SourceRef: "RP-LEDGER", Status: 1}
	if err := db.Create(&paper).Error; err != nil {
		t.Fatalf("播种真题卷失败: %v", err)
	}
	// 1) 卷不在（含未发布）= 不存在。
	if _, err := svc.StartPaperPractice(1, 999999); !errors.Is(err, points.ErrRealPaperUnavailable) {
		t.Fatalf("不存在的卷应报 points.ErrRealPaperUnavailable，实际 %v", err)
	}
	// 2) 卷在、可见，但这个人没付过 = 无权益，与「不存在」是两件事。
	if _, err := svc.StartPaperPractice(1, paper.PaperID); !errors.Is(err, ErrRealPaperNotRedeemed) {
		t.Fatalf("未兑换应报具名 ErrRealPaperNotRedeemed（此前与不存在共用 404），实际 %v", err)
	}
	if _, err := svc.StartPaperExam(1, paper.PaperID); !errors.Is(err, ErrRealPaperNotRedeemed) {
		t.Fatalf("开考侧同判，实际 %v", err)
	}
	if errors.Is(ErrRealPaperNotRedeemed, points.ErrRealPaperUnavailable) {
		t.Fatal("两个哨兵可互相顶替 —— 分档失效")
	}
}
