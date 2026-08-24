package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forklift-training/internal/config"
)

// newTestSMSProvider 构造指向测试服务器的 provider（httptest 覆盖 endpoint）。
func newTestSMSProvider(t *testing.T, handler http.HandlerFunc) *TencentSMSProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &TencentSMSProvider{
		cfg: config.SMSConfig{
			SecretID:     "AKID-test",
			SecretKey:    "secret-test",
			SdkAppID:     "1400006666",
			SignName:     "和润天下广州人工智能",
			Region:       "ap-guangzhou",
			TplRegister:  "2711711",
			TplLogin:     "2711706",
			TplPassword:  "2711713",
			TplBindPhone: "2711716",
		},
		httpClient: srv.Client(),
		endpoint:   srv.URL,
	}
}

// TestTencentSMSProvider_Send 校验 SendSms 按用途选择模板与参数：
// 登录模板双参数（验证码+分钟数），其余用途单参数（验证码）。
func TestTencentSMSProvider_Send(t *testing.T) {
	cases := []struct {
		name       string
		purpose    CodePurpose
		wantTpl    string
		wantParams []string
	}{
		{"register", CodePurposeRegister, "2711711", []string{"123456"}},
		{"login", CodePurposeLogin, "2711706", []string{"123456", "5"}},
		{"reset_password", CodePurposeResetPassword, "2711713", []string{"123456"}},
		{"change_password", CodePurposeChangePassword, "2711713", []string{"123456"}},
		{"bind", CodePurposeBind, "2711716", []string{"123456"}},
		{"account_change", CodePurposeAccountChange, "2711716", []string{"123456"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotAction string
			var gotBody map[string]any
			p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
				gotAction = r.Header.Get("X-TC-Action")
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"Response":{"SendStatusSet":[{"Code":"Ok","Message":"send success","PhoneNumber":"+8613800000000"}],"RequestId":"x"}}`))
			})

			if err := p.Send("13800000000", "123456", 5, tc.purpose); err != nil {
				t.Fatalf("Send 应成功: %v", err)
			}
			if gotAction != "SendSms" {
				t.Errorf("Action = %q, 期望 SendSms", gotAction)
			}
			if gotBody["SmsSdkAppId"] != "1400006666" {
				t.Errorf("SmsSdkAppId = %v", gotBody["SmsSdkAppId"])
			}
			if gotBody["TemplateId"] != tc.wantTpl {
				t.Errorf("TemplateId = %v, 期望 %v", gotBody["TemplateId"], tc.wantTpl)
			}
			phones, _ := gotBody["PhoneNumberSet"].([]any)
			if len(phones) != 1 || phones[0] != "+8613800000000" {
				t.Errorf("PhoneNumberSet = %v", gotBody["PhoneNumberSet"])
			}
			params, _ := gotBody["TemplateParamSet"].([]any)
			if len(params) != len(tc.wantParams) {
				t.Fatalf("TemplateParamSet = %v, 期望 %v", params, tc.wantParams)
			}
			for i, want := range tc.wantParams {
				if params[i] != want {
					t.Fatalf("TemplateParamSet = %v, 期望 %v", params, tc.wantParams)
				}
			}
		})
	}
}

// TestTencentSMSProvider_SendFailure 校验非 Ok 状态返回错误。
func TestTencentSMSProvider_SendFailure(t *testing.T) {
	p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Response":{"SendStatusSet":[{"Code":"FailedOperation.TemplateIncorrectOrUnapproved","Message":"模板未审核","PhoneNumber":"+8613800000000"}],"RequestId":"x"}}`))
	})
	err := p.Send("13800000000", "123456", 5, CodePurposeLogin)
	if err == nil || !strings.Contains(err.Error(), "模板未审核") {
		t.Fatalf("Send 应返回模板错误, got %v", err)
	}
}

// TestTencentSMSProvider_ValidateReady 校验签名与全部用途模板均审核通过。
func TestTencentSMSProvider_ValidateReady(t *testing.T) {
	p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-TC-Action") {
		case "DescribeSmsSignList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeSignListStatusSet":[{"SignName":"和润天下广州人工智能","StatusCode":0}],"RequestId":"x"}}`))
		case "DescribeSmsTemplateList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeTemplateStatusSet":[
				{"TemplateId":2711711,"TemplateName":"注册验证码","StatusCode":0},
				{"TemplateId":2711706,"TemplateName":"登录验证码","StatusCode":0},
				{"TemplateId":2711713,"TemplateName":"密码重置验证码","StatusCode":0},
				{"TemplateId":2711716,"TemplateName":"修改注册手机号码验证码","StatusCode":0}
			],"RequestId":"x"}}`))
		default:
			t.Errorf("unexpected action %q", r.Header.Get("X-TC-Action"))
		}
	})
	if err := p.ValidateReady(context.Background()); err != nil {
		t.Fatalf("ValidateReady 应通过: %v", err)
	}
}

// TestTencentSMSProvider_ValidateReady_Unapproved 模板未审核通过时返回错误。
func TestTencentSMSProvider_ValidateReady_Unapproved(t *testing.T) {
	p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-TC-Action") {
		case "DescribeSmsSignList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeSignListStatusSet":[{"SignName":"和润天下广州人工智能","StatusCode":0}],"RequestId":"x"}}`))
		case "DescribeSmsTemplateList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeTemplateStatusSet":[
				{"TemplateId":2711711,"TemplateName":"注册验证码","StatusCode":0},
				{"TemplateId":2711706,"TemplateName":"登录验证码","StatusCode":0},
				{"TemplateId":2711713,"TemplateName":"密码重置验证码","StatusCode":2,"ReviewReply":"内容违规"},
				{"TemplateId":2711716,"TemplateName":"修改注册手机号码验证码","StatusCode":0}
			],"RequestId":"x"}}`))
		}
	})
	err := p.ValidateReady(context.Background())
	if err == nil || !strings.Contains(err.Error(), "未审核通过") {
		t.Fatalf("应返回模板未审核错误, got %v", err)
	}
}

// TestTencentSMSProvider_ValidateReady_MissingTemplate 模板缺失时返回错误。
func TestTencentSMSProvider_ValidateReady_MissingTemplate(t *testing.T) {
	p := newTestSMSProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-TC-Action") {
		case "DescribeSmsSignList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeSignListStatusSet":[{"SignName":"和润天下广州人工智能","StatusCode":0}],"RequestId":"x"}}`))
		case "DescribeSmsTemplateList":
			_, _ = w.Write([]byte(`{"Response":{"DescribeTemplateStatusSet":[
				{"TemplateId":2711711,"TemplateName":"注册验证码","StatusCode":0},
				{"TemplateId":2711706,"TemplateName":"登录验证码","StatusCode":0},
				{"TemplateId":2711713,"TemplateName":"密码重置验证码","StatusCode":0}
			],"RequestId":"x"}}`))
		}
	})
	err := p.ValidateReady(context.Background())
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("应返回模板不存在错误, got %v", err)
	}
}

// TestPhoneE164 校验手机号转 E.164。
func TestPhoneE164(t *testing.T) {
	cases := map[string]string{
		"13800000000":    "+8613800000000",
		"+8613800000000": "+8613800000000",
		" 13800000000 ":  "+8613800000000",
	}
	for in, want := range cases {
		if got := phoneE164(in); got != want {
			t.Errorf("phoneE164(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
