// Package deploy 是**部署事实的唯一事实源**（ADR-0047 §5 / spec #932）。
//
// 背景：同一份 .env 默认值此前在 `.github/workflows/cd.yml`、`scripts/deploy-remote.sh`、
// `docker-compose.prod.yml` 各写一遍，且已经漂移（REDIS_POOL_SIZE 20/10/20）。本包把「变量名 +
// 默认值」收成一张 Go 声明表（与 internal/service/ai_feature_registry.go 同构），由
// cmd/gen-deploy 渲染出 shell 可 source 的 `deploy/env.defaults`，并由测试断言三份部署文件
// 与声明表逐字一致——漂移从「人工 diff 发现」变成「go test 报红」。
//
// 本期只做声明 + 生成 + 断言，不改动部署脚本的执行路径（见 spec #932 决策 5）。
//
// 片四（spec #940）把生成物接进了部署链路：cd.yml 生成的环境变量文件**先 source 它**，
// 在其之上只补写「secrets / 环境确实提供了值的变量」；因此 cd.yml 里不再出现声明表内变量的
// 默认值字面量，compose 的「有默认值的变量引用」退居最后兜底（且仍被漂移锁钉住）。
// 接线本身由 TestEnvDefaultsIsConsumed 守住 —— 否则某次重构能把消费者悄悄摘掉。
package deploy

import (
	"errors"
	"fmt"
	"strings"
)

// EnvVar 一个部署环境变量及其默认值。
type EnvVar struct {
	Name    string
	Default string
	Desc    string
}

// EnvVars 部署环境变量声明表（**取值范围**：compose 中带非空默认值的变量）。
//
// 带密钥语义的变量（DB_PASSWORD / JWT_SECRET_KEY / REDIS_PASSWORD / SMTP_PASSWORD /
// TENCENT_SMS_*_KEY / R2_* 等）默认值恒为空，不入表——空默认不是「事实」，是「必须由环境提供」。
var EnvVars = []EnvVar{
	{Name: "AUTH_COOKIE_NAME", Default: "hrwai_token", Desc: "登录态 Cookie 名（学员侧；招聘侧另见 config）"},
	{Name: "AUTH_COOKIE_SECURE", Default: "true", Desc: "Cookie 是否仅 HTTPS"},
	{Name: "BACKEND_HOST_PORT", Default: "8080", Desc: "后端对外发布端口"},
	{Name: "BACKEND_IMAGE", Default: "forklift-backend:latest", Desc: "后端镜像"},
	{Name: "DB_USER", Default: "forklift", Desc: "Postgres 用户"},
	{Name: "DIAGNOSIS_ASSISTANT_URL", Default: "http://172.17.1.23:8000", Desc: "外部诊断 RAG 服务地址（ADR-0032）"},
	{Name: "DOMAIN", Default: "localhost", Desc: "主域名"},
	{Name: "FRONTEND_IMAGE", Default: "forklift-frontend:latest", Desc: "前端镜像"},
	{Name: "JWT_EXPIRES_HOURS", Default: "2", Desc: "access token 有效期（小时，ADR-0016）"},
	{Name: "JWT_REFRESH_EXPIRES_DAYS", Default: "7", Desc: "refresh token 有效期（天，ADR-0016）"},
	{Name: "LIBREOFFICE_IMAGE", Default: "forklift-libreoffice:latest", Desc: "PPT 转图 sidecar 镜像"},
	{Name: "LOGS_VOLUME", Default: "logs-data", Desc: "日志卷"},
	{Name: "LOG_COMPRESS", Default: "true", Desc: "日志轮转压缩"},
	{Name: "LOG_DIR", Default: "/data/logs", Desc: "日志目录"},
	{Name: "LOG_FORMAT", Default: "console", Desc: "日志格式（console | json）"},
	{Name: "LOG_LEVEL", Default: "info", Desc: "日志级别"},
	{Name: "LOG_MAX_AGE_DAYS", Default: "30", Desc: "日志保留天数"},
	{Name: "LOG_MAX_BACKUPS", Default: "7", Desc: "日志备份份数"},
	{Name: "LOG_MAX_SIZE_MB", Default: "100", Desc: "单日志文件上限（MB）"},
	{Name: "PG_VOLUME", Default: "pgdata-prod", Desc: "Postgres 数据卷"},
	{Name: "PORTAL_HOST_PORT", Default: "3000", Desc: "门户对外发布端口"},
	{Name: "REDIS_DB", Default: "0", Desc: "Redis 逻辑库"},
	{Name: "REDIS_KEY_PREFIX", Default: "fl:", Desc: "Redis 键前缀"},
	{Name: "REDIS_POOL_SIZE", Default: "20", Desc: "Redis 连接池大小"},
	{Name: "REDIS_VOLUME", Default: "redisdata-prod", Desc: "Redis 数据卷"},
	{Name: "REPORTS_VOLUME", Default: "reports-data", Desc: "报告卷"},
	{Name: "SMTP_FROM_NAME", Default: "和润天下", Desc: "发件人显示名"},
	{Name: "SMTP_PORT", Default: "465", Desc: "SMTP 端口"},
	{Name: "SSL_CERT_DIR", Default: "./nginx/ssl", Desc: "证书目录"},
	{Name: "STORAGE_DRIVER", Default: "local", Desc: "存储驱动（local | r2）"},
	{Name: "SWAGGER_ENABLED", Default: "false", Desc: "是否开启 Swagger"},
	{Name: "TENCENT_SMS_REGION", Default: "ap-guangzhou", Desc: "腾讯云短信区域"},
	{Name: "TRUSTED_PROXIES", Default: "127.0.0.1/32,172.19.0.1/32,192.168.240.1/32", Desc: "受信反代网段（可信取 IP，ADR-0045）"},
	{Name: "UPLOADS_VOLUME", Default: "uploads-data", Desc: "上传目录卷"},
}

