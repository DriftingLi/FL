package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/config"
)

// 本文件：验证码用途声明表的三条锁。
//   1. 结构完整性——表覆盖全部用途且槽位可判定地填满（新增用途漏填即报红）
//   2. 行为级逐字——真跑邮件渲染与短信模板选择，断言产出与声明表一致（测真调用点，不做镜像）
//   3. 单向不变式——匿名发码白名单 ⊆ 非 RequiresSession 用途（在 api 包，见 anonymous_send_purpose_test.go）

// allCodePurposes 用途值域的权威清单（六态）。
// 值域本身不在本票范围内，此处只作为「表要覆盖到什么程度」的基准。
var allCodePurposes = []CodePurpose{
	CodePurposeRegister,
	CodePurposeLogin,
	CodePurposeBind,
	CodePurposeAccountChange,
	CodePurposeResetPassword,
	CodePurposeChangePassword,
}

// goldenPurposeRow 用途声明行的黄金副本：现网六种用途的对外文案、短信模板与占用口径。
//
// 它不是「测试里的第二份实现」——它 pin 的是**用户可见字符串与模板键**，
// 改动表值必须先改这里，于是「文案/模板悄悄变了」会变成一次显式 diff。
type goldenPurposeRow struct {
	purpose         CodePurpose
	requiresSession bool
	targetRule      codeTargetRule
	targetRuleMsg   string
	emailTitle      string
	emailOp         string
	smsTemplate     config.SMSTemplateKey
	smsParamCount   int
}

var goldenPurposeRows = []goldenPurposeRow{
	{CodePurposeRegister, false, codeTargetFree, "该%s已注册，请直接登录",
		"【和润天下】注册验证码", "注册", config.SMSTemplateKeyRegister, 1},
	{CodePurposeLogin, false, codeTargetExist, "该%s尚未注册",
		"【和润天下】登录验证码", "登录", config.SMSTemplateKeyLogin, 2},
	{CodePurposeBind, true, codeTargetFree, "该%s已被其他账号使用",
		"【和润天下】邮箱绑定验证码", "绑定/修改邮箱", config.SMSTemplateKeyBindPhone, 1},
	{CodePurposeAccountChange, true, codeTargetNone, "",
		"【和润天下】修改登录账号验证码", "修改登录账号", config.SMSTemplateKeyBindPhone, 1},
	{CodePurposeResetPassword, false, codeTargetExist, "该%s尚未注册",
		"【和润天下】找回密码验证码", "找回密码", config.SMSTemplateKeyPassword, 1},
	{CodePurposeChangePassword, true, codeTargetNone, "",
		"【和润天下】修改密码验证码", "修改密码", config.SMSTemplateKeyPassword, 1},
}

// fullSMSConfig 四个模板都配好的短信配置（把「模板键」解析成模板 ID 用）。
func fullSMSConfig() config.SMSConfig {
	return config.SMSConfig{
		SecretID:     "AKID-test",
		SecretKey:    "secret-test",
		SdkAppID:     "1400006666",
		SignName:     "和润天下广州人工智能",
		Region:       "ap-guangzhou",
		TplRegister:  "2711711",
		TplLogin:     "2711706",
		TplPassword:  "2711713",
		TplBindPhone: "2711716",
	}
}

// TestCodePurposeTable_Complete 结构完整性：表覆盖全部六种用途，且每行 7 个槽位可判定地填满。
// 「占用错误文案」对 codeTargetNone 的行必须为空——不允许留死数据（这条比「非空」更严）。
func TestCodePurposeTable_Complete(t *testing.T) {
	if len(codePurposeTable) != len(allCodePurposes) {
		t.Fatalf("用途表行数 = %d, 期望 %d（用途值域六态）", len(codePurposeTable), len(allCodePurposes))
	}

	declared := make(map[CodePurpose]bool, len(allCodePurposes))
	for _, p := range allCodePurposes {
		declared[p] = true
	}
	cfg := fullSMSConfig()
	seen := make(map[CodePurpose]bool, len(codePurposeTable))

	for _, e := range codePurposeTable {
		t.Run(string(e.Purpose), func(t *testing.T) {
			if !declared[e.Purpose] {
				t.Fatalf("用途 %q 不在用途值域内", e.Purpose)
			}
			if seen[e.Purpose] {
				t.Fatalf("用途 %q 在表里出现两次", e.Purpose)
			}
			seen[e.Purpose] = true

			switch e.Spec.TargetRule {
			case codeTargetFree, codeTargetExist:
				if strings.Count(e.Spec.TargetRuleMsg, "%s") != 1 {
					t.Errorf("占用错误文案 %q 必须恰好含一个 %%s（通道目标名占位）", e.Spec.TargetRuleMsg)
				}
			case codeTargetNone:
				if e.Spec.TargetRuleMsg != "" {
					t.Errorf("无需占用校验的用途不应带占用错误文案（死数据），得到 %q", e.Spec.TargetRuleMsg)
				}
			default:
				t.Errorf("目标占用校验规则 %q 不是已知取值", e.Spec.TargetRule)
			}

			if e.Spec.EmailTitle == "" {
				t.Error("邮件标题槽位为空")
			}
			if e.Spec.EmailOp == "" {
				t.Error("邮件操作词槽位为空")
			}
			if cfg.Template(e.Spec.SMSTemplate) == "" {
				t.Errorf("短信模板键 %q 不是已配置的模板", e.Spec.SMSTemplate)
			}
			if e.Spec.SMSParamCount < 1 {
				t.Errorf("短信参数量 = %d, 必须 ≥ 1", e.Spec.SMSParamCount)
			}
		})
	}

	if len(seen) != len(allCodePurposes) {
		t.Fatalf("表只覆盖了 %d 种用途, 期望 %d", len(seen), len(allCodePurposes))
	}
}

