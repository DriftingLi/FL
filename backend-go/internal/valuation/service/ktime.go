// Package service 实现核心业务逻辑
// 本文件：时间衰减系数 Kt
// 公式：Kt = e^(-λ·age)，age = sale_year - factory_year
// λ 区分电动（默认 0.12）与内燃（默认 0.10），可通过 coefficient_configs 调整
package service

import (
	"context"
	"fmt"
	"math"

	"forklift-training/internal/valuation/model"
)

// KtResult 时间衰减系数计算结果
type KtResult struct {
	KTime float64 // 衰减系数
	Age   int     // 使用年限
}

// CalcKTime 计算时间衰减系数 Kt
// powerType: 动力类型（electric/combustion），决定使用哪个 λ
// factoryYear: 出厂年份
// saleYear: 成交年份
// provider: 系数提供者，提供 λ
func CalcKTime(ctx context.Context, powerType model.PowerType, factoryYear, saleYear int, provider *CoefficientProvider) (KtResult, error) {
	age := saleYear - factoryYear
	if age < 0 {
		return KtResult{}, model.ErrInvalidYear
	}

	// 根据动力类型选取衰减率 λ 的 key
	var key string
	switch powerType {
	case model.PowerTypeElectric:
		key = KeyLambdaElectric
	case model.PowerTypeCombustion:
		key = KeyLambdaCombustion
	default:
		return KtResult{}, fmt.Errorf("未知的动力类型: %s", powerType)
	}

	lambda, err := provider.Get(ctx, key)
	if err != nil {
		return KtResult{}, err
	}

	// Kt = e^(-λ·age)
	kt := math.Exp(-lambda * float64(age))
	return KtResult{KTime: kt, Age: age}, nil
}
