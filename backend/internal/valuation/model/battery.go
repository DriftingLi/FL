// Package model - 电池 RUL 评估模块 DTO
// 与现有 evaluation.go 物理隔离，独立维护
package model

import "errors"

// BatteryType 电池类型枚举
type BatteryType string

const (
	// BatteryTypeLFP 磷酸铁锂
	BatteryTypeLFP BatteryType = "lfp"
	// BatteryTypeNCM 三元锂
	BatteryTypeNCM BatteryType = "ncm"
	// BatteryTypeOther 其他类型
	BatteryTypeOther BatteryType = "other"
)

// IsValid 校验电池类型是否合法
func (t BatteryType) IsValid() bool {
	switch t {
	case BatteryTypeLFP, BatteryTypeNCM, BatteryTypeOther:
		return true
	}
	return false
}

// CycleData 单次循环充放电数据
type CycleData struct {
	// CycleIndex 循环序号（从 1 开始）
	CycleIndex int `json:"cycle_index" binding:"required"`
	// VoltageSeries 充电阶段电压时序（V），长度应与 CurrentSeries 一致
	VoltageSeries []float64 `json:"voltage_series" binding:"required"`
	// CurrentSeries 充电阶段电流时序（A），长度应与 VoltageSeries 一致
	CurrentSeries []float64 `json:"current_series" binding:"required"`
	// Capacity 本次循环放电容量（Ah）
	Capacity float64 `json:"capacity" binding:"required"`
}

// CreateBatteryRequest 电池评估请求 DTO
type CreateBatteryRequest struct {
	// BatteryType 电池类型
	BatteryType BatteryType `json:"battery_type" binding:"required"`
	// BatteryModel 电池型号（可选）
	BatteryModel string `json:"battery_model"`
	// Cycles 充放电循环数据，至少 10 条
	Cycles []CycleData `json:"cycles" binding:"required"`
}

// Validate 业务级校验
func (r *CreateBatteryRequest) Validate() error {
	if !r.BatteryType.IsValid() {
		return ErrInvalidBatteryType
	}
	if len(r.Cycles) < 10 {
		return ErrBatteryCyclesInsufficient
	}
	for i, c := range r.Cycles {
		if c.CycleIndex <= 0 {
			return ErrInvalidCycleIndex
		}
		if len(c.VoltageSeries) == 0 || len(c.CurrentSeries) == 0 {
			return ErrInvalidSeriesLength
		}
		if len(c.VoltageSeries) != len(c.CurrentSeries) {
			return ErrSeriesLengthMismatch
		}
		if c.Capacity <= 0 {
			return ErrInvalidCapacity
		}
		_ = i
	}
	return nil
}

// FeatureVector 20 维特征向量定长数组
type FeatureVector [20]float64

// AsSlice 转为切片便于 JSON 序列化
func (f FeatureVector) AsSlice() []float64 {
	return f[:]
}

// CycleFeature 周期特征记录（落库与 API 共用）
type CycleFeature struct {
	ID            int64         `json:"id,omitempty"`
	EvaluationID  int64         `json:"evaluation_id"`
	CycleIndex    int           `json:"cycle_index"`
	FeatureVector FeatureVector `json:"feature_vector"`
	RawStats      RawStats      `json:"raw_stats"`
	SohAtCycle    float64       `json:"soh_at_cycle"`
}

// RawStats 原始统计摘要
type RawStats struct {
	VoltageMean float64 `json:"voltage_mean"`
	VoltageStd  float64 `json:"voltage_std"`
	CurrentMean float64 `json:"current_mean"`
	CurrentStd  float64 `json:"current_std"`
	Capacity    float64 `json:"capacity"`
	CCDuration  int     `json:"cc_duration"`     // 恒流段采样点数
	CVDuration  int     `json:"cv_duration"`     // 恒压段采样点数
	ICPeak      float64 `json:"ic_peak"`         // 增量容量峰值
	ICPeakVolt  float64 `json:"ic_peak_voltage"` // 峰值对应电压
}