// TestCodePurposeTable_MatchesGoldenCopy 声明表与黄金副本逐字一致（文案/模板键/占用口径的漂移锁）。
func TestCodePurposeTable_MatchesGoldenCopy(t *testing.T) {
	for _, want := range goldenPurposeRows {
		t.Run(string(want.purpose), func(t *testing.T) {
			got, ok := codePurposeSpecFor(want.purpose)
			if !ok {
				t.Fatalf("用途 %q 未在声明表中", want.purpose)
			}
			if got.RequiresSession != want.requiresSession {
				t.Errorf("RequiresSession = %v, 期望 %v", got.RequiresSession, want.requiresSession)
			}
			if got.TargetRule != want.targetRule {
				t.Errorf("TargetRule = %q, 期望 %q", got.TargetRule, want.targetRule)
			}
			if got.TargetRuleMsg != want.targetRuleMsg {
				t.Errorf("TargetRuleMsg = %q, 期望 %q", got.TargetRuleMsg, want.targetRuleMsg)
			}
			if got.EmailTitle != want.emailTitle {
				t.Errorf("EmailTitle = %q, 期望 %q", got.EmailTitle, want.emailTitle)
			}
			if got.EmailOp != want.emailOp {
				t.Errorf("EmailOp = %q, 期望 %q", got.EmailOp, want.emailOp)
			}
			if got.SMSTemplate != want.smsTemplate {
				t.Errorf("SMSTemplate = %q, 期望 %q", got.SMSTemplate, want.smsTemplate)
			}
			if got.SMSParamCount != want.smsParamCount {
				t.Errorf("SMSParamCount = %d, 期望 %d", got.SMSParamCount, want.smsParamCount)
			}
			if got := CodePurposeRequiresSession(want.purpose); got != want.requiresSession {
				t.Errorf("CodePurposeRequiresSession = %v, 期望 %v", got, want.requiresSession)
			}
		})
	}
}

// TestEmailChannel_RenderReadsPurposeTable 行为级：真跑邮件渲染，断言产出的主题与操作词与表一致。
func TestEmailChannel_RenderReadsPurposeTable(t *testing.T) {
	ch := &EmailChannel{} // Render 不触碰 mailer
	const code, ttl = "123456", 5 * time.Minute

	for _, want := range goldenPurposeRows {
		t.Run(string(want.purpose), func(t *testing.T) {
			title, body := ch.Render(want.purpose, code, ttl)
			if title != want.emailTitle {
				t.Errorf("邮件主题 = %q, 期望 %q", title, want.emailTitle)
			}
			if !strings.Contains(body, "您正在进行"+want.emailOp+"操作") {
				t.Errorf("邮件正文缺少操作词 %q: %s", want.emailOp, body)
			}
			if !strings.Contains(body, code) {
				t.Errorf("邮件正文缺少验证码: %s", body)
			}
		})
	}

	// 正文骨架逐字不变（整段钉住，避免只剩「包含」断言时被悄悄改掉上下文）
	_, body := ch.Render(CodePurposeRegister, code, ttl)
	const wantBody = "您好！\n\n您正在进行注册操作，本次验证码为：123456\n验证码 5 分钟内有效，请勿泄露给他人。\n\n如非本人操作，请忽略本邮件。"
	if body != wantBody {
		t.Errorf("注册邮件正文 = %q\n期望 %q", body, wantBody)
	}
}

