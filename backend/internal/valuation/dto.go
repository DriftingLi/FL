// Package valuation 残值评估域（/api/valuation/*：字典 / 评估 / 电池 RUL / 报告 / 平行 auth / 管理端 CRUD）。
//
// 本包是 internal/<域> 形态（ADR-0070 决策 3）：HTTP 出口住在 handler*.go，域实现与 DTO 同包。
// 这里的 DTO 与 HTTP 请求/响应、数据库实体解耦，便于前后端对接与单元测试。
package valuation

// PowerType 动力类型枚举（从 vehicle_types.power_type 派生）
type PowerType string

const (
	// PowerTypeElectric 电动
	PowerTypeElectric PowerType = "electric"
	// PowerTypeCombustion 内燃
	PowerTypeCombustion PowerType = "combustion"
)

// EvaluationRequest 评估请求 DTO（HTTP 入参）
// 与 evaluations 表字段一一对应，覆盖重构后的全部输入字段
type EvaluationRequest struct {
	// 字典字段
	Brand        string  `json:"brand"          binding:"required"` // 品牌
	VehicleType  string  `json:"vehicle_type"   binding:"required"` // 车型
	Series       string  `json:"series"         binding:"required"` // 系列
	Tonnage      float64 `json:"tonnage"        binding:"required"` // 吨位
	ConfigType   string  `json:"config_type"    binding:"required"` // 配置类型
	MastType     string  `json:"mast_type"      binding:"required"` // 门架类型
	MastHeightMM int     `json:"mast_height_mm"`                    // 门架高度(mm)，0 表示"无"，由 Validate 校验
	// 使用信息
	FactoryYear   int  `json:"factory_year"   binding:"required"` // 出厂年份
	SaleYear      int  `json:"sale_year"      binding:"required"` // 成交年份
	UsageHours    int  `json:"usage_hours"`                       // 累计使用小时数，0 表示新车，由 Validate 校验
	OriginalPaint bool `json:"original_paint"`                    // 是否原厂漆
	// 区域信息
	Province string `json:"province" binding:"required"` // 省份
	City     string `json:"city"     binding:"required"` // 城市
	// 证件状态
	HasLicensePlate            bool `json:"has_license_plate"`            // 是否有车牌
	HasRegistrationCertificate bool `json:"has_registration_certificate"` // 是否有登记证
	HasMaintenanceRecords      bool `json:"has_maintenance_records"`      // 是否有维保记录
	// 车况
	ConditionRating string `json:"condition_rating" binding:"required"` // 车况评级 A/B/C/D/E
}

// TrialRequest 参考试算输入。指针区分未知与明确的 false、0、"无"、"其它"。
type TrialRequest struct {
	RequestID    string  `json:"request_id"`
	SnapshotID   string  `json:"snapshot_id,omitempty" extensions:"x-optional"`
	Brand        string  `json:"brand"`
	VehicleType  string  `json:"vehicle_type"`
	Tonnage      float64 `json:"tonnage"`
	FactoryYear  *int    `json:"factory_year,omitempty" extensions:"x-optional"`
	FactoryMin   *int    `json:"factory_year_min,omitempty" extensions:"x-optional"`
	FactoryMax   *int    `json:"factory_year_max,omitempty" extensions:"x-optional"`
	SaleYear     int     `json:"sale_year,omitempty" extensions:"x-optional"`
	Series       *string `json:"series,omitempty" extensions:"x-optional"`
	ConfigType   *string `json:"config_type,omitempty" extensions:"x-optional"`
	MastType     *string `json:"mast_type,omitempty" extensions:"x-optional"`
	MastHeightMM *int    `json:"mast_height_mm,omitempty" extensions:"x-optional"`
	UsageHours   *int    `json:"usage_hours,omitempty" extensions:"x-optional"`
	Condition    *string `json:"condition_rating,omitempty" extensions:"x-optional"`
	Province     *string `json:"province,omitempty" extensions:"x-optional"`
	City         *string `json:"city,omitempty" extensions:"x-optional"`

	OriginalPaint              *bool `json:"original_paint,omitempty" extensions:"x-optional"`
	HasLicensePlate            *bool `json:"has_license_plate,omitempty" extensions:"x-optional"`
	HasRegistrationCertificate *bool `json:"has_registration_certificate,omitempty" extensions:"x-optional"`
	HasMaintenanceRecords      *bool `json:"has_maintenance_records,omitempty" extensions:"x-optional"`

	Policy string `json:"policy"`
}

