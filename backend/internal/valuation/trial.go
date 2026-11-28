package valuation

import (
	"context"
	"fmt"
	"math"
	"strings"
)

const (
	TrialStatusPriced    = "priced"
	TrialStatusNeedsInfo = "needs_info"

	maxTrialYearSpan  = 200
	maxTrialScenarios = 20000

	TrialReasonInvalidInput          = "invalid_input"
	TrialReasonPolicyDisabled        = "policy_disabled"
	TrialReasonPolicyNotConfigured   = "policy_not_configured"
	TrialReasonPolicyForbidden       = "policy_forbidden"
	TrialReasonBrandUnreliable       = "brand_unreliable"
	TrialReasonVehicleTypeUnreliable = "vehicle_type_unreliable"
	TrialReasonConditionUnreliable   = "condition_rating_unreliable"
	TrialReasonInvalidTonnage        = "invalid_tonnage"
	TrialReasonInvalidYear           = "invalid_year"
	TrialReasonOriginalPriceMissing  = "original_price_not_found"
	TrialReasonCatalogConflict       = "catalog_conflict"
	TrialReasonYearIncompatible      = "year_incompatible"
	TrialReasonNoValidScenario       = "no_valid_scenario"
	TrialReasonMidpointNonPositive   = "midpoint_non_positive"
	TrialReasonTooWide               = "too_wide"
)

// TrialRequest 参考试算输入。指针区分未知与明确的 false、0、"无"、"其它"。
type TrialRequest struct {
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

// CalculateTrial 是 HTTP 与离线回放共用的试算路径。
func CalculateTrial(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, appEnv string) (TrialResponse, error) {
	response := newTrialResponse(req)
	if problems := req.validateMinimum(); len(problems) > 0 {
		response.Status = TrialStatusNeedsInfo
		response.ReasonCode = TrialReasonInvalidInput
		response.Message = "参考试算所需关键输入不足或无效"
		response.MissingFields = problems
		return response, nil
	}
	policy, reason, message := resolveTrialPolicy(req, snapshot, appEnv)
	if reason != "" {
		response.Status = TrialStatusNeedsInfo
		response.ReasonCode = reason
		response.Message = message
		return response, nil
	}
	response.Snapshot = snapshot.meta(policy)
	return calculateTrialSnapshot(ctx, req, snapshot, policy, policy.MaxWidthRatio)
}

// CalculateTrialAtWidthLimit 供离线三档宽度比较使用；仍走同一目录枚举与 V2 核心。
func CalculateTrialAtWidthLimit(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, appEnv string, widthLimit float64) (TrialResponse, error) {
	if widthLimit <= 0 || widthLimit > 1 || math.IsNaN(widthLimit) || math.IsInf(widthLimit, 0) {
		return newTrialResponse(req), fmt.Errorf("试算宽度上限必须在 (0,1] 内")
	}
	if problems := req.validateMinimum(); len(problems) > 0 {
		response := newTrialResponse(req)
		response.Status, response.ReasonCode, response.Message = TrialStatusNeedsInfo, TrialReasonInvalidInput, "参考试算所需关键输入不足或无效"
		response.MissingFields = problems
		return response, nil
	}
	policy, reason, message := resolveTrialPolicy(req, snapshot, appEnv)
	if reason != "" {
		response := newTrialResponse(req)
		response.Status, response.ReasonCode, response.Message = TrialStatusNeedsInfo, reason, message
		return response, nil
	}
	response := newTrialResponse(req)
	response.Snapshot = snapshot.meta(policy)
	return calculateTrialSnapshot(ctx, req, snapshot, policy, widthLimit)
}

func resolveTrialPolicy(req TrialRequest, snapshot *TrialSnapshot, appEnv string) (TrialPolicy, string, string) {
	if req.Policy != "demo" && req.Policy != "pilot" {
		return TrialPolicy{}, TrialReasonPolicyDisabled, "参考试算未开放，必须显式选择已配置的 demo 或 pilot 策略"
	}
	if req.Policy == "demo" && appEnv == "production" {
		return TrialPolicy{}, TrialReasonPolicyForbidden, "生产环境禁止 demo 参考试算"
	}
	if snapshot == nil {
		return TrialPolicy{}, TrialReasonPolicyNotConfigured, "当前组合未配置参考试算策略"
	}
	for _, policy := range snapshot.TrialPolicies {
		if policy.Name == req.Policy && policy.Brand == req.Brand && policy.VehicleType == req.VehicleType && policy.Tonnage == req.Tonnage {
			if !policy.Enabled || policy.MaxWidthRatio <= 0 || policy.MaxWidthRatio > 1 || math.IsNaN(policy.MaxWidthRatio) || math.IsInf(policy.MaxWidthRatio, 0) {
				return TrialPolicy{}, TrialReasonPolicyDisabled, "当前组合的参考试算策略未启用"
			}
			return policy, "", ""
		}
	}
	return TrialPolicy{}, TrialReasonPolicyNotConfigured, "当前组合未配置参考试算策略"
}

func newTrialResponse(req TrialRequest) TrialResponse {
	return TrialResponse{
		Input:         req,
		Assumptions:   []string{},
		Candidates:    []TrialCandidate{},
		MissingFields: []string{},
	}
}

func trialPointer[T any](value T) *T { return &value }

func (r TrialRequest) validateMinimum() []string {
	missing := make([]string, 0, 5)
	if strings.TrimSpace(r.Brand) == "" {
		missing = append(missing, "brand")
	}
	if strings.TrimSpace(r.VehicleType) == "" {
		missing = append(missing, "vehicle_type")
	}
	if r.Tonnage <= 0 || math.IsNaN(r.Tonnage) || math.IsInf(r.Tonnage, 0) {
		missing = append(missing, "tonnage")
	}
	yearFields := 0
	if r.FactoryYear != nil {
		yearFields++
	}
	if r.FactoryMin != nil || r.FactoryMax != nil {
		yearFields++
	}
	if yearFields != 1 {
		missing = append(missing, "factory_year_or_range")
	}
	if r.FactoryYear != nil && (*r.FactoryYear < 1900 || (r.SaleYear > 0 && *r.FactoryYear > r.SaleYear)) {
		missing = append(missing, "factory_year")
	}
	if r.FactoryMin != nil && r.FactoryMax != nil && (*r.FactoryMin < 1900 || *r.FactoryMin > *r.FactoryMax ||
		*r.FactoryMax-*r.FactoryMin > maxTrialYearSpan || (r.SaleYear > 0 && *r.FactoryMax > r.SaleYear)) {
		missing = append(missing, "factory_year_range")
	}
	if r.SaleYear < 0 || r.SaleYear > 9999 {
		missing = append(missing, "sale_year")
	}
	if (r.FactoryMin == nil) != (r.FactoryMax == nil) {
		missing = append(missing, "factory_year_range")
	}
	if r.UsageHours != nil && *r.UsageHours < 0 {
		missing = append(missing, "usage_hours")
	}
	if r.MastHeightMM != nil && *r.MastHeightMM < 0 {
		missing = append(missing, "mast_height_mm")
	}
	return missing
}
