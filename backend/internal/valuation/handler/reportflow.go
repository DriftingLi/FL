// Package handler 实现 HTTP 处理器
// 本文件：报告端点翻译壳（评估与电池报告共用）——id 解析 + 协调器纯值结果 → HTTP 响应。
// 协调器本体在 internal/valuation/report（gin-free），此处只做薄翻译。
package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/storage"
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

// serveReportDownload 处理 GET <prefix>/:id/report：URL 有效则代理流式返回 PDF；
// 否则并发安全地再生成后返回。代理模式经 storage.Get 中转内容，
// 绕开对象存储（R2）的浏览器跨域限制——R2 bucket 未配置 CORS 时浏览器跨域
// 请求失败，后端中转后浏览器只与本域交互。
func serveReportDownload[T any](c *gin.Context, coord *report.Coordinator[T], st storage.Storage, notFoundMsg string, logger *zap.Logger) {
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

	proxyPDFDownload(c, ctx, st, pdfURL, logger)
}

// proxyPDFDownload 经 storage.Get 流式返回 PDF 内容（触发浏览器下载）。
func proxyPDFDownload(c *gin.Context, ctx context.Context, st storage.Storage, pdfURL string, logger *zap.Logger) {
	body, err := st.Get(ctx, pdfURL)
	if err != nil {
		logger.Error("读取报告内容失败", zap.Error(err), zap.String("url", pdfURL))
		response.ServerError(c, "下载失败，请稍后重试")
		return
	}
	defer body.Close()

	// 触发浏览器下载（Content-Disposition attachment + 文件名）
	filename := "evaluation_report.pdf"
	if seg := strings.Split(pdfURL, "/"); len(seg) > 0 && seg[len(seg)-1] != "" {
		filename = seg[len(seg)-1]
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Content-Type", "application/pdf")
	c.Status(http.StatusOK)
	// 流式拷贝，避免大文件整载内存
	if _, err := io.Copy(c.Writer, body); err != nil {
		logger.Warn("流式回传报告中断", zap.Error(err), zap.String("url", pdfURL))
	}
}