// TrialResponse 参考试算卡。needs_info 不暴露价格区间。
type TrialResponse struct {
	RequestID     string            `json:"request_id"`
	SnapshotID    string            `json:"snapshot_id,omitempty" extensions:"x-optional"`
	Status        string            `json:"status"`
	ReasonCode    string            `json:"reason_code,omitempty" extensions:"x-optional"`
	Message       string            `json:"message,omitempty" extensions:"x-optional"`
	Input         TrialRequest      `json:"input"`
	Assumptions   []string          `json:"assumptions" nullability:"nonnil"`
	Candidates    []TrialCandidate  `json:"candidates" nullability:"nonnil"`
	MissingFields []string          `json:"missing_fields" nullability:"nonnil"`
	Interval      *TrialInterval    `json:"interval,omitempty" extensions:"x-optional"`
	WidthRatio    *float64          `json:"width_ratio,omitempty" extensions:"x-optional"`
	Snapshot      TrialSnapshotMeta `json:"snapshot"`
}

type TrialCandidate struct {
	OriginalPrice
}

type TrialInterval struct {
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	Mid        float64 `json:"mid"`
	Width      float64 `json:"width"`
	WidthRatio float64 `json:"width_ratio"`
}

type TrialSnapshotMeta struct {
	Hash           string `json:"hash"`
	SHA7           string `json:"sha7"`
	FormulaVersion string `json:"formula_version"`
	FormulaHash    string `json:"formula_hash"`
	Policy         string `json:"policy"`
	PolicyVersion  string `json:"policy_version"`
}

// Validate 业务级参数校验（在 binding 之后补充校验）
// 支持字段值为 "无"（字符串）或 0（mast_height_mm）表示该属性不适用
func (r *EvaluationRequest) Validate() error {
	if r.Brand == "" || r.VehicleType == "" || r.Series == "" {
		return ErrInvalidDictField
	}
	if r.ConfigType == "" || r.MastType == "" {
		return ErrInvalidDictField
	}
	if r.Tonnage <= 0 {
		return ErrInvalidTonnage
	}
	// mast_height_mm 允许 0（表示 "无"）
	if r.MastHeightMM < 0 {
		return ErrInvalidMastHeight
	}
	if r.FactoryYear < 1900 || r.SaleYear < r.FactoryYear {
		return ErrInvalidYear
	}
	if r.UsageHours < 0 {
		return ErrInvalidUsageHours
	}
	if r.ConditionRating == "" {
		return ErrInvalidConditionRating
	}
	if r.Province == "" || r.City == "" {
		return ErrInvalidRegion
	}
	return nil
}

