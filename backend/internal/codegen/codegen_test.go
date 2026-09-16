package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 本文件：骨架包自身的单测。
// 它是五个生成器共享的——一个 bug 会同时打中五处，所以覆盖定位三条路径与两条失败出口。

// chdir 临时切到 dir，测试结束还原（Go 同包测试默认串行，故安全）。
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("取当前目录失败: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("切换目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// captureStdout 捕获 fn 期间的 stdout。
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	os.Stdout = old
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

const probeNotFound = "未找到目标目录：请在仓库内运行，或用 -out 指定输出路径"

// walkUpLocate 逐级向上找名为 marker 的目录，命中后返回其下的 outName。
func walkUpLocate(marker, outName string) func(string) (string, bool) {
	return func(dir string) (string, bool) {
		cand := filepath.Join(dir, marker)
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return filepath.Join(cand, outName), true
		}
		return "", false
	}
}

// TestResolve_Explicit 显式路径优先，且不校验存在性（与既有 -out 语义一致）。
func TestResolve_Explicit(t *testing.T) {
	want, err := filepath.Abs(filepath.Join("some", "where", "out.ts"))
	if err != nil {
		t.Fatalf("Abs 失败: %v", err)
	}
	got, err := Resolve("some/where/out.ts", func(string) (string, bool) {
		t.Error("显式路径不应触发逐级查找")
		return "", false
	}, probeNotFound)
	if err != nil {
		t.Fatalf("应成功: %v", err)
	}
	if got != want {
		t.Fatalf("Resolve = %q, 期望 %q", got, want)
	}
}

// TestResolve_WalkUp 未指定时逐级向上找目标目录。
func TestResolve_WalkUp(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "frontend", "src", "config")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	deep := filepath.Join(root, "backend", "internal", "authz")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	chdir(t, deep)

	got, err := Resolve("", walkUpLocate(filepath.Join("frontend", "src", "config"), "authz.ts"), probeNotFound)
	if err != nil {
		t.Fatalf("应找到目标目录: %v", err)
	}
	want := filepath.Join(marker, "authz.ts")
	if got != want {
		t.Fatalf("Resolve = %q, 期望 %q", got, want)
	}
}

// TestResolve_NotFound 一路到根都没命中时报错，错误信息逐字沿用（含 -out 提示）。
func TestResolve_NotFound(t *testing.T) {
	// 造一个没有 marker 的目录树：temp 根之上的父目录不保证没有同名目录，
	// 所以用「永不命中」的判据，只验证到根后的报错。
	chdir(t, t.TempDir())
	_, err := Resolve("", func(string) (string, bool) { return "", false }, probeNotFound)
	if err == nil {
		t.Fatal("应报错")
	}
	if err.Error() != probeNotFound {
		t.Fatalf("错误信息 = %q, 期望 %q", err.Error(), probeNotFound)
	}
}

// TestRun_WritesAndPrints 正常路径：写盘 + 逐字沿用「已生成 …（N 字节）」输出。
func TestRun_WritesAndPrints(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.ts")
	spec := Spec{
		Name:     "gen-probe",
		Hint:     "out.ts",
		Locate:   func(string) (string, bool) { return target, true },
		NotFound: probeNotFound,
		Render:   func() (string, error) { return "hello\n", nil },
	}

	var err error
	out := captureStdout(t, func() { err = Run(spec, "") })
	if err != nil {
		t.Fatalf("Run 应成功: %v", err)
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("读取生成物失败: %v", readErr)
	}
	if string(got) != "hello\n" {
		t.Fatalf("生成物内容 = %q", string(got))
	}
	want := "已生成 " + target + "（6 字节）\n"
	if out != want {
		t.Fatalf("stdout = %q, 期望 %q", out, want)
	}
}

// TestRun_RenderError 渲染失败：返回错误，且不落盘。
func TestRun_RenderError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.ts")
	renderErr := errors.New("渲染炸了")
	spec := Spec{
		Name:     "gen-probe",
		Hint:     "out.ts",
		Locate:   func(string) (string, bool) { return target, true },
		NotFound: probeNotFound,
		Render:   func() (string, error) { return "", renderErr },
	}

	err := Run(spec, "")
	if !errors.Is(err, renderErr) {
		t.Fatalf("Run 应返回渲染错误, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("渲染失败时不应落盘")
	}
}

