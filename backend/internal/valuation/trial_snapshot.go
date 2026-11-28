package valuation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const trialFormulaSpec = FormulaVersionV2 + "\nresidual=original_price*Kt^(1/(Kh*Kb))*Kc*Km;residual=min(residual,original_price)"

// TrialFormulaHash 固定公式语义的 SHA-256。
var TrialFormulaHash = sha256Hex([]byte(trialFormulaSpec))

// TrialPolicy 显式试算策略。迁移不预置任何行，未配置组合一律关闭。
type TrialPolicy struct {
	ID                               int64     `json:"id,omitempty"`
	Name                             string    `json:"policy_name"`
	Version                          int       `json:"version"`
	Brand                            string    `json:"brand"`
	VehicleType                      string    `json:"vehicle_type"`
	Tonnage                          float64   `json:"tonnage"`
	MaxWidthRatio                    float64   `json:"max_width_ratio"`
	UsageHourRatios                  []float64 `json:"usage_hour_ratios"`
	ConditionRatings                 []string  `json:"condition_ratings"`
	OriginalPaintValues              []bool    `json:"original_paint_values,omitempty"`
	HasLicensePlateValues            []bool    `json:"has_license_plate_values,omitempty"`
	HasRegistrationCertificateValues []bool    `json:"has_registration_certificate_values,omitempty"`
	HasMaintenanceRecordsValues      []bool    `json:"has_maintenance_records_values,omitempty"`
	Enabled                          bool      `json:"enabled"`
}

// TrialSnapshot 一次事务冻结的目录、系数与策略；全批回放只构造一次。
type TrialSnapshot struct {
	ReplayInput
	Hash          string
	PolicyHash    string
	PolicyVersion string
}

// NewTrialSnapshot 对规范化快照计算哈希，后续计算不再读取可变数据源。
func NewTrialSnapshot(in ReplayInput) (*TrialSnapshot, error) {
	if in.Brands == nil {
		in.Brands = []Brand{}
	}
	if in.VehicleTypes == nil {
		in.VehicleTypes = []VehicleType{}
	}
	if in.ConditionRatings == nil {
		in.ConditionRatings = []ConditionRating{}
	}
	if in.RegionCoefficients == nil {
		in.RegionCoefficients = []RegionCoefficient{}
	}
	if in.OriginalPrices == nil {
		in.OriginalPrices = []OriginalPrice{}
	}
	if in.Coefficients == nil {
		in.Coefficients = []CoefficientConfig{}
	}
	if in.TrialPolicies == nil {
		in.TrialPolicies = []TrialPolicy{}
	}
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("序列化试算快照失败: %w", err)
	}
	var frozen ReplayInput
	if err := json.Unmarshal(payload, &frozen); err != nil {
		return nil, fmt.Errorf("冻结试算快照失败: %w", err)
	}
	in = frozen
	policyPayload, err := json.Marshal(in.TrialPolicies)
	if err != nil {
		return nil, fmt.Errorf("序列化试算策略失败: %w", err)
	}
	policyHash := sha256Hex(policyPayload)
	return &TrialSnapshot{
		ReplayInput:   in,
		Hash:          sha256Hex(payload),
		PolicyHash:    policyHash,
		PolicyVersion: "trial-v1-" + shortHash(policyHash),
	}, nil
}

func (s *TrialSnapshot) meta(policy TrialPolicy) TrialSnapshotMeta {
	return TrialSnapshotMeta{
		Hash:           s.Hash,
		SHA7:           shortHash(s.Hash),
		FormulaVersion: FormulaVersionV2,
		FormulaHash:    TrialFormulaHash,
		Policy:         policy.Name,
		PolicyVersion:  fmt.Sprintf("%s-v%d-%s", policy.Name, policy.Version, shortHash(s.PolicyHash)),
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}
