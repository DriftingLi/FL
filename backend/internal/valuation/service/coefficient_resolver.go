// Package service 实现核心业务逻辑
// 本文件：系数解析单一 interface（ADR-0013 候选 3）——把「读系数 + 兜底」的 4 形态
// （ConfigReader.Get / ConfigResolver.ReadFloat / 包级 readWithFallback / khours 手写兜底）
// 收敛为单一 CoefficientResolver：Get 按 key 读系数，ReadFloat 失败或非正数时兜底。
package service

import "context"

// CoefficientResolver 系数解析单一 interface。
// CoefficientProvider（逐 key 实时查 DB）与 CoefficientSnapshot（一次全表快照）都实现它。
type CoefficientResolver interface {
	// Get 按 key 读系数；缺失返回错误。
	Get(ctx context.Context, key string) (float64, error)
	// ReadFloat 读系数，失败或非正数时返回 fallback 默认值。
	ReadFloat(ctx context.Context, key string, fallback float64) float64
}

// coefReadFloat 系数读取兜底单点：resolver 为 nil / Get 失败 / 非正数 → fallback。
// ReadFloat 的实现（provider 与 snapshot）与需要容错兜底的消费点（kcondition）共用此
// 内部 helper，消除「读系数 + 兜底」的形态漂移。
func coefReadFloat(ctx context.Context, resolver CoefficientResolver, key string, fallback float64) float64 {
	if resolver == nil {
		return fallback
	}
	v, err := resolver.Get(ctx, key)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
