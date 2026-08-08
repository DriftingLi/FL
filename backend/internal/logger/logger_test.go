package logger

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]zapcore.Level{
		"debug": zapcore.DebugLevel,
		"info":  zapcore.InfoLevel,
		"warn":  zapcore.WarnLevel,
		"error": zapcore.ErrorLevel,
		"INFO":  zapcore.InfoLevel,
	}
	for in, want := range cases {
		got, err := parseLevel(in)
		if err != nil {
			t.Fatalf("parseLevel(%q) 不应报错: %v", in, err)
		}
		if got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := parseLevel("verbose"); err == nil {
		t.Error("未知级别应报错")
	}
}

func TestNew_Stdout(t *testing.T) {
	z, err := New(Config{Level: "info", Format: "console"})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	defer func() { _ = z.Sync() }()
	z.Info("hello")
}

func TestNew_JSONEncoder(t *testing.T) {
	enc, err := buildEncoder("json")
	if err != nil {
		t.Fatalf("buildEncoder(json) 失败: %v", err)
	}
	buf := &testBuffer{}
	core := zapcore.NewCore(enc, zapcore.AddSync(buf), zapcore.InfoLevel)
	z := zap.New(core)
	z.Info("structured", zap.String("k", "v"))
	_ = z.Sync()
	if len(buf.data) == 0 || buf.data[0][0] != '{' {
		t.Errorf("json 编码器应输出 JSON, got %q", buf.data)
	}
}

func TestNew_UnknownFormat(t *testing.T) {
	if _, err := buildEncoder("yaml"); err == nil {
		t.Error("未知格式应报错")
	}
}

func TestNew_FileOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	z, err := New(Config{Level: "info", Format: "console", OutputDir: dir})
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	z.Info("file log line")
	_ = z.Sync()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("日志文件应被创建: %v", err)
	}
	if len(data) == 0 {
		t.Error("日志文件应非空")
	}
}

func TestNew_UnknownLevel_Error(t *testing.T) {
	if _, err := New(Config{Level: "verbose"}); err == nil {
		t.Error("未知级别应报错")
	}
}

type testBuffer struct {
	data [][]byte
}

func (b *testBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, append([]byte(nil), p...))
	return len(p), nil
}
