// Package handler 实现 HTTP 处理器
// 本文件：估值模块 HTTP 测试 seam——用内存 adapter 装配生产路由（RegisterRoutes），
// 提供与主体系 performRequest 一致的请求 helper。测试不连 Postgres/Redis。
// 覆盖路线与生产 wiring 同源：任何路由漂移都在这里暴露。
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
)

// =====================================================
// 内存 adapter：字典（评估路径读方法显式实现，配置 CRUD 面未实现——调用即 panic）
// =====================================================

type memDictStore struct {
	DictionaryConfigStore // 嵌入 nil：CRUD 写面与其余只读面未实现（本 seam 覆盖范围内不调用）
	vehicleTypes          map[string]repository.VehicleType
	conditions            map[string]repository.ConditionRating
	originalPrices        []repository.OriginalPrice
	coefficients          map[string]float64
}

// newSeedMemDict 与迁移种子对齐的默认字典。
func newSeedMemDict() *memDictStore {
	return &memDictStore{
		vehicleTypes: map[string]repository.VehicleType{
			"电动叉车": {ID: 1, Name: "电动叉车", PowerType: "electric", EarliestFactoryYear: 2000},
		},
		conditions: map[string]repository.ConditionRating{
			"A": {ID: 1, Rating: "A", Label: "优秀", BaseCoefficient: 1.00},
			"B": {ID: 2, Rating: "B", Label: "良好", BaseCoefficient: 0.90},
			"C": {ID: 3, Rating: "C", Label: "一般", BaseCoefficient: 0.78},
			"D": {ID: 4, Rating: "D", Label: "较差", BaseCoefficient: 0.65},
			"E": {ID: 5, Rating: "E", Label: "差", BaseCoefficient: 0.50},
		},
		originalPrices: []repository.OriginalPrice{{
			ID: 1, Brand: "合力", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
			ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, OriginalPrice: 100000,
		}},
		coefficients: map[string]float64{
			vservice.KeyLambdaElectric:             0.12,
			vservice.KeyLambdaCombustion:           0.10,
			vservice.KeyAnnualUsageHours:           1750,
			vservice.KeyConfidenceRange:            0.10,
			vservice.KeyKHoursRatioLow:             0.7,
			vservice.KeyKHoursRatioMid:             1.0,
			vservice.KeyKHoursRatioHigh:            1.3,
			vservice.KeyKHoursRatioMax:             1.6,
			vservice.KeyKcPaintBonus:               0.02,
			vservice.KeyKcMaintenanceBonus:         0.02,
			vservice.KeyKcNoLicensePenaltyPct:      0.10,
			vservice.KeyKcNoRegistrationPenaltyPct: 0.10,
		},
	}
}

func (m *memDictStore) GetVehicleTypeByName(_ context.Context, name string) (repository.VehicleType, error) {
	if vt, ok := m.vehicleTypes[name]; ok {
		return vt, nil
	}
	return repository.VehicleType{}, pgx.ErrNoRows
}

func (m *memDictStore) GetConditionRating(_ context.Context, rating string) (repository.ConditionRating, error) {
	if c, ok := m.conditions[rating]; ok {
		return c, nil
	}
	return repository.ConditionRating{}, pgx.ErrNoRows
}

func (m *memDictStore) GetRegionCoefficient(context.Context, string, string) (repository.RegionCoefficient, error) {
	return repository.RegionCoefficient{}, pgx.ErrNoRows
}

func (m *memDictStore) GetBrandByName(context.Context, string) (repository.Brand, error) {
	return repository.Brand{}, pgx.ErrNoRows
}

func (m *memDictStore) FindOriginalPriceMatch(_ context.Context, brand, vehicleType, series string, tonnage float64, configType, mastType string, mastHeightMM int) (repository.OriginalPrice, error) {
	for _, op := range m.originalPrices {
		if op.Brand == brand && op.VehicleType == vehicleType && op.Series == series &&
			op.Tonnage == tonnage && op.ConfigType == configType && op.MastType == mastType &&
			op.MastHeightMM == mastHeightMM {
			return op, nil
		}
	}
	return repository.OriginalPrice{}, pgx.ErrNoRows
}

