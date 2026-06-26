// Package service 实现核心业务逻辑
// 本文件：工况系数 Kw
// 按使用工况直接查表
package service

import "forklift-training/internal/valuation/model"

// workConditionFactors 工况 → 系数映射表
// 与开发方案一致：
//   仓储 1.05 / 港口 0.95 / 冷库 0.90 / 工地 0.85 / 其他 1.00
var workConditionFactors = map[model.WorkCondition]float64{
	model.WorkConditionStorage: 1.05,
	model.WorkConditionPort:    0.95,
	model.WorkConditionCold:    0.90,
	model.WorkConditionSite:    0.85,
	model.WorkConditionOther:   1.00,
}

// CalcKWork 计算工况系数 Kw
// 工况非法时返回 model.ErrInvalidWorkCondition
func CalcKWork(condition model.WorkCondition) (float64, error) {
	if !condition.IsValid() {
		return 0, model.ErrInvalidWorkCondition
	}
	return workConditionFactors[condition], nil
}
