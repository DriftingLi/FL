package security

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	plain := "sk-test-abcdef1234567890"
	enc, err := EncryptSecret(plain, "master-key")
	if err != nil {
		t.Fatalf("EncryptSecret 失败: %v", err)
	}
	if !strings.HasPrefix(enc, EncryptedPrefix) {
		t.Fatalf("加密结果缺少前缀: %q", enc)
	}
	if strings.Contains(enc, plain) {
		t.Fatalf("密文泄露明文内容: %q", enc)
	}
	dec, err := DecryptSecret(enc, "master-key")
	if err != nil {
		t.Fatalf("DecryptSecret 失败: %v", err)
	}
	if dec != plain {
		t.Fatalf("解密结果不一致: got %q want %q", dec, plain)
	}
}

func TestEncryptSecret_SamePlaintextDifferentCiphertext(t *testing.T) {
	a, _ := EncryptSecret("secret", "k")
	b, _ := EncryptSecret("secret", "k")
	if a == b {
		t.Fatalf("随机 nonce 应使两次加密结果不同")
	}
}

func TestDecryptSecret_LegacyPlaintext(t *testing.T) {
	// 未加密的历史数据应原样返回
	dec, err := DecryptSecret("sk-legacy-plain", "master-key")
	if err != nil || dec != "sk-legacy-plain" {
		t.Fatalf("历史明文应原样返回: got %q err %v", dec, err)
	}
}

func TestDecryptSecret_WrongKey(t *testing.T) {
	enc, err := EncryptSecret("secret", "key-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptSecret(enc, "key-b"); err == nil {
		t.Fatalf("错误密钥应解密失败")
	}
}

func TestEncryptSecret_EmptyMasterKey(t *testing.T) {
	enc, err := EncryptSecret("secret", "")
	if err != nil || enc != "secret" {
		t.Fatalf("空密钥应原样返回: got %q err %v", enc, err)
	}
}
