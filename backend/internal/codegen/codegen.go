// Package codegen 是「读声明表 → 渲染 → 覆写生成物」这一族生成器的共享骨架。
//
// 背景（ADR-0053 §9）：仓库里有五个生成器（gen-authz / gen-credscope / gen-deploy /
// gen-apitypes / gen-aifeatures），各自把同一套骨架抄了一遍——定位输出路径（从当前目录逐级
// 向上找目标目录）、失败时打印并退出、写盘、打印字节数；其中两个的定位函数逐字相同，只差一个
// 文件名字面量。Go 侧「生成物与声明表是否同步」的断言也各写一遍。
//
// 本包只承载骨架，不认识任何具体的声明表：每个生成器提供一份 Spec（程序名 / 输出定位 /
// 渲染函数），命令行用法（`go run ./cmd/gen-X` 与 `-out` 覆盖）保持不变。
package codegen

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Spec 一个生成器的声明：程序名 + 输出定位 + 渲染函数。
type Spec struct {
	// Name 程序名，用作失败信息前缀（如 "gen-authz"）。
	Name string
	// Hint 输出对象的人读描述，用于 -out 参数说明与再生成提示
	// （如 "frontend/src/config/authz.ts"）。
	Hint string
	// Locate 在 dir 下判定输出路径：命中返回 (完整路径, true)。
	// 由各生成器提供自己的判据（frontend/src/config 目录、仓库根、backend/docs…）。
	Locate func(dir string) (string, bool)
	// NotFound 逐级到文件系统根仍未命中时的错误信息（含「请在仓库内运行，或用 -out 指定」语义）。
	NotFound string
	// MkdirAll 写盘前是否先建输出目录（生成物目录可能不存在时用）。
	MkdirAll bool
	// Render 渲染生成物内容。
	Render func() (string, error)
}

// TB 同步断言所需的最小 testing 接口。
// 刻意不引 testing 包，避免把它链进生成器二进制。
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// NotFoundFrontendConfigDir 未能在 cwd 及其祖先里找到 frontend/src/config 时的错误信息。
// 四个以该目录为落点的生成器共用一份，避免同一句提示各写一遍后悄悄分叉（文案逐字沿用）。
const NotFoundFrontendConfigDir = "未找到 frontend/src/config 目录：请在仓库内运行，或用 -out 指定输出路径"

// LocateInFrontendConfig 在 dir 下定位 frontend/src/config/<file> 的通用判据。
func LocateInFrontendConfig(dir, file string) (string, bool) {
	cand := filepath.Join(dir, "frontend", "src", "config")
	if st, err := os.Stat(cand); err == nil && st.IsDir() {
		return filepath.Join(cand, file), true
	}
	return "", false
}

// Resolve 定位输出路径：explicit 非空时返回其绝对路径（不校验存在性，与既有 -out 语义一致）；
// 否则从当前工作目录逐级向上，把每个目录交给 locate；到文件系统根仍未命中即返回 notFound。
func Resolve(explicit string, locate func(dir string) (string, bool), notFound string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if path, ok := locate(dir); ok {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s", notFound)
		}
		dir = parent
	}
}

// WriteFile 写盘并打印字节数（输出与各生成器原有格式逐字一致）。
func WriteFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Printf("已生成 %s（%d 字节）\n", path, len(content))
	return nil
}

// Run 执行一个生成器：定位 →（可选建目录）→ 渲染 → 写盘 → 打印。
// 任一步失败都返回错误，由调用方决定失败出口（见 Main）。
func Run(spec Spec, explicitOut string) error {
	target, err := Resolve(explicitOut, spec.Locate, spec.NotFound)
	if err != nil {
		return err
	}
	content, err := spec.Render()
	if err != nil {
		return err
	}
	if spec.MkdirAll {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
	}
	return WriteFile(target, content)
}

// Main 生成器入口：解析 -out、执行 Run；失败即非零退出并带程序名前缀。
func Main(spec Spec) {
	fs := flag.NewFlagSet(spec.Name, flag.ExitOnError)
	out := fs.String("out", "", fmt.Sprintf("输出路径（默认从当前目录向上定位 %s）", spec.Hint))
	_ = fs.Parse(os.Args[1:])
	if err := Run(spec, *out); err != nil {
		Fatal(spec.Name, err)
	}
}

// Fatal 以「程序名: 错误」的形态打印到 stderr 并以退出码 1 结束。
func Fatal(name string, err error) {
	fmt.Fprintln(os.Stderr, name+":", err)
	os.Exit(1)
}

// AssertInSync 断言生成物与渲染结果字节级全等（五个域共用的同步契约）。
//
// 生成物路径同样走 spec.Locate，所以测试不需要再手写 `../../../` 这类相对路径；
// 比对前把 CRLF 归一为 LF —— 仅防 Windows 检出差异造成的假红（行尾不参与生成物语义），
// CI（Linux）与本地因此得到同一结论。
func AssertInSync(t TB, spec Spec) {
	t.Helper()
	path, err := Resolve("", spec.Locate, spec.NotFound)
	if err != nil {
		t.Fatalf("定位生成物失败: %v", err)
	}
	want, err := spec.Render()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取生成物 %s 失败（应先运行 cd backend && go run ./cmd/%s）: %v", path, spec.Name, err)
	}
	got := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if got != want {
		t.Fatalf("%s 与声明表不同步：请 cd backend && go run ./cmd/%s\n--- want ---\n%s\n--- got ---\n%s",
			spec.Hint, spec.Name, want, got)
	}
}
