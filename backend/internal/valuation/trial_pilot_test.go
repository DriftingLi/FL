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

func TestTrialReadinessStaysBlockedUntilApprovalsRecorded(t *testing.T) {
	readiness := trialReadiness()
	if readiness.Ready {
		t.Fatal("未取得任何批准时准备度必须 blocked，开关不得开启")
	}
	want := map[string]bool{
		"formula_regression":    false,
		"sample_replay_holdout": false,
		"launch_condition_2949": false,
		"allowlist_width_2951":  false,
		"policy_width_unified":  false,
	}
	for _, blocker := range readiness.Blockers {
		approved, ok := want[blocker.Key]
		if !ok {
			t.Fatalf("上线阻塞项未登记: %s", blocker.Key)
		}
		if blocker.Approved != approved {
			t.Fatalf("%s 的批准只能来自决策记录，不得在代码里伪造: %+v", blocker.Key, blocker)
		}
		if blocker.Detail == "" || blocker.Evidence == "" {
			t.Fatalf("阻塞项必须写清原因与依据: %+v", blocker)
		}
		delete(want, blocker.Key)
	}
	if len(want) != 0 {
		t.Fatalf("缺少必须报告的阻塞项: %v", want)
	}
	wantPending := "formula_regression, sample_replay_holdout, launch_condition_2949, allowlist_width_2951, policy_width_unified"
	if pending := trialReadinessPending(readiness); pending != wantPending {
		t.Fatalf("未批准项清单漂移: got=%s want=%s", pending, wantPending)
	}
}

