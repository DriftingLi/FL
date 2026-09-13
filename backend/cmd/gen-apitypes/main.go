// Command gen-apitypes 再生成前端契约类型（ADR-0019 专项第一步 / spec #940 片五③）：
// 读 swagger 产物（由注解生成），按域声明表渲染并覆写
// frontend/src/api/generated/<domain>.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-apitypes              # 路径自动定位
//	go run ./cmd/gen-apitypes -spec <p> -out-dir <d>     # 显式指定
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"forklift-training/internal/apitypes"
)

func main() {
	specPath := flag.String("spec", "", "swagger 产物路径（默认向上定位 backend/docs/swagger.json）")
	outDir := flag.String("out-dir", "", "输出目录（默认向上定位 frontend/src/api/generated）")
	flag.Parse()

	spec, err := resolveSpec(*specPath)
	if err != nil {
		fatal(err)
	}
	dir, err := resolveOutDir(*outDir)
	if err != nil {
		fatal(err)
	}
	parsed, err := apitypes.LoadSpec(spec)
	if err != nil {
		fatal(err)
	}
	rendered, err := apitypes.RenderAll(parsed)
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal(err)
	}
	for _, d := range apitypes.Domains {
		target := filepath.Join(dir, d.Name+".ts")
		content := rendered[d.Name]
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			fatal(err)
		}
		fmt.Printf("已生成 %s（%d 字节）\n", target, len(content))
	}
}

// resolveSpec swagger 产物路径：显式 -spec 优先；否则从 cwd 逐级向上找 backend/docs/swagger.json。
func resolveSpec(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "backend", "docs", "swagger.json")
		if st, statErr := os.Stat(cand); statErr == nil && !st.IsDir() {
			return cand, nil
		}
		cand = filepath.Join(dir, "docs", "swagger.json") // 在 backend/ 内直接跑
		if st, statErr := os.Stat(cand); statErr == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 swagger.json：请在仓库内运行，或用 -spec 指定")
		}
		dir = parent
	}
}

// resolveOutDir 输出目录：显式 -out-dir 优先；否则从 cwd 逐级向上找 frontend/src/api 目录。
func resolveOutDir(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "frontend", "src", "api")
		if st, statErr := os.Stat(cand); statErr == nil && st.IsDir() {
			return filepath.Join(cand, "generated"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 frontend/src/api 目录：请在仓库内运行，或用 -out-dir 指定")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-apitypes:", err)
	os.Exit(1)
}