// EvaluationResult 评估计算结果（业务层）
// 包含全部输入字段、查询得到的基准价、各 K 系数、最终残值与置信区间
type EvaluationResult struct {
	// 输入参数快照（与 EvaluationRequest 字段一致，便于落库）
	EvaluationRequest

	FormulaVersion string `json:"formula_version"`

	// 基准原价（从 original_prices 查询得到）
	OriginalPrice float64 `json:"original_price"`

	// 动力类型（从 vehicle_types 派生，决定 Kt 用哪个 λ）
	PowerType PowerType `json:"power_type"`

	// 各 K 系数
	KTime         float64 `json:"k_time"`
	KHours        float64 `json:"k_hours"`
	KBrand        float64 `json:"k_brand"`
	KCondition    float64 `json:"k_condition"`
	KMarket       float64 `json:"k_market"`
	KTimeAdjusted float64 `json:"k_time_adjusted"`

	// 最终结果
	EstimatedValue float64 `json:"estimated_value"`
	ConfidenceLow  float64 `json:"confidence_low"`
	ConfidenceHigh float64 `json:"confidence_high"`

	// 本次评估使用的 λ 值（评估时点锁定，供前端走势图数据驱动）
	LambdaElectric   float64 `json:"lambda_electric"`
	LambdaCombustion float64 `json:"lambda_combustion"`

	// 未来价值曲线锚点（ADR-0012 §8）：future(n) = estimated × decay_anchor^n，
	// 后端为公式唯一实现，评估时点锁定
	DecayAnchor float64 `json:"decay_anchor"`

	// 6 维度评分（固定顺序：出厂时间/使用强度/品牌价值/市场需求/车辆情况，便于前端展示）
	DimensionScores []DimensionScore `json:"dimension_scores"`
	// 文本建议
	Suggestions []string `json:"suggestions"`
}

// 五维系数标签（唯一事实源）：详情接口 BuildDimensionScores 与 PDF 雷达图共用。
// 变更标签需同步评估呈现契约（前端雷达图数据驱动，后端为唯一来源）。
const (
	DimensionLabelTime      = "出厂时间"
	DimensionLabelHours     = "使用强度"
	DimensionLabelBrand     = "品牌价值"
	DimensionLabelMarket    = "市场需求"
	DimensionLabelCondition = "车辆情况"
)

// DimensionLabels 五维标签的固定展示顺序（雷达图顺时针从顶部开始，间隔 72°）。
var DimensionLabels = []string{
	DimensionLabelTime,
	DimensionLabelHours,
	DimensionLabelBrand,
	DimensionLabelMarket,
	DimensionLabelCondition,
}

