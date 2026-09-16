// Command gen-apitypes 再生成前端契约类型（ADR-0019 专项第一步 / spec #940 片五③）：
// 读 swagger 产物（由注解生成），按域声明表渲染并覆写
// frontend/src/api/generated/<domain>.ts（生成勿改文件）。
//
// 用法：
//
//	cd backend && go run ./cmd/gen-apitypes              # 路径自动定位
//	go run ./cmd/gen-apitypes -spec <p> -out-dir <d>     # 显式指定
//
// 唯一的双输入 / 多输出生成器：它不进 codegen.Main 的单文件流程，
// 但定位、写盘与失败出口复用同一套骨架（ADR-0053 §9）。
package main

import (
	"flag"
	"os"
	"path/filepath"

	"forklift-training/internal/apitypes"
	"forklift-training/internal/codegen"
)

// progName 失败信息前缀（与 CLI 名一致）。
const progName = "gen-apitypes"

func main() {
	specPath := flag.String("spec", "", "swagger 产物路径（默认向上定位 backend/docs/swagger.json）")
	outDir := flag.String("out-dir", "", "输出目录（默认向上定位 frontend/src/api/generated）")
	flag.Parse()

	spec, err := codegen.Resolve(*specPath, apitypes.LocateSwaggerSpec, apitypes.NotFoundSwaggerSpec)
	if err != nil {
		codegen.Fatal(progName, err)
	}
	dir, err := codegen.Resolve(*outDir, apitypes.LocateGeneratedDir, apitypes.NotFoundGeneratedDir)
	if err != nil {
		codegen.Fatal(progName, err)
	}
	parsed, err := apitypes.LoadSpec(spec)
	if err != nil {
		codegen.Fatal(progName, err)
	}
	rendered, err := apitypes.RenderAll(parsed)
	if err != nil {
		codegen.Fatal(progName, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		codegen.Fatal(progName, err)
	}
	for _, d := range apitypes.Domains {
		target := filepath.Join(dir, d.Name+".ts")
		if err := codegen.WriteFile(target, rendered[d.Name]); err != nil {
			codegen.Fatal(progName, err)
		}
	}
}
