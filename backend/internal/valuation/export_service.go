// 本文件：管理端数据导出取数（CSV 由 HTTP 出口 internal/valuation/handler/export.go 生成，
// export* 文件只负责取数）：列 spec 在 export_columns.go、取数 seam 在 export_store.go。
//
// 原为估值域内叶子子包 internal/valuation/export（波 4f 建；理由是 repository 的导出 adapter 要看见
// EvaluationExportRow / BuildEvalExportSelect / ScanEvalExportDestinations，与 internal/valuation/service
// 同包必成环）；#1514 波 3 起并回域包，环随之消失，ExportStore 作为**消费方定义的 seam** 保留
// （测试替身仍经它注入）。
package valuation

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/timefmt"
)

// ExportService 数据导出服务（学员名单 / 题库 / 评估记录三条取数）。
type ExportService struct {
	db      *gorm.DB
	exports ExportStore

	logger *zap.Logger
}

// NewExportService 构造导出服务（exports 经 ExportStore seam 注入，生产为估值侧 repository adapter）。
func NewExportService(db *gorm.DB, exports ExportStore, logger *zap.Logger) *ExportService {
	return &ExportService{db: db, exports: exports, logger: logger}
}

// Students 学员名单导出行（首行为表头）。
func (s *ExportService) Students() ([][]any, error) {
	var rows []struct {
		ID        int
		Account   string
		Username  string
		Phone     string
		Email     string
		Company   string
		Status    int16
		CreatedAt time.Time
	}
	if err := s.db.Table("hrwai_users").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := [][]any{{"ID", "账号", "昵称", "手机号", "邮箱", "公司", "状态", "注册时间"}}
	for _, r := range rows {
		status := "禁用"
		if r.Status == 1 {
			status = "启用"
		}
		out = append(out, []any{
			r.ID, r.Account, r.Username, r.Phone, r.Email, r.Company, status, timefmt.FormatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// Questions 题库导出行。
func (s *ExportService) Questions() ([][]any, error) {
	var rows []struct {
		ID          int
		Type        string
		Content     string
		Options     []byte
		Answer      string
		Explanation string
		Status      string
		CreatedAt   time.Time
	}
	err := s.db.Table("question AS q").
		Select("q.id, q.type, q.content, q.options, q.answer, q.explanation, " +
			"q.status, q.created_at").
		Order("q.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := [][]any{{"ID", "类型", "题干", "选项", "答案", "解析", "状态", "创建时间"}}
	for _, r := range rows {
		options := ""
		if len(r.Options) > 0 {
			options = string(r.Options)
		}
		out = append(out, []any{
			r.ID, r.Type, r.Content, options, r.Answer, r.Explanation,
			r.Status, timefmt.FormatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// Evaluations 残值评估记录导出行（数据经 ExportStore seam 取自估值侧 repository，见 spec #75 D4）。
// 表头与取值均从 EvaluationExportColumns 单点 spec 派生（#229），与既有输出逐字一致。
func (s *ExportService) Evaluations() ([][]any, error) {
	rows, err := s.exports.ListEvaluationExports(context.Background())
	if err != nil {
		return nil, err
	}
	hdr := make([]any, 0, len(EvaluationExportColumns))
	for _, col := range EvaluationExportColumns {
		hdr = append(hdr, col.Header)
	}
	out := [][]any{hdr}
	for _, r := range rows {
		cell := make([]any, 0, len(EvaluationExportColumns))
		for _, col := range EvaluationExportColumns {
			cell = append(cell, col.Value(r))
		}
		out = append(out, cell)
	}
	return out, nil
}

// yesNo 布尔值转中文。
func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

// coeff 系数指针格式化（保留 4 位小数）。
func coeff(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.4f", *v)
}

// nullableFloat 浮点指针格式化（保留 2 位小数）。
func nullableFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}
