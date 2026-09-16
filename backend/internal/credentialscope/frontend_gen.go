// 本文件：gen-credscope 的生成器声明——输出定位与渲染函数同源，
// cmd/gen-credscope 与 internal/credentialscope 的同步断言共用一份（ADR-0053 §9）。
package credentialscope

import "forklift-training/internal/codegen"

// FrontendCredentialScopeGen gen-credscope：例外登记表 → frontend/src/config/credentialScope.ts。
var FrontendCredentialScopeGen = codegen.Spec{
	Name: "gen-credscope",
	Hint: "frontend/src/config/credentialScope.ts",
	Locate: func(dir string) (string, bool) {
		return codegen.LocateInFrontendConfig(dir, "credentialScope.ts")
	},
	NotFound: codegen.NotFoundFrontendConfigDir,
	Render:   RenderFrontendTS,
}
