// Package handler 实现 HTTP 处理器
// 本文件：历史成交数据导入接口（CSV 文件解析）
package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// HistoricalHandler 历史成交 HTTP 处理器
type HistoricalHandler struct {
	queries *repository.Queries
	logger  *zap.Logger
}

// NewHistoricalHandler 构造历史成交处理器
func NewHistoricalHandler(q *repository.Queries, l *zap.Logger) *HistoricalHandler {
	return &HistoricalHandler{queries: q, logger: l}
}

// Import 处理 POST /api/v1/historical-sales/import
// 支持 multipart/form-data 上传 CSV 文件
// 也支持直接 POST application/json 批量导入
//
// CSV 格式（首行为表头）：
//   forklift_type,brand,model,original_price,purchase_year,sale_year,
//   usage_hours,work_condition,fuel_type,sale_price
func (h *HistoricalHandler) Import(c *gin.Context) {
	// 1. 读取请求体：优先用文件上传，备选 JSON 数组
	var records [][]string
	var err error

	contentType := c.GetHeader("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		records, err = readCSVFromUpload(c)
		if err != nil {
			Error(c, http.StatusBadRequest, CodeBadRequest, "解析上传 CSV 失败: "+err.Error())
			return
		}
	} else {
		records, err = readCSVFromBody(c)
		if err != nil {
			Error(c, http.StatusBadRequest, CodeBadRequest, "解析请求体失败: "+err.Error())
			return
		}
	}

	// 2. 校验表头
	if len(records) < 2 {
		Error(c, http.StatusBadRequest, CodeBadRequest, "CSV 数据不能为空（至少含表头+1行数据）")
		return
	}
	header := records[0]
	expectedHeader := []string{
		"forklift_type", "brand", "model", "original_price", "purchase_year",
		"sale_year", "usage_hours", "work_condition", "fuel_type", "sale_price",
	}
	if !sameHeader(header, expectedHeader) {
		Error(c, http.StatusBadRequest, CodeBadRequest,
			"表头格式错误，应为: "+strings.Join(expectedHeader, ","))
		return
	}

	// 3. 逐行解析并入库
	success := 0
	failed := 0
	errors := []string{}
	for i, row := range records[1:] {
		if len(row) != len(expectedHeader) {
			failed++
			errors = append(errors, fmt.Sprintf("第 %d 行字段数不匹配（期望 %d，实际 %d）", i+2, len(expectedHeader), len(row)))
			continue
		}

		params, parseErr := parseRow(row)
		if parseErr != nil {
			failed++
			errors = append(errors, fmt.Sprintf("第 %d 行解析失败: %s", i+2, parseErr.Error()))
			continue
		}

		_, dbErr := h.queries.CreateHistoricalSale(c.Request.Context(), params)
		if dbErr != nil {
			failed++
			errors = append(errors, fmt.Sprintf("第 %d 行入库失败: %s", i+2, dbErr.Error()))
			h.logger.Error("导入历史成交记录失败", zap.Error(dbErr), zap.Int("row", i+2))
			continue
		}
		success++
	}

	OK(c, gin.H{
		"total":   len(records) - 1,
		"success": success,
		"failed":  failed,
		"errors":  errors,
	})
}

// readCSVFromUpload 解析 multipart 上传的 CSV 文件
func readCSVFromUpload(c *gin.Context) ([][]string, error) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("未找到上传文件: %w", err)
	}
	defer file.Close()
	return readCSV(file)
}

// readCSVFromBody 直接读取请求体作为 CSV
func readCSVFromBody(c *gin.Context) ([][]string, error) {
	return readCSV(c.Request.Body)
}

// readCSV 通用 CSV 读取
func readCSV(r io.Reader) ([][]string, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // 允许变长行，由调用方校验
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}

// sameHeader 比对表头（忽略大小写与空白）
func sameHeader(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if strings.TrimSpace(strings.ToLower(actual[i])) != expected[i] {
			return false
		}
	}
	return true
}

// parseRow 解析单行 CSV → CreateHistoricalSaleParams
func parseRow(row []string) (repository.CreateHistoricalSaleParams, error) {
	originalPrice, err := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
	if err != nil {
		return repository.CreateHistoricalSaleParams{}, fmt.Errorf("original_price 非法: %s", err.Error())
	}
	purchaseYear, err := strconv.Atoi(strings.TrimSpace(row[4]))
	if err != nil {
		return repository.CreateHistoricalSaleParams{}, fmt.Errorf("purchase_year 非法: %s", err.Error())
	}
	saleYear, err := strconv.Atoi(strings.TrimSpace(row[5]))
	if err != nil {
		return repository.CreateHistoricalSaleParams{}, fmt.Errorf("sale_year 非法: %s", err.Error())
	}
	usageHours, err := strconv.Atoi(strings.TrimSpace(row[6]))
	if err != nil {
		return repository.CreateHistoricalSaleParams{}, fmt.Errorf("usage_hours 非法: %s", err.Error())
	}
	salePrice, err := strconv.ParseFloat(strings.TrimSpace(row[9]), 64)
	if err != nil {
		return repository.CreateHistoricalSaleParams{}, fmt.Errorf("sale_price 非法: %s", err.Error())
	}

	// 叉车类型与工况校验
	ft := model.ForkliftType(strings.TrimSpace(row[0]))
	if !ft.IsValid() {
		return repository.CreateHistoricalSaleParams{}, errors.New("forklift_type 非法")
	}
	wc := model.WorkCondition(strings.TrimSpace(row[7]))
	if !wc.IsValid() {
		return repository.CreateHistoricalSaleParams{}, errors.New("work_condition 非法")
	}

	fuel := strings.TrimSpace(row[8])
	fuelText := pgtypeText(fuel)

	return repository.CreateHistoricalSaleParams{
		ForkliftType:  string(ft),
		Brand:         strings.TrimSpace(row[1]),
		Model:         pgtypeText(row[2]),
		OriginalPrice: originalPrice,
		PurchaseYear:  int32(purchaseYear),
		SaleYear:      int32(saleYear),
		UsageHours:    int32(usageHours),
		WorkCondition: string(wc),
		FuelType:      fuelText,
		SalePrice:     salePrice,
	}, nil
}

// pgtypeText 将空字符串转为无效 pgtype.Text，否则为有效值
func pgtypeText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}
