package valuation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const maxSuggestionResponseBytes = 64 * 1024

var suggestionSupplementKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

var forbiddenSuggestionSupplementKeys = map[string]bool{
	"action": true, "estimated_value": true, "valuation": true, "evaluation": true,
	"evaluation_id": true, "operation_id": true, "save": true, "submit": true,
}

type suggestionCatalogOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
}

type suggestionCatalogLists interface {
	ListBrands(context.Context) ([]Brand, error)
	ListConditionRatings(context.Context) ([]ConditionRating, error)
	ListProvinces(context.Context) ([]string, error)
	ListCities(context.Context, string) ([]string, error)
}

func decodeSuggestionResult(content string) (*SuggestionModelResult, error) {
	if len(content) > maxSuggestionResponseBytes {
		return nil, fmt.Errorf("%w: response too large", ErrSuggestionValidation)
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var result SuggestionModelResult
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSuggestionValidation, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: response must contain one JSON object", ErrSuggestionValidation)
	}
	if result.SchemaVersion != SuggestionSchemaVersion || result.Patches == nil || result.Supplements == nil || result.Questions == nil {
		return nil, fmt.Errorf("%w: schema_version/patches/supplements/questions are required", ErrSuggestionValidation)
	}
	if len(result.Patches) > len(requiredDraftFields) || len(result.Supplements) > 20 || len(result.Questions) > 5 {
		return nil, fmt.Errorf("%w: collection limit exceeded", ErrSuggestionValidation)
	}
	seenFields := map[string]bool{}
	for i := range result.Patches {
		patch := &result.Patches[i]
		if !isRequiredDraftField(patch.Field) || seenFields[patch.Field] {
			return nil, fmt.Errorf("%w: unknown or duplicate field %q", ErrSuggestionValidation, patch.Field)
		}
		seenFields[patch.Field] = true
		if err := validateSuggestionPatchShape(patch); err != nil {
			return nil, err
		}
	}
	for key, value := range result.Supplements {
		if !suggestionSupplementKey.MatchString(key) || forbiddenSuggestionSupplementKeys[key] {
			return nil, fmt.Errorf("%w: forbidden supplement key %q", ErrSuggestionValidation, key)
		}
		if strings.TrimSpace(value) == "" || len([]rune(value)) > 500 {
			return nil, fmt.Errorf("%w: invalid supplement %q", ErrSuggestionValidation, key)
		}
	}
	seenQuestions := map[string]bool{}
	for i, question := range result.Questions {
		question = strings.TrimSpace(question)
		if question == "" || len([]rune(question)) > 300 || seenQuestions[question] {
			return nil, fmt.Errorf("%w: invalid or duplicate question", ErrSuggestionValidation)
		}
		seenQuestions[question] = true
		result.Questions[i] = question
	}
	return &result, nil
}

func validateSuggestionPatchShape(patch *SuggestionPatch) error {
	invalid := func(reason string) error {
		return fmt.Errorf("%w: %s %s", ErrSuggestionValidation, patch.Field, reason)
	}
	if len(patch.CandidateKeys) > 8 || len(patch.Evidence) > 8 {
		return invalid("candidate/evidence limit exceeded")
	}
	seenKeys := map[string]bool{}
	for _, key := range patch.CandidateKeys {
		if strings.TrimSpace(key) == "" || len(key) > 160 || seenKeys[key] {
			return invalid("candidate key is invalid or duplicated")
		}
		seenKeys[key] = true
	}
	for _, evidence := range patch.Evidence {
		if strings.TrimSpace(evidence) == "" || len([]rune(evidence)) > 300 {
			return invalid("evidence is invalid")
		}
	}
	switch patch.State {
	case DraftFieldMissing, DraftFieldUnknown:
		if patch.Value != nil || patch.Range != nil || len(patch.CandidateKeys) != 0 {
			return invalid("missing/unknown cannot carry value, range or candidates")
		}
	case DraftFieldValue:
		if patch.Value == nil || patch.Range != nil || len(patch.CandidateKeys) != 0 {
			return invalid("value state requires only value")
		}
	case DraftFieldRange:
		if patch.Value != nil || patch.Range == nil || len(patch.CandidateKeys) != 0 {
			return invalid("range state requires only range")
		}
	case DraftFieldCandidate:
		if patch.Value != nil || patch.Range != nil || len(patch.CandidateKeys) == 0 {
			return invalid("candidate state requires candidate_keys")
		}
	default:
		return invalid("unknown state")
	}
	return nil
}

func suggestionOptionKey(field string, value any) string {
	return field + ":" + fmt.Sprint(value)
}

