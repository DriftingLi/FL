package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forklift-training/internal/config"
)

// 腾讯云短信 API 3.0 固定参数（SendSms / DescribeSmsSignList / DescribeSmsTemplateList）。
const (
	smsHost     = "sms.tencentcloudapi.com"
	smsEndpoint = "https://" + smsHost
	smsVersion  = "2021-01-11"
	smsService  = "sms"
)

// TencentSMSProvider 腾讯云短信发送器（API 3.0，TC3-HMAC-SHA256 签名）。
// 不引入腾讯云 SDK，直接以签名 HTTP 请求调用短信 API，仅依赖标准库。
type TencentSMSProvider struct {
	cfg        config.SMSConfig
	httpClient *http.Client
	endpoint   string
}

// NewTencentSMSProvider 构造腾讯云短信发送器。
func NewTencentSMSProvider(cfg config.SMSConfig) *TencentSMSProvider {
	return &TencentSMSProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		endpoint:   smsEndpoint,
	}
}

// Send 发送验证码短信。code 与 minutes 分别填充已审核模板的前两个变量
// （模板形如「您的验证码为 {1}，{2} 分钟内有效」）。
func (p *TencentSMSProvider) Send(to, code string, minutes int) error {
	payload := map[string]any{
		"PhoneNumberSet":   []string{phoneE164(to)},
		"SmsSdkAppId":      p.cfg.SdkAppID,
		"SignName":         p.cfg.SignName,
		"TemplateId":       p.cfg.TemplateID,
		"TemplateParamSet": []string{code, strconv.Itoa(minutes)},
	}
	var resp smsAPIResponse
	if err := p.doAction(context.Background(), "SendSms", payload, &resp); err != nil {
		return err
	}
	if len(resp.Response.SendStatusSet) == 0 {
		return errors.New("短信发送失败：未返回发送状态")
	}
	for _, st := range resp.Response.SendStatusSet {
		if st.Code != "Ok" {
			return fmt.Errorf("短信发送失败：%s（%s）", st.Message, st.Code)
		}
	}
	return nil
}

// ValidateReady 校验签名与模板均已审核通过（启动自检用，失败不阻断发送）。
func (p *TencentSMSProvider) ValidateReady(ctx context.Context) error {
	// 签名审核状态
	var signResp smsAPIResponse
	if err := p.doAction(ctx, "DescribeSmsSignList", map[string]any{"International": 0, "Limit": 100}, &signResp); err != nil {
		return fmt.Errorf("查询短信签名失败: %w", err)
	}
	signOK := false
	for _, s := range signResp.Response.DescribeSignListStatusSet {
		if s.SignName == p.cfg.SignName && s.StatusCode == 0 {
			signOK = true
			break
		}
	}
	if !signOK {
		return fmt.Errorf("短信签名「%s」未审核通过或不存在", p.cfg.SignName)
	}

	// 模板审核状态
	tplID, err := strconv.ParseInt(p.cfg.TemplateID, 10, 64)
	if err != nil {
		return fmt.Errorf("短信模板 ID 非法: %s", p.cfg.TemplateID)
	}
	var tplResp smsAPIResponse
	if err := p.doAction(ctx, "DescribeSmsTemplateList", map[string]any{"International": 0, "TemplateIdSet": []int64{tplID}}, &tplResp); err != nil {
		return fmt.Errorf("查询短信模板失败: %w", err)
	}
	if len(tplResp.Response.DescribeTemplateStatusSet) == 0 {
		return fmt.Errorf("短信模板 ID %s 不存在", p.cfg.TemplateID)
	}
	for _, t := range tplResp.Response.DescribeTemplateStatusSet {
		if t.StatusCode != 0 {
			return fmt.Errorf("短信模板「%s」未审核通过（%s）", t.TemplateName, t.ReviewReply)
		}
	}
	return nil
}

// doAction 发送一个腾讯云短信 API 3.0 请求并解析 Response。
// Host 头来自 endpoint URL，须与签名里的 canonical host（smsHost）一致。
func (p *TencentSMSProvider) doAction(ctx context.Context, action string, payload any, out *smsAPIResponse) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	now := time.Now()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	date := now.UTC().Format("2006-01-02")

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", smsVersion)
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("X-TC-Region", p.cfg.Region)
	req.Header.Set("Authorization", tc3Authorization(p.cfg.SecretID, p.cfg.SecretKey, date, timestamp, string(body)))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析腾讯云短信响应失败: %w", err)
	}
	if e := out.Response.Error; e.Code != "" {
		return fmt.Errorf("腾讯云短信错误 %s: %s", e.Code, e.Message)
	}
	return nil
}

// smsAPIResponse 腾讯云短信 API 3.0 统一响应（仅声明本实现用到的字段）。
type smsAPIResponse struct {
	Response struct {
		SendStatusSet []struct {
			Code        string `json:"Code"`
			Message     string `json:"Message"`
			PhoneNumber string `json:"PhoneNumber"`
		} `json:"SendStatusSet"`
		DescribeSignListStatusSet []struct {
			SignName    string `json:"SignName"`
			StatusCode  int64  `json:"StatusCode"`
			ReviewReply string `json:"ReviewReply"`
		} `json:"DescribeSignListStatusSet"`
		DescribeTemplateStatusSet []struct {
			TemplateID      int64  `json:"TemplateId"`
			TemplateName    string `json:"TemplateName"`
			StatusCode      int64  `json:"StatusCode"`
			ReviewReply     string `json:"ReviewReply"`
			TemplateContent string `json:"TemplateContent"`
		} `json:"DescribeTemplateStatusSet"`
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
		RequestId string `json:"RequestId"`
	} `json:"Response"`
}

// phoneE164 将归一化后的 11 位手机号转为 E.164 格式（+86...）。
func phoneE164(phone string) string {
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	return "+86" + phone
}

// tc3Authorization 生成腾讯云 API 3.0 TC3-HMAC-SHA256 签名的 Authorization 头。
func tc3Authorization(secretID, secretKey, date, timestamp, payload string) string {
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + smsHost + "\n"
	signedHeaders := "content-type;host"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders,
		signedHeaders,
		sha256Hex([]byte(payload)),
	}, "\n")

	credentialScope := date + "/" + smsService + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		timestamp,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, smsService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
}

// sha256Hex 计算 sha256 摘要并返回十六进制小写字符串。
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hmacSHA256 计算 HMAC-SHA256 摘要。
func hmacSHA256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(msg))
	return h.Sum(nil)
}
