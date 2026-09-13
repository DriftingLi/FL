// Command gen-deploy 再生成部署默认值文件（ADR-0047 §5 / spec #932）：读声明表，渲染并覆写
// deploy/env.defaults（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-deploy            # 输出路径自动定位
//	go run ./cmd/gen-deploy -out <path>             # 显式指定输出路径
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"forklift-training/internal/deploy"
)

func main() {
	out := flag.String("out", "", "输出路径（默认从当前目录向上定位 deploy/env.defaults）")
	flag.Parse()
	target, err := resolveOut(*out)
	if err != nil {
		fatal(err)
	}
	content, err := deploy.RenderEnvDefaults()
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("已生成 %s（%d 字节）\n", target, len(content))
}

// resolveOut 输出路径：显式 -out 优先；否则从 cwd 逐级向上定位仓库根的 deploy/ 目录。
func resolveOut(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, statErr := os.Stat(filepath.Join(dir, "docker-compose.prod.yml")); statErr == nil && !st.IsDir() {
			return filepath.Join(dir, "deploy", "env.defaults"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到仓库根（docker-compose.prod.yml）：请在仓库内运行，或用 -out 指定输出路径")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-deploy:", err)
	os.Exit(1)
}
