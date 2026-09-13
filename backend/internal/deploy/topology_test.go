package deploy

import (
	"fmt"
	"os"
	"os/exec"
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
	// 2026-09-13 生产事故的文本级回归锁：部署脚本**不得**再 source 生成物。
	// source 是无条件赋值，它把 CD 传入的 PG_VOLUME（生产真实数据目录 /srv/ceph/pgdata）盖成
	// 默认值 pgdata-prod，compose 遂把 postgres 挂到另一个空卷，表现为「生产数据全没了」。
	remote, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "deploy-remote.sh"))
	if err != nil {
		t.Fatalf("读取 scripts/deploy-remote.sh 失败: %v", err)
	}
	for i, line := range strings.Split(string(remote), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue // 注释里提到生成物是允许的
		}
		if (strings.HasPrefix(trimmed, ". ") || strings.HasPrefix(trimmed, "source ")) &&
			strings.Contains(trimmed, "env.defaults") {
			t.Fatalf("deploy-remote.sh:%d 又 source 了生成物（会无条件覆盖 CD 传入的环境）：%s", i+1, trimmed)
		}
	}
}

// TestDeployEnvChainPreservesProvidedValues 把「CD 生成环境变量文件 → 远端 source」这条链路
// 真跑一遍 bash，断言：CD 提供的值活到最后，生成物的默认值只对「未提供的变量」生效。
//
// 这是 2026-09-13 生产事故的**行为级**回归锁（上面那条只是文本级）：事故当天 PG_VOLUME 在
// 链路中段被无条件赋值盖掉，而当天的 testing 冒烟环境恰好没有这个 secret → 没有任何测试变红。
func TestDeployEnvChainPreservesProvidedValues(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("环境无 bash，跳过")
	}
	root := filepath.Join("..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "cd.yml"))
	if err != nil {
		t.Fatalf("读取 cd.yml 失败: %v", err)
	}
	src := string(raw)
	const start = "# 用 printf %q 生成安全转义的环境变量文件"
	const end = "} > /tmp/deploy-env.sh"
	i := strings.Index(src, start)
	if i < 0 {
		t.Fatal("cd.yml 里找不到环境变量生成块的起点")
	}
	block := src[i:]
	j := strings.Index(block, end)
	if j < 0 {
		t.Fatal("cd.yml 里找不到环境变量生成块的终点")
	}
	block = block[:j+len(end)]
	if !strings.Contains(block, "for _name in $(grep -E '^export [A-Z_0-9]+='") {
		t.Fatal("cd.yml 生成块结构变了（找不到只覆盖已提供值的 loop）：本测试需要跟着改")
	}

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "deploy"), 0o755); err != nil {
		t.Fatalf("建临时 deploy/ 失败: %v", err)
	}
	defaults, err := os.ReadFile(filepath.Join(root, "deploy", "env.defaults"))
	if err != nil {
		t.Fatalf("读取 deploy/env.defaults 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "deploy", "env.defaults"), defaults, 0o644); err != nil {
		t.Fatalf("写临时生成物失败: %v", err)
	}
	envFile := filepath.Join(tmp, "deploy-env.sh")
	genScript := filepath.Join(tmp, "gen.sh")
	if err := os.WriteFile(genScript, []byte(strings.ReplaceAll(block, "/tmp/deploy-env.sh", envFile)), 0o755); err != nil {
		t.Fatalf("写生成脚本失败: %v", err)
	}

	// 模拟 CD runner：cwd = 仓库根（生成块里 grep deploy/env.defaults 是相对路径，CI 就是这么跑的），
	// DEPLOY_PATH 指向临时目录（deploy/env.defaults 就是真生成物），PG_VOLUME/DOMAIN 模拟
	// production 环境 secret；REDIS_POOL_SIZE 故意不提供，用于验证默认值仍生效。
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("解析仓库根失败: %v", err)
	}
	gen := exec.Command("bash", genScript)
	gen.Dir = rootAbs
	gen.Env = []string{"PATH=/usr/bin:/bin", "DEPLOY_PATH=" + tmp, "PG_VOLUME=/srv/ceph/pgdata", "DOMAIN=prod.example.com"}
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("生成环境变量文件失败: %v\n%s", err, out)
	}

	// 模拟远端：source 之后看生效值（set -u 一并验证：未定义的变量不许炸）。
	probeScript := filepath.Join(tmp, "probe.sh")
	probeBody := "set -eu\nsource " + envFile + "\nprintf '%s|%s|%s\\n' \"$PG_VOLUME\" \"$DOMAIN\" \"$REDIS_POOL_SIZE\"\n"
	if err := os.WriteFile(probeScript, []byte(probeBody), 0o755); err != nil {
		t.Fatalf("写探针脚本失败: %v", err)
	}
	probe := exec.Command("bash", probeScript)
	probe.Env = []string{"PATH=/usr/bin:/bin", "DEPLOY_PATH=" + tmp}
	out, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("远端 source 失败: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	want := "/srv/ceph/pgdata|prod.example.com|20"
	if got != want {
		t.Fatalf("链路把 CD 提供的值弄丢了：\n  got  = %s\n  want = %s\n"+
			"（第 1 段 = CD 提供的 PG_VOLUME，必须原样保留；第 3 段 = 未提供变量的默认值，必须来自生成物）", got, want)
	}
}
