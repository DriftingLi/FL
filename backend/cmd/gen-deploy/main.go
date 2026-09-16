// Command gen-deploy 再生成部署默认值文件（ADR-0047 §5 / spec #932）：读声明表，渲染并覆写
// deploy/env.defaults（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-deploy            # 输出路径自动定位
//	go run ./cmd/gen-deploy -out <path>             # 显式指定输出路径
package main

import (
	"forklift-training/internal/codegen"
	"forklift-training/internal/deploy"
)

func main() {
	codegen.Main(deploy.EnvDefaultsGen)
}
