package valuation

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

type trialFollowUpOption struct {
	name         string
	apply        func(*TrialRequest)
	alternatives int
}

type trialFollowUpImpact struct {
	name               string
	widthReduction     float64
	candidateReduction int
	distinctOutcomes   int
	bestWidth          float64
}

func rankTrialFollowUpFields(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, policy TrialPolicy, baseline TrialResponse) []string {
	options := trialFollowUpOptions(ctx, req, snapshot, policy)
	baselineWidth := math.Inf(1)
	if baseline.WidthRatio != nil {
		baselineWidth = *baseline.WidthRatio
	}
	impacts := make([]trialFollowUpImpact, 0, len(options))
	for _, option := range options {
		bestWidth := math.Inf(1)
		bestCandidates := len(baseline.Candidates)
		outcomes := make(map[string]bool)
		for index := 0; index < option.alternatives; index++ {
			if err := ctx.Err(); err != nil {
				return trialMissingFields(req)
			}
			filled := req
			option.apply(&filled)
			result, err := calculateTrialSnapshot(ctx, filled, snapshot, policy, math.Inf(1))
			if err != nil || result.Status != TrialStatusPriced || result.Interval == nil {
				continue
			}
			width := result.Interval.WidthRatio
			if width < bestWidth || (width == bestWidth && len(result.Candidates) < bestCandidates) {
				bestWidth = width
				bestCandidates = len(result.Candidates)
			}
			outcomes[fmt.Sprintf("%.6f:%d", width, len(result.Candidates))] = true
		}
		if math.IsInf(bestWidth, 1) {
			continue
		}
		reduction := baselineWidth - bestWidth
		if reduction < 0 {
			reduction = 0
		}
		candidateReduction := len(baseline.Candidates) - bestCandidates
		if candidateReduction < 0 {
			candidateReduction = 0
		}
		impacts = append(impacts, trialFollowUpImpact{
			name: option.name, widthReduction: reduction, candidateReduction: candidateReduction,
			distinctOutcomes: len(outcomes), bestWidth: bestWidth,
		})
	}
	if len(impacts) == 0 {
		return trialMissingFields(req)
	}
	sort.SliceStable(impacts, func(i, j int) bool {
		left, right := impacts[i], impacts[j]
		if left.widthReduction != right.widthReduction {
			return left.widthReduction > right.widthReduction
		}
		if left.candidateReduction != right.candidateReduction {
			return left.candidateReduction > right.candidateReduction
		}
		if left.distinctOutcomes != right.distinctOutcomes {
			return left.distinctOutcomes > right.distinctOutcomes
		}
		if left.bestWidth != right.bestWidth {
			return left.bestWidth < right.bestWidth
		}
		return left.name < right.name
	})
	return []string{impacts[0].name}
}

func trialFollowUpOptions(ctx context.Context, req TrialRequest, snapshot *TrialSnapshot, policy TrialPolicy) []trialFollowUpOption {
	options := make([]trialFollowUpOption, 0, 12)
	prices := trialBasePrices(req, snapshot.OriginalPrices)
	for _, field := range []struct {
		name  string
		value *string
	}{
		{name: "series", value: req.Series},
		{name: "config_type", value: req.ConfigType},
		{name: "mast_type", value: req.MastType},
	} {
		if field.value != nil {
			continue
		}
		values := distinctTrialStrings(trialPriceStringValues(req, prices, field.name))
		fieldName := field.name
		for _, value := range values {
			candidate := value
			options = append(options, trialFollowUpOption{name: fieldName, alternatives: 1, apply: func(r *TrialRequest) {
				switch fieldName {
				case "series":
					r.Series = &candidate
				case "config_type":
					r.ConfigType = &candidate
				case "mast_type":
					r.MastType = &candidate
				}
			}})
		}
	}
	if req.MastHeightMM == nil {
		for _, height := range distinctTrialInts(trialPriceIntValues(req, prices)) {
			candidate := height
			options = append(options, trialFollowUpOption{name: "mast_height_mm", alternatives: 1, apply: func(r *TrialRequest) { r.MastHeightMM = &candidate }})
		}
	}
	if req.FactoryYear == nil {
		for _, year := range trialYearValues(req, prices) {
			candidate := year
			options = append(options, trialFollowUpOption{name: "factory_year_or_range", alternatives: 1, apply: func(r *TrialRequest) {
				r.FactoryYear, r.FactoryMin, r.FactoryMax = &candidate, nil, nil
			}})
		}
	}
	if req.UsageHours == nil {
		for _, hours := range trialUsageHourValues(ctx, req, prices, policy, snapshot) {
			candidate := hours
			options = append(options, trialFollowUpOption{name: "usage_hours", alternatives: 1, apply: func(r *TrialRequest) { r.UsageHours = &candidate }})
		}
	}
	if req.Condition == nil {
		for _, rating := range distinctTrialStrings(policy.ConditionRatings) {
			candidate := rating
			options = append(options, trialFollowUpOption{name: "condition_rating", alternatives: 1, apply: func(r *TrialRequest) { r.Condition = &candidate }})
		}
	}
	for _, field := range []struct {
		name  string
		value *bool
		allow []bool
		set   func(*TrialRequest, *bool)
	}{
		{name: "original_paint", value: req.OriginalPaint, allow: policy.OriginalPaintValues, set: func(r *TrialRequest, v *bool) { r.OriginalPaint = v }},
		{name: "has_license_plate", value: req.HasLicensePlate, allow: policy.HasLicensePlateValues, set: func(r *TrialRequest, v *bool) { r.HasLicensePlate = v }},
		{name: "has_registration_certificate", value: req.HasRegistrationCertificate, allow: policy.HasRegistrationCertificateValues, set: func(r *TrialRequest, v *bool) { r.HasRegistrationCertificate = v }},
		{name: "has_maintenance_records", value: req.HasMaintenanceRecords, allow: policy.HasMaintenanceRecordsValues, set: func(r *TrialRequest, v *bool) { r.HasMaintenanceRecords = v }},
	} {
		if field.value != nil {
			continue
		}
		for _, value := range trialBoolValues(nil, field.allow) {
			candidate := value
			options = append(options, trialFollowUpOption{name: field.name, alternatives: 1, apply: func(r *TrialRequest) { field.set(r, &candidate) }})
		}
	}
	if req.Province == nil || req.City == nil {
		seen := make(map[string]bool)
		for _, region := range snapshot.RegionCoefficients {
			if (req.Province != nil && *req.Province != region.Province) || (req.City != nil && *req.City != region.City) {
				continue
			}
			key := region.Province + "\x00" + region.City
			if seen[key] {
				continue
			}
			seen[key] = true
			province, city := region.Province, region.City
			options = append(options, trialFollowUpOption{name: "province_or_city", alternatives: 1, apply: func(r *TrialRequest) {
				r.Province, r.City = &province, &city
			}})
		}
	}
	return options
}

