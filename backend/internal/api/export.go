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

	"forklift-training/internal/authz"
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
// 文件名为后端唯一真值（随 Content-Disposition 下发，前端优先读取、拿不到再回退，#230）。
func RegisterExportRoutes(rg *gin.RouterGroup, rd RouterDeps, svc *service.ExportService) {
	h := NewExportHandler(svc)

	g := rg.Group("/admin/export", middleware.JWTAuth(rd.Session), middleware.CapabilityRequired(authz.CapExportRun))

	// 注解携带在具名包装方法上：swag 只能从函数声明的注释块取注解，匿名闭包无法被登记（片七）。
	g.GET("/students", h.ExportStudents)
	g.GET("/questions", h.ExportQuestions)
	g.GET("/evaluations", h.ExportEvaluations)
}

// ExportStudents 导出学员名单 GET /api/admin/export/students
// @Summary 导出学员名单（CSV）
// @Description 响应是 CSV 附件（非统一信封）：文件名随 Content-Disposition 下发（#230），前端按 blob 消费
// @Tags 管理端-导出
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {string} string "CSV 附件（UTF-8 BOM）"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "导出失败"
// @Router /admin/export/students [get]
func (h *ExportHandler) ExportStudents(c *gin.Context) {
	h.exportCSV(func() ([][]any, error) { return h.svc.Students() }, "学员名单.csv")(c)
}

// ExportQuestions 导出题库 GET /api/admin/export/questions
// @Summary 导出题库（CSV）
// @Description 响应是 CSV 附件（非统一信封）：文件名随 Content-Disposition 下发（#230），前端按 blob 消费
// @Tags 管理端-导出
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {string} string "CSV 附件（UTF-8 BOM）"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "导出失败"
// @Router /admin/export/questions [get]
func (h *ExportHandler) ExportQuestions(c *gin.Context) {
	h.exportCSV(func() ([][]any, error) { return h.svc.Questions() }, "题库.csv")(c)
}

// ExportEvaluations 导出评估记录 GET /api/admin/export/evaluations
// @Summary 导出评估记录（CSV）
// @Description 响应是 CSV 附件（非统一信封）：文件名随 Content-Disposition 下发（#230），前端按 blob 消费
// @Tags 管理端-导出
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {string} string "CSV 附件（UTF-8 BOM）"
// @Failure 401 {object} response.R "未认证"
// @Failure 500 {object} response.R "导出失败"
// @Router /admin/export/evaluations [get]
func (h *ExportHandler) ExportEvaluations(c *gin.Context) {
	h.exportCSV(func() ([][]any, error) { return h.svc.Evaluations() }, "评估记录.csv")(c)
}

// exportCSV 将取数结果生成为 CSV 附件响应（带 UTF-8 BOM，Excel 可直接打开不乱码）。
func (h *ExportHandler) exportCSV(fetch func() ([][]any, error), filename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := fetch()
		if err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}
		buf, err := encodeCSV(rows)
		if err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}
		c.Header("Content-Disposition", contentDisposition(filename))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Data(http.StatusOK, "text/csv; charset=utf-8", buf)
	}
}

// encodeCSV 将取数行序列化为带 UTF-8 BOM 的 CSV 字节。
// 逗号/引号由 encoding/csv 转义，单元格值经 cellString 归一化（#230 独立可测函数）。
func encodeCSV(rows [][]any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	w := csv.NewWriter(&buf)
	for _, row := range rows {
		rec := make([]string, len(row))
		for i, v := range row {
			rec[i] = cellString(v)
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// contentDisposition 生成 RFC 5987 附件响应头（ASCII 兜底名 + UTF-8 文件名）。
func contentDisposition(filename string) string {
	encoded := url.PathEscape(filename)
	return "attachment; filename=\"export.csv\"; filename*=UTF-8''" + encoded
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
