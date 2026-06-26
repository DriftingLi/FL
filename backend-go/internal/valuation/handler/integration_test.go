// Package handler 实现 HTTP 处理器
// 本文件：集成测试，使用 httptest 模拟 HTTP 请求
// 测试策略：依赖真实 PostgreSQL（已在测试环境准备 seed 数据）
// 每个测试结束后清理新建的评估记录与历史成交数据
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/valuation/repository"
	"forklift-training/internal/valuation/service"
)

// testEnv 集成测试环境
type testEnv struct {
	router    *gin.Engine
	queries   *repository.Queries
	pool      *pgxpool.Pool
	brand     *service.BrandLoader
	parts     *service.PartConfigLoader
	coef      *service.CoefficientLoader
}

// setupTestEnv 构造测试环境
// 跳过条件：DB 不可达（CI 环境无 DB 时可跳过）
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// 使用与生产一致的 DSN（也可通过 TEST_DATABASE_DSN 覆盖）
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgresql://luohao:123456@localhost:5432/forklift_valuation?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := config.NewPostgresPool(ctx, dsn, 5, 1, 60)
	if err != nil {
		t.Skipf("跳过集成测试：DB 不可用: %v", err)
	}

	queries := repository.New(pool)
	coef := service.NewCoefficientLoader(queries)
	brand := service.NewBrandLoader(queries)
	parts := service.NewPartConfigLoader(queries)
	if err := coef.LoadAll(ctx); err != nil {
		t.Fatalf("加载系数失败: %v", err)
	}
	if err := brand.LoadAll(ctx); err != nil {
		t.Fatalf("加载品牌失败: %v", err)
	}
	if err := parts.LoadAll(ctx); err != nil {
		t.Fatalf("加载部件配置失败: %v", err)
	}

	valuation := service.NewValuationService(coef, brand, parts)
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS([]string{"*"}))

	api := router.Group("/api/v1")
	{
		api.GET("/health", NewHealthHandler().Check)

		evalHandler := NewEvaluationHandler(queries, valuation, logger)
		api.POST("/evaluations", evalHandler.Create)
		api.GET("/evaluations", evalHandler.List)
		api.GET("/evaluations/:id", evalHandler.Get)

		configHandler := NewConfigHandler(queries, brand, logger)
		api.GET("/part-configs", configHandler.ListPartConfigs)
		api.GET("/brands", configHandler.ListBrands)
		api.GET("/coefficients", configHandler.ListCoefficients)
		api.PUT("/coefficients/:key", configHandler.UpdateCoefficient)

		histHandler := NewHistoricalHandler(queries, logger)
		api.POST("/historical-sales/import", histHandler.Import)
	}

	return &testEnv{
		router:  router,
		queries: queries,
		pool:    pool,
		brand:   brand,
		parts:   parts,
		coef:    coef,
	}
}

// doJSON 发送 JSON 请求并解析响应
func (e *testEnv) doJSON(t *testing.T, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w
}

// doMultipart 发送 multipart 文件上传请求
func (e *testEnv) doMultipart(t *testing.T, path, fieldName, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("创建表单文件失败: %v", err)
	}
	if _, err := io.WriteString(fw, content); err != nil {
		t.Fatalf("写入文件内容失败: %v", err)
	}
	w.Close()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// decodeResponse 解析统一响应
func decodeResponse(t *testing.T, body io.Reader) Response {
	t.Helper()
	var resp Response
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("解码响应失败: %v", err)
	}
	return resp
}

// TestHealth 验证健康检查
func TestHealth(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodGet, "/api/v1/health", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	resp := decodeResponse(t, w.Body)
	if resp.Code != CodeOK {
		t.Errorf("code=%d, want 0", resp.Code)
	}
}

// TestListBrands 验证品牌列表
func TestListBrands(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodGet, "/api/v1/brands", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	resp := decodeResponse(t, w.Body)
	brands, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("data 不是数组: %T", resp.Data)
	}
	if len(brands) == 0 {
		t.Error("brands 列表为空")
	}
}

