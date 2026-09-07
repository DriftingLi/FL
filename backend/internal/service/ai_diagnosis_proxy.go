// Package service 外部诊断 RAG 助手只读代理（计划 批次2）：
// 供学员端「智能维修诊断」周边面板（品牌/车型联动、故障码查询、手册静态资源）使用。
// 与 diagnosis adapter（ai_diagnosis_adapter.go）同源但不同消费面：本 service 直连助手
// 只读 GET 端点（绕过 nginx Basic Auth 层——后端直连 172.17.1.23），鉴权由 handler 层
// 与 chat 一致（OptionalAuth）。SSRF 防御：手册子路径仅允许 [a-zA-Z0-9_\-/.]且必须带扩展名。
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

// manualPathPattern 手册子路径白名单（防 SSRF/路径穿越；允许带扩展名的静态文件）。
// 扩展名大小写不敏感（外部助手吐 PNG/JPG）。白名单锚定「相对子路径」形状：
// 以 / 开头或含 assistant/static/ 前缀的一律非法（strip 唯一在前端 DiagnosisSources 组件完成）。
var manualPathPattern = regexp.MustCompile(`(?i)^[a-zA-Z0-9_\-/]+\.(png|jpg|jpeg|pdf)$`)

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

// OpenManual 打开手册静态资源（GET /assistant/static/manual/<subpath>）。
// 返回响应体与 Content-Type；调用方负责关闭。path 不合法（SSRF/穿越）时返回错误。
func (s *DiagnosisProxyService) OpenManual(ctx context.Context, subpath string) (io.ReadCloser, string, error) {
	// 白名单只接受相对子路径：前导 / 先去掉再校验仍非法的（如 assistant/static/… 双前缀
	// 形状），显式拒绝——strip 唯一在前端 DiagnosisSources 组件完成，后端不兜底。
	trimmed := strings.TrimPrefix(subpath, "/")
	if !manualPathPattern.MatchString(trimmed) || strings.Contains(trimmed, "assistant/static/") {
		return nil, "", errors.New("无效的手册资源路径")
	}
	subpath = trimmed
	u := s.baseURL + "/assistant/static/manual/" + subpath
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