func (m *memDictStore) FindOriginalPriceFuzzy(context.Context, string, string, string, float64) (repository.OriginalPrice, error) {
	return repository.OriginalPrice{}, pgx.ErrNoRows
}

func (m *memDictStore) GetCoefficientByKey(_ context.Context, key string) (repository.CoefficientConfig, error) {
	if v, ok := m.coefficients[key]; ok {
		return repository.CoefficientConfig{Key: key, Value: v}, nil
	}
	return repository.CoefficientConfig{}, pgx.ErrNoRows
}

func (m *memDictStore) ListCoefficientConfigs(_ context.Context) ([]repository.CoefficientConfig, error) {
	out := make([]repository.CoefficientConfig, 0, len(m.coefficients))
	for k, v := range m.coefficients {
		out = append(out, repository.CoefficientConfig{Key: k, Value: v})
	}
	return out, nil
}

// =====================================================
// 内存 adapter：评估存储 / 电池存储 / PDF 生成
// =====================================================

type memEvalStore struct {
	nextID  int64
	records map[int64]model.EvaluationDetail
}

func newMemEvalStore() *memEvalStore {
	return &memEvalStore{nextID: 1, records: map[int64]model.EvaluationDetail{}}
}

func (m *memEvalStore) CreateEvaluation(_ context.Context, p *repository.CreateEvaluationParams) (int64, error) {
	id := m.nextID
	m.nextID++
	m.records[id] = model.EvaluationDetail{
		ID: id, Brand: p.Brand, VehicleType: p.VehicleType, Series: p.Series, Tonnage: p.Tonnage,
		ConfigType: p.ConfigType, MastType: p.MastType, MastHeightMM: p.MastHeightMM,
		FactoryYear: p.FactoryYear, SaleYear: p.SaleYear, UsageHours: p.UsageHours,
		OriginalPaint: p.OriginalPaint, Province: p.Province, City: p.City,
		HasLicensePlate: p.HasLicensePlate, HasRegistrationCertificate: p.HasRegistrationCertificate,
		HasMaintenanceRecords: p.HasMaintenanceRecords, ConditionRating: p.ConditionRating,
		OriginalPrice: p.OriginalPrice, KTime: p.KTime, KHours: p.KHours, KBrand: p.KBrand,
		KCondition: p.KCondition, KMarket: p.KMarket,
		EstimatedValue: p.EstimatedValue, ConfidenceLow: p.ConfidenceLow, ConfidenceHigh: p.ConfidenceHigh,
		Suggestions: p.Suggestions, LambdaElectric: p.LambdaElectric, LambdaCombustion: p.LambdaCombustion,
	}
	return id, nil
}

func (m *memEvalStore) GetEvaluation(_ context.Context, id int64) (*model.EvaluationDetail, error) {
	d, ok := m.records[id]
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return &d, nil
}

func (m *memEvalStore) GetEvaluationByUser(ctx context.Context, id int64, _ int) (*model.EvaluationDetail, error) {
	return m.GetEvaluation(ctx, id)
}

func (m *memEvalStore) CountEvaluations(context.Context, string, int) (int, error) {
	return len(m.records), nil
}

func (m *memEvalStore) ListEvaluations(context.Context, string, int, int, int) ([]model.EvaluationDetail, error) {
	out := make([]model.EvaluationDetail, 0, len(m.records))
	for _, d := range m.records {
		out = append(out, d)
	}
	return out, nil
}

func (m *memEvalStore) UpdateEvaluationReportPath(_ context.Context, id int64, path string) error {
	d, ok := m.records[id]
	if !ok {
		return pgx.ErrNoRows
	}
	d.ReportPdfPath = path
	m.records[id] = d
	return nil
}

type memBatteryStore struct{}