// TestListPartConfigs 验证部件配置查询
func TestListPartConfigs(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	// 电动 - 68 条
	w := env.doJSON(t, http.MethodGet, "/api/v1/part-configs?forklift_type=electric", nil)
	resp := decodeResponse(t, w.Body)
	items := resp.Data.([]interface{})
	if len(items) != 68 {
		t.Errorf("电动部件配置数量=%d, want 68", len(items))
	}

	// 内燃 - 85 条
	w = env.doJSON(t, http.MethodGet, "/api/v1/part-configs?forklift_type=combustion", nil)
	resp = decodeResponse(t, w.Body)
	items = resp.Data.([]interface{})
	if len(items) != 85 {
		t.Errorf("内燃部件配置数量=%d, want 85", len(items))
	}

	// 缺少参数 - 400
	w = env.doJSON(t, http.MethodGet, "/api/v1/part-configs", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}

	// 非法类型 - 400
	w = env.doJSON(t, http.MethodGet, "/api/v1/part-configs?forklift_type=hybrid", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

// TestListCoefficients 验证系数列表
func TestListCoefficients(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodGet, "/api/v1/coefficients", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	resp := decodeResponse(t, w.Body)
	items := resp.Data.([]interface{})
	if len(items) == 0 {
		t.Error("coefficients 列表为空")
	}
}

// TestCreateEvaluation 验证创建评估（电动 5 年全正常）
func TestCreateEvaluation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	req := buildFullElectricRequest("丰田", 10.0, 2020, 2025, 8000, "仓储",
		"normal", "normal", "normal", "normal")

	w := env.doJSON(t, http.MethodPost, "/api/v1/evaluations", req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body)
	if resp.Code != CodeOK {
		t.Fatalf("code=%d, want 0; message=%s", resp.Code, resp.Message)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是 map: %T", resp.Data)
	}
	if dataMap["id"] == nil {
		t.Error("返回数据中未包含 id")
	}
	if v, ok := dataMap["estimated_value"].(float64); !ok || v <= 0 {
		t.Errorf("estimated_value=%v, want > 0", dataMap["estimated_value"])
	}
}

// TestCreateEvaluationInvalidBrand 验证品牌未找到时报错
func TestCreateEvaluationInvalidBrand(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	req := buildFullElectricRequest("不存在的品牌", 10.0, 2020, 2025, 8000, "仓储",
		"normal", "normal", "normal", "normal")
	// 替换所有 68 个条目为 normal
	req["items"] = allItems("normal")

	w := env.doJSON(t, http.MethodPost, "/api/v1/evaluations", req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400", w.Code)
	}
}

// TestGetEvaluation 验证查询评估详情
func TestGetEvaluation(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	// 先创建一条
	req := buildFullElectricRequest("丰田", 10.0, 2020, 2025, 8000, "仓储",
		"normal", "normal", "normal", "normal")
	w := env.doJSON(t, http.MethodPost, "/api/v1/evaluations", req)
	createResp := decodeResponse(t, w.Body)
	dataMap := createResp.Data.(map[string]interface{})
	id := int64(dataMap["id"].(float64))
	defer cleanupEvaluation(t, env, id)

	// 查询详情
	w = env.doJSON(t, http.MethodGet, fmt.Sprintf("/api/v1/evaluations/%d", id), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	resp := decodeResponse(t, w.Body)
	got, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是 map: %T", resp.Data)
	}
	items, ok := got["items"].([]interface{})
	if !ok {
		t.Fatalf("items 不是数组: %T", got["items"])
	}
	if len(items) != 68 {
		t.Errorf("item 数量=%d, want 68", len(items))
	}
}

// TestGetEvaluationNotFound 验证 404
func TestGetEvaluationNotFound(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodGet, "/api/v1/evaluations/999999999", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
}

// TestListEvaluations 验证分页查询
func TestListEvaluations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodGet, "/api/v1/evaluations?page=1&page_size=5", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	resp := decodeResponse(t, w.Body)
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是 map: %T", resp.Data)
	}
	if data["total"] == nil {
		t.Error("缺少 total 字段")
	}
	if data["list"] == nil {
		t.Error("缺少 list 字段")
	}
}

// TestUpdateCoefficient 验证系数更新
func TestUpdateCoefficient(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	// 备份原值
	original, err := env.queries.GetCoefficientByKey(context.Background(), "k_market")
	if err != nil {
		t.Fatalf("查询原值失败: %v", err)
	}
	defer func() {
		_, _ = env.queries.UpdateCoefficientValue(context.Background(), repository.UpdateCoefficientValueParams{
			Key: "k_market", Value: original.Value,
		})
	}()

	// 更新为 0.95
	w := env.doJSON(t, http.MethodPut, "/api/v1/coefficients/k_market", map[string]interface{}{"value": 0.95})
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}

	// 验证已更新
	updated, err := env.queries.GetCoefficientByKey(context.Background(), "k_market")
	if err != nil {
		t.Fatalf("查询更新后值失败: %v", err)
	}
	if updated.Value != 0.95 {
		t.Errorf("更新后值=%f, want 0.95", updated.Value)
	}
}

// TestUpdateCoefficientNotFound 验证 key 不存在
func TestUpdateCoefficientNotFound(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	w := env.doJSON(t, http.MethodPut, "/api/v1/coefficients/nonexistent_key", map[string]interface{}{"value": 0.5})
	if w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
}

