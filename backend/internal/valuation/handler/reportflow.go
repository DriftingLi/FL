// Package handler 实现 HTTP 处理器
// 本文件：报告端点翻译壳（评估与电池报告共用）——id 解析 + 协调器纯值结果 → HTTP 响应。
// 协调器本体在 internal/valuation/report（gin-free），此处只做薄翻译。
package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/report"
	"forklift-training/pkg/response"
)

// serveReportGenerate 处理 POST <prefix>/:id/report（评估与电池报告端点共用翻译）。
func serveReportGenerate[T any](c *gin.Context, coord *report.Coordinator[T], notFoundMsg string, logger *zap.Logger) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	res, err := coord.Generate(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, notFoundMsg)
			return
		}
		logger.Error("生成报告失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(c, "生成报告失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{
		"evaluation_id": res.ID,
		"pdf_url":       res.PDFURL,
		"file_size":     res.FileSize,
	})
}

// serveReportDownload 处理 GET <prefix>/:id/report：URL 有效则 302；否则并发安全地再生成后 302。
func serveReportDownload[T any](c *gin.Context, coord *report.Coordinator[T], notFoundMsg string, logger *zap.Logger) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	pdfURL, err := coord.DownloadURL(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, notFoundMsg)
			return
		}
		logger.Error("查询记录失败", zap.Error(err), zap.Int64("id", id))
		response.ServerError(c, "查询记录失败")
		return
	}

	// 302 重定向到公开访问 URL（浏览器直连下载）
	c.Redirect(http.StatusFound, pdfURL)
}
