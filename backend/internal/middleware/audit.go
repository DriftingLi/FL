package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
)

// AuditLog 管理员/讲师写操作审计中间件。
// 挂载在 /api 路由组，不依赖中间件顺序：请求处理完成后读取 JWT 上下文，
// 仅记录 admin / tutor 角色的 POST/PUT/PATCH/DELETE 请求。
func AuditLog(cfg *config.Config, db *gorm.DB, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method != http.MethodPost && method != http.MethodPut &&
			method != http.MethodPatch && method != http.MethodDelete {
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		// 仅缓存小体积 JSON 请求体用于审计；大文件上传 / 未知长度请求体跳过，
		// 避免内存开销，也避免截断后破坏后续 handler 读取。
		var body string
		if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") &&
			c.Request.ContentLength > 0 && c.Request.ContentLength <= 64*1024 {
			if data, err := io.ReadAll(c.Request.Body); err == nil {
				body = string(data)
				c.Request.Body = io.NopCloser(bytes.NewReader(data))
			}
		}

		start := time.Now()
		c.Next()

		role := CurrentRole(c)
		userID := CurrentUserID(c)
		if userID <= 0 || (role != "admin" && role != "tutor") {
			return
		}

		detail, _ := json.Marshal(map[string]any{
			"query":        c.Request.URL.RawQuery,
			"request_body": body,
			"status":       c.Writer.Status(),
			"duration_ms":  time.Since(start).Milliseconds(),
		})
		record := model.AuditLog{
			ActorID:   userID,
			ActorRole: role,
			ActorName: CurrentUsername(c),
			Action:    describeAuditAction(method, c.Request.URL.Path),
			Path:      c.Request.URL.Path,
			Method:    method,
			RequestID: c.GetString(string(CtxRequestID)),
			IP:        c.ClientIP(),
			Status:    c.Writer.Status(),
			Detail:    model.JSONB(detail),
			CreatedAt: time.Now(),
		}
		if err := db.Create(&record).Error; err != nil {
			logger.Warn("写入审计日志失败", zap.Error(err), zap.String("path", c.Request.URL.Path))
		}
	}
}

// auditResourceNames 路径资源关键词 → 大白话资源名（按关键词长度降序，先匹配更具体的资源）。
var auditResourceNames = []struct {
	key  string
	name string
}{
	{"profile-reviews", "资料审核"},
	{"hrwai-users", "学员"},
	{"exam-sessions", "考试场次"},
	{"featured-content", "精选内容"},
	{"ai-configs", "AI配置"},
	{"forum/replies", "论坛回复"},
	{"forum/topics", "论坛帖子"},
	{"question", "题目"},
	{"chapter", "章节"},
	{"course", "课程"},
	{"grading", "阅卷"},
	{"tutor", "导师"},
}

// describeAuditAction 将 HTTP 方法与路径转成普通人能看懂的操作描述。
func describeAuditAction(method, path string) string {
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, "/approve") {
		return "通过审核"
	}
	if strings.HasSuffix(lower, "/reject") {
		return "驳回申请"
	}
	if strings.HasSuffix(lower, "/password") {
		return "重置密码"
	}
	if strings.HasSuffix(lower, "/status") {
		return "调整状态"
	}

	verb := ""
	switch method {
	case http.MethodPost:
		verb = "新增"
	case http.MethodPut, http.MethodPatch:
		verb = "修改"
	case http.MethodDelete:
		verb = "删除"
	default:
		return method + " " + path
	}

	for _, r := range auditResourceNames {
		if strings.Contains(lower, r.key) {
			return verb + r.name
		}
	}
	return verb + "数据"
}
