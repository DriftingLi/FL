// Package config 微信三组凭证的键位测试：键名各归各的，且移动应用那组不得回退到 legacy 别名。
// 这条锁的是 config.go 的 wechatAppConfig 调用点——不是结构体字段本身（自建字面量断言自己
// 造的值是同义反复，测不到任何会被改坏的东西）。
package config

import (
	"testing"
)

func TestWechatAppConfigKeyAssignment(t *testing.T) {
	t.Setenv("WECHAT_MOBILE_APP_ID", "mob-id")
	t.Setenv("WECHAT_MOBILE_APP_SECRET", "mob-secret")
	t.Setenv("WECHAT_OPEN_PLATFORM_APP_ID", "web-id")
	t.Setenv("WECHAT_OPEN_PLATFORM_APP_SECRET", "web-secret")
	t.Setenv("WECHAT_APP_ID", "legacy-mp-id")
	t.Setenv("WECHAT_APP_SECRET", "legacy-mp-secret")

	mobile := wechatAppConfig("wechat_mobile_app_id", "wechat_mobile_app_secret")
	if mobile.AppID != "mob-id" || mobile.AppSecret != "mob-secret" {
		t.Fatalf("移动应用凭证应读 WECHAT_MOBILE_*: %+v", mobile)
	}
	web := wechatAppConfig("wechat_open_platform_app_id", "wechat_open_platform_app_secret")
	if web.AppID != "web-id" || web.AppSecret != "web-secret" {
		t.Fatalf("网页扫码凭证应读 WECHAT_OPEN_PLATFORM_*: %+v", web)
	}
	// 小程序那侧仍接 legacy 回退（这是既有行为，本测试负责在它被删掉时判红）。
	mp := wechatAppConfig("wechat_mini_program_app_id", "wechat_mini_program_app_secret", "wechat_app_id", "wechat_app_secret")
	if mp.AppID != "legacy-mp-id" || mp.AppSecret != "legacy-mp-secret" {
		t.Fatalf("小程序凭证应回退到 legacy WECHAT_APP_*: %+v", mp)
	}
}

// TestWechatMobileNeverFallsBackToLegacyPair 是关键的那一支：
// legacy WECHAT_APP_ID/SECRET 的历史语义是**小程序**凭证（config.go 的注释自述），
// 一旦有人把这对也接给移动应用当回退，App 端就会拿小程序凭证去调 /sns/oauth2/access_token——
// 那是票面与 CONTEXT.md 判死的混用，而且它只会在真机登录时报一个看不懂的错。
func TestWechatMobileNeverFallsBackToLegacyPair(t *testing.T) {
	t.Setenv("WECHAT_MOBILE_APP_ID", "")
	t.Setenv("WECHAT_MOBILE_APP_SECRET", "")
	t.Setenv("WECHAT_APP_ID", "legacy-mp-id")
	t.Setenv("WECHAT_APP_SECRET", "legacy-mp-secret")

	mobile := wechatAppConfig("wechat_mobile_app_id", "wechat_mobile_app_secret")
	if mobile.Configured() {
		t.Fatalf("移动应用凭证未配置却从 legacy 别名取到了值: %+v", mobile)
	}
}
