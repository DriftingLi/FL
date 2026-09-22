// Package service 外部诊断 RAG 助手只读代理（计划 批次2）：
// 供学员端「智能维修诊断」周边面板（品牌/车型联动、故障码查询、手册与案例静态资源）使用。
// 与 diagnosis adapter（ai_diagnosis_adapter.go）同源但不同消费面：本 service 直连助手
// 只读 GET 端点（绕过 nginx Basic Auth 层——后端直连 172.17.1.23），鉴权由 handler 层
// 与 chat 一致（OptionalAuth）。SSRF 防御：子路径按段做 Unicode 白名单校验 + 扩展名白名单，
// 静态根只允许 manual 与 fault_images（见 resolveStaticSubpath）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// DiagnosisBrandOption 品牌选项（[{value,label}]，value=all 为全部品牌）。
type DiagnosisBrandOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// DiagnosisFaultCodeItem 故障码条目（字段与助手 /fault-codes 一致）。
type DiagnosisFaultCodeItem struct {
	ID            int    `json:"id"`
	Brand         string `json:"brand"`
	BrandCN       string `json:"brand_cn"`
	ModelSeries   string `json:"model_series"`
	FaultCode     string `json:"fault_code"`
	FaultName     string `json:"fault_name"`
	Symptom       string `json:"symptom"`
	Causes        string `json:"causes"`
	SOPSteps      string `json:"sop_steps"`
	SafetyWarning string `json:"safety_warning"`
	PartNumbers   string `json:"part_numbers"`
	SourceFile    string `json:"source_file"`
	PageNum       int    `json:"page_num"`
}

// DiagnosisFaultCodePage 故障码分页（items + total）。
type DiagnosisFaultCodePage struct {
	Items []DiagnosisFaultCodeItem `json:"items"`
	Total int                      `json:"total"`
}

// 助手静态资源根白名单（防 SSRF/路径穿越）：manual = 手册页图与 PDF；
// fault_images = 图文维修案例配图（20260921 新增，目录名是中文系统名，故段校验按
// Unicode 字母放行——旧 ASCII 正则会把这些资源全部判非法）。
var staticRoots = map[string]bool{"manual": true, "fault_images": true}

