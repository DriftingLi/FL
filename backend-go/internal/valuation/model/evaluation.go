// Package model 定义业务层 DTO（数据传输对象）
// 与 HTTP 请求/响应、数据库实体解耦，便于前后端对接与单元测试
package model

// ForkliftType 叉车类型枚举
type ForkliftType string

const (
	// ForkliftTypeElectric 电动叉车
	ForkliftTypeElectric ForkliftType = "electric"
	// ForkliftTypeCombustion 内燃叉车
	ForkliftTypeCombustion ForkliftType = "combustion"
)

// IsValid 校验叉车类型是否合法
func (t ForkliftType) IsValid() bool {
	return t == ForkliftTypeElectric || t == ForkliftTypeCombustion
}

// WorkCondition 使用工况枚举
type WorkCondition string

const (
	WorkConditionStorage    WorkCondition = "仓储" // 仓储
	WorkConditionPort       WorkCondition = "港口" // 港口
	WorkConditionCold       WorkCondition = "冷库" // 冷库
	WorkConditionSite       WorkCondition = "工地" // 工地
	WorkConditionOther      WorkCondition = "其他" // 其他
)

// IsValid 校验工况是否合法
func (w WorkCondition) IsValid() bool {
	switch w {
	case WorkConditionStorage, WorkConditionPort, WorkConditionCold,
		WorkConditionSite, WorkConditionOther:
		return true
	}
	return false
}

// FuelType 燃料类型枚举（仅内燃叉车使用）
type FuelType string

const (
	FuelTypeDiesel  FuelType = "柴油"
	FuelTypeGasoline FuelType = "汽油"
	FuelTypeLPG     FuelType = "液化石油气(LPG)"
	FuelTypeCNG     FuelType = "天然气(CNG)"
)

// IsValid 校验燃料类型是否合法
func (f FuelType) IsValid() bool {
	switch f {
	case FuelTypeDiesel, FuelTypeGasoline, FuelTypeLPG, FuelTypeCNG:
		return true
	}
	return false
}

// ItemStatus 部件状态枚举
type ItemStatus string

const (
	// ItemStatusNormal 正常 - Kcij = 1.0
	ItemStatusNormal ItemStatus = "normal"
	// ItemStatusSlightWear 轻微磨损 - Kcij = 0.85
	ItemStatusSlightWear ItemStatus = "slight_wear"
	// ItemStatusNeedRepair 需维修 - Kcij = 0.6
	ItemStatusNeedRepair ItemStatus = "need_repair"
	// ItemStatusNeedReplace 需更换 - Kcij = 0.3
	ItemStatusNeedReplace ItemStatus = "need_replace"
)

// IsValid 校验部件状态是否合法
func (s ItemStatus) IsValid() bool {
	switch s {
	case ItemStatusNormal, ItemStatusSlightWear, ItemStatusNeedRepair, ItemStatusNeedReplace:
		return true
	}
	return false
}

// Score 将状态映射为评分系数
func (s ItemStatus) Score() float64 {
	switch s {
	case ItemStatusNormal:
		return 1.0
	case ItemStatusSlightWear:
		return 0.85
	case ItemStatusNeedRepair:
		return 0.6
	case ItemStatusNeedReplace:
		return 0.3
	default:
		return 0.0
	}
}

// ItemInput 用户提交的部件状态条目
type ItemInput struct {
	ItemCode string    `json:"item_code" binding:"required"` // 条目编码
	Status   ItemStatus `json:"status"     binding:"required"` // 状态
}

// EvaluationRequest 评估请求 DTO（HTTP 入参）
type EvaluationRequest struct {
	ForkliftType  ForkliftType  `json:"forklift_type" binding:"required"`     // 叉车类型
	Brand         string        `json:"brand"          binding:"required"`    // 品牌
	Model         string        `json:"model"`                                // 型号（可选）
	OriginalPrice float64       `json:"original_price" binding:"required"`    // 原始购买价格（万元）
	PurchaseYear  int           `json:"purchase_year"  binding:"required"`    // 购置年份
	SaleYear      int           `json:"sale_year"      binding:"required"`    // 成交年份
	UsageHours    int           `json:"usage_hours"    binding:"required"`    // 累计使用小时数
	WorkCondition WorkCondition `json:"work_condition" binding:"required"`    // 工况
	FuelType      FuelType      `json:"fuel_type"`                            // 燃料类型（仅内燃）
	CanDrive      bool          `json:"can_drive"`                             // 能否正常行驶
	HydraulicOk   bool          `json:"hydraulic_ok"`                          // 液压功能是否正常
	Items         []ItemInput   `json:"items"          binding:"required"`    // 部件状态列表
}

// Validate 业务级参数校验（在 binding 之后补充校验）
func (r *EvaluationRequest) Validate() error {
	if !r.ForkliftType.IsValid() {
		return ErrInvalidForkliftType
	}
	if !r.WorkCondition.IsValid() {
		return ErrInvalidWorkCondition
	}
	if r.ForkliftType == ForkliftTypeCombustion && !r.FuelType.IsValid() {
		return ErrInvalidFuelType
	}
	if r.OriginalPrice <= 0 {
		return ErrInvalidOriginalPrice
	}
	if r.PurchaseYear < 1900 || r.SaleYear < r.PurchaseYear {
		return ErrInvalidYear
	}
	if r.UsageHours < 0 {
		return ErrInvalidUsageHours
	}
	for i, it := range r.Items {
		if !it.Status.IsValid() {
			return ErrInvalidItemStatus
		}
		_ = i
	}
	return nil
}

