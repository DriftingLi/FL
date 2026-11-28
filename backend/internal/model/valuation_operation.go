package model

import "time"

type ValuationOperation struct {
	OperationKey        string    `gorm:"column:operation_key;primaryKey;size:128"`
	RequestHash         string    `gorm:"column:request_hash;size:64;not null"`
	DraftID             string    `gorm:"column:draft_id;size:128;not null"`
	DraftVersion        int       `gorm:"column:draft_version;not null"`
	FirstAcceptedUserID *int      `gorm:"column:first_accepted_user_id"`
	EvaluationID        *int64    `gorm:"column:evaluation_id"`
	Status              string    `gorm:"column:status;size:16;not null"`
	ErrorCode           *string   `gorm:"column:error_code;size:64"`
	CreatedAt           time.Time `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null"`
}

func (ValuationOperation) TableName() string { return "valuation_operations" }
