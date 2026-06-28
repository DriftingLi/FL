// Package model 定义业务层错误
package model

import "errors"

// 业务错误定义（用于 service/handler 透传）
var (
	// ErrInvalidYear 年份非法：成交年份必须 ≥ 出厂年份
	ErrInvalidYear = errors.New("年份非法：成交年份必须 ≥ 出厂年份")
	// ErrInvalidUsageHours 累计小时数非法
	ErrInvalidUsageHours = errors.New("累计使用小时数必须 ≥ 0")
	// ErrInvalidTonnage 吨位非法
	ErrInvalidTonnage = errors.New("吨位必须大于 0")
	// ErrInvalidMastHeight 门架高度非法
	ErrInvalidMastHeight = errors.New("门架高度必须大于 0")
	// ErrInvalidDictField 字典字段不能为空
	ErrInvalidDictField = errors.New("字典字段不能为空")
	// ErrInvalidConditionRating 车况评级非法
	ErrInvalidConditionRating = errors.New("车况评级非法：仅支持 A/B/C/D/E")
	// ErrInvalidRegion 省份或城市非法
	ErrInvalidRegion = errors.New("省份与城市不能为空")
	// ErrBrandNotFound 品牌未找到
	ErrBrandNotFound = errors.New("品牌未找到")
	// ErrBrandTypeNotFound 品牌类型未找到
	ErrBrandTypeNotFound = errors.New("品牌类型未找到")
	// ErrVehicleTypeNotFound 车型未找到
	ErrVehicleTypeNotFound = errors.New("车型未找到")
	// ErrConditionRatingNotFound 车况评级未找到
	ErrConditionRatingNotFound = errors.New("车况评级未找到")
	// ErrOriginalPriceNotFound 基准原价未匹配到
	ErrOriginalPriceNotFound = errors.New("未匹配到基准原价：精确匹配与模糊匹配均未命中")
	// ErrCoefficientNotFound 系数配置未找到
	ErrCoefficientNotFound = errors.New("系数配置未找到")
	// ErrForbidden 管理员权限不足
	ErrForbidden = errors.New("权限不足：仅管理员可访问")
)
