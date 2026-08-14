// Package api 实现 HTTP handlers。
// 本文件：管理端数据导出（xlsx）。
package api

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// ExportHandler 管理端数据导出 handler。
type ExportHandler struct {
	svc *service.ExportService
}

// NewExportHandler 创建管理端数据导出 handler。
func NewExportHandler(svc *service.ExportService) *ExportHandler {
	return &ExportHandler{svc: svc}
}

// RegisterExportRoutes 注册 /api/admin/export 蓝图（仅管理员，返回 CSV 附件）。
func RegisterExportRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ExportService) {
	h := NewExportHandler(svc)

	g := rg.Group("/admin/export", middleware.JWTAuth(rd.Session), middleware.RoleRequired("admin"))

	g.GET("/students", h.exportCSV(func() ([][]any, error) { return svc.Students() }, "学员名单.csv"))
	g.GET("/exam-records", h.exportCSV(func() ([][]any, error) { return svc.ExamRecords() }, "成绩单.csv"))
	g.GET("/questions", h.exportCSV(func() ([][]any, error) { return svc.Questions() }, "题库.csv"))
	g.GET("/evaluations", h.exportCSV(func() ([][]any, error) { return svc.Evaluations() }, "评估记录.csv"))
}

// exportCSV 将取数结果生成为 CSV 附件响应（带 UTF-8 BOM，Excel 可直接打开不乱码）。
func (h *ExportHandler) exportCSV(fetch func() ([][]any, error), filename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := fetch()
		if err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}

		var buf bytes.Buffer
		buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
		w := csv.NewWriter(&buf)
		for _, row := range rows {
			rec := make([]string, len(row))
			for i, v := range row {
				rec[i] = cellString(v)
			}
			if err := w.Write(rec); err != nil {
				response.ServerError(c, "导出失败: "+err.Error())
				return
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}

		encoded := url.PathEscape(filename)
		c.Header("Content-Disposition", `attachment; filename="export.csv"; filename*=UTF-8''`+encoded)
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
	}
}

// cellString 将单元格值转为 CSV 字符串。
func cellString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}
