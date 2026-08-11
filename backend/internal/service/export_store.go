package service

import (
	"context"
	"time"
)

// EvaluationExportRow 评估记录导出行（ExportStore 契约的数据形状，由主 app 导出模块定义）。
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

// ExportStore 是主 app 导出模块对估值数据访问的消费接口（seam 定义在消费方，
// 估值侧 repository 提供 pgx adapter 实现，测试用 fake adapter）。
type ExportStore interface {
	ListEvaluationExports(ctx context.Context) ([]EvaluationExportRow, error)
}
