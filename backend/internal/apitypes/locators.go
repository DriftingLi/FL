// 本文件：gen-apitypes 的路径定位规则——输入面（swagger 产物）与输出面（前端契约类型生成物）
// 同源，cmd/gen-apitypes 与 codegen_test.go 的同步断言共用（ADR-0053 §9）。
package apitypes

import (
	"os"
	"path/filepath"

	"forklift-training/internal/codegen"
)

// NotFoundSwaggerSpec 未找到 swagger 产物时的错误信息（逐字沿用）。
const NotFoundSwaggerSpec = "未找到 swagger.json：请在仓库内运行，或用 -spec 指定"

// NotFoundGeneratedDir 未找到前端契约类型生成目录时的错误信息（逐字沿用）。
const NotFoundGeneratedDir = "未找到 frontend/src/api 目录：请在仓库内运行，或用 -out-dir 指定"

// LocateSwaggerSpec 在 dir 下定位 swagger 产物：<仓库根>/backend/docs/swagger.json，
// 或已身处 backend/ 时的 <backend>/docs/swagger.json。
func LocateSwaggerSpec(dir string) (string, bool) {
	for _, cand := range []string{
		filepath.Join(dir, "backend", "docs", "swagger.json"),
		filepath.Join(dir, "docs", "swagger.json"),
	} {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, true
		}
	}
	return "", false
}

// LocateGeneratedDir 在 dir 下定位 frontend/src/api/generated 目录。
func LocateGeneratedDir(dir string) (string, bool) {
	cand := filepath.Join(dir, "frontend", "src", "api")
	if st, err := os.Stat(cand); err == nil && st.IsDir() {
		return filepath.Join(cand, "generated"), true
	}
	return "", false
}

// GeneratedTSGen 某个域的前端契约类型生成物声明：cmd/gen-apitypes 写入这些文件，
// 同步断言也用同一份定位规则（不再手写 `../../../`）。
//
// 它是多输出生成器，故不进 codegen.Main 的单文件流程，只复用 codegen 的定位/写盘/失败出口。
func GeneratedTSGen(domain string, render func() (string, error)) codegen.Spec {
	return codegen.Spec{
		Name: "gen-apitypes",
		Hint: "frontend/src/api/generated/" + domain + ".ts",
		Locate: func(dir string) (string, bool) {
			base, ok := LocateGeneratedDir(dir)
			if !ok {
				return "", false
			}
			return filepath.Join(base, domain+".ts"), true
		},
		NotFound: NotFoundGeneratedDir,
		Render:   render,
	}
}
