package model

import "time"

// ValuationTrialPolicy 参考试算显式 allowlist 与宽度策略。
type ValuationTrialPolicy struct {
	ID                               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyName                       string    `gorm:"size:16;not null" json:"policy_name"`
	Version                          int       `gorm:"not null;default:1" json:"version"`
	Brand                            string    `gorm:"size:100;not null" json:"brand"`
	VehicleType                      string    `gorm:"size:100;not null" json:"vehicle_type"`
	Tonnage                          float64   `gorm:"type:numeric(10,2);not null" json:"tonnage"`
	MaxWidthRatio                    float64   `gorm:"type:numeric(7,6);not null" json:"max_width_ratio"`
	UsageHourRatios                  []float64 `gorm:"type:jsonb;serializer:json;not null" json:"usage_hour_ratios"`
	ConditionRatings                 []string  `gorm:"type:jsonb;serializer:json;not null" json:"condition_ratings"`
	OriginalPaintValues              []bool    `gorm:"type:jsonb;serializer:json;not null" json:"original_paint_values"`
	HasLicensePlateValues            []bool    `gorm:"type:jsonb;serializer:json;not null" json:"has_license_plate_values"`
	HasRegistrationCertificateValues []bool    `gorm:"type:jsonb;serializer:json;not null" json:"has_registration_certificate_values"`
	HasMaintenanceRecordsValues      []bool    `gorm:"type:jsonb;serializer:json;not null" json:"has_maintenance_records_values"`
	Enabled                          bool      `gorm:"not null;default:false" json:"enabled"`
	CreatedAt                        time.Time `json:"created_at"`
	UpdatedAt                        time.Time `json:"updated_at"`
}

func (ValuationTrialPolicy) TableName() string { return "valuation_trial_policy" }

// ValuationTrialSnapshot 冻结的试算快照与 request_id 幂等映射。
type ValuationTrialSnapshot struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SnapshotID   string    `gorm:"size:64;not null;index" json:"snapshot_id"`
	RequestID    string    `gorm:"size:128;not null;uniqueIndex" json:"request_id"`
	RequestJSON  string    `gorm:"type:jsonb;not null" json:"request_json"`
	SnapshotJSON string    `gorm:"type:jsonb;not null" json:"snapshot_json"`
	FormulaHash  string    `gorm:"size:64;not null" json:"formula_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func (ValuationTrialSnapshot) TableName() string { return "valuation_trial_snapshots" }

// ValuationTrialControl 独立全局试算开关，固定单行且默认关闭。
type ValuationTrialControl struct {
	ID        uint16    `gorm:"primaryKey" json:"id"`
	Enabled   bool      `gorm:"not null;default:false" json:"enabled"`
	Version   int64     `gorm:"not null;default:1" json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ValuationTrialControl) TableName() string { return "valuation_trial_control" }

// ValuationTrialMetric 无请求标识、正文与自由文本的试算观测事件。
type ValuationTrialMetric struct {
	ID               uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	OccurredAt       time.Time `json:"occurred_at"`
	Brand            string    `gorm:"size:100;not null" json:"brand"`
	VehicleType      string    `gorm:"size:100;not null" json:"vehicle_type"`
	Tonnage          float64   `gorm:"type:numeric(10,2);not null" json:"tonnage"`
	PolicyName       string    `gorm:"size:16;not null" json:"policy_name"`
	Scenario         string    `gorm:"size:32;not null" json:"scenario"`
	FormulaVersion   string    `gorm:"size:32;not null" json:"formula_version"`
	FormulaHash      string    `gorm:"size:64;not null" json:"formula_hash"`
	PolicyVersion    string    `gorm:"size:128;not null" json:"policy_version"`
	Denominator      int       `gorm:"not null;default:1" json:"denominator"`
	Priced           int       `gorm:"not null;default:0" json:"priced"`
	Covered          int       `gorm:"not null;default:0" json:"covered"`
	TooWide          int       `gorm:"not null;default:0" json:"too_wide"`
	UnpricedReason   string    `gorm:"size:64;not null;default:''" json:"unpriced_reason"`
	FollowUpCount    int       `gorm:"not null;default:0" json:"follow_up_count"`
	DurationMS       int64     `gorm:"not null;default:0" json:"duration_ms"`
	ModelName        string    `gorm:"size:100;not null;default:''" json:"model_name"`
	PromptTokens     int64     `gorm:"not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64     `gorm:"not null;default:0" json:"completion_tokens"`
	CostMicros       int64     `gorm:"not null;default:0" json:"cost_micros"`
}

func (ValuationTrialMetric) TableName() string { return "valuation_trial_metrics" }
