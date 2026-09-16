// 本文件：gen-authz 的生成器声明——输出定位与渲染函数同源，
// cmd/gen-authz 与 internal/authz 的同步断言共用一份（ADR-0053 §9）。
package authz

import "forklift-training/internal/codegen"

// FrontendAuthzGen gen-authz：能力表 → frontend/src/config/authz.ts。
var FrontendAuthzGen = codegen.Spec{
	Name: "gen-authz",
	Hint: "frontend/src/config/authz.ts",
	Locate: func(dir string) (string, bool) {
		return codegen.LocateInFrontendConfig(dir, "authz.ts")
	},
	NotFound: codegen.NotFoundFrontendConfigDir,
	Render:   RenderFrontendAuthzTS,
}