// EvaluationResult 评估计算结果（业务层）
// 与持久化的 Evaluation 表结构对齐，便于直接落库
type EvaluationResult struct {
	// 输入参数
	ForkliftType  ForkliftType
	Brand         string
	Model         string
	OriginalPrice float64
	PurchaseYear  int
	SaleYear      int
	UsageHours    int
	WorkCondition WorkCondition
	FuelType      FuelType
	CanDrive      bool
	HydraulicOk   bool

	// 部件状态（含权重与评分快照，便于溯源）
	Items []ItemResult

	// 各系数
	KTime      float64
	KHours     float64
	KWork      float64
	KBrand     float64
	KCondition float64
	KMarket    float64

	// 最终结果
	EstimatedValue float64
	ConfidenceLow  float64
	ConfidenceHigh float64

	// 6 维度评分（中文标签 → 0~1）
	DimensionScores map[string]float64
	// 文本建议（基于评估结果生成）
	Suggestions []string
}

// ItemResult 评估结果中保存的部件状态（含权重与评分）
type ItemResult struct {
	CategoryCode   string
	CategoryName   string
	ItemCode       string
	ItemName       string
	Status         ItemStatus
	CategoryWeight float64
	ItemWeight     float64
	Score          float64
}

// EvaluationResponse 评估响应 DTO（HTTP 出参）
type EvaluationResponse struct {
	ID             int64              `json:"id"`              // 评估记录 ID（持久化后回填）
	KTime          float64            `json:"k_time"`          // 时间衰减系数
	KHours         float64            `json:"k_hours"`         // 使用强度系数
	KWork          float64            `json:"k_work"`          // 工况系数
	KBrand         float64            `json:"k_brand"`         // 品牌系数
	KCondition     float64            `json:"k_condition"`     // 车况系数
	KMarket        float64            `json:"k_market"`        // 市场系数
	EstimatedValue float64            `json:"estimated_value"` // 估算残值（万元）
	ConfidenceLow  float64            `json:"confidence_low"`  // 置信区间下限
	ConfidenceHigh float64            `json:"confidence_high"` // 置信区间上限
	OriginalPrice  float64            `json:"original_price"`  // 原始购买价格（万元），便于前端算残值率
	DimensionScores map[string]float64 `json:"dimension_scores"` // 6 维度评分（中文标签 → 0~1）
	Suggestions     []string          `json:"suggestions"`       // 文本建议
}

// EvaluationDetailResponse 评估详情响应（包含完整输入参数 + 计算结果 + 部件状态）
type EvaluationDetailResponse struct {
	ID             int64             `json:"id"`
	ForkliftType   ForkliftType      `json:"forklift_type"`
	Brand          string            `json:"brand"`
	Model          string            `json:"model"`
	OriginalPrice  float64           `json:"original_price"`
	PurchaseYear   int               `json:"purchase_year"`
	SaleYear       int               `json:"sale_year"`
	UsageHours     int               `json:"usage_hours"`
	WorkCondition  WorkCondition     `json:"work_condition"`
	FuelType       FuelType          `json:"fuel_type"`
	CanDrive       bool              `json:"can_drive"`
	HydraulicOk    bool              `json:"hydraulic_ok"`
	KTime          float64           `json:"k_time"`
	KHours         float64           `json:"k_hours"`
	KWork          float64           `json:"k_work"`
	KBrand         float64           `json:"k_brand"`
	KCondition     float64           `json:"k_condition"`
	KMarket        float64           `json:"k_market"`
	EstimatedValue float64           `json:"estimated_value"`
	ConfidenceLow  float64           `json:"confidence_low"`
	ConfidenceHigh float64           `json:"confidence_high"`
	ReportPdfPath  string            `json:"report_pdf_path"`
	Items          []EvaluationItemDTO `json:"items,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
	// 派生字段：从 K 系数 + 部件状态重建（详情接口实时计算，不入库）
	DimensionScores map[string]float64 `json:"dimension_scores"`
	Suggestions     []string          `json:"suggestions"`
}

// EvaluationItemDTO 部件状态 DTO
type EvaluationItemDTO struct {
	ID             int64     `json:"id"`
	CategoryCode   string    `json:"category_code"`
	CategoryName   string    `json:"category_name"`
	ItemCode       string    `json:"item_code"`
	ItemName       string    `json:"item_name"`
	Status         ItemStatus `json:"status"`
	CategoryWeight float64   `json:"category_weight"`
	ItemWeight     float64   `json:"item_weight"`
	Score          float64   `json:"score"`
}

// CalcWeights 加权权重（用于 PDF 计算过程展示，从 coefficient_configs 表加载）
type CalcWeights struct {
	WWorkCondition float64 `json:"w_work_condition"` // 工况权重 w₁
	WBrand         float64 `json:"w_brand"`          // 品牌权重 w₂
	WCondition     float64 `json:"w_condition"`      // 车况权重 w₃
	WMarket        float64 `json:"w_market"`         // 市场权重 w₄
}

// BrandInfo 品牌信息 DTO
type BrandInfo struct {
	ID     int32    `json:"id"`
	Name   string   `json:"name"`
	Tier   string   `json:"tier"`
	KBrand float64  `json:"k_brand"`
	Models []string `json:"models"`
}

// PartConfigInfo 部件配置 DTO（用于前端动态渲染）
type PartConfigInfo struct {
	CategoryCode   string  `json:"category_code"`
	CategoryName   string  `json:"category_name"`
	CategoryWeight float64 `json:"category_weight"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	ItemWeight     float64 `json:"item_weight"`
}

// CoefficientInfo 系数配置 DTO
type CoefficientInfo struct {
	Key         string  `json:"key"`
	Value       float64 `json:"value"`
	Description string  `json:"description"`
}
