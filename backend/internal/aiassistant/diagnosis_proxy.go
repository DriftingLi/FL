// 外部诊断助手回答配图的白名单代理（ADR-0091）：
// AI 生成的正文里图片以第三方 OSS 公网链接出现（对方 v2 的约定），本站把**白名单内**的那些
// 改写成 /api/ai-assistant/diagnosis/ossimg/* 由这里取字节回源，于是学员浏览器不会因为一段
// AI 生成的正文去请求任意主机（ADR-0063 决策 3 的原始动机）。白名单外的图片在 adapter 侧就
// 已降级成文字，走不到这里。
//
// 与 chat 面鉴权一致（handler 层 OptionalAuth），图片不得混进计量面（ADR-0031）。
// SSRF 防御见 diagnosisOSSProxyPath 的五道闸。
package aiassistant

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// staticSegmentPattern 单个路径段的合法形状：Unicode 字母/数字开头，后续可含 _ - . 。
// 白名单式（而非黑名单列危险字符）：`..`、以 . 开头的隐藏段、空段、`\`、`%`、`?`、`#`、
// `;`、NUL 等全部天然落网。OSS 侧的目录名与文件名可能是中文（对方按规范名落盘），
// 故必须按 Unicode 字母放行。
var staticSegmentPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}_\-.]*$`)

// diagnosisImageExtPattern 图片扩展名白名单。
// 与退役前手册代理用的 staticExtPattern 分开：那个还放行 pdf（手册代理要发 PDF），
// 而本路由只出现在 ![]() 图片语法里，放行 pdf 只会得到一张坏图（ADR-0091 决策 2）。
var diagnosisImageExtPattern = regexp.MustCompile(`(?i)\.(png|jpg|jpeg|webp)$`)

// diagnosisOSSRelPrefix 白名单路径前缀的相对形态（不含前导 /）。
// 由 ossBase 的 path 推出，配置校验保证它落在 /forklift/repair_img 之下。
const diagnosisOSSRelPrefixDefault = "forklift/repair_img"

// diagnosisOSSProxyPath 判定一个第三方图片 URL 是否在白名单内，并给出本站代理路径。
//
// 五道闸（ADR-0091 决策 2），任一不过即返回 false —— 调用方据此降级成文字：
//  1. scheme 必须 https，host 必须与 ossBase 的 host 逐字相同（防同形异域）；
//  2. 剥掉 ?# 后，path 必须落在 ossBase 的路径前缀之下（`/forklift/repair_img/`）；
//  3. PathUnescape 解一次（对方生成 URL 时已 quote() 过文件名），解完逐段过
//     staticSegmentPattern——`..`、`%2e%2e`、绝对路径、空段全部落网；
//  4. 末段过 diagnosisImageExtPattern；
//  5. 输出逐段 PathEscape 后拼到 diagnosisImageRoute 之后（代理请求侧解一次即可还原）。
//
// ossBase 为空/不可解析时一律 false：配置没给 OSS 基准，就没有任何图可服务。
func diagnosisOSSProxyPath(ossBase, raw string) (string, bool) {
	base, err := url.Parse(strings.TrimSuffix(strings.TrimSpace(ossBase), "/"))
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return "", false
	}
	// 剥 ?#：查询串与片段不参与资源定位，也不许借它们绕过前缀判定。
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Host, base.Host) {
		return "", false
	}

	rel, ok := diagnosisOSSRelPath(base.Path, u.Path)
	if !ok {
		return "", false
	}
	segs := strings.Split(rel, "/")
	if len(segs) == 0 {
		return "", false
	}
	for _, seg := range segs {
		if !staticSegmentPattern.MatchString(seg) {
			return "", false
		}
	}
	if !diagnosisImageExtPattern.MatchString(segs[len(segs)-1]) {
		return "", false
	}
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return diagnosisImageRoute + strings.Join(segs, "/"), true
}

// diagnosisOSSRelPath 把绝对 path 折成「相对 ossBase 前缀」的子路径。
// 入参 path 可能仍是百分号编码形态（对方 quote() 过），故先解一次再比前缀；
// 只解一次：二次编码（%252e）解完残留 `%`，会被 staticSegmentPattern 拒掉。
func diagnosisOSSRelPath(basePath, absPath string) (string, bool) {
	prefix := strings.TrimSuffix(basePath, "/")
	if prefix == "" {
		prefix = "/" + diagnosisOSSRelPrefixDefault
	}
	decoded, err := url.PathUnescape(absPath)
	if err != nil {
		return "", false
	}
	if !strings.HasPrefix(decoded, prefix+"/") {
		return "", false
	}
	rel := strings.TrimPrefix(decoded, prefix+"/")
	if rel == "" {
		return "", false
	}
	return rel, true
}

// diagnosisHostOf 取 URL 的 host（降级日志用；解析失败返回空串）。
func diagnosisHostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// resolveOSSSubpath 把路由参数折成上游 OSS 子路径（相对 ossBase 的前缀），返回 false 即非法。
//
// 与 resolveStaticSubpath（退役前的手册代理）同构，只是白名单换成本路由的那一套：
// 解一次 → 逐段 staticSegmentPattern → 末段 diagnosisImageExtPattern → 逐段 PathEscape。
// 解一次是硬要求：对方生成 URL 时已 quote() 过文件名，不解就永远匹配不上；
// 而多解一次会让 `%252e` 变成 `%2e` 再被当成合法段——这里只解一次，二次编码残留 `%` 即落网。
func resolveOSSSubpath(subpath string) (string, bool) {
	trimmed := strings.TrimPrefix(subpath, "/")
	if trimmed == "" {
		return "", false
	}
	if decoded, err := url.PathUnescape(trimmed); err == nil {
		trimmed = decoded
	}
	segs := strings.Split(trimmed, "/")
	for _, seg := range segs {
		if !staticSegmentPattern.MatchString(seg) {
			return "", false
		}
	}
	if !diagnosisImageExtPattern.MatchString(segs[len(segs)-1]) {
		return "", false
	}
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/"), true
}

// DiagnosisProxyService 诊断回答配图的回源代理。
// 名字沿用（票 2237 决策：不为了改名去改装配链），但它现在只服务一个端点：
// ossimg。旧的手册/品牌/车型/故障码代理已随 v1 通道退役一并删除。
type DiagnosisProxyService struct {
	ossBase string
	client  *http.Client
	logger  *zap.Logger
}

// NewDiagnosisProxyService 构建回源代理。ossBase 为空时 OpenOSSImage 一律返回「未配置」错误
// （此时 adapter 侧也不会产出任何 ossimg 路径——两处同源，不会出现半可用态）。
func NewDiagnosisProxyService(ossBase string, logger *zap.Logger) *DiagnosisProxyService {
	return &DiagnosisProxyService{
		ossBase: strings.TrimSuffix(strings.TrimSpace(ossBase), "/"),
		// 不跟随重定向：白名单只对声明的 host 成立，302 到别处必须显式失败（ADR-0091 决策 8）。
		client: &http.Client{
			Timeout: aiStreamTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// OpenOSSImage 打开一张白名单内的回答配图（GET <ossBase>/<子路径>），返回响应体与 Content-Type；
// 调用方负责关闭。subpath 是 route 里的 *filepath（已是逐段转义形态，可能含中文目录名）。
func (s *DiagnosisProxyService) OpenOSSImage(ctx context.Context, subpath string) (io.ReadCloser, string, error) {
	if s.ossBase == "" {
		return nil, "", errors.New("诊断配图服务未配置，请联系管理员")
	}
	upstream, ok := resolveOSSSubpath(subpath)
	if !ok {
		return nil, "", errors.New("无效的图片路径")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.ossBase+"/"+upstream, nil)
	if err != nil {
		return nil, "", fmt.Errorf("构造图片请求失败: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("诊断配图回源失败", zap.Error(err))
		}
		return nil, "", errors.New("诊断配图暂时不可用")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("诊断配图不存在（HTTP %d）", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}
