package valuation

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

var nullableOutletsValuation = map[string]func(t *testing.T) any{
	"valuation.AlgorithmParameters.coefficients":        outletAlgorithmParameters,
	"valuation.AlgorithmParameters.brands":              outletAlgorithmParameters,
	"valuation.AlgorithmParameters.condition_ratings":   outletAlgorithmParameters,
	"valuation.AlgorithmParameters.region_coefficients": outletAlgorithmParameters,
	"valuation.SeriesConfigOptions.transmission":        outletSeriesConfigOptions,
	"valuation.SeriesConfigOptions.engine":              outletSeriesConfigOptions,
	"valuation.SeriesConfigOptions.battery":             outletSeriesConfigOptions,
}

func TestNullableDeclaredOutletsEmitNull(t *testing.T) {
	for key, build := range nullableOutletsValuation {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			if got := testutil.MarshalKey(t, build(t), jsonKey); got != "null" {
				t.Fatalf("声明 nullable 的 %s 实际发出 %s", key, got)
			}
		})
	}
}

type emptyConfigStore struct{ DictionaryConfigStore }

func (emptyConfigStore) ListAlgorithmParameters(context.Context) (AlgorithmParameters, error) {
	return AlgorithmParameters{}, nil
}

func (emptyConfigStore) ListSeriesConfigOptions(context.Context, string, string) (SeriesConfigOptions, error) {
	return SeriesConfigOptions{}, nil
}

func emptyConfigRouter() *gin.Engine {
	r := gin.New()
	h := NewConfigHandler(emptyConfigStore{}, zap.NewNop())
	r.GET("/api/valuation/dictionaries/algorithm-parameters", h.ListAlgorithmParameters)
	r.GET("/api/valuation/dictionaries/series-config-options", h.ListSeriesConfigOptions)
	return r
}

func outletAlgorithmParameters(t *testing.T) any {
	t.Helper()
	w := performRequest(emptyConfigRouter(), http.MethodGet, "/api/valuation/dictionaries/algorithm-parameters", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("算法参数状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}

func outletSeriesConfigOptions(t *testing.T) any {
	t.Helper()
	w := performRequest(emptyConfigRouter(), http.MethodGet, "/api/valuation/dictionaries/series-config-options?brand=missing&series=missing", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("系列配置状态码 = %d: %s", w.Code, w.Body.String())
	}
	return mustDecodeData(t, w)
}
