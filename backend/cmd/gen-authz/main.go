// Command gen-authz 再生成前端授权配置（ADR-0047 §1 / spec #928）：读能力表，渲染并覆写
// frontend/src/config/authz.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-authz            # 输出路径自动定位
//	go run ./cmd/gen-authz -out <path>             # 显式指定输出路径
package main

import (
	"forklift-training/internal/authz"
	"forklift-training/internal/codegen"
)

func main() {
	codegen.Main(authz.FrontendAuthzGen)
}