// DimensionScore 维度评分条目（label + value）
type DimensionScore struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// EvaluationDetail 评估详情（GET /evaluations/:id 返回）
// 包含全部输入字段 + 结果字段 + ID 与时间戳
type EvaluationDetail struct {
	// 主键与时间戳
	ID        int64  `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// 输入字段（与 EvaluationRequest 一致）
	Brand                      string  `json:"brand"`
	VehicleType                string  `json:"vehicle_type"`
	Series                     string  `json:"series"`
	Tonnage                    float64 `json:"tonnage"`
	ConfigType                 string  `json:"config_type"`
	MastType                   string  `json:"mast_type"`
	MastHeightMM               int     `json:"mast_height_mm"`
	FactoryYear                int     `json:"factory_year"`
	SaleYear                   int     `json:"sale_year"`
	UsageHours                 int     `json:"usage_hours"`
	OriginalPaint              bool    `json:"original_paint"`
	Province                   string  `json:"province"`
	City                       string  `json:"city"`
	HasLicensePlate            bool    `json:"has_license_plate"`
	HasRegistrationCertificate bool    `json:"has_registration_certificate"`
	HasMaintenanceRecords      bool    `json:"has_maintenance_records"`
	ConditionRating            string  `json:"condition_rating"`

	// 结果字段
	FormulaVersion  string           `json:"formula_version"`
	OriginalPrice   float64          `json:"original_price"`
	KTime           float64          `json:"k_time"`
	KHours          float64          `json:"k_hours"`
	KBrand          float64          `json:"k_brand"`
	KCondition      float64          `json:"k_condition"`
	KMarket         float64          `json:"k_market"`
	KTimeAdjusted   float64          `json:"k_time_adjusted"`
	EstimatedValue  float64          `json:"estimated_value"`
	ConfidenceLow   float64          `json:"confidence_low"`
	ConfidenceHigh  float64          `json:"confidence_high"`
	ReportPdfPath   string           `json:"report_pdf_path,omitempty" extensions:"x-optional"` // omitempty：键可能整个不存在
	DimensionScores []DimensionScore `json:"dimension_scores" nullability:"nonnil"`
	// 评估时点锁定的建议与 λ 值（ADR-0004 评估事实性）
	Suggestions      []string `json:"suggestions" nullability:"nonnil"`
	LambdaElectric   float64  `json:"lambda_electric"`
	LambdaCombustion float64  `json:"lambda_combustion"`
	// 未来价值曲线锚点（ADR-0012 §8，评估时点锁定）
	DecayAnchor float64 `json:"decay_anchor"`
}

// EvaluationResponse 创建评估响应 DTO（HTTP 出参）
// 返回 ID + 输入参数 + 全部 K 系数 + 残值 + 置信区间 + 维度评分 + 建议。
// 输入参数随创建响应一并返回（与详情接口同源，ADR-0004）：
// 匿名用户提交评估后可直接渲染结果页，无需调用需登录的详情接口。
type EvaluationResponse struct {
	ID int64 `json:"id"`
	// 输入参数（与 EvaluationRequest / evaluations 表字段一致）
	Brand                      string  `json:"brand"`
	VehicleType                string  `json:"vehicle_type"`
	Series                     string  `json:"series"`
	Tonnage                    float64 `json:"tonnage"`
	ConfigType                 string  `json:"config_type"`
	MastType                   string  `json:"mast_type"`
	MastHeightMM               int     `json:"mast_height_mm"`
	FactoryYear                int     `json:"factory_year"`
	SaleYear                   int     `json:"sale_year"`
	UsageHours                 int     `json:"usage_hours"`
	OriginalPaint              bool    `json:"original_paint"`
	Province                   string  `json:"province"`
	City                       string  `json:"city"`
	HasLicensePlate            bool    `json:"has_license_plate"`
	HasRegistrationCertificate bool    `json:"has_registration_certificate"`
	HasMaintenanceRecords      bool    `json:"has_maintenance_records"`
	ConditionRating            string  `json:"condition_rating"`
	// 计算结果
	FormulaVersion  string           `json:"formula_version"`
	OriginalPrice   float64          `json:"original_price"`
	KTime           float64          `json:"k_time"`
	KHours          float64          `json:"k_hours"`
	KBrand          float64          `json:"k_brand"`
	KCondition      float64          `json:"k_condition"`
	KMarket         float64          `json:"k_market"`
	KTimeAdjusted   float64          `json:"k_time_adjusted"`
	EstimatedValue  float64          `json:"estimated_value"`
	ConfidenceLow   float64          `json:"confidence_low"`
	ConfidenceHigh  float64          `json:"confidence_high"`
	DimensionScores []DimensionScore `json:"dimension_scores" nullability:"nonnil"`
	Suggestions     []string         `json:"suggestions" nullability:"nonnil"`
	// 本次评估使用的 λ 值（评估时点锁定，供前端走势图数据驱动）
	LambdaElectric   float64 `json:"lambda_electric"`
	LambdaCombustion float64 `json:"lambda_combustion"`
	// 未来价值曲线锚点（ADR-0012 §8，评估时点锁定）
	DecayAnchor float64 `json:"decay_anchor"`
}

// CalcWeights 加权权重（用于 PDF 计算过程展示）
// 重构后残值公式不再使用加权求和（改为乘法），此结构体保留仅供 PDF 兼容签名
type CalcWeights struct {
	WWorkCondition float64 `json:"w_work_condition"`
	WBrand         float64 `json:"w_brand"`
	WCondition     float64 `json:"w_condition"`
	WMarket        float64 `json:"w_market"`
}

// EvaluationItemDTO 部件状态 DTO（保留兼容 PDF 旧签名，重构后不再使用）
type EvaluationItemDTO struct {
	CategoryCode   string  `json:"category_code"`
	CategoryName   string  `json:"category_name"`
	ItemCode       string  `json:"item_code"`
	ItemName       string  `json:"item_name"`
	Status         string  `json:"status"`
	CategoryWeight float64 `json:"category_weight"`
	ItemWeight     float64 `json:"item_weight"`
	Score          float64 `json:"score"`
}
