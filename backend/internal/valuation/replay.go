package valuation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type ReplayInput struct {
	Request            EvaluationRequest   `json:"request"`
	Brands             []Brand             `json:"brands"`
	VehicleTypes       []VehicleType       `json:"vehicle_types"`
	ConditionRatings   []ConditionRating   `json:"condition_ratings"`
	RegionCoefficients []RegionCoefficient `json:"region_coefficients"`
	OriginalPrices     []OriginalPrice     `json:"original_prices"`
	Coefficients       []CoefficientConfig `json:"coefficients"`
	TrialPolicies      []TrialPolicy       `json:"trial_policies,omitempty"`
}

func (in ReplayInput) Validate() error {
	if err := in.Request.Validate(); err != nil {
		return err
	}
	if len(in.Brands) == 0 || len(in.VehicleTypes) == 0 || len(in.ConditionRatings) == 0 ||
		len(in.RegionCoefficients) == 0 || len(in.OriginalPrices) == 0 || len(in.Coefficients) == 0 {
		return fmt.Errorf("离线回放需要完整目录与系数快照")
	}
	return nil
}

func Replay(ctx context.Context, in ReplayInput) (*EvaluationResult, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	service := &Service{}
	return service.evaluateWithDictionary(ctx, &in.Request, newReplayDictionary(in))
}

func (s *Service) Replay(ctx context.Context, in ReplayInput) (*EvaluationResult, error) {
	return Replay(ctx, in)
}

type replayDictionary struct {
	input ReplayInput
}

func newReplayDictionary(in ReplayInput) *replayDictionary {
	return &replayDictionary{input: in}
}

func (r *replayDictionary) GetBrandByName(_ context.Context, name string) (Brand, error) {
	for _, brand := range r.input.Brands {
		if brand.Name == name {
			return brand, nil
		}
	}
	return Brand{}, pgx.ErrNoRows
}

func (r *replayDictionary) GetConditionRating(_ context.Context, rating string) (ConditionRating, error) {
	for _, condition := range r.input.ConditionRatings {
		if condition.Rating == rating {
			return condition, nil
		}
	}
	return ConditionRating{}, pgx.ErrNoRows
}

func (r *replayDictionary) GetRegionCoefficient(_ context.Context, province, city string) (RegionCoefficient, error) {
	for _, region := range r.input.RegionCoefficients {
		if region.Province == province && region.City == city {
			return region, nil
		}
	}
	return RegionCoefficient{}, pgx.ErrNoRows
}

func (r *replayDictionary) GetVehicleTypeByName(_ context.Context, name string) (VehicleType, error) {
	for _, vehicleType := range r.input.VehicleTypes {
		if vehicleType.Name == name {
			return vehicleType, nil
		}
	}
	return VehicleType{}, pgx.ErrNoRows
}

func (r *replayDictionary) FindOriginalPriceMatch(_ context.Context, brand, vehicleType, series string, tonnage float64, configType, mastType string, mastHeightMM int) (OriginalPrice, error) {
	for _, price := range r.input.OriginalPrices {
		if price.Brand == brand && price.VehicleType == vehicleType && price.Series == series &&
			price.Tonnage == tonnage && price.ConfigType == configType && price.MastType == mastType &&
			price.MastHeightMM == mastHeightMM {
			return price, nil
		}
	}
	return OriginalPrice{}, pgx.ErrNoRows
}

func (r *replayDictionary) FindOriginalPriceFuzzy(_ context.Context, brand, vehicleType, series string, tonnage float64) (OriginalPrice, error) {
	var matched OriginalPrice
	found := false
	for _, price := range r.input.OriginalPrices {
		if price.Brand == brand && price.VehicleType == vehicleType && price.Tonnage == tonnage &&
			(series == "" || price.Series == series) && (!found || price.OriginalPrice > matched.OriginalPrice) {
			matched = price
			found = true
		}
	}
	if !found {
		return OriginalPrice{}, pgx.ErrNoRows
	}
	return matched, nil
}

func (r *replayDictionary) GetCoefficientByKey(_ context.Context, key string) (CoefficientConfig, error) {
	for _, coefficient := range r.input.Coefficients {
		if coefficient.Key == key {
			return coefficient, nil
		}
	}
	return CoefficientConfig{}, pgx.ErrNoRows
}

func (r *replayDictionary) ListCoefficientConfigs(context.Context) ([]CoefficientConfig, error) {
	return r.input.Coefficients, nil
}
