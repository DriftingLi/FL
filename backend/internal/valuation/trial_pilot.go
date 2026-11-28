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

// TrialControlResponse 返回当前全局开关、单调版本与上线准备度。
type TrialControlResponse struct {
	TrialControl
	Readiness TrialReadiness `json:"readiness"`
}

// TrialReadinessBlocker 是一项上线前必须取得的批准；Approved=false 即阻塞项。
type TrialReadinessBlocker struct {
	Key      string `json:"key"`
	Approved bool   `json:"approved"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence"`
}

// TrialReadiness 汇总上线准备度；Ready=false 时全局开关不得开启（见 SetControl 的守卫）。
type TrialReadiness struct {
	Ready    bool                    `json:"ready"`
	Blockers []TrialReadinessBlocker `json:"blockers" nullability:"nonnil"`
}

// trialReadinessPending 列出未批准项，供开关拒绝时的人话说明使用。
func trialReadinessPending(readiness TrialReadiness) string {
	pending := make([]string, 0, len(readiness.Blockers))
	for _, blocker := range readiness.Blockers {
		if !blocker.Approved {
			pending = append(pending, blocker.Key)
		}
	}
	return strings.Join(pending, ", ")
}

// trialReadinessBlockers 是上线批准状态的唯一台账（#2958）。
//
// 逐项翻成 Approved=true 必须同时引用对应决策记录（#2949 首发条件、#2951 名单与宽度、
// 回归与样本回放结论），并同批改这里——**不得预填任何「已批准」**：readiness 的全部作用
// 就是挡住「审批还没落地就把开关打开」，在代码里先填 true 等于伪造批准。
func trialReadinessBlockers() []TrialReadinessBlocker {
	return []TrialReadinessBlocker{
		{Key: "formula_regression", Approved: false,
			Detail: "新公式（V2 工时衰减方向修正后）回归结果尚未取得批准", Evidence: "#2953"},
		{Key: "sample_replay_holdout", Approved: false,
			Detail: "脱敏样本回放与留出集复验结果尚未取得批准", Evidence: "#2952"},
		{Key: "launch_condition_2949", Approved: false,
			Detail: "对话估值验收指标与首发条件（#2949）尚未给出批准结论", Evidence: "#2949"},
		{Key: "allowlist_width_2951", Approved: false,
			Detail: "首发车型名单与统一宽度上限（#2951）尚未给出批准结论", Evidence: "#2951"},
		{Key: "policy_width_unified", Approved: false,
			Detail: "统一宽度上限尚未实现强制：当前仅按每条策略的 max_width_ratio 逐条生效", Evidence: "#2951"},
	}
}

// trialReadiness 汇总台账；任一阻塞项未批准即 Ready=false，且不允许开启全局开关。
func trialReadiness() TrialReadiness {
	blockers := trialReadinessBlockers()
	ready := len(blockers) > 0
	for _, blocker := range blockers {
		if !blocker.Approved {
			ready = false
		}
	}
	return TrialReadiness{Ready: ready, Blockers: blockers}
}

// TrialPoliciesResponse 管理端策略全量读面。
type TrialPoliciesResponse struct {
	Policies []TrialPolicy `json:"policies" nullability:"nonnil"`
}

// TrialPoliciesRequest 全量替换策略；空列表用于清空 allowlist，null 与「没给」一样拒绝
// （nil 语义与空数组不同：前者是请求写错，后者是一次显式清空）。
type TrialPoliciesRequest struct {
	Policies []TrialPolicyInput `json:"policies"`
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
		// 维度只承载「用了哪份策略」；原因码一律落 unpriced_reason（见下方 else 分支），
		// 不得写进 policy_version——否则聚合读面会把关闭态混成一个假策略版本。
		policyVersion = "unresolved"
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
