package valuation

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

type exactOnlyDictionary struct {
	DictionaryReader
}

func (exactOnlyDictionary) FindOriginalPriceFuzzy(context.Context, string, string, string, float64) (OriginalPrice, error) {
	return OriginalPrice{}, pgx.ErrNoRows
}

func calculateTrialSnapshot(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, policy TrialPolicy, widthLimit float64) (TrialResponse, error) {
	response := newTrialResponse(req)
	response.Snapshot = snapshot.meta(policy)
	response.SnapshotID = snapshot.Hash
	if missing := unsupportedTrialScenarioFields(req, policy, snapshot.ConditionRatings); len(missing) > 0 {
		return trialNeedsInfo(response, TrialReasonNoValidScenario, "存在未获策略批准的未知情景，暂不给价", missing), nil
	}
	if !trialCoefficientsComplete(snapshot.Coefficients) {
		return trialNeedsInfo(response, TrialReasonNoValidScenario, "系数快照不完整或无效，暂不给价", []string{"coefficient_snapshot"}), nil
	}
	dict := exactOnlyDictionary{DictionaryReader: newReplayDictionary(snapshot.ReplayInput)}

	brand, err := dict.GetBrandByName(ctx, req.Brand)
	if err != nil || !brand.IsActive || brand.KBrand <= 0 || math.IsNaN(brand.KBrand) || math.IsInf(brand.KBrand, 0) {
		return trialNeedsInfo(response, TrialReasonBrandUnreliable, "品牌不在可靠目录中", []string{"brand"}), nil
	}
	if _, err := dict.GetVehicleTypeByName(ctx, req.VehicleType); err != nil {
		return trialNeedsInfo(response, TrialReasonVehicleTypeUnreliable, "车型不在可靠目录中", []string{"vehicle_type"}), nil
	}
	if req.Condition != nil && !trialConditionSupported(snapshot.ConditionRatings, *req.Condition) {
		return trialNeedsInfo(response, TrialReasonConditionUnreliable, "车况评级不在可靠目录中", []string{"condition_rating"}), nil
	}

	base := make([]OriginalPrice, 0)
	for _, price := range snapshot.OriginalPrices {
		if price.Brand == req.Brand && price.VehicleType == req.VehicleType && price.Tonnage == req.Tonnage {
			base = append(base, price)
		}
	}
	if len(base) == 0 {
		return trialNeedsInfo(response, TrialReasonOriginalPriceMissing, "目录中没有可核对的同品牌、车型、吨位原价", []string{"original_price"}), nil
	}
	filtered := make([]OriginalPrice, 0, len(base))
	for _, price := range base {
		if trialKnownStringMatches(req.Series, price.Series) &&
			trialKnownStringMatches(req.ConfigType, price.ConfigType) &&
			trialKnownStringMatches(req.MastType, price.MastType) &&
			trialKnownIntMatches(req.MastHeightMM, price.MastHeightMM) {
			filtered = append(filtered, price)
		}
	}
	if len(filtered) == 0 {
		return trialNeedsInfo(response, TrialReasonCatalogConflict, "已知规格与目录候选冲突", []string{"series", "config_type", "mast_type", "mast_height_mm"}), nil
	}

	saleYear := req.SaleYear
	if saleYear == 0 {
		saleYear = time.Now().Year()
	}
	response.Assumptions = trialAssumptions(req, saleYear)
	compatible := 0
	scenarioCount := 0
	values := make([]float64, 0)
	for _, price := range filtered {
		years, ok := compatibleTrialYears(req, price, saleYear)
		if !ok {
			continue
		}
		compatible++
		response.Candidates = append(response.Candidates, TrialCandidate{OriginalPrice: price})
		for _, factoryYear := range years {
			scenarios := trialScenarios(ctx, req, price, factoryYear, saleYear, policy, dict, snapshot.ConditionRatings)
			scenarioCount += len(scenarios)
			if scenarioCount > maxTrialScenarios {
				return trialNeedsInfo(response, TrialReasonNoValidScenario, "试算情景数量超过安全上限，请缩小年份范围或补充已知字段", []string{"factory_year_or_range"}), nil
			}
			for _, scenario := range scenarios {
				result, err := evaluateWithOriginalPrice(ctx, &scenario, dict, price.OriginalPrice)
				if err != nil {
					continue
				}
				value := result.EstimatedValue
				if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
					continue
				}
				values = append(values, value)
			}
		}
	}
	if compatible == 0 {
		return trialNeedsInfo(response, TrialReasonYearIncompatible, "出厂年份或范围与目录候选不相容", []string{"factory_year_or_range"}), nil
	}
	interval, intervalReason := BuildTrialInterval(values)
	if intervalReason == TrialReasonMidpointNonPositive {
		return trialNeedsInfo(response, intervalReason, "试算区间中点非正，暂不给价", trialMissingFields(req)), nil
	}
	if interval == nil {
		return trialNeedsInfo(response, TrialReasonNoValidScenario, "没有有效、有限且为正的试算情景", trialMissingFields(req)), nil
	}
	response.WidthRatio = trialPointer(interval.WidthRatio)
	if interval.WidthRatio > widthLimit+1e-6 {
		response.Interval = nil
		return trialNeedsInfo(response, TrialReasonTooWide, "试算区间超过可用宽度上限，请补充最能缩小范围的资料", rankTrialFollowUpFields(ctx, req, snapshot, policy, response)), nil
	}
	response.Status = TrialStatusPriced
	response.Interval = interval
	return response, nil
}

