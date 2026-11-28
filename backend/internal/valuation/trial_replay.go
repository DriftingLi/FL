package valuation

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// TrialReplayCase 一条 JSONL 遮挡用例；complete_request 是正式完整估值对照。
type TrialReplayCase struct {
	Group           string             `json:"group"`
	Request         TrialRequest       `json:"request"`
	CompleteRequest *EvaluationRequest `json:"complete_request,omitempty"`
}

type TrialReplayResult struct {
	Index         int            `json:"index"`
	Group         string         `json:"group"`
	TrialStatus   string         `json:"trial_status"`
	ReasonCode    string         `json:"reason_code,omitempty"`
	Interval      *TrialInterval `json:"interval,omitempty"`
	WidthRatio    *float64       `json:"width_ratio,omitempty"`
	CompleteValid bool           `json:"complete_valid"`
	CompleteValue float64        `json:"complete_value,omitempty"`
	CompleteError string         `json:"complete_error,omitempty"`
}

type TrialWidthMetrics struct {
	N                 int            `json:"n"`
	Valid             int            `json:"valid"`
	Excluded          int            `json:"excluded"`
	Priced            int            `json:"priced"`
	Covered           int            `json:"covered"`
	TooWide           int            `json:"too_wide"`
	PricedOverValid   float64        `json:"priced_over_valid"`
	CoveredOverPriced float64        `json:"covered_over_priced"`
	TooWideOverValid  float64        `json:"too_wide_over_valid"`
	UnpricedReasons   map[string]int `json:"unpriced_reasons"`
}

type TrialReplayGroup struct {
	N          int                          `json:"n"`
	Valid      int                          `json:"valid"`
	Excluded   int                          `json:"excluded"`
	Thresholds map[string]TrialWidthMetrics `json:"thresholds"`
}

type TrialReplayReport struct {
	SnapshotHash   string                      `json:"snapshot_hash"`
	SnapshotSHA7   string                      `json:"snapshot_sha7"`
	PolicyHash     string                      `json:"policy_hash"`
	PolicyVersion  string                      `json:"policy_version"`
	FormulaVersion string                      `json:"formula_version"`
	FormulaHash    string                      `json:"formula_hash"`
	Groups         map[string]TrialReplayGroup `json:"groups"`
	Results        []TrialReplayResult         `json:"results,omitempty"`
}

var trialReplayThresholds = []struct {
	key   string
	value float64
}{
	{key: "20", value: 0.20},
	{key: "30", value: 0.30},
	{key: "40", value: 0.40},
}

// ReplayTrialBatch 全批共享同一冻结快照，并调用在线试算的同一 V2 路径。
func ReplayTrialBatch(ctx context.Context, snapshot *TrialSnapshot, cases []TrialReplayCase, appEnv string) (*TrialReplayReport, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("批量试算回放需要冻结快照")
	}
	report := &TrialReplayReport{
		SnapshotHash:   snapshot.Hash,
		SnapshotSHA7:   shortHash(snapshot.Hash),
		PolicyHash:     snapshot.PolicyHash,
		PolicyVersion:  snapshot.PolicyVersion,
		FormulaVersion: FormulaVersionV2,
		FormulaHash:    TrialFormulaHash,
		Groups:         make(map[string]TrialReplayGroup),
		Results:        make([]TrialReplayResult, 0, len(cases)),
	}
	for index, testCase := range cases {
		groupName := testCase.Group
		if groupName == "" {
			groupName = "default"
		}
		group := report.Groups[groupName]
		if group.Thresholds == nil {
			group.Thresholds = make(map[string]TrialWidthMetrics, len(trialReplayThresholds))
			for _, threshold := range trialReplayThresholds {
				group.Thresholds[threshold.key] = TrialWidthMetrics{UnpricedReasons: make(map[string]int)}
			}
		}
		group.N++
		result := TrialReplayResult{Index: index, Group: groupName}
		if testCase.CompleteRequest == nil {
			result.CompleteError = "complete_request_missing"
			group.Excluded++
		} else {
			formalInput := snapshot.ReplayInput
			formalInput.Request = *testCase.CompleteRequest
			if err := formalInput.Validate(); err != nil {
				result.CompleteError = err.Error()
				group.Excluded++
			} else {
				formal, err := (&Service{}).evaluateWithDictionary(ctx, &formalInput.Request,
					exactOnlyDictionary{DictionaryReader: newReplayDictionary(formalInput)})
				if err != nil {
					if errors.Is(err, ErrOriginalPriceNotFound) {
						result.CompleteError = "complete_catalog_not_exact"
					} else {
						result.CompleteError = err.Error()
					}
					group.Excluded++
				} else {
					result.CompleteValid = true
					result.CompleteValue = formal.EstimatedValue
					group.Valid++
				}
			}
		}
		for _, threshold := range trialReplayThresholds {
			trial, err := CalculateTrialAtWidthLimit(ctx, testCase.Request, snapshot, appEnv, threshold.value)
			if err != nil {
				result.TrialStatus, result.ReasonCode = TrialStatusNeedsInfo, "trial_error"
			} else {
				result.TrialStatus, result.ReasonCode = trial.Status, trial.ReasonCode
				result.Interval, result.WidthRatio = trial.Interval, trial.WidthRatio
			}
			metrics := group.Thresholds[threshold.key]
			metrics.N, metrics.Valid, metrics.Excluded = group.N, group.Valid, group.Excluded
			if result.CompleteValid {
				if result.TrialStatus == TrialStatusPriced && result.Interval != nil {
					metrics.Priced++
					if result.CompleteValue >= result.Interval.Min-1e-9 && result.CompleteValue <= result.Interval.Max+1e-9 {
						metrics.Covered++
					}
				} else {
					reason := result.ReasonCode
					if reason == "" {
						reason = TrialReasonNoValidScenario
					}
					metrics.UnpricedReasons[reason]++
					if reason == TrialReasonTooWide {
						metrics.TooWide++
					}
				}
			}
			metrics.PricedOverValid = ratio(metrics.Priced, metrics.Valid)
			metrics.CoveredOverPriced = ratio(metrics.Covered, metrics.Priced)
			metrics.TooWideOverValid = ratio(metrics.TooWide, metrics.Valid)
			group.Thresholds[threshold.key] = metrics
		}
		report.Groups[groupName] = group
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	value := float64(numerator) / float64(denominator)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
