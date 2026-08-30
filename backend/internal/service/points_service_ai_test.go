// Package service AI 计费编排回归（#396，ADR-0023）：tokens 估算与兜底阈值单点、
// 分桶换算 ceil×10、下限/上限、余额预检、幂等键不变、ErrInsufficientPoints 哨兵匹配。
package service

import (
	"context"
	"errors"
	"testing"

	"forklift-training/internal/model"
)

// TestEstimateAITokens 估算口径：ceil(len/4) 分项 + 合计兜底阈值（<10 → 100）单点。
func TestEstimateAITokens(t *testing.T) {
	cases := []struct {
		name                         string
		promptChars, completionChars int
		total, prompt, completion    int
	}{
		{"空输入走兜底", 0, 0, 100, 0, 0},
		{"合计9字符走兜底", 36, 0, 100, 9, 0},
		{"合计恰10不兜底", 40, 0, 10, 10, 0},
		{"向上取整", 5, 3, 100, 2, 1}, // 合计8 → 兜底；分项 ceil(5/4)=2, ceil(3/4)=1
		{"常规合计", 4000, 1000, 1250, 1000, 250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total, prompt, completion := estimateAITokens(tc.promptChars, tc.completionChars)
			if total != tc.total || prompt != tc.prompt || completion != tc.completion {
				t.Fatalf("estimateAITokens(%d,%d) = (%d,%d,%d), want (%d,%d,%d)",
					tc.promptChars, tc.completionChars, total, prompt, completion, tc.total, tc.prompt, tc.completion)
			}
		})
	}
}

// TestAIPointsForTokens 分桶换算：ceil(tokens/1000)×10，下限 5 上限 100。
func TestAIPointsForTokens(t *testing.T) {
	cases := []struct {
		name   string
		tokens int
		want   int
	}{
		{"零 tokens 走下限", 0, 5},
		{"不足一桶仍为一桶", 1, 10},
		{"整桶", 1000, 10},
		{"跨桶 ceil", 1001, 20},
		{"接近上限", 9900, 100},
		{"超上限封顶", 11000, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := aiPointsForTokens(tc.tokens); got != tc.want {
				t.Fatalf("aiPointsForTokens(%d) = %d, want %d", tc.tokens, got, tc.want)
			}
		})
	}
}

// TestAIPreflight 余额预检：≥下限放行，<下限返回哨兵错误。
func TestAIPreflight(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 5)

	if err := svc.AIPreflight(uid); err != nil {
		t.Fatalf("余额恰为下限应放行, got %v", err)
	}
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", uid).UpdateColumn("points_balance", 4).Error; err != nil {
		t.Fatalf("调整余额失败: %v", err)
	}
	if err := svc.AIPreflight(uid); !errors.Is(err, ErrInsufficientPoints) {
		t.Fatalf("余额低于下限应返回 ErrInsufficientPoints, got %v", err)
	}
}

// TestDeductAIBillingOrchestration 端到端编排：长度事实 → 估算 → 分桶 → 扣费 →
// usage 数据面（points/total_tokens/balance/prompt_tokens/completion_tokens）。
func TestDeductAIBillingOrchestration(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 1000)

	// 4000+4000 字符 → 2000 tokens → ceil(2000/1000)×10 = 20 积分
	res, err := svc.DeductAI(context.Background(), uid, "req-orchestration", 4000, 4000)
	if err != nil {
		t.Fatalf("扣费失败: %v", err)
	}
	if res.Points != 20 || res.TotalTokens != 2000 || res.PromptTokens != 1000 || res.CompletionTokens != 1000 {
		t.Fatalf("usage 数据面不符: %+v", res)
	}
	if res.Balance != 980 {
		t.Fatalf("扣费后余额应为 980, got %d", res.Balance)
	}
	if got := ledgerCount(t, db, "user_id = ? AND ref_id = ? AND reason = ?", uid, "req-orchestration", "ai_tokens"); got != 1 {
		t.Fatalf("ai_tokens 流水应恰一行, got %d", got)
	}
}

// TestDeductAIFallbackThresholdSinglePoint 兜底阈值单点：合计 <10 字符 → 100 tokens →
// 一桶 10 积分（handler 侧不再重复估算，同样的长度事实只可能得到同样的账）。
func TestDeductAIFallbackThresholdSinglePoint(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 100)

	res, err := svc.DeductAI(context.Background(), uid, "req-tiny", 8, 0)
	if err != nil {
		t.Fatalf("扣费失败: %v", err)
	}
	if res.TotalTokens != 100 || res.Points != 10 {
		t.Fatalf("兜底应为 100 tokens / 10 积分: %+v", res)
	}
}

// TestDeductAIInsufficientBalanceSentinel 余额不足返回哨兵错误（供 errors.Is 消费），
// 无流水落库。
func TestDeductAIInsufficientBalanceSentinel(t *testing.T) {
	svc, db := newPointsSvc(t)
	uid := seedUserWithBalance(t, db, 3)

	res, err := svc.DeductAI(context.Background(), uid, "req-broke", 0, 4000)
	if res != nil || !errors.Is(err, ErrInsufficientPoints) {
		t.Fatalf("余额不足应返回 ErrInsufficientPoints, got res=%v err=%v", res, err)
	}
	if got := ledgerCount(t, db, "user_id = ?", uid); got != 0 {
		t.Fatalf("失败扣减不得留下流水, got %d 行", got)
	}
}