func (m *memBatteryStore) CreateEvaluation(_ context.Context, eval *model.BatteryEvaluation, _ []model.CycleFeature, _ int) (*model.BatteryEvaluation, error) {
	eval.ID = 1
	return eval, nil
}

func (m *memBatteryStore) GetEvaluation(context.Context, int64) (*model.BatteryEvaluation, error) {
	return nil, pgx.ErrNoRows
}

func (m *memBatteryStore) GetEvaluationByUser(context.Context, int64, int) (*model.BatteryEvaluation, error) {
	return nil, pgx.ErrNoRows
}

func (m *memBatteryStore) ListEvaluations(context.Context, string, int, int, int) ([]model.BatteryEvaluationSummary, int, error) {
	return nil, 0, nil
}

func (m *memBatteryStore) UpdateReportPath(context.Context, int64, string) error {
	return nil
}

type memReportGenerator struct{}

func (m *memReportGenerator) GenerateReport(*model.EvaluationDetail, map[string]float64, []string) ([]byte, error) {
	return []byte("fake-pdf"), nil
}

// =====================================================
// 测试引擎装配 + 请求 helper
// =====================================================

// newTestValuationEngine 用内存 adapter 装配生产路由（与 main 相同的 RegisterRoutes）。
// 返回引擎与各 store 引用，测试可按需断言或注入数据。
func newTestValuationEngine(t *testing.T) (*gin.Engine, *memDictStore, *memEvalStore) {
	return newTestValuationEngineWithStorage(t, storage.NewLocalStorage(t.TempDir()))
}

// newTestValuationEngineWithStorage 允许注入自定义存储（报告测试用计数 fake）。
func newTestValuationEngineWithStorage(t *testing.T, st storage.Storage) (*gin.Engine, *memDictStore, *memEvalStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dict := newSeedMemDict()
	evalStore := newMemEvalStore()

	valuationSvc, err := vservice.NewValuationService(dict, evalStore)
	if err != nil {
		t.Fatalf("构造估值服务失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	r.Use(gin.Recovery())

	RegisterRoutes(r, cfg, zap.NewNop(),
		dict, evalStore, &memBatteryStore{},
		valuationSvc, vservice.NewBatteryRULService(),
		&memReportGenerator{}, st,
		nil, // ValuationAuthService 未装配：本 seam 覆盖的公开评估路径不触达 /auth/*
	)
	return r, dict, evalStore
}

// performRequest 向测试引擎发起请求并返回响应记录器（与主体系 router_test_helper 同模式）。
func performRequest(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	return performRequestWithAuth(r, method, path, body, "")
}

// performRequestWithAuth 携带 Bearer token 发起请求。
func performRequestWithAuth(r *gin.Engine, method, path string, body interface{}, auth string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// decodeBody 解包统一信封 {code, message, data}。
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) (int, string, map[string]interface{}) {
	t.Helper()
	var env struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应不是统一信封 JSON: %v\nbody=%s", err, w.Body.String())
	}
	return env.Code, env.Message, env.Data
}

// authHeader 用 seam 的测试密钥签发 Bearer token。
// 黑名单检查无 Redis 时 fail-open（IsRevoked 出错视为未吊销），认证组路由可测。
func authHeader(t *testing.T, userID int) string {
	t.Helper()
	sess := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	token, err := sess.Issue(userID, "testuser", "hrwai_user")
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return "Bearer " + token
}

// =====================================================
// 冒烟测试：评估创建成功路径穿过整个 seam
// =====================================================

func TestEvaluationCreate_Smoke(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}

	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d, 期望 0\nbody=%s", code, w.Body.String())
	}
	if data["id"] == nil || data["id"].(float64) <= 0 {
		t.Errorf("缺少持久化 id: %v", data["id"])
	}
	est, ok := data["estimated_value"].(float64)
	if !ok || est <= 0 {
		t.Errorf("estimated_value 缺失或非正数: %v", data["estimated_value"])
	}
	if est > 100000 {
		t.Errorf("残值超过原价钳制: %v > 100000", est)
	}
	dims, ok := data["dimension_scores"].([]interface{})
	if !ok || len(dims) != 5 {
		t.Errorf("dimension_scores 应为 5 项: %v", data["dimension_scores"])
	}
	// 维度顺序锁定（与雷达图一致）：出厂时间 / 使用强度 / 品牌价值 / 市场需求 / 车辆情况
	wantOrder := []string{"出厂时间", "使用强度", "品牌价值", "市场需求", "车辆情况"}
	for i, d := range dims {
		item := d.(map[string]interface{})
		if item["label"] != wantOrder[i] {
			t.Errorf("维度顺序错位: 第 %d 项 = %v, 期望 %s", i, item["label"], wantOrder[i])
		}
	}
	sugs, ok := data["suggestions"].([]interface{})
	if !ok || len(sugs) == 0 {
		t.Errorf("suggestions 应为非空数组: %v", data["suggestions"])
	}
}