// TestRun_WriteError 写盘失败（目标是已存在的目录）：返回错误。
func TestRun_WriteError(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.ts")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	spec := Spec{
		Name:     "gen-probe",
		Hint:     "out.ts",
		Locate:   func(string) (string, bool) { return target, true },
		NotFound: probeNotFound,
		Render:   func() (string, error) { return "x", nil },
	}

	if err := Run(spec, ""); err == nil {
		t.Fatal("写盘失败应报错")
	}
}

// TestRun_MkdirAll MkdirAll 档：生成的目录不存在时先建出来。
func TestRun_MkdirAll(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep", "env.defaults")

	base := Spec{
		Name:     "gen-probe",
		Hint:     "env.defaults",
		Locate:   func(string) (string, bool) { return target, true },
		NotFound: probeNotFound,
		Render:   func() (string, error) { return "X=1\n", nil },
	}

	if err := Run(base, ""); err == nil {
		t.Fatal("未开 MkdirAll 时目录不存在应报错")
	}

	base.MkdirAll = true
	if err := Run(base, ""); err != nil {
		t.Fatalf("开 MkdirAll 后应成功: %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "X=1\n" {
		t.Fatalf("生成物 = %q, err = %v", string(got), err)
	}
}

// fakeTB 记录同步断言报红信息的最小 TB 实现。
type fakeTB struct {
	msgs []string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.msgs = append(f.msgs, fmt.Sprintf(format, args...))
}

// runAssertInSync 在独立 goroutine 里跑断言（真 Fatalf 会终止测试 goroutine，这里隔离）。
func runAssertInSync(spec Spec) *fakeTB {
	fake := &fakeTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		AssertInSync(fake, spec)
	}()
	<-done
	return fake
}

// TestAssertInSync 同步断言：全等时不报红，不同步时报红并给出再生成命令。
func TestAssertInSync(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.ts")
	spec := Spec{
		Name:     "gen-probe",
		Hint:     "out.ts",
		Locate:   func(string) (string, bool) { return target, true },
		NotFound: probeNotFound,
		Render:   func() (string, error) { return "hello\n", nil },
	}

	// 全等 → 不报红
	if err := os.WriteFile(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	if fake := runAssertInSync(spec); len(fake.msgs) != 0 {
		t.Fatalf("应通过，却报红: %v", fake.msgs)
	}

	// CRLF 检出差异不报红（行尾不参与生成物语义）
	if err := os.WriteFile(target, []byte("hello\r\n"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	if fake := runAssertInSync(spec); len(fake.msgs) != 0 {
		t.Fatalf("CRLF 不应报红: %v", fake.msgs)
	}

	// 内容不同步 → 报红，且提示再生成命令
	if err := os.WriteFile(target, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	fake := runAssertInSync(spec)
	if len(fake.msgs) == 0 {
		t.Fatal("不同步应报红")
	}
	if !strings.Contains(fake.msgs[0], "go run ./cmd/gen-probe") {
		t.Fatalf("报红信息应给出再生成命令: %v", fake.msgs)
	}

	// 生成物缺失 → 报红
	if err := os.Remove(target); err != nil {
		t.Fatalf("删文件失败: %v", err)
	}
	if fake := runAssertInSync(spec); len(fake.msgs) == 0 {
		t.Fatal("生成物缺失应报红")
	}

	// 渲染失败 → 报红
	spec.Render = func() (string, error) { return "", errors.New("boom") }
	if fake := runAssertInSync(spec); len(fake.msgs) == 0 {
		t.Fatal("渲染失败应报红")
	}
}

// TestMain_ExitCodeAndPrefix 执行入口的失败出口：非零退出 + stderr 带程序名前缀。
// 退出码只能在子进程里观测，故用子进程复跑本测试（标准 Go 手法）。
func TestMain_ExitCodeAndPrefix(t *testing.T) {
	const helperEnv = "CODEGEN_MAIN_HELPER"
	if os.Getenv(helperEnv) == "1" {
		os.Args = []string{"gen-probe"}
		Main(Spec{
			Name:     "gen-probe",
			Hint:     "out.ts",
			Locate:   func(string) (string, bool) { return "", false },
			NotFound: probeNotFound,
			Render:   func() (string, error) { return "x", nil },
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMain_ExitCodeAndPrefix")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("子进程应非零退出, got err=%v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("退出码 = %d, 期望 1", exitErr.ExitCode())
	}
	if !strings.HasPrefix(stderr.String(), "gen-probe: ") {
		t.Fatalf("stderr 应带程序名前缀: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), probeNotFound) {
		t.Fatalf("stderr 应含定位失败信息: %q", stderr.String())
	}
}
