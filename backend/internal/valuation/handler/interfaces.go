// Package handler 实现 HTTP 处理器
// 本文件：handler 消费的存储/生成窄接口——生产为 pgx 仓储与 PDF 生成器，测试为内存替身。
// 测试穿过与生产调用方相同的 seam（"interface is the test surface"）。
package handler

import (
	"context"

	mainmodel "forklift-training/internal/model"
	"forklift-training/internal/valuation/model"
)

// EvaluationStore 评估记录存储接口（EvaluationHandler / ReportHandler 消费）。
type EvaluationStore interface {
	GetEvaluation(ctx context.Context, id int64) (*model.EvaluationDetail, error)
	GetEvaluationByUser(ctx context.Context, id int64, userID int) (*model.EvaluationDetail, error)
	CountEvaluations(ctx context.Context, brand string, userID int) (int, error)
	ListEvaluations(ctx context.Context, brand string, userID int, limit, offset int) ([]model.EvaluationDetail, error)
	UpdateEvaluationReportPath(ctx context.Context, id int64, path string) error
}

// BatteryStore 电池 RUL 评估存储接口（BatteryHandler 消费）。
type BatteryStore interface {
	CreateEvaluation(ctx context.Context, eval *model.BatteryEvaluation, features []model.CycleFeature, userID int) (*model.BatteryEvaluation, error)
	GetEvaluation(ctx context.Context, id int64) (*model.BatteryEvaluation, error)
	GetEvaluationByUser(ctx context.Context, id int64, userID int) (*model.BatteryEvaluation, error)
	ListEvaluations(ctx context.Context, batteryType string, userID int, limit, offset int) ([]model.BatteryEvaluationSummary, int, error)
	UpdateReportPath(ctx context.Context, id int64, path string) error
}

// ReportGenerator PDF 报告生成接口（ReportHandler 消费；生产为 pdf.Generator，测试为内存替身）。
// dimensionScores 为 typed 维度评分切片（标签契约见 model.DimensionLabels）。
type ReportGenerator interface {
	GenerateReport(r *model.EvaluationDetail, dimensionScores []model.DimensionScore, suggestions []string) ([]byte, error)
}

// ValuationAuth 估值模块消费的认证窄接口（主体系 AuthService 直接满足，
// 取代旧薄包装的 Main()/DB() 泄漏，见 spec #75 D4）。
// token 提取/吊销经会话模块处理（Logout 直接使用注入的 Session 实例）。
type ValuationAuth interface {
	GetHrwaiUserByID(id int) (*mainmodel.HrwaiUser, error)
}
