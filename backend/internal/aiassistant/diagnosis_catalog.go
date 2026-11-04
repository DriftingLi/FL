// v2 周边只读面的服务层（票 2242）：意图引导 / 系统型号 / 故障现象 / 故障码体检 / 健康检查。
//
// 与 diagnosis_adapter.go 的分工：那边是**对话通道**（POST /api/v2/chat 的 SSE 消费者，
// 产回答正文）；这边是**引导面**（五个 GET，产点选面板的数据）。两者共用同一组上游配置
// （DIAGNOSIS_ASSISTANT_URL + DIAGNOSIS_V2_API_KEY）与同一条错误翻译（v2StatusError），
// 但生命周期不同——对话是流式的、每个请求一条连接；引导面是一次性 JSON。
//
// 为什么单独一个 service 而不是塞进 adapter：adapter 实现的是 ModelPort（Stream/Complete
// 两个方法），把五个 GET 挂上去等于让一个「模型端口」承担 UI 数据面，端口就不再是可替换的
// 模型抽象。这里的 service 只被 handler 消费，没有端口契约要守。
package aiassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ---- 响应 DTO ----
//
// 字段名一律照 v2 契约（§4.1~§4.4）：对方的键本身就是小写单词或 snake_case，
// 转成本仓的蛇形只会凭空造一层映射，且将来上游加字段时两边对不上。

// DiagnosisIntentBrand 引导第一层的品牌卡（v2 `intents.brands[]`）。
type DiagnosisIntentBrand struct {
	ID      string `json:"id"`      // 品牌标识，调 /diagnosis/systems 时用作 brand 参数
	Name    string `json:"name"`    // 展示名称
	Tagline string `json:"tagline"` // 一句话简介
	Icon    string `json:"icon"`    // emoji 图标
	Cls     string `json:"cls"`     // 对方的样式类名（我方按自己的品牌色渲染，原样保留仅为对齐契约）
	Picker  string `json:"picker"`  // 第二层形态：system=选系统，model=选型号
}

// DiagnosisFAQ 高频问题（v2 `intents.faq[]`）：点一下就把 q 当 content 发出去。
type DiagnosisFAQ struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// DiagnosisIntents 引导第一层（GET /diagnosis/intents）。
type DiagnosisIntents struct {
	Brands []DiagnosisIntentBrand `json:"brands" nullability:"nonnil"` // 空集而不是 null：前端直接 .length / v-for
	FAQ    []DiagnosisFAQ         `json:"faq" nullability:"nonnil"`    // 同上
}

// DiagnosisCatalogItem 引导第二/三层的条目（系统、型号、故障现象共用一种形状）。
// Icon 只在中力的系统列表里出现，林德型号没有 —— omitempty 让「没有这个字段」
// 与「有这个字段但为空」在 wire 上长得一样，这正是契约描述的差异。
type DiagnosisCatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
}

// DiagnosisCatalogItems 第二/三层列表（GET /diagnosis/systems、/diagnosis/faults）。
// 包一层 `items` 而不是裸数组：与 v2 契约同形，上游将来加字段（如每条带 count）不必改我方形状。
type DiagnosisCatalogItems struct {
	Items []DiagnosisCatalogItem `json:"items" nullability:"nonnil"`
}

// DiagnosisCodeHint 故障码实时体检（GET /diagnosis/code-hint）。
// Status 五态：known / unknown / format / none / typo（契约 §4.4）。
type DiagnosisCodeHint struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Fix     string   `json:"fix"`     // 纠偏建议（应为这个写法）；无建议时空串
	Samples []string `json:"samples" nullability:"nonnil"` // 仅 format 状态返回，供点选填入
}

// DiagnosisHealth 上游可达性（GET /diagnosis/health）。
//
// **有意只投影两字段**：v2 的 /api/health 返回 20 个字段，其中 key_fp8（百炼 Key 的
// sha256 前 8 位）、data_dir（ECS 绝对路径）、workflow_id_head、memory_mode 都是对方
// 内部拓扑，不该出现在一个未登录也能读的端点里。前端进页探测只需要「能不能用」。
type DiagnosisHealth struct {
	OK            bool `json:"ok"`             // 上游进程存活
	AppConfigured bool `json:"app_configured"` // 上游必填配置（Key / 工作流）齐全
}

// ---- 服务 ----

// diagnosisCatalogTimeout 引导面的单次超时：这些是一击即回的 JSON 查询，
// 不像对话要等 8~15 秒。10 秒足够覆盖上游冷启动，又不会让进页探测卡住整页。
const diagnosisCatalogTimeout = 10 * time.Second

// DiagnosisCatalogService v2 周边只读面的代理。
// baseURL 或 apiKey 为空时各方法返回「未配置」错误（与 adapter 同一条友好文案）。
type DiagnosisCatalogService struct {
	baseURL string
	apiKey  string
	client  *http.Client
	logger  *zap.Logger
}

