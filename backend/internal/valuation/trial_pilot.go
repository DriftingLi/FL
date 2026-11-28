package valuation

import (
	"fmt"
	"math"
	"strings"
)

// TrialPolicyInput 管理端提交的精确组合 allowlist；全量替换时版本从 1 重新计算。
type TrialPolicyInput struct {
	Name                             string    `json:"policy_name" binding:"required"`
	Brand                            string    `json:"brand" binding:"required"`
	VehicleType                      string    `json:"vehicle_type" binding:"required"`
	Tonnage                          float64   `json:"tonnage" binding:"required"`
	MaxWidthRatio                    float64   `json:"max_width_ratio" binding:"required"`
	UsageHourRatios                  []float64 `json:"usage_hour_ratios" binding:"required"`
	ConditionRatings                 []string  `json:"condition_ratings" binding:"required"`
	OriginalPaintValues              []bool    `json:"original_paint_values" binding:"required"`
	HasLicensePlateValues            []bool    `json:"has_license_plate_values" binding:"required"`
	HasRegistrationCertificateValues []bool    `json:"has_registration_certificate_values" binding:"required"`
	HasMaintenanceRecordsValues      []bool    `json:"has_maintenance_records_values" binding:"required"`
	Enabled                          bool      `json:"enabled"`
}

// TrialControlRequest 更新独立全局开关；Enabled 零值即关闭。
type TrialControlRequest struct {
	Enabled bool `json:"enabled"`
}

// TrialControlResponse 返回当前全局开关与单调版本。
type TrialControlResponse struct {
	TrialControl
}

// TrialPoliciesResponse 管理端策略全量读面。
type TrialPoliciesResponse struct {
	Policies []TrialPolicy `json:"policies" nullability:"nonnil"`
}

// TrialPoliciesRequest 全量替换策略；空列表用于清空 allowlist。
type TrialPoliciesRequest struct {
	Policies []TrialPolicyInput `json:"policies" binding:"required"`
}

