// Package handler 实现 HTTP 处理器
// 本文件：估值模块 HTTP 测试 seam——用内存 adapter 装配生产路由（RegisterRoutes），
// 提供与主体系 performRequest 一致的请求 helper。测试不连 Postgres/Redis。
// 覆盖路线与生产 wiring 同源：任何路由漂移都在这里暴露。
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/valuation/dictcrud"
	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
	vservice "forklift-training/internal/valuation/service"
)

// =====================================================
// 内存 adapter：字典（评估路径读方法显式实现，其余读面未实现——调用即 panic）
// =====================================================

// memTable 描述符驱动写面的通用内存表：id 自增 + 值按 JSON 字段名存放。
type memTable struct {
	nextID int64
	rows   []memRow
}

type memRow struct {
	id     int64
	values map[string]any
}

type memDictStore struct {
	DictionaryConfigStore // 嵌入 nil：未实现的读面调用即 panic（本 seam 覆盖范围内不调用）
	vehicleTypes          map[string]repository.VehicleType
	conditions            map[string]repository.ConditionRating
	originalPrices        []repository.OriginalPrice
	coefficients          map[string]float64
	regions               []repository.RegionCoefficient
	nextRegionID          int
	tables                map[string]*memTable
}

// table 按实体名取通用内存表（惰性初始化）。
func (m *memDictStore) table(name string) *memTable {
	if m.tables == nil {
		m.tables = map[string]*memTable{}
	}
	t, ok := m.tables[name]
	if !ok {
		t = &memTable{nextID: 1}
		m.tables[name] = t
	}
	return t
}

// rowsOf 读取通用表全部行（契约测试断言用）。
func rowsOf(m *memDictStore, name string) []memRow {
	return m.table(name).rows
}

// memFieldNameByColumn 列名 → 字段 JSON 名（唯一列冲突匹配用）。
func memFieldNameByColumn(d dictcrud.Descriptor, column string) string {
	for _, f := range d.Fields {
		if f.Column == column {
			return f.Name
		}
	}
	return ""
}

// rowMatches 行值是否命中全部唯一列（upsert 冲突判定）。
func rowMatches(d dictcrud.Descriptor, row, values map[string]any) bool {
	for _, col := range d.UniqueColumns {
		name := memFieldNameByColumn(d, col)
		if row[name] != values[name] {
			return false
		}
	}
	return true
}

