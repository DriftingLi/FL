// Package service 实现核心业务逻辑
// 本文件：测试辅助函数，用于在单元测试中快速构造各加载器的内存数据
// 避免单元测试对真实数据库的依赖
package service

import (
	"forklift-training/internal/valuation/model"
)

// newTestCoefficientLoader 构造带默认测试数据的 CoefficientLoader
// 数值与数据库 seed.sql 中插入的初始数据一致
func newTestCoefficientLoader() *CoefficientLoader {
	return &CoefficientLoader{
		queries: nil,
		cached: map[string]float64{
			KeyLambdaElectric:   0.12,
			KeyLambdaCombustion: 0.10,
			KeyWWorkCondition:   0.20,
			KeyWBrand:           0.20,
			KeyWCondition:       0.50,
			KeyWMarket:          0.10,
			KeyKMarket:          1.00,
			KeyConfidenceRange:  0.05,
		},
	}
}

// newTestBrandLoader 构造带默认测试数据的 BrandLoader
// 覆盖四个档次的代表品牌
func newTestBrandLoader() *BrandLoader {
	return &BrandLoader{
		queries: nil,
		byName: map[string]model.BrandInfo{
			"丰田":   {ID: 1, Name: "丰田", Tier: "tier1_intl", KBrand: 1.10},
			"林德":   {ID: 2, Name: "林德", Tier: "tier1_intl", KBrand: 1.10},
			"海斯特": {ID: 3, Name: "海斯特", Tier: "tier2_intl", KBrand: 1.00},
			"合力":   {ID: 4, Name: "合力", Tier: "tier1_domestic", KBrand: 0.95},
			"比亚迪": {ID: 5, Name: "比亚迪", Tier: "tier2_domestic", KBrand: 0.85},
		},
	}
}

// newTestPartConfigLoader 构造带最小化测试数据的 PartConfigLoader
// 包含电动与内燃两套基础配置，便于端到端测试
func newTestPartConfigLoader() *PartConfigLoader {
	return &PartConfigLoader{
		queries: nil,
		cache: map[model.ForkliftType][]model.PartConfigInfo{
			model.ForkliftTypeElectric: {
				{CategoryCode: "motor", CategoryName: "电机系统", CategoryWeight: 0.20, ItemCode: "m1", ItemName: "左电机", ItemWeight: 0.5},
				{CategoryCode: "motor", CategoryName: "电机系统", CategoryWeight: 0.20, ItemCode: "m2", ItemName: "右电机", ItemWeight: 0.5},
				{CategoryCode: "battery", CategoryName: "蓄电池", CategoryWeight: 0.80, ItemCode: "b1", ItemName: "电池", ItemWeight: 1.0},
			},
			model.ForkliftTypeCombustion: {
				{CategoryCode: "engine", CategoryName: "发动机", CategoryWeight: 0.22, ItemCode: "e1", ItemName: "发动机启动", ItemWeight: 0.5},
				{CategoryCode: "engine", CategoryName: "发动机", CategoryWeight: 0.22, ItemCode: "e2", ItemName: "怠速", ItemWeight: 0.5},
				{CategoryCode: "body", CategoryName: "车身", CategoryWeight: 0.78, ItemCode: "bd1", ItemName: "车身", ItemWeight: 1.0},
			},
		},
	}
}
