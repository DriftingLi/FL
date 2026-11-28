package valuation

import "time"

const (
	DraftStatusEditing   = "editing"
	DraftStatusSubmitted = "submitted"
)

type DraftFieldState string

const (
	DraftFieldMissing   DraftFieldState = "missing"
	DraftFieldUnknown   DraftFieldState = "unknown"
	DraftFieldValue     DraftFieldState = "value"
	DraftFieldRange     DraftFieldState = "range"
	DraftFieldCandidate DraftFieldState = "candidate"
)

type draftFieldSpec struct {
	name       string
	kind       string
	allowRange bool
	min        float64
	max        float64
	yearUpper  bool
	vehicle    bool
	catalog    bool
	requestPtr func(*EvaluationRequest) any
}

var draftFieldSpecs = []draftFieldSpec{
	{name: "brand", kind: "string", vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.Brand }},
	{name: "vehicle_type", kind: "string", vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.VehicleType }},
	{name: "series", kind: "string", vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.Series }},
	{name: "tonnage", kind: "number", allowRange: true, min: 0.1, max: 100, vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.Tonnage }},
	{name: "config_type", kind: "string", vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.ConfigType }},
	{name: "mast_type", kind: "string", vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.MastType }},
	{name: "mast_height_mm", kind: "integer", allowRange: true, max: 15000, vehicle: true, catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.MastHeightMM }},
	{name: "factory_year", kind: "integer", allowRange: true, min: 1900, yearUpper: true, requestPtr: func(r *EvaluationRequest) any { return &r.FactoryYear }},
	{name: "sale_year", kind: "integer", allowRange: true, min: 1900, yearUpper: true, requestPtr: func(r *EvaluationRequest) any { return &r.SaleYear }},
	{name: "usage_hours", kind: "integer", allowRange: true, max: 100000, requestPtr: func(r *EvaluationRequest) any { return &r.UsageHours }},
	{name: "original_paint", kind: "bool", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.OriginalPaint }},
	{name: "province", kind: "string", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.Province }},
	{name: "city", kind: "string", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.City }},
	{name: "has_license_plate", kind: "bool", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.HasLicensePlate }},
	{name: "has_registration_certificate", kind: "bool", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.HasRegistrationCertificate }},
	{name: "has_maintenance_records", kind: "bool", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.HasMaintenanceRecords }},
	{name: "condition_rating", kind: "string", catalog: true, requestPtr: func(r *EvaluationRequest) any { return &r.ConditionRating }},
}

var draftFieldSpecByName = func() map[string]draftFieldSpec {
	byName := make(map[string]draftFieldSpec, len(draftFieldSpecs))
	for _, spec := range draftFieldSpecs {
		byName[spec.name] = spec
	}
	return byName
}()

var requiredDraftFields = func() []string {
	names := make([]string, 0, len(draftFieldSpecs))
	for _, spec := range draftFieldSpecs {
		names = append(names, spec.name)
	}
	return names
}()

type DraftRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type DraftCandidate struct {
	ID       string `json:"id,omitempty" extensions:"x-optional"`
	Label    string `json:"label,omitempty" extensions:"x-optional"`
	Value    any    `json:"value"`
	Source   string `json:"source,omitempty" extensions:"x-optional"`
	Evidence any    `json:"evidence,omitempty" extensions:"x-optional"`
}

type DraftField struct {
	State      DraftFieldState  `json:"state"`
	Value      any              `json:"value,omitempty" extensions:"x-optional"`
	Range      *DraftRange      `json:"range,omitempty" extensions:"x-optional"`
	Candidates []DraftCandidate `json:"candidates,omitempty" extensions:"x-optional" nullability:"nonnil"`
	Source     string           `json:"source,omitempty" extensions:"x-optional"`
	Evidence   any              `json:"evidence,omitempty" extensions:"x-optional"`
}

type CreateDraftRequest struct {
	Fields       map[string]DraftField `json:"fields"`
	Supplemental map[string]any        `json:"supplemental,omitempty"`
}

type PatchDraftRequest struct {
	BaseVersion  *int                  `json:"base_version"`
	Fields       map[string]DraftField `json:"fields,omitempty"`
	Supplemental *map[string]any       `json:"supplemental,omitempty"`
}

type DraftUndoRequest struct {
	BaseVersion *int `json:"base_version"`
}

type ValuationDraft struct {
	ID           string                `json:"id"`
	Version      int                   `json:"version"`
	Status       string                `json:"status"`
	Fields       map[string]DraftField `json:"fields" nullability:"nonnil"`
	Supplemental map[string]any        `json:"supplemental" nullability:"nonnil"`
	CanUndo      bool                  `json:"can_undo"`
	CanSubmit    bool                  `json:"can_submit"`
	UpdatedAt    time.Time             `json:"updated_at"`
	ExpiresAt    time.Time             `json:"expires_at"`
}

type storedDraft struct {
	Draft    ValuationDraft  `json:"draft"`
	Previous *ValuationDraft `json:"previous,omitempty"`
	Raw      string          `json:"-"`
}
