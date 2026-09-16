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
	"forklift-training/internal/codegen"
	"forklift-training/internal/service"
)

func main() {
	codegen.Main(service.FrontendAIFeaturesGen)
}