// staticSegmentPattern 单个路径段的合法形状：Unicode 字母/数字开头，后续可含 _ - . 。
// 白名单式（而非黑名单列危险字符）：`..`、以 . 开头的隐藏段、空段、`\`、`%`、`?`、`#`、
// `;`、NUL 等全部天然落网。
var staticSegmentPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}_\-.]*$`)

// staticExtPattern 末段扩展名白名单，大小写不敏感（外部助手吐 PNG/JPG）。
var staticExtPattern = regexp.MustCompile(`(?i)\.(png|jpg|jpeg|pdf)$`)

// resolveStaticSubpath 把客户端子路径解析为助手侧完整子路径（恒含静态根），返回 false 即非法。
// 两种输入形状：
//   - 带根（manual/… 或 fault_images/…）⇒ 按原根透传；
//   - 无根（ep_xxx/page_1.png）⇒ 恒归 manual —— 这是既有 Web/移动端与**全部历史行**
//     所发的形状（ADR-0033 明令不回填，旧标记形状永存），不是对越界输入的兜底。
//
// 绝对形状（/assistant/static/… 与 /app/static/…）显式拒绝：strip 唯一在前端 DiagnosisSources
// 组件完成，后端不兜底 —— 否则同一入参会被拼成 manual/assistant/static/… 这类双前缀越界形状。
//
// 入参可能是百分号编码形态（Web/移动端 manualUrl 逐段 encodeURIComponent，中文案例目录必为
// 此形，而 gin 是否已解码不该由这里去赌），故先解一次再走同一白名单：编码后穿越
// （`%2e%2e/`）解码即落网，二次编码（`%252e`）解一次后残留 `%` 同样被拒；出参逐段转义，
// 交给上游的恒为合法 URL 路径。
func resolveStaticSubpath(subpath string) (string, bool) {
	trimmed := strings.TrimPrefix(subpath, "/")
	if trimmed == "" {
		return "", false
	}
	if decoded, err := url.PathUnescape(trimmed); err == nil {
		trimmed = decoded
	}
	// 绝对形状（助手内网前缀）一律拒绝，见函数头「后端不兜底 strip」条。
	if strings.Contains(trimmed, "assistant/static/") || strings.Contains(trimmed, "app/static/") {
		return "", false
	}
	segs := strings.Split(trimmed, "/")
	for _, seg := range segs {
		if !staticSegmentPattern.MatchString(seg) {
			return "", false
		}
	}
	if !staticExtPattern.MatchString(segs[len(segs)-1]) {
		return "", false
	}
	if !staticRoots[segs[0]] {
		segs = append([]string{"manual"}, segs...)
	}
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return strings.Join(segs, "/"), true
}

// DiagnosisProxyService 助手只读端点代理（品牌/车型/故障码/手册资源）。
type DiagnosisProxyService struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewDiagnosisProxyService 构建代理 service。baseURL 为空时各方法返回「未配置」错误。
func NewDiagnosisProxyService(baseURL string, logger *zap.Logger) *DiagnosisProxyService {
	return &DiagnosisProxyService{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{Timeout: aiStreamTimeout},
		logger:  logger,
	}
}

// ListBrands 品牌列表（GET /assistant/api/brands）。
func (s *DiagnosisProxyService) ListBrands(ctx context.Context) ([]DiagnosisBrandOption, error) {
	var out []DiagnosisBrandOption
	if err := s.getJSON(ctx, "/assistant/api/brands", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListModels 某品牌车型列表（GET /assistant/api/models?brand=）。
func (s *DiagnosisProxyService) ListModels(ctx context.Context, brand string) ([]string, error) {
	q := ""
	if brand != "" && brand != "all" {
		q = "?brand=" + url.QueryEscape(brand)
	}
	var out []string
	if err := s.getJSON(ctx, "/assistant/api/models"+q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListFaultCodes 故障码分页查询（GET /assistant/api/fault-codes?brand&keyword&page&page_size）。
func (s *DiagnosisProxyService) ListFaultCodes(ctx context.Context, brand, keyword string, page, pageSize int) (*DiagnosisFaultCodePage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := url.Values{}
	if brand != "" && brand != "all" {
		q.Set("brand", brand)
	}
	if keyword != "" {
		q.Set("keyword", keyword)
	}
	q.Set("page", fmt.Sprint(page))
	q.Set("page_size", fmt.Sprint(pageSize))
	var out DiagnosisFaultCodePage
	if err := s.getJSON(ctx, "/assistant/api/fault-codes?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OpenManual 打开助手静态资源（GET /assistant/static/<root>/<子路径>，root ∈ manual/fault_images）。
// 返回响应体与 Content-Type；调用方负责关闭。路径非法（SSRF/穿越）时返回错误。
func (s *DiagnosisProxyService) OpenManual(ctx context.Context, subpath string) (io.ReadCloser, string, error) {
	upstream, ok := resolveStaticSubpath(subpath)
	if !ok {
		return nil, "", errors.New("无效的手册资源路径")
	}
	u := s.baseURL + "/assistant/static/" + upstream
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", fmt.Errorf("构造手册请求失败: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", errors.New("诊断手册服务暂时不可用")
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("手册资源不存在（HTTP %d）", resp.StatusCode)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

// ---- 内部：统一 GET + data 解码（{code, data: <T>} 信封）----

func (s *DiagnosisProxyService) getJSON(ctx context.Context, path string, out any) error {
	if s.baseURL == "" {
		return errors.New("智能维修诊断服务未配置，请联系管理员")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("构造诊断查询失败: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("诊断代理查询失败", zap.String("path", path), zap.Error(err))
		return errors.New("智能维修诊断服务暂时不可用")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New("读取诊断服务响应失败")
	}
	if resp.StatusCode != http.StatusOK {
		var env struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(raw, &env) == nil && env.Detail != "" {
			return errors.New("诊断失败：" + env.Detail)
		}
		return fmt.Errorf("诊断服务响应异常（HTTP %d）", resp.StatusCode)
	}
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return errors.New("解析诊断响应失败")
	}
	if env.Code != 0 && env.Code != 200 {
		return fmt.Errorf("诊断失败（code %d）", env.Code)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return errors.New("解析诊断数据失败")
	}
	return nil
}
