package valuation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

func TestTrialGlobalSwitchDefaultsClosedAndOverridesConfiguredPolicy(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	snapshot.Control = TrialControl{}

	response, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonGlobalDisabled || response.Interval != nil {
		t.Fatalf("全局开关默认关闭时必须优先于组合策略 fail-closed: %+v", response)
	}
}

func TestTrialPolicyValidationRejectsUnapprovedAndInvalidScenarios(t *testing.T) {
	valid := TrialPolicyInput{Name: "pilot", Brand: "林德", VehicleType: "电动叉车", Tonnage: 3,
		MaxWidthRatio: 0.4, UsageHourRatios: []float64{0, 1}, ConditionRatings: []string{"B"},
		OriginalPaintValues: []bool{false}, HasLicensePlateValues: []bool{false},
		HasRegistrationCertificateValues: []bool{false}, HasMaintenanceRecordsValues: []bool{false}, Enabled: true}
	if err := validateTrialPolicyInput(valid); err != nil {
		t.Fatalf("完整获批策略应通过: %v", err)
	}
	for name, mutate := range map[string]func(*TrialPolicyInput){
		"宽度":  func(p *TrialPolicyInput) { p.MaxWidthRatio = 1.1 },
		"工时":  func(p *TrialPolicyInput) { p.UsageHourRatios = []float64{} },
		"情景":  func(p *TrialPolicyInput) { p.OriginalPaintValues = []bool{} },
		"策略名": func(p *TrialPolicyInput) { p.Name = "public" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := valid
			mutate(&invalid)
			if err := validateTrialPolicyInput(invalid); err == nil {
				t.Fatal("非法策略必须拒绝")
			}
		})
	}
}

func TestTrialMetricEventContainsOnlyApprovedDimensions(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	result, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	event := NewTrialMetricEvent(trialCompleteRequest(), result, 12)
	if event.Denominator != 1 || event.Priced != 1 || event.Covered != 1 || event.TooWide != 0 || event.FollowUpCount != 0 {
		t.Fatalf("给价指标分母/分子错误: %+v", event)
	}
	if event.Scenario != "complete" || event.PolicyVersion != result.Snapshot.PolicyVersion || event.FormulaHash != TrialFormulaHash {
		t.Fatalf("指标维度不完整: %+v", event)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"request_id", "snapshot_id", "K系列", "合肥市", "input", "body"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("指标不得包含请求标识、正文或目录自由文本: %s", encoded)
		}
	}
}

func TestTrialMetricEventRecordsTooWideAndFollowUp(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 40000}
	higher := price
	higher.ID, higher.OriginalPrice = 2, 60000
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price, higher})
	snapshot.TrialPolicies[0].MaxWidthRatio = 0.2
	req := trialCompleteRequest()
	req.UsageHours = nil
	result, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	event := NewTrialMetricEvent(req, result, 9)
	if event.Priced != 0 || event.Covered != 0 || event.TooWide != 1 || event.UnpricedReason != TrialReasonTooWide || event.FollowUpCount == 0 {
		t.Fatalf("过宽与补问指标错误: %+v", event)
	}
}