// newSeedMemDict 与迁移种子对齐的默认字典。
func newSeedMemDict() *memDictStore {
	return &memDictStore{
		nextRegionID: 1,
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
// 描述符驱动写面内存替身（DictWriter；契约测试走此路径）
// 区域系数保持 typed 存储（既有契约测试断言 dict.regions）；其余实体走通用内存表。
// =====================================================

func (m *memDictStore) Create(_ context.Context, d dictcrud.Descriptor, fields map[string]any) (int64, error) {
	if d.Name == "region_coefficients" {
		rc := repository.RegionCoefficient{
			ID:          m.nextRegionID,
			Province:    fields["province"].(string),
			City:        fields["city"].(string),
			Coefficient: fields["coefficient"].(float64),
		}
		m.nextRegionID++
		m.regions = append(m.regions, rc)
		return int64(rc.ID), nil
	}
	t := m.table(d.Name)
	for _, row := range t.rows {
		if rowMatches(d, row.values, fields) {
			switch d.Upsert {
			case dictcrud.UpsertDoUpdate:
				for _, name := range d.Create.Fields {
					row.values[name] = fields[name]
				}
				return row.id, nil
			default:
				// DO NOTHING 冲突无行返回（与 pgx RETURNING 行为一致 → 500）
				return 0, pgx.ErrNoRows
			}
		}
	}
	id := t.nextID
	t.nextID++
	values := make(map[string]any, len(d.Create.Fields)+1)
	for _, name := range d.Create.Fields {
		values[name] = fields[name]
	}
	t.rows = append(t.rows, memRow{id: id, values: values})
	return id, nil
}

func (m *memDictStore) Update(_ context.Context, d dictcrud.Descriptor, id int64, fields map[string]any) error {
	if d.Name == "region_coefficients" {
		for i := range m.regions {
			if m.regions[i].ID == int(id) {
				m.regions[i].Coefficient = fields["coefficient"].(float64)
				return nil
			}
		}
		return pgx.ErrNoRows
	}
	t := m.table(d.Name)
	for _, row := range t.rows {
		if row.id == id {
			for k, v := range fields {
				row.values[k] = v
			}
			return nil
		}
	}
	return pgx.ErrNoRows
}

// UpdateByKey 按唯一 key 列更新（coefficient_configs）：写回系数表并返回完整行。
func (m *memDictStore) UpdateByKey(_ context.Context, d dictcrud.Descriptor, key string, fields map[string]any) (map[string]any, error) {
	if d.Name != "coefficient_configs" {
		return nil, errors.New("未实现的字典实体: " + d.Name)
	}
	if _, ok := m.coefficients[key]; !ok {
		return nil, pgx.ErrNoRows
	}
	m.coefficients[key] = fields["value"].(float64)
	return map[string]any{
		"id":          1,
		"key":         key,
		"value":       m.coefficients[key],
		"description": "",
		"updated_at":  "2026-01-01T00:00:00Z",
	}, nil
}

func (m *memDictStore) Delete(_ context.Context, d dictcrud.Descriptor, id int64) error {
	if d.Name == "region_coefficients" {
		for i, rc := range m.regions {
			if rc.ID == int(id) {
				m.regions = append(m.regions[:i], m.regions[i+1:]...)
				return nil
			}
		}
		return pgx.ErrNoRows
	}
	t := m.table(d.Name)
	for i, row := range t.rows {
		if row.id == id {
			t.rows = append(t.rows[:i], t.rows[i+1:]...)
			return nil
		}
	}
	return pgx.ErrNoRows
}

// =====================================================
// 内存 adapter：评估存储 / 电池存储 / PDF 生成
// =====================================================

type memEvalStore struct {
	mu      sync.Mutex
	nextID  int64
	records map[int64]model.EvaluationDetail
}

func newMemEvalStore() *memEvalStore {
	return &memEvalStore{nextID: 1, records: map[int64]model.EvaluationDetail{}}
}

func (m *memEvalStore) CreateEvaluation(_ context.Context, p *repository.CreateEvaluationParams) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records), nil
}

func (m *memEvalStore) ListEvaluations(context.Context, string, int, int, int) ([]model.EvaluationDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]model.EvaluationDetail, 0, len(m.records))
	for _, d := range m.records {
		out = append(out, d)
	}
	return out, nil
}

func (m *memEvalStore) UpdateEvaluationReportPath(_ context.Context, id int64, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.records[id]
	if !ok {
		return pgx.ErrNoRows
	}
	d.ReportPdfPath = path
	m.records[id] = d
	return nil
}

// memBatteryStore 持久化电池评估内存 store（电池报告测试用：创建/加载/回写）。
type memBatteryStore struct {
	mu      sync.Mutex
	nextID  int64
	records map[int64]*model.BatteryEvaluation
}

func (m *memBatteryStore) init() {
	if m.records == nil {
		m.records = map[int64]*model.BatteryEvaluation{}
	}
}

func (m *memBatteryStore) CreateEvaluation(_ context.Context, eval *model.BatteryEvaluation, _ []model.CycleFeature, _ int) (*model.BatteryEvaluation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	m.nextID++
	cp := *eval
	cp.ID = m.nextID
	m.records[cp.ID] = &cp
	return &cp, nil
}

