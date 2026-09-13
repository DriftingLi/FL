// Command gen-authz 再生成前端授权配置（ADR-0047 §1 / spec #928）：读能力表，渲染并覆写
// frontend/src/config/authz.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-authz            # 输出路径自动定位
//	go run ./cmd/gen-authz -out <path>             # 显式指定输出路径
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"forklift-training/internal/authz"
)

func main() {
	out := flag.String("out", "", "输出路径（默认从当前目录向上定位 frontend/src/config/authz.ts）")
	flag.Parse()
	target, err := resolveOut(*out)
	if err != nil {
		fatal(err)
	}
	content, err := authz.RenderFrontendAuthzTS()
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("已生成 %s（%d 字节）\n", target, len(content))
}

// resolveOut 输出路径：显式 -out 优先；否则从 cwd 逐级向上找 frontend/src/config 目录。
func resolveOut(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "frontend", "src", "config")
		if st, statErr := os.Stat(cand); statErr == nil && st.IsDir() {
			return filepath.Join(cand, "authz.ts"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 frontend/src/config 目录：请在仓库内运行，或用 -out 指定输出路径")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-authz:", err)
	os.Exit(1)
}