func TestEvaluationCreate_InvalidParam_Envelope(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	// factory_year < 1900 → 业务校验失败（Evaluate 返回错误 → HTTP 400 + 非 0 业务码）
	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", map[string]interface{}{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列",
		"tonnage": 3, "config_type": "标准", "mast_type": "标准门架",
		"mast_height_mm": 3000, "factory_year": 1800, "sale_year": 2024,
		"usage_hours": 1000, "province": "安徽省", "city": "合肥市", "condition_rating": "A",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("状态码 = %d, 期望 400\nbody=%s", w.Code, w.Body.String())
	}
	code, _, _ := decodeBody(t, w)
	if code != http.StatusBadRequest {
		t.Fatal("非法参数应返回非 0 业务码")
	}
}

func TestHealthCheck_Smoke(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)
	w := performRequest(r, http.MethodGet, "/api/valuation/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	// 健康检查是探活端点，非业务信封（{status, service, timestamp}）
	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil || health.Status != "ok" {
		t.Errorf("健康检查响应异常: %s", w.Body.String())
	}
}

// TestEvaluationFactuality_LockedSuggestionsAndLambda 评估事实性（ADR-0004）：
// 创建时锁定的建议与 λ 值，在系数配置变更后从详情读取仍保持不变；创建响应携带 λ。
func TestEvaluationFactuality_LockedSuggestionsAndLambda(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)

	req := baseEvalRequest()
	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", req)
	if w.Code != http.StatusOK {
		t.Fatalf("创建状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("创建业务码 = %d\nbody=%s", code, w.Body.String())
	}

	createSuggestions := mustStringSlice(t, data["suggestions"])
	if len(createSuggestions) == 0 {
		t.Fatal("创建响应缺少建议")
	}
	lambdaE := data["lambda_electric"].(float64)
	if lambdaE != 0.12 {
		t.Errorf("创建响应 lambda_electric = %v, 期望 0.12", lambdaE)
	}
	if data["lambda_combustion"].(float64) != 0.10 {
		t.Errorf("创建响应 lambda_combustion = %v, 期望 0.10", data["lambda_combustion"])
	}

	// 修改系数配置（影响建议文案的 Kc 修正项）
	dict.coefficients[vservice.KeyKcPaintBonus] = 0.05

	// 登录用户读取详情：建议与 λ 必须是评估时点锁定值（不被新配置改写）
	w = performRequestWithAuth(r, http.MethodGet, "/api/valuation/evaluations/1", nil, authHeader(t, 1))
	if w.Code != http.StatusOK {
		t.Fatalf("详情状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, detail := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("详情业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if got := mustStringSlice(t, detail["suggestions"]); !equalStrings(got, createSuggestions) {
		t.Errorf("详情建议与创建时不一致:\n创建: %v\n详情: %v", createSuggestions, got)
	}
	if detail["lambda_electric"].(float64) != lambdaE {
		t.Errorf("详情 lambda_electric 漂移: %v ≠ %v", detail["lambda_electric"], lambdaE)
	}
}

// baseEvalRequest 评估创建请求（与冒烟测试共用）。
func baseEvalRequest() model.EvaluationRequest {
	return model.EvaluationRequest{
		Brand: "合力", VehicleType: "电动叉车", Series: "K系列",
		Tonnage: 3, ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
		FactoryYear: 2019, SaleYear: 2024, UsageHours: 1000,
		Province: "安徽省", City: "合肥市", ConditionRating: "A",
	}
}

func mustStringSlice(t *testing.T, v interface{}) []string {
	t.Helper()
	raw, ok := v.([]interface{})
	if !ok {
		t.Fatalf("期望字符串数组, got %T: %v", v, v)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("数组元素非字符串: %T", item)
		}
		out = append(out, s)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// =====================================================
// 报告协调器测试（#16）
// =====================================================

// memStorage 计数存储 fake：统计 Save 次数（断言并发下载不重复上传）。
type memStorage struct {
	saves int
	urls  map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{urls: map[string][]byte{}}
}

func (m *memStorage) Save(_ context.Context, key string, content []byte, _ string) (string, error) {
	m.saves++
	m.urls[key] = content
	return "https://fake-cdn/" + key, nil
}

func (m *memStorage) Delete(context.Context, string) error { return nil }

func (m *memStorage) Exists(_ context.Context, url string) (bool, error) {
	key := strings.TrimPrefix(url, "https://fake-cdn/")
	_, ok := m.urls[key]
	return ok, nil
}

func createEvalForReport(t *testing.T, r *gin.Engine) float64 {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations", baseEvalRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("创建评估失败: %d\n%s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	return data["id"].(float64)
}

// TestReportGenerate_WritesPath 生成报告：上传 + 回写路径（既有行为保留）。
func TestReportGenerate_WritesPath(t *testing.T) {
	st := newMemStorage()
	r, _, _ := newTestValuationEngineWithStorage(t, st)
	id := createEvalForReport(t, r)

	w := performRequest(r, http.MethodPost, "/api/valuation/evaluations/1/report", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("生成报告状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d\nbody=%s", code, w.Body.String())
	}
	if data["pdf_url"] == nil || data["pdf_url"].(string) == "" {
		t.Error("生成报告缺少 pdf_url")
	}
	if int(data["file_size"].(float64)) != len("fake-pdf") {
		t.Errorf("file_size 错误: %v", data["file_size"])
	}
	if st.saves != 1 {
		t.Errorf("应上传 1 次, got %d", st.saves)
	}
	if id != 1 {
		t.Fatalf("预期 id=1, got %v", id)
	}
}

// TestReportDownload_RegeneratesOnMissing 下载时路径缺失 → 再生成 → 302 回写（既有行为保留）。
func TestReportDownload_RegeneratesOnMissing(t *testing.T) {
	st := newMemStorage()
	r, _, _ := newTestValuationEngineWithStorage(t, st)
	createEvalForReport(t, r)

	w := performRequest(r, http.MethodGet, "/api/valuation/evaluations/1/report", nil)
	if w.Code != http.StatusFound {
		t.Fatalf("下载状态码 = %d, 期望 302\nbody=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc == "" {
		t.Error("302 缺少 Location")
	}
	if st.saves != 1 {
		t.Errorf("缺失路径应再生成上传 1 次, got %d", st.saves)
	}
}

// TestReportDownload_ConcurrentSingleGeneration 并发下载同 ID 只产生一份 PDF（singleflight 去重）。
func TestReportDownload_ConcurrentSingleGeneration(t *testing.T) {
	st := newMemStorage()
	r, _, _ := newTestValuationEngineWithStorage(t, st)
	createEvalForReport(t, r)

	const n = 8
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/valuation/evaluations/1/report", nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			codes[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusFound {
			t.Errorf("并发请求 %d 状态码 = %d, 期望 302", i, c)
		}
	}
	if st.saves != 1 {
		t.Errorf("并发下载应只上传 1 份 PDF（无孤儿文件）, got %d", st.saves)
	}
}