// NewDiagnosisCatalogService 构建引导面代理。
// 不跟随重定向：凭证与白名单只对声明的上游成立，302 到别处必须显式失败（ADR-0091 决策 8 同款）。
func NewDiagnosisCatalogService(baseURL, apiKey string, logger *zap.Logger) *DiagnosisCatalogService {
	return &DiagnosisCatalogService{
		baseURL: strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client: &http.Client{
			Timeout: diagnosisCatalogTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
	}
}

// ErrDiagnosisNotConfigured 上游基址或 Key 未配置（部署态，不是故障）。
var ErrDiagnosisNotConfigured = errors.New("智能维修诊断服务未配置，请联系管理员")

// getJSON 发起一次带鉴权的 GET，把 2xx 的响应体解到 out。path 以 / 开头且不含查询串时由调用方拼好。
func (s *DiagnosisCatalogService) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	if s.baseURL == "" || s.apiKey == "" {
		return ErrDiagnosisNotConfigured
	}
	full := s.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return fmt.Errorf("构造诊断请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if s.logger != nil {
			s.logger.Error("诊断引导面调用失败", zap.String("path", path), zap.Error(err))
		}
		return errors.New("智能维修诊断服务暂时不可用，请稍后重试")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return v2StatusError(resp, s.logger)
	}
	// 上游 JSON 可能有未预料的字段：未知键忽略，缺字段落零值（契约字段都不可空，
	// 缺了就是上游违约，这时零值 + 空集比 500 更能让页面降级）。
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(out); err != nil {
		if s.logger != nil {
			s.logger.Error("解析诊断引导面响应失败", zap.String("path", path), zap.Error(err))
		}
		return errors.New("诊断服务返回了无法解析的数据，请稍后重试")
	}
	return nil
}

// Intents 引导第一层：品牌卡 + 高频问题。
func (s *DiagnosisCatalogService) Intents(ctx context.Context) (DiagnosisIntents, error) {
	var out DiagnosisIntents
	if err := s.getJSON(ctx, "/api/v2/intents", nil, &out); err != nil {
		return DiagnosisIntents{}, err
	}
	// 集合恒非 null（判据 5）：上游返回 null / 缺字段时归一成空集，
	// 前端才敢直接 brands.length，不必写 brands?.length 这种防御。
	if out.Brands == nil {
		out.Brands = []DiagnosisIntentBrand{}
	}
	if out.FAQ == nil {
		out.FAQ = []DiagnosisFAQ{}
	}
	return out, nil
}

// Systems 引导第二层：某品牌下的系统（中力）或型号（林德）列表。brand 必填。
func (s *DiagnosisCatalogService) Systems(ctx context.Context, brand string) (DiagnosisCatalogItems, error) {
	var out DiagnosisCatalogItems
	if err := s.getJSON(ctx, "/api/v2/systems", url.Values{"brand": {brand}}, &out); err != nil {
		return DiagnosisCatalogItems{}, err
	}
	out.Items = normalizeCatalogItems(out.Items)
	return out, nil
}

// Faults 引导第三层：某系统/型号下的故障现象入口。second 为空 = 该品牌的根列表。
func (s *DiagnosisCatalogService) Faults(ctx context.Context, brand, second string) (DiagnosisCatalogItems, error) {
	var out DiagnosisCatalogItems
	q := url.Values{"brand": {brand}}
	if second != "" {
		q.Set("second", second)
	}
	if err := s.getJSON(ctx, "/api/v2/faults", q, &out); err != nil {
		return DiagnosisCatalogItems{}, err
	}
	out.Items = normalizeCatalogItems(out.Items)
	return out, nil
}

// CodeHint 故障码实时体检。q 为空 = 用户还没输入，上游回 status=none。
func (s *DiagnosisCatalogService) CodeHint(ctx context.Context, q string) (DiagnosisCodeHint, error) {
	var out DiagnosisCodeHint
	values := url.Values{}
	if q != "" {
		values.Set("q", q)
	}
	if err := s.getJSON(ctx, "/api/v2/code_hint", values, &out); err != nil {
		return DiagnosisCodeHint{}, err
	}
	if out.Samples == nil {
		out.Samples = []string{}
	}
	return out, nil
}

// Health 上游可达性。**上游的 /api/health 无鉴权**，故这里不发 X-API-Key，
// 也允许在 apiKey 为空时调用（Key 还没签发时仍能判断服务端起没起）。
//
// 永不返回 error：健康检查的语义是「回答一个问题」，不是「抛一个异常」——
// 连不上就是 ok=false，让前端拿到结构化结果而不是一句 errmsg。
func (s *DiagnosisCatalogService) Health(ctx context.Context) DiagnosisHealth {
	if s.baseURL == "" {
		return DiagnosisHealth{OK: false, AppConfigured: false}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/api/health", nil)
	if err != nil {
		return DiagnosisHealth{}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("诊断服务健康检查失败", zap.Error(err))
		}
		return DiagnosisHealth{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if s.logger != nil {
			s.logger.Warn("诊断服务健康检查返回非 200", zap.Int("status", resp.StatusCode))
		}
		return DiagnosisHealth{}
	}
	// 只解两个字段：其余 18 个是对方内部拓扑，见 DiagnosisHealth 的注释。
	var upstream struct {
		OK            bool `json:"ok"`
		AppConfigured bool `json:"app_configured"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&upstream); err != nil {
		if s.logger != nil {
			s.logger.Warn("解析诊断服务健康检查响应失败", zap.Error(err))
		}
		return DiagnosisHealth{}
	}
	return DiagnosisHealth{OK: upstream.OK, AppConfigured: upstream.AppConfigured}
}

// normalizeCatalogItems 集合恒非 null 的归一：上游回 null / 缺 items 时给空集。
func normalizeCatalogItems(items []DiagnosisCatalogItem) []DiagnosisCatalogItem {
	if items == nil {
		return []DiagnosisCatalogItem{}
	}
	return items
}