func (s *ConversationService) prepareSuggestion(ctx context.Context, base *ValuationDraft, modelResult *SuggestionModelResult, now time.Time) (*storedDraft, *ValuationSuggestionResult, error) {
	working, err := cloneDraft(base)
	if err != nil {
		return nil, nil, err
	}
	catalog, err := s.buildSuggestionCatalog(ctx, base)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: catalog: %v", ErrSuggestionValidation, err)
	}
	changed := map[string]DraftField{}
	for _, patch := range modelResult.Patches {
		if patch.State == DraftFieldCandidate {
			continue
		}
		field, err := s.normalizeSuggestionPatch(working, patch, catalog)
		if err != nil {
			return nil, nil, err
		}
		working.Fields[patch.Field] = field
		changed[patch.Field] = field
	}
	contextualCatalog, err := s.buildSuggestionCatalog(ctx, working)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: catalog: %v", ErrSuggestionValidation, err)
	}
	for _, patch := range modelResult.Patches {
		if patch.State == DraftFieldValue {
			value, err := normalizeSuggestionValue(patch.Field, patch.Value)
			if err != nil {
				return nil, nil, err
			}
			if err := ensureSuggestionCatalogValue(patch.Field, value, contextualCatalog); err != nil {
				return nil, nil, err
			}
		}
	}
	for _, patch := range modelResult.Patches {
		if patch.State != DraftFieldCandidate {
			continue
		}
		field, err := s.normalizeSuggestionPatch(working, patch, contextualCatalog)
		if err != nil {
			return nil, nil, err
		}
		working.Fields[patch.Field] = field
		changed[patch.Field] = field
	}
	if suggestionPatchesFullVehicle(changed) {
		if err := s.validateSuggestionVehicleCombination(ctx, working); err != nil {
			return nil, nil, err
		}
	}
	if err := s.revalidateDraftDownstream(ctx, working, changed); err != nil {
		return nil, nil, fmt.Errorf("%w: cascade: %v", ErrSuggestionValidation, err)
	}
	for _, name := range requiredDraftFields {
		if _, ok := changed[name]; ok {
			if err := validateSuggestionField(name, working.Fields[name], now); err != nil {
				return nil, nil, err
			}
		}
	}
	factoryYear, factoryOK := draftNumberValue(working, "factory_year")
	saleYear, saleOK := draftNumberValue(working, "sale_year")
	if factoryOK && saleOK && saleYear < factoryYear {
		return nil, nil, fmt.Errorf("%w: sale_year precedes factory_year", ErrSuggestionValidation)
	}

	acceptedSupplements := map[string]string{}
	for key, value := range modelResult.Supplements {
		if _, exists := working.Supplemental[key]; !exists {
			working.Supplemental[key] = value
			acceptedSupplements[key] = value
		}
	}
	result := &ValuationSuggestionResult{
		SchemaVersion: SuggestionSchemaVersion, Patches: map[string]DraftField{},
		Supplements: acceptedSupplements, Questions: append([]string{}, modelResult.Questions...),
		DraftVersion: base.Version,
	}
	for _, name := range requiredDraftFields {
		if !reflect.DeepEqual(base.Fields[name], working.Fields[name]) {
			result.Patches[name] = working.Fields[name]
		}
	}
	if len(result.Patches) == 0 && len(acceptedSupplements) == 0 {
		return nil, result, nil
	}
	previous, err := cloneDraft(base)
	if err != nil {
		return nil, nil, err
	}
	working.Version = base.Version + 1
	working.Status = DraftStatusEditing
	working.UpdatedAt = now
	working.ExpiresAt = now.Add(DraftIdleTTL)
	working.CanUndo = true
	working.CanSubmit = draftComplete(*working)
	result.DraftVersion = working.Version
	return &storedDraft{Draft: *working, Previous: previous}, result, nil
}

func suggestionPatchesFullVehicle(changed map[string]DraftField) bool {
	for _, name := range []string{"brand", "vehicle_type", "series", "tonnage", "config_type", "mast_type", "mast_height_mm"} {
		if field, ok := changed[name]; !ok || field.State != DraftFieldValue {
			return false
		}
	}
	return true
}

func (s *ConversationService) validateSuggestionVehicleCombination(ctx context.Context, draft *ValuationDraft) error {
	brand, brandOK := draftStringValue(draft, "brand")
	vehicleType, vehicleOK := draftStringValue(draft, "vehicle_type")
	series, seriesOK := draftStringValue(draft, "series")
	tonnage, tonnageOK := draftNumberValue(draft, "tonnage")
	configType, configOK := draftStringValue(draft, "config_type")
	mastType, mastOK := draftStringValue(draft, "mast_type")
	mastHeight, heightOK := draftNumberValue(draft, "mast_height_mm")
	if !brandOK || !vehicleOK || !seriesOK || !tonnageOK || !configOK || !mastOK || !heightOK || s.valuation == nil {
		return fmt.Errorf("%w: full vehicle suggestion must contain confirmed values", ErrSuggestionValidation)
	}
	if _, err := s.valuation.dictRepo.FindOriginalPriceMatch(ctx, brand, vehicleType, series, tonnage, configType, mastType, int(mastHeight)); err != nil {
		return fmt.Errorf("%w: vehicle combination is not in the real catalog", ErrSuggestionValidation)
	}
	return nil
}

