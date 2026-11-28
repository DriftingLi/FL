package model

import "time"

type ValuationSuggestionCall struct {
	CallKey      string    `gorm:"column:call_key;primaryKey;size:320"`
	OperationKey string    `gorm:"column:operation_key;size:320;not null"`
	DraftID      string    `gorm:"column:draft_id;size:128;not null"`
	IPHash       *string   `gorm:"column:ip_hash;size:64"`
	ReservedAt   time.Time `gorm:"column:reserved_at;not null"`
}

func (ValuationSuggestionCall) TableName() string { return "valuation_suggestion_calls" }

type ValuationSuggestionBudget struct {
	Scope           string    `gorm:"column:scope;primaryKey;size:16"`
	BudgetKey       string    `gorm:"column:budget_key;primaryKey;size:128"`
	Used            int       `gorm:"column:used;not null"`
	WindowStartedAt time.Time `gorm:"column:window_started_at;not null"`
}

func (ValuationSuggestionBudget) TableName() string { return "valuation_suggestion_budgets" }
