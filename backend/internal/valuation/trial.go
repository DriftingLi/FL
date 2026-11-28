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

// CalculateTrial 是 HTTP 与离线回放共用的试算路径。
func CalculateTrial(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, appEnv string) (TrialResponse, error) {
	prepared, policy, ok := preflightTrial(req, snapshot, appEnv, true)
	if !ok {
		return prepared, nil
	}
	return calculateTrialSnapshot(ctx, req, snapshot, policy, policy.MaxWidthRatio)
}

// CalculateTrialAtWidthLimit 供离线三档宽度比较使用；仍走同一目录枚举与 V2 核心。
func CalculateTrialAtWidthLimit(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, appEnv string, widthLimit float64) (TrialResponse, error) {
	if widthLimit <= 0 || widthLimit > 1 || math.IsNaN(widthLimit) || math.IsInf(widthLimit, 0) {
		return newTrialResponse(req), fmt.Errorf("试算宽度上限必须在 (0,1] 内")
	}
	prepared, policy, ok := preflightTrial(req, snapshot, appEnv, true)
	if !ok {
		return prepared, nil
	}
	return calculateTrialSnapshot(ctx, req, snapshot, policy, widthLimit)
}

func preflightTrial(req TrialRequest, snapshot *TrialSnapshot, appEnv string, checkSnapshot bool) (TrialResponse, TrialPolicy, bool) {
	response := newTrialResponse(req)
	if problems := req.validateMinimum(); len(problems) > 0 {
		return trialNeedsInfo(response, TrialReasonInvalidInput, "参考试算所需关键输入不足或无效", problems), TrialPolicy{}, false
	}
	if reason, message := validateTrialPolicySelection(req, appEnv); reason != "" {
		return trialNeedsInfo(response, reason, message, nil), TrialPolicy{}, false
	}
	if !checkSnapshot {
		return response, TrialPolicy{}, true
	}
	policy, reason, message := resolveConfiguredTrialPolicy(req, snapshot)
	if reason != "" {
		return trialNeedsInfo(response, reason, message, nil), TrialPolicy{}, false
	}
	if req.SnapshotID != "" && req.SnapshotID != snapshot.Hash {
		return trialNeedsInfo(response, TrialReasonInvalidInput, "指定快照与本次加载的快照不一致", []string{"snapshot_id"}), TrialPolicy{}, false
	}
	response.Snapshot = snapshot.meta(policy)
	response.SnapshotID = snapshot.Hash
	return response, policy, true
}

func validateTrialPolicySelection(req TrialRequest, appEnv string) (string, string) {
	if req.Policy != "demo" && req.Policy != "pilot" {
		return TrialReasonPolicyDisabled, "参考试算未开放，必须显式选择已配置的 demo 或 pilot 策略"
	}
	if req.Policy == "demo" && appEnv == "production" {
		return TrialReasonPolicyForbidden, "生产环境禁止 demo 参考试算"
	}
	return "", ""
}

func resolveConfiguredTrialPolicy(req TrialRequest, snapshot *TrialSnapshot) (TrialPolicy, string, string) {
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
		RequestID:     req.RequestID,
		SnapshotID:    req.SnapshotID,
		Input:         req,
		Assumptions:   []string{},
		Candidates:    []TrialCandidate{},
		MissingFields: []string{},
	}
}

func trialPointer[T any](value T) *T { return &value }

func (r TrialRequest) validateMinimum() []string {
	missing := make([]string, 0, 6)
	if strings.TrimSpace(r.RequestID) == "" || len(r.RequestID) > 128 {
		missing = append(missing, "request_id")
	}
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
