// Package model 定义业务层错误
package model

import "errors"

// 业务错误定义（用于 service/handler 透传）
var (
	// ErrInvalidForkliftType 叉车类型非法
	ErrInvalidForkliftType = errors.New("叉车类型非法：仅支持 electric / combustion")
	// ErrInvalidWorkCondition 工况非法
	ErrInvalidWorkCondition = errors.New("工况非法：仅支持 仓储/港口/冷库/工地/其他")
	// ErrInvalidFuelType 燃料类型非法（仅内燃叉车校验）
	ErrInvalidFuelType = errors.New("燃料类型非法：仅支持 柴油/汽油/液化石油气(LPG)/天然气(CNG)")
	// ErrInvalidOriginalPrice 原始价格非法
	ErrInvalidOriginalPrice = errors.New("原始购买价格必须大于 0")
	// ErrInvalidYear 年份非法
	ErrInvalidYear = errors.New("年份非法：成交年份必须 ≥ 购置年份")
	// ErrInvalidUsageHours 累计小时数非法
	ErrInvalidUsageHours = errors.New("累计使用小时数必须 ≥ 0")
	// ErrInvalidItemStatus 部件状态非法
	ErrInvalidItemStatus = errors.New("部件状态非法")
	// ErrBrandNotFound 品牌未找到
	ErrBrandNotFound = errors.New("品牌未找到")
	// ErrCoefficientNotFound 系数配置未找到
	ErrCoefficientNotFound = errors.New("系数配置未找到")
	// ErrItemsEmpty 部件状态列表为空
	ErrItemsEmpty = errors.New("部件状态列表不能为空")
	// ErrPartConfigMissing 部件配置缺失
	ErrPartConfigMissing = errors.New("部件配置缺失")
)