// TestImportHistoricalSales 验证 CSV 导入
func TestImportHistoricalSales(t *testing.T) {
	env := setupTestEnv(t)
	defer env.pool.Close()

	csv := "forklift_type,brand,model,original_price,purchase_year,sale_year,usage_hours,work_condition,fuel_type,sale_price\n" +
		"electric,丰田,TEST-MODEL,10,2020,2023,5000,仓储,,5.0\n" +
		"combustion,合力,TEST-COMB,15,2021,2024,4000,港口,柴油,8.0\n"

	w := env.doMultipart(t, "/api/v1/historical-sales/import", "file", "test.csv", csv)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body)
	data, _ := resp.Data.(map[string]interface{})
	if success, _ := data["success"].(float64); int(success) != 2 {
		t.Errorf("success=%v, want 2", data["success"])
	}

	// 清理测试数据
	defer cleanupHistoricalSales(t, env, "TEST-MODEL")
	defer cleanupHistoricalSales(t, env, "TEST-COMB")
}

// allItems 生成 68 个全正常（或指定状态）的 items
func allItems(status string) []map[string]string {
	codes := []string{
		"left_drive_motor", "right_drive_motor", "lift_motor", "steer_motor", "motor_other",
		"hydraulic_pump", "hydraulic_tank", "multiway_valve", "lever_mechanism", "hydraulic_hose", "hydraulic_other",
		"overhead_guard", "seat", "cover_shell", "counterweight", "nameplate", "shock_mount", "body_frame", "cab", "body_other",
		"lift_cylinder", "tilt_cylinder", "mast", "fork_carriage", "load_backrest", "roller", "chain", "mast_hose", "attachment", "mast_other",
		"charge_cable", "battery_cell", "battery_case", "battery_other",
		"left_gearbox", "right_gearbox", "drive_coupling", "rigid_flex_hose", "transmission_other",
		"steer_valve", "steer_cylinder", "steer_axle", "steer_hose", "tire_wheel", "rim", "steer_wheel_other",
		"combo_gauge", "drive_controller", "hydraulic_controller", "steer_controller", "drive_module", "hydraulic_module", "steer_module", "accel_sensor", "electric_1_other",
		"speed_sensor", "angle_sensor", "other_sensor", "contactor", "cooling_fan", "light", "warning_light", "key_switch", "cable_wiring", "electric_2_other",
		"internal_element", "charger_cable", "charger_other",
	}
	out := make([]map[string]string, 0, len(codes))
	for _, c := range codes {
		out = append(out, map[string]string{"item_code": c, "status": status})
	}
	return out
}

// buildFullElectricRequest 构造电动叉车完整评估请求
// 可指定部分关键条目的状态（其他全为 normal）
func buildFullElectricRequest(
	brand string, originalPrice float64,
	purchaseYear, saleYear, usageHours int,
	workCondition, liftMotor, coverShell, battery, tire string,
) map[string]interface{} {
	items := allItems("normal")
	override := func(code, status string) {
		for i, it := range items {
			if it["item_code"] == code {
				items[i]["status"] = status
			}
		}
	}
	override("lift_motor", liftMotor)
	override("cover_shell", coverShell)
	override("battery_cell", battery)
	override("tire_wheel", tire)

	return map[string]interface{}{
		"forklift_type":  "electric",
		"brand":          brand,
		"model":          "TEST",
		"original_price": originalPrice,
		"purchase_year":  purchaseYear,
		"sale_year":      saleYear,
		"usage_hours":    usageHours,
		"work_condition": workCondition,
		"can_drive":      true,
		"hydraulic_ok":   true,
		"items":          items,
	}
}

// cleanupEvaluation 清理测试产生的评估记录
func cleanupEvaluation(t *testing.T, env *testEnv, id int64) {
	t.Helper()
	_, err := env.pool.Exec(context.Background(),
		"DELETE FROM evaluation_items WHERE evaluation_id = $1", id)
	if err != nil {
		t.Logf("清理 evaluation_items 失败: %v", err)
	}
	_, err = env.pool.Exec(context.Background(),
		"DELETE FROM evaluations WHERE id = $1", id)
	if err != nil {
		t.Logf("清理 evaluations 失败: %v", err)
	}
}

// cleanupHistoricalSales 按 model 清理测试导入的历史成交数据
func cleanupHistoricalSales(t *testing.T, env *testEnv, model string) {
	t.Helper()
	_, err := env.pool.Exec(context.Background(),
		"DELETE FROM historical_sales WHERE model = $1", model)
	if err != nil {
		t.Logf("清理 historical_sales 失败: %v", err)
	}
}

// 确保测试期间路径存在
var _ = filepath.Join
