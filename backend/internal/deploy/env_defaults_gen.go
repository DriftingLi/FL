// 本文件：gen-deploy 的生成器声明——输出定位与渲染函数同源，
// cmd/gen-deploy 与 internal/deploy 的同步断言共用一份（ADR-0053 §9）。
package deploy

import (
	"os"
	"path/filepath"

	"forklift-training/internal/codegen"
)

// NotFoundRepoRoot 未能在 cwd 及其祖先里找到仓库根时的错误信息（逐字沿用）。
const NotFoundRepoRoot = "未找到仓库根（docker-compose.prod.yml）：请在仓库内运行，或用 -out 指定输出路径"

// EnvDefaultsGen gen-deploy：部署拓扑声明表 → deploy/env.defaults。
// 输出目录（deploy/）在建库较早的阶段可能不存在，故开启 MkdirAll。
var EnvDefaultsGen = codegen.Spec{
	Name: "gen-deploy",
	Hint: "deploy/env.defaults",
	Locate: func(dir string) (string, bool) {
		// 仓库根判据：docker-compose.prod.yml
		if st, err := os.Stat(filepath.Join(dir, "docker-compose.prod.yml")); err == nil && !st.IsDir() {
			return filepath.Join(dir, "deploy", "env.defaults"), true
		}
		return "", false
	},
	NotFound: NotFoundRepoRoot,
	MkdirAll: true,
	Render:   RenderEnvDefaults,
}
