package logger

import (
	"regexp"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// sensitiveKeySubstrs 敏感字段 key 黑名单（大小写不敏感、子串匹配）。
// 命中时值将被打码，防止凭证与 PII 落入日志流。
var sensitiveKeySubstrs = []string{
	"password",
	"passwd",
	"token",
	"secret",
	"authorization",
	"api-key",
	"apikey",
	"access-key",
	"accesskey",
	"secret-key",
	"secretkey",
	"code",
	"phone",
	"mobile",
	"email",
}

// credentialURLRe 匹配带内嵌凭证的连接串（postgres://user:pass@host、http(s)://user:pass@host 等），
// 脱敏为 scheme://***@host。
var credentialURLRe = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/@\s]+@`)

// isSensitiveKey 判断结构化字段 key 是否命中敏感黑名单。
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeySubstrs {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// Redact 对结构化字段做脱敏：key 命中黑名单时值替换为 ***，否则原样返回。
// 用法：logger.Info("x", logger.Redact(zap.String("token", t)))
func Redact(field zap.Field) zap.Field {
	if field.Type == zapcore.SkipType || !isSensitiveKey(field.Key) {
		return field
	}
	return zap.String(field.Key, "***")
}

// RedactError 脱敏错误信息中内嵌的连接串凭证（如 DATABASE_URL 泄漏进错误串）。
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return credentialURLRe.ReplaceAllString(err.Error(), "${1}***@")
}