func TestPostgresTrialPilotControlMetricsAndRollback(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	var schema string
	if err := db.Raw(`SELECT current_schema()`).Scan(&schema).Error; err != nil {
		t.Fatalf("读取测试 schema 失败: %+v", err)
	}
	base := os.Getenv("DATABASE_URL")
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, base+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	store := NewPostgresTrialAdminStore(pool)
	control, err := store.GetTrialControl(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if control.Enabled || control.Version != 1 {
		t.Fatalf("迁移必须预置独立关闭开关: %+v", control)
	}

	session := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	r := newPilotPostgresRouter(t, pool, session, store)
	request := EvaluationRequest{
		Brand: "林德", VehicleType: "电动平衡重式", Series: "E系列", Tonnage: 1.6,
		ConfigType: "磷酸铁锂(LFP)", MastType: "两级门架", MastHeightMM: 3000,
		FactoryYear: 2021, SaleYear: 2026, UsageHours: 0,
		Province: "安徽省", City: "合肥市", ConditionRating: "B",
		HasRegistrationCertificate: true, HasLicensePlate: true,
	}
	trialReq := TrialRequest{
		RequestID: "pg-pilot-priced", Policy: "pilot", Brand: request.Brand, VehicleType: request.VehicleType,
		Tonnage: request.Tonnage, FactoryYear: trialPtr(request.FactoryYear), SaleYear: request.SaleYear,
		Series: trialPtr(request.Series), ConfigType: trialPtr(request.ConfigType), MastType: trialPtr(request.MastType),
		MastHeightMM: trialPtr(request.MastHeightMM), UsageHours: trialPtr(request.UsageHours),
		Condition: trialPtr(request.ConditionRating), Province: trialPtr(request.Province), City: trialPtr(request.City),
		OriginalPaint: trialPtr(false), HasLicensePlate: trialPtr(true),
		HasRegistrationCertificate: trialPtr(true), HasMaintenanceRecords: trialPtr(false),
	}
	policy := TrialPolicyInput{
		Name: "pilot", Brand: request.Brand, VehicleType: request.VehicleType, Tonnage: request.Tonnage,
		MaxWidthRatio: 0.4, UsageHourRatios: []float64{0, 1}, ConditionRatings: []string{"B"},
		OriginalPaintValues: []bool{false, true}, HasLicensePlateValues: []bool{false, true},
		HasRegistrationCertificateValues: []bool{false, true}, HasMaintenanceRecordsValues: []bool{false, true}, Enabled: true,
	}

	closedReq := trialReq
	closedReq.RequestID = "pg-pilot-default-closed"
	if w := performRequest(r, http.MethodPost, "/api/valuation/trials", closedReq); w.Code != http.StatusForbidden {
		t.Fatalf("默认关闭应拒绝 trials: %d %s", w.Code, w.Body.String())
	}
	if w := performRequest(r, http.MethodPut, "/api/valuation/admin/trials/control", map[string]any{"enabled": true}); w.Code != http.StatusUnauthorized {
		t.Fatalf("管理开关必须要求管理员: %d %s", w.Code, w.Body.String())
	}
	adminAuth := adminAuthHeader(t)
	if w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/control", map[string]any{"enabled": true}, adminAuth); w.Code != http.StatusOK {
		t.Fatalf("开启全局开关失败: %d %s", w.Code, w.Body.String())
	}
	if w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/policies", TrialPoliciesRequest{Policies: []TrialPolicyInput{policy}}, adminAuth); w.Code != http.StatusOK {
		t.Fatalf("更新获批策略失败: %d %s", w.Code, w.Body.String())
	}
	if w := performRequest(r, http.MethodPost, "/api/valuation/trials", trialReq); w.Code != http.StatusOK {
		policies, _ := store.ListTrialPolicies(ctx, TrialPolicyFilter{})
		t.Fatalf("已开启且命中策略应可试算: %d %s policies=%+v", w.Code, w.Body.String(), policies)
	}
	var metricCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuation_trial_metrics`).Scan(&metricCount); err != nil {
		t.Fatal(err)
	}
	if metricCount == 0 {
		err := store.RecordTrialMetric(ctx, NewTrialMetricEvent(trialReq, TrialResponse{Status: TrialStatusPriced, Interval: &TrialInterval{}}, 1))
		t.Fatalf("HTTP 试算未落指标；直接写入错误=%v", err)
	}
	metrics := performRequestWithAuth(r, http.MethodGet, "/api/valuation/admin/trials/metrics?scenario=complete", nil, adminAuth)
	if metrics.Code != http.StatusOK {
		t.Fatalf("指标查询失败: %d %s", metrics.Code, metrics.Body.String())
	}
	_, _, metricData := decodeBody(t, metrics)
	items := metricData["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("应分别聚合关闭态与给价态指标组: %v", metricData)
	}
	var pricedMetric map[string]any
	for _, raw := range items {
		candidate := raw.(map[string]any)
		if candidate["priced"].(float64) == 1 {
			pricedMetric = candidate
		}
	}
	if pricedMetric == nil || pricedMetric["denominator"].(float64) != 1 || pricedMetric["covered"].(float64) != 1 {
		t.Fatalf("试算指标分母/给价/覆盖错误: %v", metricData)
	}
	for _, raw := range items {
		candidate := raw.(map[string]any)
		if candidate["priced"].(float64) == 0 {
			reasons := candidate["unpriced_reasons"].(map[string]any)
			if reasons[TrialReasonGlobalDisabled].(float64) != 1 {
				t.Fatalf("关闭态无价原因未单列: %v", candidate)
			}
		}
	}
	exported := performRequestWithAuth(r, http.MethodGet, "/api/valuation/admin/trials/metrics/export", nil, adminAuth)
	if exported.Code != http.StatusOK || !strings.Contains(exported.Header().Get("Content-Type"), "application/json") {
		_, directErr := store.ListTrialMetrics(ctx, TrialMetricFilter{Scenario: "complete", Limit: 200})
		t.Fatalf("JSON 指标导出失败: %d %s direct=%v", exported.Code, exported.Body.String(), directErr)
	}
	var exportItems []TrialMetricSummary
	if err := json.Unmarshal(exported.Body.Bytes(), &exportItems); err != nil || len(exportItems) != 2 {
		t.Fatalf("JSON 指标导出内容无效: err=%v body=%s", err, exported.Body.String())
	}
	if strings.Contains(exported.Body.String(), trialReq.RequestID) || strings.Contains(exported.Body.String(), request.City) {
		t.Fatalf("指标导出包含 PII 或请求标识: %s", exported.Body.String())
	}

	closed := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/control", map[string]any{"enabled": false}, adminAuth)
	if closed.Code != http.StatusOK {
		t.Fatalf("关闭全局开关失败: %d %s", closed.Code, closed.Body.String())
	}
	retry := trialReq
	retry.RequestID = "pg-pilot-after-close"
	if w := performRequest(r, http.MethodPost, "/api/valuation/trials", retry); w.Code != http.StatusForbidden {
		t.Fatalf("关闭后 trials 必须 disabled: %d %s", w.Code, w.Body.String())
	}
	if w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", request); w.Code != http.StatusOK {
		t.Fatalf("关闭试算不得影响正式评估: %d %s", w.Code, w.Body.String())
	} else {
		_, _, evaluation := decodeBody(t, w)
		id := int64(evaluation["id"].(float64))
		if w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/evaluations/%d/report", id), nil); w.Code != http.StatusOK {
			t.Fatalf("关闭试算不得影响正式报告: %d %s", w.Code, w.Body.String())
		}
	}
}

func newPilotPostgresRouter(t *testing.T, pool *pgxpool.Pool, session *security.Session, store TrialAdminStore) *gin.Engine {
	t.Helper()
	dict := NewDictionaryRepository(pool)
	evaluations := NewEvaluationRepository(pool)
	valuationSvc, err := NewService(dict, evaluations)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	attachAdminCapabilities(r)
	RegisterRoutes(r, session, zap.NewNop(), nil, dict, evaluations, NewBatteryRepository(pool), valuationSvc,
		NewBatteryRULService(), &memReportGenerator{}, storage.NewLocalStorage(t.TempDir()), nil, nil, nil)
	RegisterTrialRoutes(r, NewPostgresTrialSnapshotReader(pool), zap.NewNop(), "development", store)
	RegisterTrialAdminRoutes(r.Group("/api"), session, store, zap.NewNop())
	return r
}
