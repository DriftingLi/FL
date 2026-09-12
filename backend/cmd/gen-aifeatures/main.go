// Command gen-aifeatures 前端 AI 功能配置再生成入口（ADR-0030 决策 3，#613）：读后端
// AI 功能注册表，渲染并覆写 frontend/src/config/aiFeatures.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-aifeatures          # 输出路径自动定位
//	go run ./cmd/gen-aifeatures -out <path>            # 显式指定输出路径
//
// 覆写后查看 git diff：若 aiFeatureUI.ts（手写展示数据）缺新功能键，vue-tsc 会按
// Record<AIFeatureKey, …> 报缺键，补齐对应展示条目即可。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"forklift-training/internal/service"
)

func main() {
	out := flag.String("out", "", "输出路径（默认从当前目录向上定位 frontend/src/config/aiFeatures.ts）")
	flag.Parse()

	target, err := resolveOut(*out)
	if err != nil {
		fatal(err)
	}
	content, err := service.GenerateFrontendAIFeaturesTS()
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
			return filepath.Join(cand, "aiFeatures.ts"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("未找到 frontend/src/config 目录：请在仓库内运行，或用 -out 指定输出路径")
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-aifeatures:", err)
	os.Exit(1)
}