func TestTrialMetricEventKeepsReasonOutOfPolicyVersion(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	snapshot.Control = TrialControl{}
	result, err := CalculateTrial(context.Background(), trialCompleteRequest(), snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	event := NewTrialMetricEvent(trialCompleteRequest(), result, 3)
	if event.PolicyVersion == TrialReasonGlobalDisabled {
		t.Fatalf("关闭原因不得写进 policy_version 维度: %+v", event)
	}
	if event.UnpricedReason != TrialReasonGlobalDisabled {
		t.Fatalf("关闭原因必须落独立原因字段 unpriced_reason: %+v", event)
	}
}

func TestTrialPolicyAllowlistGapIsStructured(t *testing.T) {
	snapshot, err := NewTrialSnapshot(ReplayInput{})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Control = TrialControl{Enabled: true, Version: 1}
	req := trialMinimumRequest()
	req.Policy = "pilot"
	result, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if result.ReasonCode != TrialReasonPolicyNotConfigured {
		t.Fatalf("名单外必须 fail-closed: %+v", result)
	}
	if len(result.MissingFields) != 1 || result.MissingFields[0] != "policy_allowlist" {
		t.Fatalf("名单外必须给出结构化缺失项而不是空提示: %+v", result.MissingFields)
	}
	if result.NextAction != TrialNextActionSwitchToFormal {
		t.Fatalf("名单外的下一步必须是转正式评估: %q", result.NextAction)
	}
}

func TestTrialTooWideCarriesStructuredNextAction(t *testing.T) {
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
	if result.ReasonCode != TrialReasonTooWide || result.Interval != nil {
		t.Fatalf("过宽必须不给价: %+v", result)
	}
	if len(result.MissingFields) == 0 || result.NextAction != TrialNextActionSupplementFields {
		t.Fatalf("过宽必须给出补件字段与下一步: fields=%v next=%q", result.MissingFields, result.NextAction)
	}
}

func TestTrialMetricProjectionMatchesEventValues(t *testing.T) {
	columns := 1 + len(trialMetricDimensionColumns) + len(trialMetricMeasureColumns())
	if got := len(trialMetricEventValues(TrialMetricEvent{})); got != columns {
		t.Fatalf("指标列与取值错位: columns=%d values=%d", columns, got)
	}
	// Summary 的聚合清单由同一张度量表派生：只跳过 unpriced_reason / model_name 两个只写列。
	if got := strings.Count(trialMetricSummaryAggregates(), "SUM("); got != len(trialMetricMeasureColumns())-2 {
		t.Fatalf("Summary 聚合列数漂移: aggregates=%d measures=%d", got, len(trialMetricMeasureColumns()))
	}
}

func TestTrialEmptyAllowlistIsAValidReplacement(t *testing.T) {
	if err := validateTrialPolicySet(nil); err != nil {
		t.Fatalf("空列表用于清空 allowlist，必须合法: %v", err)
	}
	if err := validateTrialPolicySet([]TrialPolicyInput{}); err != nil {
		t.Fatalf("空数组用于清空 allowlist，必须合法: %v", err)
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
	// 上线准备度未通过时开关不可开（#2958 审查项 9）：API 必须明确拒绝，且不得在代码里伪造批准。
	if w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/control", map[string]any{"enabled": true}, adminAuth); w.Code != http.StatusConflict {
		t.Fatalf("准备度未通过时开启开关必须 409: %d %s", w.Code, w.Body.String())
	}
	readinessResp := performRequestWithAuth(r, http.MethodGet, "/api/valuation/admin/trials/control", nil, adminAuth)
	if readinessResp.Code != http.StatusOK {
		t.Fatalf("读取开关与准备度失败: %d %s", readinessResp.Code, readinessResp.Body.String())
	}
	_, _, controlData := decodeBody(t, readinessResp)
	readiness := controlData["readiness"].(map[string]any)
	if readiness["ready"].(bool) {
		t.Fatalf("未取得任何批准时准备度必须 blocked: %v", readiness)
	}
	blockers := readiness["blockers"].([]any)
	if len(blockers) == 0 {
		t.Fatalf("准备度必须逐项列出阻塞项: %v", readiness)
	}
	for _, raw := range blockers {
		if raw.(map[string]any)["approved"].(bool) {
			t.Fatalf("代码里不得预填已批准: %v", raw)
		}
	}
	// 测试用直连 SQL 模拟「批准后」的开关状态：批准本身只能来自决策记录，
	// 这里只让下游计算与观测链路可被验证。
	if _, err := pool.Exec(ctx, `UPDATE valuation_trial_control SET enabled = TRUE, version = version + 1 WHERE id = 1`); err != nil {
		t.Fatal(err)
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

	// 空数组是显式清空 allowlist；null 才是请求写错（nil 语义与空数组不同）。
	if w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/policies", map[string]any{"policies": []any{}}, adminAuth); w.Code != http.StatusOK {
		t.Fatalf("空数组必须允许清空 allowlist: %d %s", w.Code, w.Body.String())
	}
	if cleared, err := store.ListTrialPolicies(ctx, TrialPolicyFilter{}); err != nil || len(cleared) != 0 {
		t.Fatalf("清空后 allowlist 必须为空: policies=%v err=%v", cleared, err)
	}
	if w := performRequestWithAuth(r, http.MethodPut, "/api/valuation/admin/trials/policies", map[string]any{"policies": nil}, adminAuth); w.Code != http.StatusBadRequest {
		t.Fatalf("null policies 必须拒绝（与空数组合法清空区分）: %d %s", w.Code, w.Body.String())
	}
	// 名单外 403 必须带结构化缺失项与下一步，不是空提示（#2958 审查项 8）。
	clearedReq := trialReq
	clearedReq.RequestID = "pg-pilot-cleared-allowlist"
	if w := performRequest(r, http.MethodPost, "/api/valuation/trials", clearedReq); w.Code != http.StatusForbidden {
		t.Fatalf("清空 allowlist 后必须 403: %d %s", w.Code, w.Body.String())
	} else if _, _, data := decodeBody(t, w); data["reason_code"] != TrialReasonPolicyNotConfigured ||
		data["next_action"] != TrialNextActionSwitchToFormal ||
		len(data["missing_fields"].([]any)) != 1 || data["missing_fields"].([]any)[0] != "policy_allowlist" {
		t.Fatalf("名单外必须返回结构化缺失项与转正式下一步: %v", data)
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
		suggestions, ok := evaluation["suggestions"].([]any)
		if !ok || len(suggestions) == 0 {
			t.Fatalf("关闭试算后正式评估必须仍产出建议: %v", evaluation["suggestions"])
		}
		id := int64(evaluation["id"].(float64))
		if w := performRequest(r, http.MethodPost, fmt.Sprintf("/api/valuation/evaluations/%d/report", id), nil); w.Code != http.StatusOK {
			t.Fatalf("关闭试算不得影响正式报告: %d %s", w.Code, w.Body.String())
		}
	}
	// 回退演练：关停参考试算后对话草稿照常可用（#2958 审查项 11）。
	if w := performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts", map[string]any{}); w.Code != http.StatusOK {
		t.Fatalf("关闭试算不得影响对话草稿: %d %s", w.Code, w.Body.String())
	} else if _, _, draft := decodeBody(t, w); draft["id"] == nil || draft["id"] == "" {
		t.Fatalf("对话草稿必须返回可用能力 ID: %v", draft)
	}
	// 开关记录缺失按关闭处理：403 global_disabled，不得 500（#2958 审查项 5）。
	if _, err := pool.Exec(ctx, `DELETE FROM valuation_trial_control`); err != nil {
		t.Fatal(err)
	}
	missingControl := trialReq
	missingControl.RequestID = "pg-pilot-missing-control"
	if w := performRequest(r, http.MethodPost, "/api/valuation/trials", missingControl); w.Code != http.StatusForbidden {
		t.Fatalf("开关记录缺失必须 403 关闭态而不是 500: %d %s", w.Code, w.Body.String())
	} else if _, _, data := decodeBody(t, w); data["reason_code"] != TrialReasonGlobalDisabled || data["next_action"] != TrialNextActionSwitchToFormal {
		t.Fatalf("开关记录缺失必须回 global_disabled 且给出转正式下一步: %v", data)
	}
	if w := performRequestWithAuth(r, http.MethodGet, "/api/valuation/admin/trials/control", nil, adminAuth); w.Code != http.StatusOK {
		t.Fatalf("开关记录缺失时管理读面也按关闭处理而不是 500: %d %s", w.Code, w.Body.String())
	} else if _, _, data := decodeBody(t, w); data["enabled"].(bool) {
		t.Fatalf("开关记录缺失必须报关闭: %v", data)
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
	conversationSvc := NewConversationService(NewDraftRepository(pool), NewOperationRepository(pool), valuationSvc, evaluations, time.Now)
	r := gin.New()
	r.Use(gin.Recovery())
	attachAdminCapabilities(r)
	RegisterRoutes(r, session, zap.NewNop(), nil, dict, evaluations, NewBatteryRepository(pool), valuationSvc,
		NewBatteryRULService(), &memReportGenerator{}, storage.NewLocalStorage(t.TempDir()), nil, nil, conversationSvc)
	RegisterTrialRoutes(r, NewPostgresTrialSnapshotReader(pool), zap.NewNop(), "development", store)
	RegisterTrialAdminRoutes(r.Group("/api"), session, store, zap.NewNop(), nil)
	return r
}
