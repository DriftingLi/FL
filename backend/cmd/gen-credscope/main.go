// Command gen-credscope 再生成前端「按证件重装」例外登记表（ADR-0047 §4 收尾 / spec #940 片六）：
// 读声明表，渲染并覆写 frontend/src/config/credentialScope.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-credscope            # 输出路径自动定位
//	go run ./cmd/gen-credscope -out <path>             # 显式指定输出路径
package main

import (
	"forklift-training/internal/codegen"
	"forklift-training/internal/credentialscope"
)

func main() {
	codegen.Main(credentialscope.FrontendCredentialScopeGen)
}