func (m *memBatteryStore) GetEvaluation(_ context.Context, id int64) (*model.BatteryEvaluation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	if r, ok := m.records[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, pgx.ErrNoRows
}

func (m *memBatteryStore) GetEvaluationByUser(ctx context.Context, id int64, _ int) (*model.BatteryEvaluation, error) {
	return m.GetEvaluation(ctx, id)
}

func (m *memBatteryStore) ListEvaluations(_ context.Context, _ string, _ int, _, _ int) ([]model.BatteryEvaluationSummary, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	var items []model.BatteryEvaluationSummary
	for _, r := range m.records {
		items = append(items, model.BatteryEvaluationSummary{
			ID: r.ID, BatteryType: r.BatteryType, BatteryModel: r.BatteryModel,
			CycleCount: r.CycleCount, RulCycles: r.RulCycles, SohPercent: r.SohPercent,
			Confidence: r.Confidence, CreatedAt: r.CreatedAt,
		})
	}
	return items, len(items), nil
}

func (m *memBatteryStore) UpdateReportPath(_ context.Context, id int64, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	if r, ok := m.records[id]; ok {
		r.ReportPdfPath = path
	}
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
	r, dict, evalStore, _ := newTestValuationEngineWithStorage(t, storage.NewLocalStorage(t.TempDir()))
	return r, dict, evalStore
}

// newTestValuationEngineWithStorage 允许注入自定义存储（报告测试用计数 fake）。
// 第 4 个返回值是电池 store（电池报告测试用）。
func newTestValuationEngineWithStorage(t *testing.T, st storage.Storage) (*gin.Engine, *memDictStore, *memEvalStore, *memBatteryStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dict := newSeedMemDict()
	evalStore := newMemEvalStore()
	batteryStore := &memBatteryStore{}

	valuationSvc, err := vservice.NewValuationService(dict, evalStore)
	if err != nil {
		t.Fatalf("构造估值服务失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	sess := security.SessionFromConfig(cfg)
	r := gin.New()
	r.Use(gin.Recovery())

	RegisterRoutes(r, sess, zap.NewNop(),
		dict, evalStore, batteryStore,
		valuationSvc, vservice.NewBatteryRULService(),
		&memReportGenerator{}, st,
		nil, // ValuationAuthService 未装配：本 seam 覆盖的公开评估路径不触达 /auth/*
	)
	return r, dict, evalStore, batteryStore
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

// adminAuthHeader 签发 role=admin 的 Bearer token（管理员字典 CRUD 路由用）。
func adminAuthHeader(t *testing.T) string {
	t.Helper()
	sess := security.NewSession("test-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	token, err := sess.Issue(1, "admin", "admin")
	if err != nil {
		t.Fatalf("签发管理员测试 token 失败: %v", err)
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
	// 创建响应必须包含输入参数（匿名提交流程不再依赖需登录的详情接口）
	if data["brand"] != "合力" || data["sale_year"] == nil || data["factory_year"] == nil {
		t.Errorf("创建响应缺少输入参数（brand/sale_year/factory_year）: %v", data["brand"])
	}
	if data["condition_rating"] == nil || data["province"] == nil || data["city"] == nil {
		t.Errorf("创建响应缺少输入参数（condition_rating/province/city）: %v", data)
	}
}

// TestBatteryCreate_Anonymous 匿名提交电池评估应成功（历史/详情才需登录）。
func TestBatteryCreate_Anonymous(t *testing.T) {
	r, _, _, _ := newTestValuationEngineWithStorage(t, storage.NewLocalStorage(t.TempDir()))

	w := performRequest(r, http.MethodPost, "/api/valuation/battery/evaluations", batteryCreateRequest())
	if w.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("业务码 = %d, 期望 200\nbody=%s", code, w.Body.String())
	}
	if data["evaluation_id"] == nil || data["rul_cycles"] == nil || data["soh_percent"] == nil {
		t.Errorf("电池创建响应缺少 evaluation_id/rul_cycles/soh_percent: %v", data)
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
