package model

import "time"

// ValuationTrialPolicy 参考试算显式 allowlist 与宽度策略。
type ValuationTrialPolicy struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyName       string    `gorm:"size:16;not null" json:"policy_name"`
	Version          int       `gorm:"not null;default:1" json:"version"`
	Brand            string    `gorm:"size:100;not null" json:"brand"`
	VehicleType      string    `gorm:"size:100;not null" json:"vehicle_type"`
	Tonnage          float64   `gorm:"type:numeric(10,2);not null" json:"tonnage"`
	MaxWidthRatio    float64   `gorm:"type:numeric(7,6);not null" json:"max_width_ratio"`
	UsageHourRatios  []float64 `gorm:"type:jsonb;serializer:json;not null" json:"usage_hour_ratios"`
	ConditionRatings []string  `gorm:"type:jsonb;serializer:json;not null" json:"condition_ratings"`
	Enabled          bool      `gorm:"not null;default:false" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (ValuationTrialPolicy) TableName() string { return "valuation_trial_policy" }