func trialNeedsInfo(response TrialResponse, reason, message string, missing []string) TrialResponse {
	response.Status = TrialStatusNeedsInfo
	response.ReasonCode = reason
	response.Message = message
	response.Interval = nil
	if missing == nil {
		missing = []string{}
	}
	response.MissingFields = missing
	return response
}

func trialKnownStringMatches(known *string, catalog string) bool {
	return known == nil || *known == catalog
}

func trialKnownIntMatches(known *int, catalog int) bool {
	return known == nil || *known == catalog
}

func compatibleTrialYears(req TrialRequest, price OriginalPrice, saleYear int) ([]int, bool) {
	minYear, maxYear := 0, 0
	if req.FactoryYear != nil {
		minYear, maxYear = *req.FactoryYear, *req.FactoryYear
	} else {
		minYear, maxYear = *req.FactoryMin, *req.FactoryMax
	}
	if minYear < price.EarliestFactoryYear {
		minYear = price.EarliestFactoryYear
	}
	if minYear > maxYear || minYear > saleYear {
		return nil, false
	}
	years := make([]int, 0, maxYear-minYear+1)
	for year := minYear; year <= maxYear; year++ {
		years = append(years, year)
	}
	return years, true
}

func trialScenarios(ctx context.Context, req TrialRequest, price OriginalPrice, factoryYear, saleYear int, policy TrialPolicy, dict DictionaryReader, availableConditions []ConditionRating) []EvaluationRequest {
	hours := []int{0}
	if req.UsageHours != nil {
		hours = []int{*req.UsageHours}
	} else {
		annual := NewCoefficientProvider(dict).ReadFloat(ctx, KeyAnnualUsageHours, defaultAnnualUsageHours)
		age := saleYear - factoryYear
		if age < 1 {
			age = 1
		}
		seen := map[int]bool{}
		for _, ratio := range policy.UsageHourRatios {
			value := int(math.Round(annual * float64(age) * ratio))
			if value >= 0 && !seen[value] {
				hours = append(hours, value)
				seen[value] = true
			}
		}
	}
	var conditions []string
	if req.Condition != nil {
		conditions = []string{*req.Condition}
	} else {
		conditions = make([]string, 0, len(policy.ConditionRatings))
		for _, rating := range policy.ConditionRatings {
			if trialConditionSupported(availableConditions, rating) {
				conditions = append(conditions, rating)
			}
		}
	}
	paint := trialBoolValues(req.OriginalPaint, policy.OriginalPaintValues)
	plates := trialBoolValues(req.HasLicensePlate, policy.HasLicensePlateValues)
	registrations := trialBoolValues(req.HasRegistrationCertificate, policy.HasRegistrationCertificateValues)
	maintenance := trialBoolValues(req.HasMaintenanceRecords, policy.HasMaintenanceRecordsValues)
	out := make([]EvaluationRequest, 0, len(hours)*len(conditions)*len(paint)*len(plates)*len(registrations)*len(maintenance))
	for _, usageHours := range hours {
		for _, condition := range conditions {
			for _, originalPaint := range paint {
				for _, hasPlate := range plates {
					for _, hasRegistration := range registrations {
						for _, hasMaintenance := range maintenance {
							out = append(out, EvaluationRequest{
								Brand: req.Brand, VehicleType: req.VehicleType, Series: price.Series, Tonnage: price.Tonnage,
								ConfigType: price.ConfigType, MastType: price.MastType, MastHeightMM: price.MastHeightMM,
								FactoryYear: factoryYear, SaleYear: saleYear, UsageHours: usageHours,
								OriginalPaint: originalPaint, Province: trialString(req.Province, "中性"),
								City: trialString(req.City, "中性"), HasLicensePlate: hasPlate,
								HasRegistrationCertificate: hasRegistration, HasMaintenanceRecords: hasMaintenance,
								ConditionRating: condition,
							})
						}
					}
				}
			}
		}
	}
	return out
}

