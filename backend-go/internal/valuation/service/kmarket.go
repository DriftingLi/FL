// Package service 实现核心业务逻辑
// 本文件：市场系数 Km
// Demo 阶段直接从数据库 coefficient_configs 读取 k_market 配置
// 真实场景可按行业供需指数动态调整
package service

// CalcKMarket 读取市场系数 Km
func CalcKMarket(loader *CoefficientLoader) (float64, error) {
	return loader.Get(KeyKMarket)
}