// byName 变量名索引（渲染与断言共用）。
var byName = func() map[string]EnvVar {
	m := make(map[string]EnvVar, len(EnvVars))
	for _, v := range EnvVars {
		m[v.Name] = v
	}
	return m
}()

// Lookup 按变量名取声明（供断言与后续消费方使用）。
func Lookup(name string) (EnvVar, bool) {
	v, ok := byName[name]
	return v, ok
}

// RenderEnvDefaults 渲染 `deploy/env.defaults`（纯函数、确定性输出、无时间戳）。
//
// 输出为 shell 可 source 的 `export VAR=值` 行；值按单引号包裹转义（含 `和润天下` 这类
// 非 ASCII 值），保证与三份部署文件的默认值逐字一致。
func RenderEnvDefaults() (string, error) {
	if len(EnvVars) == 0 {
		return "", errors.New("部署声明表为空，拒绝生成空的默认值文件")
	}
	var b strings.Builder
	b.WriteString("# 生成文件，勿手改（ADR-0047 §5 / spec #932）。\n")
	b.WriteString("# 唯一事实源：backend/internal/deploy/topology.go。\n")
	b.WriteString("# 再生成：cd backend && go run ./cmd/gen-deploy\n")
	b.WriteString("# 同步契约：backend/internal/deploy/topology_test.go 与本文件全等比对，\n")
	b.WriteString("# 并由漂移锁断言 compose 的默认值一致。\n")
	b.WriteString("# 消费方：CD 生成的环境变量文件 source 本文件（spec #940 片四 / ADR-0047 §5），\n")
	b.WriteString("# 随后只覆盖「secrets / 环境确实提供了值」的变量。\n")
	for _, v := range EnvVars {
		fmt.Fprintf(&b, "# %s\n", v.Desc)
		fmt.Fprintf(&b, "export %s=%s\n", v.Name, shellQuote(v.Default))
	}
	return b.String(), nil
}

// shellQuote 单引号转义（内部单引号用 '\” 拼接）。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
