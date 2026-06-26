// Package service 实现核心业务逻辑
// 本文件：时间衰减系数 Kt
// 公式：Kt = e^(-λ·t)，t 为使用年限（成交年份 - 购置年份）
// λ 区分电动（0.12）与内燃（0.10），电动因电池衰减更快
package service

import (
	"math"

	"forklift-training/internal/valuation/model"
)

// KtResult 时间衰减系数计算结果
type KtResult struct {
	KTime float64 // 衰减系数
	Years int     // 使用年限
}

// CalcKTime 计算时间衰减系数 Kt
// forkliftType: 叉车类型，决定使用哪个 λ
// purchaseYear: 购置年份
// saleYear: 成交年份
// loader: 系数加载器，提供 λ
// 使用年限 t < 0 时返回错误（成交年份早于购置年份）
func CalcKTime(forkliftType model.ForkliftType, purchaseYear, saleYear int, loader *CoefficientLoader) (KtResult, error) {
	t := saleYear - purchaseYear
	if t < 0 {
		return KtResult{}, model.ErrInvalidYear
	}

	// 根据叉车类型选取衰减率 λ
	var key string
	switch forkliftType {
	case model.ForkliftTypeElectric:
		key = KeyLambdaElectric
	case model.ForkliftTypeCombustion:
		key = KeyLambdaCombustion
	default:
		return KtResult{}, model.ErrInvalidForkliftType
	}

	lambda, err := loader.Get(key)
	if err != nil {
		return KtResult{}, err
	}

	// Kt = e^(-λ·t)
	kt := math.Exp(-lambda * float64(t))
	return KtResult{KTime: kt, Years: t}, nil
}
