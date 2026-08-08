package logger

import (
	"errors"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{"password", "Password", "api_secret", "jwt_token", "auth_code", "SMTP_PASSWORD", "token", "mobile_phone"}
	for _, k := range sensitive {
		if !isSensitiveKey(k) {
			t.Errorf("%q 应判为敏感", k)
		}
	}
	plain := []string{"user_id", "path", "duration_ms", "message", "level"}
	for _, k := range plain {
		if isSensitiveKey(k) {
			t.Errorf("%q 不应判为敏感", k)
		}
	}
}

func TestRedact(t *testing.T) {
	if got := Redact(zap.String("password", "s3cret")); got.String != "***" {
		t.Errorf("敏感 key 值应打码, got %q", got.String)
	}
	if got := Redact(zap.Int("user_id", 7)); got.Integer != 7 {
		t.Errorf("非敏感 key 原样, got %v", got.Integer)
	}
}

func TestRedactError(t *testing.T) {
	err := errors.New(`连接失败: postgres://user:topsecret@db:5432/x?sslmode=disable`)
	got := RedactError(err)
	if got == err.Error() {
		t.Errorf("错误串应脱敏, got %q", got)
	}
	if contains(got, "topsecret") {
		t.Errorf("密码不应残留, got %q", got)
	}
	plain := errors.New("普通错误")
	if RedactError(plain) != plain.Error() {
		t.Error("无凭证错误应原样")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRedactHook_AppliedByFactory(t *testing.T) {
	dir := t.TempDir()
	z, err := New(Config{Level: "info", Format: "json", OutputDir: dir})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	z.Warn("credential leak",
		zap.String("token", "abc123"),
		zap.Error(errors.New("连接失败: postgres://user:topsecret@db:5432/x")),
	)
	_ = z.Sync()

	data, err := os.ReadFile(dir + "/app.log")
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	out := string(data)
	if contains(out, "abc123") {
		t.Errorf("token 值应被打码: %s", out)
	}
	if contains(out, "topsecret") {
		t.Errorf("错误串内嵌密码应被脱敏: %s", out)
	}
	if !contains(out, "***") {
		t.Errorf("应出现打码占位: %s", out)
	}
}