func (s *ConversationService) buildSuggestionCatalog(ctx context.Context, draft *ValuationDraft) (map[string][]suggestionCatalogOption, error) {
	catalog := map[string][]suggestionCatalogOption{}
	add := func(field string, value any) {
		catalog[field] = append(catalog[field], suggestionCatalogOption{
			Key: suggestionOptionKey(field, value), Label: fmt.Sprint(value), Value: value,
		})
	}
	for _, field := range []string{"original_paint", "has_license_plate", "has_registration_certificate", "has_maintenance_records"} {
		add(field, true)
		add(field, false)
	}
	if s.valuation != nil {
		if lists, ok := s.valuation.dictRepo.(suggestionCatalogLists); ok {
			brands, err := lists.ListBrands(ctx)
			if err != nil {
				return nil, err
			}
			for _, brand := range brands {
				if brand.IsActive {
					add("brand", brand.Name)
				}
			}
			conditions, err := lists.ListConditionRatings(ctx)
			if err != nil {
				return nil, err
			}
			for _, condition := range conditions {
				add("condition_rating", condition.Rating)
			}
			provinces, err := lists.ListProvinces(ctx)
			if err != nil {
				return nil, err
			}
			for _, province := range provinces {
				add("province", province)
			}
			if province, ok := draftStringValue(draft, "province"); ok {
				cities, err := lists.ListCities(ctx, province)
				if err != nil {
					return nil, err
				}
				for _, city := range cities {
					add("city", city)
				}
			}
		} else {
			if brand, ok := draftStringValue(draft, "brand"); ok {
				add("brand", brand)
			}
			if condition, ok := draftStringValue(draft, "condition_rating"); ok {
				add("condition_rating", condition)
			}
		}
		if dictionary, ok := s.valuation.dictRepo.(draftCascadeDictionary); ok {
			for _, name := range []string{"vehicle_type", "series", "tonnage", "config_type", "mast_type", "mast_height_mm"} {
				values, ready, err := allowedDraftValues(ctx, dictionary, draft, name)
				if err != nil {
					return nil, err
				}
				if ready {
					for _, value := range values {
						add(name, value)
					}
				}
			}
		}
	}
	return catalog, nil
}

func (s *ConversationService) normalizeSuggestionPatch(draft *ValuationDraft, patch SuggestionPatch, catalog map[string][]suggestionCatalogOption) (DraftField, error) {
	field := DraftField{State: patch.State, Source: "ai", Evidence: append([]string{}, patch.Evidence...), Candidates: []DraftCandidate{}}
	if patch.State == DraftFieldValue {
		value, err := normalizeSuggestionValue(patch.Field, patch.Value)
		if err != nil {
			return field, err
		}
		field.Value = value
	}
	if patch.State == DraftFieldRange {
		field.Range = &DraftRange{Min: patch.Range.Min, Max: patch.Range.Max}
	}
	if patch.State == DraftFieldCandidate {
		for _, key := range patch.CandidateKeys {
			option, ok := findSuggestionCatalogOption(catalog, patch.Field, key)
			if !ok {
				return field, fmt.Errorf("%w: %s candidate_key %q not in catalog", ErrSuggestionValidation, patch.Field, key)
			}
			field.Candidates = append(field.Candidates, DraftCandidate{
				ID: option.Key, Label: option.Label, Value: option.Value, Source: "dictionary", Evidence: append([]string{}, patch.Evidence...),
			})
		}
	}
	field = resolveSuggestionConflict(draft.Fields[patch.Field], field)
	if err := validateDraftField(patch.Field, field); err != nil {
		return field, fmt.Errorf("%w: %v", ErrSuggestionValidation, err)
	}
	return field, nil
}

func normalizeSuggestionValue(field string, value any) (any, error) {
	switch draftFieldKind(field) {
	case "string":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" || len([]rune(text)) > 100 {
			return nil, fmt.Errorf("%w: %s must be a non-empty string", ErrSuggestionValidation, field)
		}
		return strings.TrimSpace(text), nil
	case "bool":
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%w: %s must be boolean", ErrSuggestionValidation, field)
		}
		return boolean, nil
	case "integer":
		number, ok := draftNumber(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number {
			return nil, fmt.Errorf("%w: %s must be an integer", ErrSuggestionValidation, field)
		}
		return int(number), nil
	case "number":
		number, ok := draftNumber(value)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, fmt.Errorf("%w: %s must be a finite number", ErrSuggestionValidation, field)
		}
		return number, nil
	default:
		return nil, fmt.Errorf("%w: unsupported field %s", ErrSuggestionValidation, field)
	}
}