func unsupportedTrialScenarioFields(req TrialRequest, policy TrialPolicy, conditions []ConditionRating) []string {
	missing := make([]string, 0, 6)
	if req.UsageHours == nil {
		valid := len(policy.UsageHourRatios) > 0
		for _, ratio := range policy.UsageHourRatios {
			if ratio < 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) {
				valid = false
			}
		}
		if !valid {
			missing = append(missing, "usage_hours")
		}
	}
	if req.Condition == nil {
		supported := false
		for _, rating := range policy.ConditionRatings {
			if trialConditionSupported(conditions, rating) {
				supported = true
				break
			}
		}
		if !supported {
			missing = append(missing, "condition_rating")
		}
	}
	for _, field := range []struct {
		name  string
		value *bool
		allow []bool
	}{
		{name: "original_paint", value: req.OriginalPaint, allow: policy.OriginalPaintValues},
		{name: "has_license_plate", value: req.HasLicensePlate, allow: policy.HasLicensePlateValues},
		{name: "has_registration_certificate", value: req.HasRegistrationCertificate, allow: policy.HasRegistrationCertificateValues},
		{name: "has_maintenance_records", value: req.HasMaintenanceRecords, allow: policy.HasMaintenanceRecordsValues},
	} {
		if field.value == nil && len(trialBoolValues(nil, field.allow)) == 0 {
			missing = append(missing, field.name)
		}
	}
	return missing
}

func trialBoolValues(value *bool, approved []bool) []bool {
	if value != nil {
		return []bool{*value}
	}
	seen := make(map[bool]bool, len(approved))
	values := make([]bool, 0, len(approved))
	for _, candidate := range approved {
		if !seen[candidate] {
			values = append(values, candidate)
			seen[candidate] = true
		}
	}
	return values
}

func trialCoefficientsComplete(configs []CoefficientConfig) bool {
	values := make(map[string]float64, len(configs))
	for _, config := range configs {
		if _, duplicate := values[config.Key]; duplicate || math.IsNaN(config.Value) || math.IsInf(config.Value, 0) {
			return false
		}
		values[config.Key] = config.Value
	}
	for _, key := range []string{KeyLambdaElectric, KeyLambdaCombustion, KeyAnnualUsageHours,
		KeyKHoursRatioLow, KeyKHoursRatioMid, KeyKHoursRatioHigh, KeyKHoursRatioMax} {
		if value, ok := values[key]; !ok || value <= 0 {
			return false
		}
	}
	if !(values[KeyKHoursRatioLow] < values[KeyKHoursRatioMid] &&
		values[KeyKHoursRatioMid] < values[KeyKHoursRatioHigh] &&
		values[KeyKHoursRatioHigh] < values[KeyKHoursRatioMax]) {
		return false
	}
	for _, key := range []string{KeyKcPaintBonus, KeyKcMaintenanceBonus} {
		if value, ok := values[key]; !ok || value < 0 {
			return false
		}
	}
	for _, key := range []string{KeyKcNoLicensePenaltyPct, KeyKcNoRegistrationPenaltyPct} {
		if value, ok := values[key]; !ok || value < 0 || value >= 1 {
			return false
		}
	}
	return true
}

