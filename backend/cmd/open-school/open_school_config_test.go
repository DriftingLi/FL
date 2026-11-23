package main

import (
	"strings"
	"testing"
)

// TestConfig 配置只走环境变量、端口只绑回环。
func TestConfig(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}

	cfg, err := envConfig(env(nil))
	if err != nil {
		t.Fatalf("默认配置装配失败：%v", err)
	}
	if cfg.Listen != "127.0.0.1:18090" || cfg.APIBase != "http://127.0.0.1:18080" || cfg.LearnBase != "http://training.localhost:18091" {
		t.Errorf("默认值不符：%+v", cfg)
	}

	cfg, err = envConfig(env(map[string]string{
		"OPEN_SCHOOL_LISTEN":     "127.0.0.1:19090/",
		"OPEN_SCHOOL_API":        "http://127.0.0.1:9999/",
		"OPEN_SCHOOL_LEARN_BASE": "https://learn.example.com/",
	}))
	if err == nil {
		// 监听地址带斜杠应当报错
		t.Fatalf("非法监听地址未报错：%+v", cfg)
	}

	cfg, err = envConfig(env(map[string]string{
		"OPEN_SCHOOL_API":        "http://127.0.0.1:9999/",
		"OPEN_SCHOOL_LEARN_BASE": "https://learn.example.com/",
	}))
	if err != nil {
		t.Fatalf("覆盖配置装配失败：%v", err)
	}
	// 结尾斜杠被规范化掉：拼 /api/... 时不会出现双斜杠
	if cfg.APIBase != "http://127.0.0.1:9999" || cfg.LearnBase != "https://learn.example.com" {
		t.Errorf("基址规范化失败：%+v", cfg)
	}

	// 非回环监听默认一律拒绝（公开面不允许进程默认直接暴露全部网卡）
	for _, listen := range []string{"0.0.0.0:18090", ":18090", "192.168.1.5:18090", "example.com:18090", "[::]:18090"} {
		_, err := envConfig(env(map[string]string{"OPEN_SCHOOL_LISTEN": listen}))
		if err == nil {
			t.Errorf("监听地址 %q 未被默认拒绝", listen)
			continue
		}
		// 拒绝文案必须告诉人怎么开：容器里经网关接入是正常部署形状。
		if !strings.Contains(err.Error(), "OPEN_SCHOOL_ALLOW_NON_LOOPBACK") {
			t.Errorf("监听地址 %q 的拒绝文案没提到开关：%v", listen, err)
		}
	}
	for _, listen := range []string{"localhost:18090", "[::1]:18090"} {
		if _, err := envConfig(env(map[string]string{"OPEN_SCHOOL_LISTEN": listen})); err != nil {
			t.Errorf("回环地址 %q 被误拒：%v", listen, err)
		}
	}

	// 显式开关置真后允许绑非回环（容器内网关反代的形状，见 #2753 节点 E）。
	allow := map[string]string{"OPEN_SCHOOL_ALLOW_NON_LOOPBACK": "1"}
	for _, listen := range []string{"0.0.0.0:8080", ":8080", "[::]:8080", "192.168.1.5:18090"} {
		values := map[string]string{"OPEN_SCHOOL_LISTEN": listen, "OPEN_SCHOOL_ALLOW_NON_LOOPBACK": allow["OPEN_SCHOOL_ALLOW_NON_LOOPBACK"]}
		if _, err := envConfig(env(values)); err != nil {
			t.Errorf("开关置真时 %q 仍被拒：%v", listen, err)
		}
	}
	// 开关置真不改变回环地址的可用性
	if _, err := envConfig(env(map[string]string{
		"OPEN_SCHOOL_LISTEN": "127.0.0.1:18090", "OPEN_SCHOOL_ALLOW_NON_LOOPBACK": "true",
	})); err != nil {
		t.Errorf("开关置真时回环地址被拒：%v", err)
	}
	// 只有 1/true 算开：别的取值一律当关，仍然拒绝非回环
	for _, raw := range []string{"0", "false", "", "yes", "on", "t"} {
		_, err := envConfig(env(map[string]string{
			"OPEN_SCHOOL_LISTEN": "0.0.0.0:8080", "OPEN_SCHOOL_ALLOW_NON_LOOPBACK": raw,
		}))
		if err == nil {
			t.Errorf("OPEN_SCHOOL_ALLOW_NON_LOOPBACK=%q 被当成开启", raw)
		}
	}

	// 非 http/https 基址拒绝（否则进 href 就是脚本执行入口）
	for _, bad := range []string{"javascript:alert(1)", "ftp://x/y", "/relative"} {
		if _, err := envConfig(env(map[string]string{"OPEN_SCHOOL_API": bad})); err == nil {
			t.Errorf("基址 %q 未被拒绝", bad)
		}
	}
}
