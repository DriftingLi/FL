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