// TrialMetricEvent 仅保存组合、枚举情景与数值元数据，不保存请求标识或自由文本。
type TrialMetricEvent struct {
	OccurredAt       string  `json:"occurred_at"`
	Brand            string  `json:"brand"`
	VehicleType      string  `json:"vehicle_type"`
	Tonnage          float64 `json:"tonnage"`
	PolicyName       string  `json:"policy_name"`
	Scenario         string  `json:"scenario"`
	FormulaVersion   string  `json:"formula_version"`
	FormulaHash      string  `json:"formula_hash"`
	PolicyVersion    string  `json:"policy_version"`
	Denominator      int     `json:"denominator"`
	Priced           int     `json:"priced"`
	Covered          int     `json:"covered"`
	TooWide          int     `json:"too_wide"`
	UnpricedReason   string  `json:"unpriced_reason,omitempty"`
	FollowUpCount    int     `json:"follow_up_count"`
	DurationMS       int64   `json:"duration_ms"`
	ModelName        string  `json:"model_name,omitempty"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	CostMicros       int64   `json:"cost_micros"`
}

// TrialMetricSummary 按组合/情景/公式/策略版本聚合的管理读面。
type TrialMetricSummary struct {
	Brand            string           `json:"brand"`
	VehicleType      string           `json:"vehicle_type"`
	Tonnage          float64          `json:"tonnage"`
	PolicyName       string           `json:"policy_name"`
	Scenario         string           `json:"scenario"`
	FormulaVersion   string           `json:"formula_version"`
	FormulaHash      string           `json:"formula_hash"`
	PolicyVersion    string           `json:"policy_version"`
	Denominator      int64            `json:"denominator"`
	Priced           int64            `json:"priced"`
	Covered          int64            `json:"covered"`
	TooWide          int64            `json:"too_wide"`
	FollowUpCount    int64            `json:"follow_up_count"`
	DurationSumMS    int64            `json:"duration_sum_ms"`
	PromptTokens     int64            `json:"prompt_tokens"`
	CompletionTokens int64            `json:"completion_tokens"`
	CostMicros       int64            `json:"cost_micros"`
	UnpricedReasons  map[string]int64 `json:"unpriced_reasons" nullability:"nonnil"`
}

// TrialMetricsResponse 管理查询与 JSON 导出共用。
type TrialMetricsResponse struct {
	Items []TrialMetricSummary `json:"items" nullability:"nonnil"`
}

func validateTrialPolicyInput(policy TrialPolicyInput) error {
	policy.Name = strings.TrimSpace(policy.Name)
	policy.Brand = strings.TrimSpace(policy.Brand)
	policy.VehicleType = strings.TrimSpace(policy.VehicleType)
	if policy.Name != "demo" && policy.Name != "pilot" {
		return fmt.Errorf("policy_name 必须是 demo 或 pilot")
	}
	if policy.Brand == "" || len(policy.Brand) > 100 || policy.VehicleType == "" || len(policy.VehicleType) > 100 {
		return fmt.Errorf("品牌与车型必须非空且不超过 100 字符")
	}
	if policy.Tonnage <= 0 || math.IsNaN(policy.Tonnage) || math.IsInf(policy.Tonnage, 0) || policy.Tonnage > 99999999.99 {
		return fmt.Errorf("吨位必须为有限正数")
	}
	if policy.MaxWidthRatio <= 0 || policy.MaxWidthRatio > 1 || math.IsNaN(policy.MaxWidthRatio) || math.IsInf(policy.MaxWidthRatio, 0) {
		return fmt.Errorf("宽度上限必须在 (0,1] 内")
	}
	if len(policy.UsageHourRatios) == 0 || len(policy.ConditionRatings) == 0 {
		return fmt.Errorf("工时与车况情景不能为空")
	}
	for _, ratio := range policy.UsageHourRatios {
		if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
			return fmt.Errorf("工时比例必须为有限非负数")
		}
	}
	for _, rating := range policy.ConditionRatings {
		if strings.TrimSpace(rating) == "" {
			return fmt.Errorf("车况情景不能包含空值")
		}
	}
	for name, values := range map[string][]bool{
		"original_paint_values":               policy.OriginalPaintValues,
		"has_license_plate_values":            policy.HasLicensePlateValues,
		"has_registration_certificate_values": policy.HasRegistrationCertificateValues,
		"has_maintenance_records_values":      policy.HasMaintenanceRecordsValues,
	} {
		if len(values) == 0 {
			return fmt.Errorf("%s 情景不能为空", name)
		}
	}
	return nil
}

func validateTrialPolicySet(policies []TrialPolicyInput) error {
	seen := make(map[string]struct{}, len(policies))
	for _, policy := range policies {
		if err := validateTrialPolicyInput(policy); err != nil {
			return err
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%.2f", strings.TrimSpace(policy.Name), strings.TrimSpace(policy.Brand), strings.TrimSpace(policy.VehicleType), policy.Tonnage)
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("试算策略组合重复")
		}
		seen[key] = struct{}{}
	}
	return nil
}

// NewTrialMetricEvent 将 HTTP 结果投影为无 PII 指标；模型额度在当前纯公式链路中为零。
func NewTrialMetricEvent(req TrialRequest, result TrialResponse, durationMS int64) TrialMetricEvent {
	if durationMS < 0 {
		durationMS = 0
	}
	scenario := "masked"
	if trialOccludedDimensions(req) == 0 {
		scenario = "complete"
	} else if req.FactoryYear == nil && req.FactoryMin != nil && req.FactoryMax != nil {
		scenario = "year_range"
	}
	policyVersion := result.Snapshot.PolicyVersion
	if policyVersion == "" {
		policyVersion = "unresolved"
		if result.ReasonCode == TrialReasonGlobalDisabled {
			policyVersion = TrialReasonGlobalDisabled
		}
	}
	event := TrialMetricEvent{
		Brand: req.Brand, VehicleType: req.VehicleType, Tonnage: req.Tonnage,
		PolicyName: req.Policy, Scenario: scenario,
		FormulaVersion: FormulaVersionV2, FormulaHash: TrialFormulaHash, PolicyVersion: policyVersion,
		Denominator: 1, DurationMS: durationMS,
	}
	if result.Status == TrialStatusPriced && result.Interval != nil {
		event.Priced, event.Covered = 1, 1
	} else {
		event.UnpricedReason = result.ReasonCode
		if event.UnpricedReason == "" {
			event.UnpricedReason = TrialReasonNoValidScenario
		}
		event.FollowUpCount = len(result.MissingFields)
		if event.UnpricedReason == TrialReasonTooWide {
			event.TooWide = 1
		}
	}
	return event
}