// BatteryEvaluation 电池评估主记录
type BatteryEvaluation struct {
	ID                int64               `json:"id"`
	BatteryType       BatteryType         `json:"battery_type"`
	BatteryModel      string              `json:"battery_model"`
	CycleCount        int                 `json:"cycle_count"`
	RulCycles         int                 `json:"rul_cycles"`
	SohPercent        float64             `json:"soh_percent"`
	Confidence        float64             `json:"confidence"`
	ConfidenceLow     int                 `json:"confidence_low"`
	ConfidenceHigh    int                 `json:"confidence_high"`
	FeatureImportance []FeatureImportance `json:"feature_importance,omitempty"`
	ReportPdfPath     string              `json:"report_pdf_path"`
	CreatedAt         string              `json:"created_at"`
	UpdatedAt         string              `json:"updated_at"`
	// 详情时填充
	CycleFeatures []CycleFeature `json:"cycle_features,omitempty"`
	// 评估建议（基于 SOH/RUL/电池类型生成）
	Suggestions []string `json:"suggestions,omitempty"`
}

// FeatureImportance 特征重要性条目
type FeatureImportance struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	Group      string  `json:"group"`
	Weight     float64 `json:"weight"`
	Normalized float64 `json:"normalized"` // 归一化到 0~1
}

// BatteryEvaluationSummary 列表查询的摘要项
type BatteryEvaluationSummary struct {
	ID           int64       `json:"id"`
	BatteryType  BatteryType `json:"battery_type"`
	BatteryModel string      `json:"battery_model"`
	CycleCount   int         `json:"cycle_count"`
	RulCycles    int         `json:"rul_cycles"`
	SohPercent   float64     `json:"soh_percent"`
	Confidence   float64     `json:"confidence"`
	CreatedAt    string      `json:"created_at"`
}

// CreateBatteryResponse 创建评估响应
type CreateBatteryResponse struct {
	EvaluationID   int64       `json:"evaluation_id"`
	BatteryType    BatteryType `json:"battery_type"`
	CycleCount     int         `json:"cycle_count"`
	RulCycles      int         `json:"rul_cycles"`
	SohPercent     float64     `json:"soh_percent"`
	Confidence     float64     `json:"confidence"`
	ConfidenceLow  int         `json:"confidence_low"`
	ConfidenceHigh int         `json:"confidence_high"`
	Suggestions    []string    `json:"suggestions"`
	CreatedAt      string      `json:"created_at"`
}

// ListBatteryResponse 列表查询响应
type ListBatteryResponse struct {
	Total int                        `json:"total"`
	Items []BatteryEvaluationSummary `json:"items"`
}

// BatteryReportResponse 报告生成响应
type BatteryReportResponse struct {
	EvaluationID int64  `json:"evaluation_id"`
	ReportPath   string `json:"report_path"`
	GeneratedAt  string `json:"generated_at"`
}

// 电池模块业务错误
var (
	// ErrInvalidBatteryType 电池类型非法
	ErrInvalidBatteryType = errors.New("电池类型非法：仅支持 lfp / ncm / other")
	// ErrBatteryCyclesInsufficient 循环数据不足
	ErrBatteryCyclesInsufficient = errors.New("至少需要 10 个完整循环")
	// ErrInvalidCycleIndex 循环序号非法
	ErrInvalidCycleIndex = errors.New("循环序号必须 ≥ 1")
	// ErrInvalidSeriesLength 序列长度为 0
	ErrInvalidSeriesLength = errors.New("电压/电流序列不能为空")
	// ErrSeriesLengthMismatch 序列长度不匹配
	ErrSeriesLengthMismatch = errors.New("电压序列与电流序列长度必须一致")
	// ErrInvalidCapacity 容量非法
	ErrInvalidCapacity = errors.New("循环容量必须大于 0")
	// ErrBatteryEvalNotFound 评估记录未找到
	ErrBatteryEvalNotFound = errors.New("电池评估记录未找到")
)
