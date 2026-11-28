package valuation

import (
	"context"
	"fmt"
	"reflect"
)

type draftCascadeDictionary interface {
	ListVehicleTypesByBrand(context.Context, string) ([]VehicleType, error)
	ListSeriesByCascade(context.Context, string, string) ([]Series, error)
	ListTonnagesByCascade(context.Context, string, string, string) ([]Tonnage, error)
	ListConfigOptionsByCascade(context.Context, string, string, string, string) ([]ConfigOption, error)
	ListMastTypesByCascade(context.Context, string, string, string, string, string) ([]MastType, error)
	ListMastHeightsByCascade(context.Context, string, string, string, string, string, string) ([]MastHeight, error)
}

var draftDownstreamFields = map[string][]string{
	"brand":        {"vehicle_type", "series"},
	"vehicle_type": {"series", "tonnage"},
	"series":       {"tonnage", "config_type"},
	"tonnage":      {"config_type", "mast_type"},
	"config_type":  {"mast_type", "mast_height_mm"},
	"mast_type":    {"mast_height_mm"},
}

func (s *ConversationService) revalidateDraftDownstream(ctx context.Context, draft *ValuationDraft, changed map[string]DraftField) error {
	if len(changed) == 0 || s.valuation == nil {
		return nil
	}
	affected := map[string]bool{}
	for name := range changed {
		markDraftDownstream(affected, name)
	}
	dictionary, supported := s.valuation.dictRepo.(draftCascadeDictionary)
	var err error
	for _, name := range requiredDraftFields {
		if !affected[name] {
			continue
		}
		if _, explicitlyChanged := changed[name]; explicitlyChanged {
			continue
		}
		field := draft.Fields[name]
		if field.State != DraftFieldValue {
			continue
		}
		var allowed []any
		var ready bool
		if supported {
			allowed, ready, err = allowedDraftValues(ctx, dictionary, draft, name)
			if err != nil {
				return fmt.Errorf("revalidate valuation draft field %s: %w", name, err)
			}
		}
		if ready && draftValueAllowed(field.Value, allowed) {
			continue
		}
		draft.Fields[name] = unconfirmedDraftValue(field)
	}
	return nil
}

func markDraftDownstream(affected map[string]bool, name string) {
	for _, downstream := range draftDownstreamFields[name] {
		if affected[downstream] {
			continue
		}
		affected[downstream] = true
		markDraftDownstream(affected, downstream)
	}
}

func allowedDraftValues(ctx context.Context, dictionary draftCascadeDictionary, draft *ValuationDraft, name string) ([]any, bool, error) {
	brand, brandOK := draftStringValue(draft, "brand")
	vehicleType, vehicleOK := draftStringValue(draft, "vehicle_type")
	series, seriesOK := draftStringValue(draft, "series")
	tonnage, tonnageOK := draftNumberValue(draft, "tonnage")
	tonnageText := fmt.Sprintf("%v", tonnage)
	configType, configOK := draftStringValue(draft, "config_type")
	mastType, mastOK := draftStringValue(draft, "mast_type")

	var values []any
	switch name {
	case "vehicle_type":
		if !brandOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListVehicleTypesByBrand(ctx, brand)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.Name)
		}
	case "series":
		if !brandOK || !vehicleOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListSeriesByCascade(ctx, brand, vehicleType)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.Name)
		}
	case "tonnage":
		if !brandOK || !vehicleOK || !seriesOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListTonnagesByCascade(ctx, brand, vehicleType, series)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.Value)
		}
	case "config_type":
		if !brandOK || !vehicleOK || !seriesOK || !tonnageOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListConfigOptionsByCascade(ctx, brand, vehicleType, series, tonnageText)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.Name)
		}
	case "mast_type":
		if !brandOK || !vehicleOK || !seriesOK || !tonnageOK || !configOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListMastTypesByCascade(ctx, brand, vehicleType, series, tonnageText, configType)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.Name)
		}
	case "mast_height_mm":
		if !brandOK || !vehicleOK || !seriesOK || !tonnageOK || !configOK || !mastOK {
			return nil, false, nil
		}
		rows, err := dictionary.ListMastHeightsByCascade(ctx, brand, vehicleType, series, tonnageText, configType, mastType)
		if err != nil {
			return nil, false, err
		}
		for _, row := range rows {
			values = append(values, row.ValueMM)
		}
	default:
		return nil, false, nil
	}
	return values, true, nil
}

func draftStringValue(draft *ValuationDraft, name string) (string, bool) {
	field := draft.Fields[name]
	value, ok := field.Value.(string)
	return value, field.State == DraftFieldValue && ok
}

func draftNumberValue(draft *ValuationDraft, name string) (float64, bool) {
	field := draft.Fields[name]
	value, ok := draftNumber(field.Value)
	return value, field.State == DraftFieldValue && ok
}

func draftValueAllowed(value any, allowed []any) bool {
	for _, candidate := range allowed {
		if reflect.DeepEqual(value, candidate) {
			return true
		}
		left, leftOK := draftNumber(value)
		right, rightOK := draftNumber(candidate)
		if leftOK && rightOK && left == right {
			return true
		}
	}
	return false
}

func unconfirmedDraftValue(field DraftField) DraftField {
	return DraftField{
		State:    DraftFieldCandidate,
		Source:   field.Source,
		Evidence: field.Evidence,
		Candidates: []DraftCandidate{{
			Label: "待确认", Value: field.Value, Source: field.Source, Evidence: field.Evidence,
		}},
	}
}
