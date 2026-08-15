package captcha

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"time"
)

// memStore 内存答案存储（测试替身）。
type memStore struct {
	m map[string]string
}

func (s *memStore) Get(_ context.Context, key string) (string, error) {
	v, ok := s.m[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (s *memStore) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.m[key] = value
	return nil
}

func (s *memStore) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(s.m, k)
	}
	return nil
}

// TestNewEquation 算式与答案自洽（500 次采样）。
func TestNewEquation(t *testing.T) {
	for i := 0; i < 500; i++ {
		eq := NewEquation()
		if eq.Answer < 0 {
			t.Fatalf("答案不能为负: %+v", eq)
		}
		got := parseEquation(t, eq.Text)
		if got != eq.Answer {
			t.Fatalf("算式 %q 答案应为 %d，实际 %d", eq.Text, got, eq.Answer)
		}
		if len([]rune(eq.Text)) < 4 || !strings.HasSuffix(eq.Text, "=?") {
			t.Fatalf("算式格式异常: %q", eq.Text)
		}
	}
}

// parseEquation 解析 "a+b=?" 形式并计算结果。
func parseEquation(t *testing.T, text string) int {
	t.Helper()
	text = strings.TrimSuffix(text, "=?")
	for _, op := range []string{"+", "-", "×", "÷"} {
		if i := strings.Index(text, op); i >= 0 {
			a, err1 := strconv.Atoi(text[:i])
			b, err2 := strconv.Atoi(text[i+len(op):])
			if err1 != nil || err2 != nil {
				t.Fatalf("算式解析失败: %q", text)
			}
			switch op {
			case "+":
				return a + b
			case "-":
				return a - b
			case "×":
				return a * b
			default:
				if b == 0 {
					t.Fatalf("除数不能为 0: %q", text)
				}
				return a / b
			}
		}
	}
	t.Fatalf("未识别运算符: %q", text)
	return 0
}

// TestRenderPNG 渲染结果应为可解码的 PNG。
func TestRenderPNG(t *testing.T) {
	for _, text := range []string{"7+3=?", "12-4=?", "3×4=?", "81÷9=?"} {
		data := RenderPNG(text)
		if len(data) < 100 {
			t.Fatalf("%q 渲染结果过小: %d", text, len(data))
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%q PNG 解码失败: %v", text, err)
		}
		b := img.Bounds()
		if b.Dx() < 100 || b.Dy() < 40 {
			t.Errorf("%q 图片尺寸异常: %dx%d", text, b.Dx(), b.Dy())
		}
	}
}

// TestServiceGenerateVerify 生成→校验→消费 全流程。
func TestServiceGenerateVerify(t *testing.T) {
	store := &memStore{m: map[string]string{}}
	s := NewService(store)
	ctx := context.Background()

	id, img, err := s.Generate(ctx)
	if err != nil || id == "" || !strings.HasPrefix(img, "data:image/png;base64,") {
		t.Fatalf("Generate 异常: id=%q err=%v", id, err)
	}
	answer, ok := store.m[storeKey(id)]
	if !ok || answer == "" {
		t.Fatal("答案未写入存储")
	}

	// 正确答案通过
	if !s.Verify(ctx, id, answer) {
		t.Error("正确答案应通过")
	}
	// 已消费，复用失败
	if s.Verify(ctx, id, answer) {
		t.Error("答案已消费，复用应失败")
	}
}

// TestServiceVerifyConsumedOnWrong 错误答案同样消费。
func TestServiceVerifyConsumedOnWrong(t *testing.T) {
	store := &memStore{m: map[string]string{}}
	s := NewService(store)
	ctx := context.Background()

	id, _, err := s.Generate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer := store.m[storeKey(id)]

	if s.Verify(ctx, id, "99999") {
		t.Error("错误答案不应通过")
	}
	if _, ok := store.m[storeKey(id)]; ok {
		t.Error("错误尝试后答案应已消费")
	}
	if s.Verify(ctx, id, answer) {
		t.Error("消费后用正确答案也不应通过")
	}
	// 空 id 直接拒绝
	if s.Verify(ctx, "", "1") {
		t.Error("空 id 应拒绝")
	}
}
