package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 生成物同步契约（ADR-0047 §5 / spec #932）：deploy/env.defaults 必须与声明表渲染结果字节级全等。
// 手改生成物、或改了声明表却忘记再生成，本测试即红（prior art：TestFrontendAuthzTSInSync）。
func TestEnvDefaultsInSync(t *testing.T) {
	want, err := RenderEnvDefaults()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	path := filepath.Join("..", "..", "..", "deploy", "env.defaults")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取生成物 %s 失败（应先运行 cd backend && go run ./cmd/gen-deploy）: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("deploy/env.defaults 与声明表不同步：请 cd backend && go run ./cmd/gen-deploy")
	}
}

// 漂移锁：三份部署文件里的 `VAR:-默认值` 必须与声明表逐字一致。
//
// 这条测试正是能抓住已发生的漂移（REDIS_POOL_SIZE 在 CD/compose 是 20、在部署脚本曾是 10）的那条：
// 「两条部署路径默认值不同」以前只有对着生产跑一次才发现。
func TestEnvDefaultsNoDrift(t *testing.T) {
	files := []string{
		filepath.Join("..", "..", "..", "docker-compose.prod.yml"),
		filepath.Join("..", "..", "..", "scripts", "deploy-remote.sh"),
		filepath.Join("..", "..", "..", ".github", "workflows", "cd.yml"),
	}
	// 两种默认值写法都要认：
	//   shell/compose 形态  ${VAR:-default}
	//   GitHub 表达式形态    ${{ secrets.VAR || 'default' }}（cd.yml 的 env: 段用它，曾是漏检面）
	pat := regexp.MustCompile(`([A-Z_][A-Z0-9_]*):-([^}]*)}`)
	ghaPat := regexp.MustCompile(`secrets\.([A-Z_][A-Z0-9_]*)\s*\|\|\s*'([^']*)'`)
	dollar := string(rune(36))
	var violations []string
	checked := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", f, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue // 注释里的示例不算声明（cd.yml 说明文字里出现过 ${VAR:-默认值}）
			}
			matches := pat.FindAllStringSubmatch(line, -1)
			matches = append(matches, ghaPat.FindAllStringSubmatch(line, -1)...)
			for _, m := range matches {
				name, def := m[1], m[2]
				v, ok := Lookup(name)
				if !ok || v.Default == "" {
					continue // 未声明的变量（脚本内部量/密钥类）不约束
				}
				if def == "" {
					continue // 空默认 = 「交给下一层兜底」（cd.yml 写空串让 compose 默认生效），不是漂移
				}
				if strings.Contains(def, dollar+"{") {
					continue // 嵌套默认值（如 ${SMTP_FROM:-}）不在本期声明面
				}
				checked++
				if def != v.Default {
					violations = append(violations, fmt.Sprintf("%s: %s:-%s（声明为 %q）", filepath.Base(f), name, def, v.Default))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("未在三份部署文件里找到任何已声明变量的默认值——漂移锁失效（文件结构变了？）")
	}
	if len(violations) > 0 {
		t.Fatalf("部署默认值漂移（声明表 %d 项，检查 %d 处）：\n%s", len(EnvVars), checked, strings.Join(violations, "\n"))
	}
}

// 声明表形状锁：变量名唯一、默认值非空（空默认不是事实，是「必须由环境提供」）。
func TestEnvVarsShape(t *testing.T) {
	seen := map[string]bool{}
	for _, v := range EnvVars {
		if v.Name == "" || v.Default == "" || v.Desc == "" {
			t.Fatalf("声明项不完整: %+v", v)
		}
		if seen[v.Name] {
			t.Fatalf("变量名重复: %s", v.Name)
		}
		seen[v.Name] = true
	}
}

// 生成物的真实消费者锁（spec #940 片四）：ADR-0047 §5 收尾时 deploy/env.defaults 是
// 「零消费者」的死文件 —— 只有全等契约钉着它，部署链路仍各自写默认值。这条锁把接线钉住：
// CD 生成的环境变量文件必须 source 它，且打包清单必须带上它（否则远端 source 会失败）。
// 没有这条锁，某次重构可以把接线悄悄摘掉，「生成物与运行期行为脱节」会原样回来。
func TestEnvDefaultsIsConsumed(t *testing.T) {
	cdPath := filepath.Join("..", "..", "..", ".github", "workflows", "cd.yml")
	data, err := os.ReadFile(cdPath)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", cdPath, err)
	}
	cd := string(data)
	if !strings.Contains(cd, "source \"$DEPLOY_PATH/deploy/env.defaults\"") {
		t.Fatal("CD 链路没有 source deploy/env.defaults：生成物又变回零消费者的死文件")
	}
	if !strings.Contains(cd, "deploy/env.defaults \\") {
		t.Fatal("CD 的打包清单里没有 deploy/env.defaults：远端 source 会失败")
	}
	// 生成物是**整体 export** 的，而 compose 的取值优先级是「shell 环境 > .env」：
	// 镜像引用由 deploy-remote.sh 按 registry + tag 现算后写进 .env（不 export），
	// 一旦生成物把 BACKEND_IMAGE/FRONTEND_IMAGE/LIBREOFFICE_IMAGE 留在环境里，就会盖掉计算值，
	// compose 转而去 Docker Hub 拉不存在的 forklift-backend:latest（2026-09-13 testing 冒烟实测）。
	if !strings.Contains(cd, "unset BACKEND_IMAGE FRONTEND_IMAGE LIBREOFFICE_IMAGE") {
		t.Fatal("CD 的环境变量文件没有 unset 生成物里的镜像名：会把部署脚本算好的镜像引用盖掉")
	}
}