// TestTencentSMSProvider_TemplateForReadsPurposeTable 行为级：真跑模板选择，断言模板 ID 与参数量与表一致。
func TestTencentSMSProvider_TemplateForReadsPurposeTable(t *testing.T) {
	cfg := fullSMSConfig()
	p := &TencentSMSProvider{cfg: cfg}

	for _, want := range goldenPurposeRows {
		t.Run(string(want.purpose), func(t *testing.T) {
			tplID, params := p.templateFor(want.purpose, "123456", 5)
			if wantID := cfg.Template(want.smsTemplate); tplID != wantID {
				t.Errorf("模板 ID = %q, 期望 %q（模板键 %q）", tplID, wantID, want.smsTemplate)
			}
			if len(params) != want.smsParamCount {
				t.Fatalf("模板参数 = %v, 期望 %d 个", params, want.smsParamCount)
			}
			if params[0] != "123456" {
				t.Errorf("模板参数[0] = %q, 期望验证码", params[0])
			}
			if want.smsParamCount > 1 && params[1] != "5" {
				t.Errorf("模板参数[1] = %q, 期望分钟数 5", params[1])
			}
		})
	}
}

// TestTencentSMSProvider_ValidateReady_TemplateSetFromTable 启动自检的模板集合由用途表派生。
func TestTencentSMSProvider_ValidateReady_TemplateSetFromTable(t *testing.T) {
	var gotTemplateIDs []int64
	p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-TC-Action") {
		case "DescribeSmsSignList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeSignListStatusSet":[{"SignName":"和润天下广州人工智能","StatusCode":0}],"RequestId":"x"}}`))
		case "DescribeSmsTemplateList":
			var body struct {
				TemplateIdSet []int64 `json:"TemplateIdSet"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotTemplateIDs = body.TemplateIdSet
			// 回显本请求问到的全部模板（都审核通过），测试里不硬编码模板数
			set := make([]string, 0, len(body.TemplateIdSet))
			for _, id := range body.TemplateIdSet {
				set = append(set, `{"TemplateId":`+strconv.FormatInt(id, 10)+`,"TemplateName":"t","StatusCode":0}`)
			}
			_, _ = w.Write([]byte(`{"Response":{"DescribeTemplateStatusSet":[` + strings.Join(set, ",") + `],"RequestId":"x"}}`))
		default:
			t.Errorf("unexpected action %q", r.Header.Get("X-TC-Action"))
		}
	})

	if err := p.ValidateReady(context.Background()); err != nil {
		t.Fatalf("ValidateReady 应通过: %v", err)
	}

	want := make([]int64, 0, len(CodePurposeSMSTemplates()))
	for _, key := range CodePurposeSMSTemplates() {
		id, err := strconv.ParseInt(p.cfg.Template(key), 10, 64)
		if err != nil {
			t.Fatalf("模板键 %q 的模板 ID 非法: %v", key, err)
		}
		want = append(want, id)
	}
	if len(gotTemplateIDs) != len(want) {
		t.Fatalf("自检查询的模板数 = %d, 期望 %d（用途表派生的模板键数）", len(gotTemplateIDs), len(want))
	}
	for i := range want {
		if gotTemplateIDs[i] != want[i] {
			t.Fatalf("自检查询的模板集合 = %v, 期望 %v", gotTemplateIDs, want)
		}
	}
}

// TestSMSConfig_ConfiguredRequiresTableTemplates 配置就绪断言的模板集合同样由用途表派生。
func TestSMSConfig_ConfiguredRequiresTableTemplates(t *testing.T) {
	keys := CodePurposeSMSTemplates()
	if len(keys) == 0 {
		t.Fatal("用途表未派生任何短信模板键")
	}

	if !fullSMSConfig().Configured(keys...) {
		t.Fatal("四个模板齐备时应判为已配置")
	}

	// 逐个清空模板 ID：任一模板缺失都必须判为未配置（fail-closed）
	for _, key := range keys {
		cfg := fullSMSConfig()
		switch key {
		case config.SMSTemplateKeyRegister:
			cfg.TplRegister = ""
		case config.SMSTemplateKeyLogin:
			cfg.TplLogin = ""
		case config.SMSTemplateKeyPassword:
			cfg.TplPassword = ""
		case config.SMSTemplateKeyBindPhone:
			cfg.TplBindPhone = ""
		default:
			t.Fatalf("未知模板键 %q", key)
		}
		if cfg.Configured(keys...) {
			t.Errorf("模板键 %q 缺失时应判为未配置", key)
		}
	}

	// 凭证缺失同样判为未配置
	noCred := fullSMSConfig()
	noCred.SecretID = ""
	if noCred.Configured(keys...) {
		t.Error("凭证缺失时应判为未配置")
	}
}
