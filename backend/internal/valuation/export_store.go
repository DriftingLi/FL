package valuation

import (
	"context"
	"time"
)

// EvaluationExportRow 评估记录导出行（本包 ExportStore 契约的数据形状：估值侧 repository 的
// pgx adapter 按此形状取数，列序真值见 columns.go）。
type EvaluationExportRow struct {
	ID                    int64
	Account               string
	Username              string
	Brand                 string
	VehicleType           string
	Series                string
	Tonnage               float64
	ConfigType            string
	MastType              string
	MastHeightMM          int
	FactoryYear           int
	SaleYear              int
	UsageHours            int
	OriginalPaint         bool
	Province              string
	City                  string
	HasLicensePlate       bool
	HasRegistrationCert   bool
	HasMaintenanceRecords bool
	ConditionRating       string
	OriginalPrice         float64
	KTime                 *float64
	KHours                *float64
	KBrand                *float64
	KCondition            *float64
	KMarket               *float64
	EstimatedValue        float64
	ConfidenceLow         *float64
	ConfidenceHigh        *float64
	ReportPDFPath         string
	CreatedAt             time.Time
}

// ExportStore 是估值数据访问的消费接口（seam 定义在消费方——本包；生产实现为估值侧
// internal/valuation/repository 的 pgx adapter，测试用 fake adapter）。
type ExportStore interface {
	ListEvaluationExports(ctx context.Context) ([]EvaluationExportRow, error)
}