func trialBasePrices(req TrialRequest, prices []OriginalPrice) []OriginalPrice {
	base := make([]OriginalPrice, 0, len(prices))
	for _, price := range prices {
		if price.Brand == req.Brand && price.VehicleType == req.VehicleType && price.Tonnage == req.Tonnage && trialPriceMatchesExcept(req, price, "") {
			base = append(base, price)
		}
	}
	return base
}

func trialPriceMatchesExcept(req TrialRequest, price OriginalPrice, skip string) bool {
	if skip != "series" && !trialKnownStringMatches(req.Series, price.Series) {
		return false
	}
	if skip != "config_type" && !trialKnownStringMatches(req.ConfigType, price.ConfigType) {
		return false
	}
	if skip != "mast_type" && !trialKnownStringMatches(req.MastType, price.MastType) {
		return false
	}
	if skip != "mast_height_mm" && !trialKnownIntMatches(req.MastHeightMM, price.MastHeightMM) {
		return false
	}
	_, ok := compatibleTrialYears(req, price, trialSaleYear(req))
	return ok
}

func trialPriceStringValues(req TrialRequest, prices []OriginalPrice, field string) []string {
	values := make([]string, 0, len(prices))
	for _, price := range prices {
		if !trialPriceMatchesExcept(req, price, field) {
			continue
		}
		switch field {
		case "series":
			values = append(values, price.Series)
		case "config_type":
			values = append(values, price.ConfigType)
		case "mast_type":
			values = append(values, price.MastType)
		}
	}
	return values
}

func trialPriceIntValues(req TrialRequest, prices []OriginalPrice) []int {
	values := make([]int, 0, len(prices))
	for _, price := range prices {
		if trialPriceMatchesExcept(req, price, "mast_height_mm") {
			values = append(values, price.MastHeightMM)
		}
	}
	return values
}

func trialYearValues(req TrialRequest, prices []OriginalPrice) []int {
	values := make([]int, 0)
	for _, price := range prices {
		if !trialPriceMatchesExcept(req, price, "") {
			continue
		}
		years, _ := compatibleTrialYears(req, price, trialSaleYear(req))
		values = append(values, years...)
	}
	return distinctTrialInts(values)
}

func trialUsageHourValues(ctx context.Context, req TrialRequest, prices []OriginalPrice, policy TrialPolicy, snapshot *TrialSnapshot) []int {
	annual := NewCoefficientProvider(newReplayDictionary(snapshot.ReplayInput)).ReadFloat(ctx, KeyAnnualUsageHours, defaultAnnualUsageHours)
	values := make([]int, 0, len(prices)*len(policy.UsageHourRatios))
	for _, price := range prices {
		if !trialPriceMatchesExcept(req, price, "") {
			continue
		}
		years, _ := compatibleTrialYears(req, price, trialSaleYear(req))
		for _, year := range years {
			age := trialSaleYear(req) - year
			if age < 1 {
				age = 1
			}
			for _, ratio := range policy.UsageHourRatios {
				values = append(values, int(math.Round(annual*float64(age)*ratio)))
			}
		}
	}
	return distinctTrialInts(values)
}

func trialSaleYear(req TrialRequest) int {
	if req.SaleYear == 0 {
		return time.Now().Year()
	}
	return req.SaleYear
}

func distinctTrialStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func distinctTrialInts(values []int) []int {
	seen := make(map[int]bool, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