func trialConditionSupported(conditions []ConditionRating, rating string) bool {
	for _, condition := range conditions {
		if condition.Rating == rating {
			return condition.BaseCoefficient > 0 && !math.IsNaN(condition.BaseCoefficient) && !math.IsInf(condition.BaseCoefficient, 0)
		}
	}
	return false
}

func trialString(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

// BuildTrialInterval 只聚合有限正情景；空集或非有限中点均 fail-closed。
func BuildTrialInterval(values []float64) (*TrialInterval, string) {
	interval := &TrialInterval{}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			continue
		}
		if interval.Min == 0 || value < interval.Min {
			interval.Min = value
		}
		if value > interval.Max {
			interval.Max = value
		}
	}
	if interval.Min == 0 {
		return nil, TrialReasonNoValidScenario
	}
	interval.Mid = (interval.Min + interval.Max) / 2
	if interval.Mid <= 0 || math.IsNaN(interval.Mid) || math.IsInf(interval.Mid, 0) {
		return nil, TrialReasonMidpointNonPositive
	}
	interval.Width = interval.Max - interval.Min
	interval.WidthRatio = interval.Width / interval.Mid
	if math.IsNaN(interval.WidthRatio) || math.IsInf(interval.WidthRatio, 0) {
		return nil, TrialReasonNoValidScenario
	}
	return interval, ""
}

func trialAssumptions(req TrialRequest, saleYear int) []string {
	assumptions := make([]string, 0, 8)
	if req.SaleYear == 0 {
		assumptions = append(assumptions, fmt.Sprintf("估价年份使用 %d 年", saleYear))
	}
	if req.FactoryYear == nil {
		assumptions = append(assumptions, "按出厂年份范围内的相容目录年份枚举")
	}
	if req.Series == nil || req.ConfigType == nil || req.MastType == nil || req.MastHeightMM == nil {
		assumptions = append(assumptions, "未知系列、配置或门架由各目录候选补齐")
	}
	if req.UsageHours == nil {
		assumptions = append(assumptions, "未知工时按策略批准的工时比例情景枚举")
	}
	if req.Condition == nil {
		assumptions = append(assumptions, "未知车况按策略批准的车况评级枚举")
	}
	if req.HasLicensePlate == nil || req.HasRegistrationCertificate == nil {
		assumptions = append(assumptions, "未知证件按策略批准的有、无情景枚举")
	}
	if req.OriginalPaint == nil {
		assumptions = append(assumptions, "未知原厂漆按策略批准的有、无情景枚举")
	}
	if req.HasMaintenanceRecords == nil {
		assumptions = append(assumptions, "未知维保记录按策略批准的有、无情景枚举")
	}
	if req.Province == nil || req.City == nil {
		assumptions = append(assumptions, "未知区域采用中性市场系数 1.0")
	}
	return assumptions
}

func trialMissingFields(req TrialRequest) []string {
	fields := []struct {
		missing bool
		name    string
	}{
		{req.Series == nil, "series"},
		{req.ConfigType == nil, "config_type"},
		{req.MastType == nil, "mast_type"},
		{req.MastHeightMM == nil, "mast_height_mm"},
		{req.FactoryYear == nil, "factory_year_or_range"},
		{req.UsageHours == nil, "usage_hours"},
		{req.Condition == nil, "condition_rating"},
		{req.OriginalPaint == nil, "original_paint"},
		{req.HasRegistrationCertificate == nil, "has_registration_certificate"},
		{req.HasLicensePlate == nil, "has_license_plate"},
		{req.HasMaintenanceRecords == nil, "has_maintenance_records"},
		{req.Province == nil || req.City == nil, "province_or_city"},
	}
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field.missing {
			out = append(out, field.name)
		}
	}
	return out
}