func ensureSuggestionCatalogValue(field string, value any, catalog map[string][]suggestionCatalogOption) error {
	if !suggestionCatalogField(field) {
		return nil
	}
	if _, ok := findSuggestionCatalogOption(catalog, field, suggestionOptionKey(field, value)); !ok {
		return fmt.Errorf("%w: %s value is not in the real catalog", ErrSuggestionValidation, field)
	}
	return nil
}

func suggestionCatalogField(field string) bool {
	switch field {
	case "brand", "vehicle_type", "series", "tonnage", "config_type", "mast_type", "mast_height_mm", "condition_rating", "province", "city",
		"original_paint", "has_license_plate", "has_registration_certificate", "has_maintenance_records":
		return true
	default:
		return false
	}
}

func findSuggestionCatalogOption(catalog map[string][]suggestionCatalogOption, field, key string) (suggestionCatalogOption, bool) {
	for _, option := range catalog[field] {
		if option.Key == key {
			return option, true
		}
	}
	return suggestionCatalogOption{}, false
}

func resolveSuggestionConflict(current, suggested DraftField) DraftField {
	if suggested.State == DraftFieldValue {
		return suggested
	}
	if current.State != DraftFieldValue || current.Value == nil {
		if current.State == DraftFieldCandidate && len(current.Candidates) == 1 {
			candidate := current.Candidates[0]
			if suggested.State == DraftFieldCandidate && len(suggested.Candidates) == 1 && draftValuesEqual(candidate.Value, suggested.Candidates[0].Value) {
				return DraftField{State: DraftFieldValue, Value: candidate.Value, Source: "ai", Evidence: suggested.Evidence}
			}
		}
		return suggested
	}
	if suggested.State != DraftFieldCandidate {
		return suggested
	}
	if len(suggested.Candidates) == 1 && draftValuesEqual(current.Value, suggested.Candidates[0].Value) {
		return DraftField{State: DraftFieldValue, Value: current.Value, Source: "ai", Evidence: suggested.Evidence}
	}
	combined := []DraftCandidate{{Label: "当前确认值", Value: current.Value, Source: current.Source, Evidence: current.Evidence}}
	values := []any{current.Value}
	for _, candidate := range suggested.Candidates {
		if !containsDraftValue(values, candidate.Value) {
			values = append(values, candidate.Value)
			combined = append(combined, candidate)
		}
	}
	return DraftField{State: DraftFieldCandidate, Source: "ai", Evidence: suggested.Evidence, Candidates: combined}
}

func draftValuesEqual(left, right any) bool {
	if reflect.DeepEqual(left, right) {
		return true
	}
	a, aOK := draftNumber(left)
	b, bOK := draftNumber(right)
	return aOK && bOK && a == b
}

func containsDraftValue(values []any, candidate any) bool {
	for _, value := range values {
		if draftValuesEqual(value, candidate) {
			return true
		}
	}
	return false
}

func validateSuggestionField(name string, field DraftField, now time.Time) error {
	if err := validateDraftField(name, field); err != nil {
		return fmt.Errorf("%w: %v", ErrSuggestionValidation, err)
	}
	if field.State == DraftFieldRange {
		min, max := field.Range.Min, field.Range.Max
		low, high, integer := suggestionFieldBounds(name, now)
		if min < low || max > high || (integer && (math.Trunc(min) != min || math.Trunc(max) != max)) {
			return fmt.Errorf("%w: %s range exceeds field bounds or unit", ErrSuggestionValidation, name)
		}
	}
	if field.State == DraftFieldValue {
		value, _ := draftNumber(field.Value)
		low, high, _ := suggestionFieldBounds(name, now)
		if draftFieldKind(name) == "integer" || draftFieldKind(name) == "number" {
			if value < low || value > high {
				return fmt.Errorf("%w: %s value exceeds field bounds or unit", ErrSuggestionValidation, name)
			}
		}
	}
	return nil
}

func suggestionFieldBounds(name string, now time.Time) (float64, float64, bool) {
	switch name {
	case "tonnage":
		return 0.1, 100, false
	case "usage_hours":
		return 0, 100000, true
	case "mast_height_mm":
		return 0, 15000, true
	case "factory_year", "sale_year":
		return 1900, float64(now.Year() + 1), true
	default:
		return 0, math.MaxFloat64, false
	}
}
