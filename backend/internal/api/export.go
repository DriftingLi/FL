// Package api 实现 HTTP handlers。
// 本文件：管理端数据导出（xlsx）。
package api

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterExportRoutes 注册 /api/admin/export 蓝图（仅管理员，返回 xlsx 附件）。
func RegisterExportRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	svc := service.NewExportService(db)
	g := rg.Group("/admin/export", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	g.GET("/students", exportXLSXHandler(func() ([][]any, error) { return svc.Students() }, "学员名单.xlsx"))
	g.GET("/exam-records", exportXLSXHandler(func() ([][]any, error) { return svc.ExamRecords() }, "成绩单.xlsx"))
	g.GET("/questions", exportXLSXHandler(func() ([][]any, error) { return svc.Questions() }, "题库.xlsx"))
	g.GET("/evaluations", exportXLSXHandler(func() ([][]any, error) { return svc.Evaluations() }, "评估记录.xlsx"))
}

// exportXLSXHandler 将取数结果生成为 xlsx 附件响应。
func exportXLSXHandler(fetch func() ([][]any, error), filename string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := fetch()
		if err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}
		f := excelize.NewFile()
		defer f.Close()
		sheet := f.GetSheetName(0)
		for i, row := range rows {
			cell, err := excelize.CoordinatesToCellName(1, i+1)
			if err != nil {
				response.ServerError(c, "导出失败: "+err.Error())
				return
			}
			rowCopy := append([]any(nil), row...)
			if err := f.SetSheetRow(sheet, cell, &rowCopy); err != nil {
				response.ServerError(c, "导出失败: "+err.Error())
				return
			}
		}
		buf, err := f.WriteToBuffer()
		if err != nil {
			response.ServerError(c, "导出失败: "+err.Error())
			return
		}
		encoded := url.PathEscape(filename)
		c.Header("Content-Disposition", `attachment; filename="export.xlsx"; filename*=UTF-8''`+encoded)
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
	}
}
