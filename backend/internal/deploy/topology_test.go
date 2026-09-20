package deploy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/codegen"
)

// 生成物同步契约（ADR-0047 §5 / spec #932）：deploy/env.defaults 必须与声明表渲染结果字节级全等。
// 手改生成物、或改了声明表却忘记再生成，本测试即红（定位与提示走 codegen.AssertInSync，
// ADR-0053 §9；行尾按 LF 语义比对，避免 Windows 检出差异假红）。
func TestEnvDefaultsInSync(t *testing.T) {
	codegen.AssertInSync(t, EnvDefaultsGen)
}

// 漂移锁：四份部署文件里的 `VAR:-默认值`、以及运行期配置 internal/config/config.go 里的
// viper 默认值，都必须与声明表逐字一致。
//
// 这条测试正是能抓住已发生的漂移（REDIS_POOL_SIZE 在 CD/compose 是 20、在部署脚本曾是 10）的那条：
// 「两条部署路径默认值不同」以前只有对着生产跑一次才发现。
//
// 运行期一侧是 ADR-0060 票5 扩进来的，形态与部署侧不同：**只比对重叠子集**（见下文
// runtimeEnvDefaults 与 layerDivergences 的判据），既不做「抽中立叶子表让 config 派生」，
// 也不给运行期加一条指向本包的依赖边（被否备选见 ADR-0060）。
func TestEnvDefaultsNoDrift(t *testing.T) {
	// 声明这些默认值的**全部**位置都要进锁：漏一个就漏一处漂移（deploy.sh 的 ${DB_USER:-forklift}
	// 此前在锁外，ADR-0053 §10）。deployFiles 是部署侧（shell/compose/GitHub 表达式三种写法），
	// runtimeFiles 是运行期侧（viper 形态，ADR-0060 票5 起纳入；比对判据见该段注释）。
	deployFiles := []string{
		filepath.Join("..", "..", "..", "docker-compose.prod.yml"),
		filepath.Join("..", "..", "..", "scripts", "deploy-remote.sh"),
		filepath.Join("..", "..", "..", "deploy.sh"),
		filepath.Join("..", "..", "..", ".github", "workflows", "cd.yml"),
	}
	runtimeFiles := []string{
		// 测试的 cwd 是 backend/internal/deploy，运行期配置在同级的 internal/config 下。
		filepath.Join("..", "config", "config.go"),
	}
	// 两种默认值写法都要认：
	//   shell/compose 形态  ${VAR:-default}
	//   GitHub 表达式形态    ${{ secrets.VAR || 'default' }}（cd.yml 的 env: 段用它，曾是漏检面）
	pat := regexp.MustCompile(`([A-Z_][A-Z0-9_]*):-([^}]*)}`)
	ghaPat := regexp.MustCompile(`secrets\.([A-Z_][A-Z0-9_]*)\s*\|\|\s*'([^']*)'`)
	dollar := string(rune(36))
	var violations []string
	checked := 0
	for _, f := range deployFiles {
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
		t.Fatal("未在部署文件里找到任何已声明变量的默认值——漂移锁失效（文件结构变了？）")
	}

	// —— 运行期侧（ADR-0060 票5「配置默认值只扩锁不派生」）——
	//
	// 判据是**重叠子集**，不是双向全覆盖：
	//   1. 变量名两侧都出现（运行期 viper 键按 AutomaticEnv 的口径大写成环境变量名，见 runtimeEnvDefaults）；
	//   2. 两侧的默认值都非空 —— 任一侧为空是「本层不主张默认值、由另一层给」，
	//      与部署侧「空默认不算漂移」同口径（TRUSTED_PROXIES 与 DIAGNOSIS_ASSISTANT_URL 即此形：
	//      运行期空串 = 不信任任何代理 / RAG 功能不可用，是显式兜底而非漏写）；
	//   3. 落在这本子集里的值必须逐字相等；不等又没登记在 layerDivergences 的即漂移。
	//
	// 运行期独有的键（RATE_LIMIT_RPS、REDIS_MIN_IDLE_CONNS …）与只在部署侧出现的卷名
	// （PG_VOLUME、*_IMAGE …）都不约束 —— 声明表自陈范围是「compose 中带非空默认值的变量」，
	// 把它扩进运行期是范围变更而非收敛。
	runtimeChecked := 0
	overlapSeen := map[string]bool{}
	divergentRegistered := map[string]bool{}
	for _, f := range runtimeFiles {
		declared := runtimeEnvDefaults(t, f)
		if len(declared) == 0 {
			t.Fatalf("运行期文件 %s 里没解析到任何默认值声明——漂移锁失效（读取形态变了？）", filepath.Base(f))
		}
		// 运行期**自身**的双写也是漂移候选：同一个键在本文件里被登记成两个不同默认值时，
		// 生效的只是读取回退那一份，另一份是等着漂移的第二事实
		// （redis_pool_size / redis_dial_timeout 曾各写两遍，票5 已收掉，ADR-0060 决策 5）。
		byRuntimeName := map[string][]runtimeDefault{}
		for _, d := range declared {
			byRuntimeName[d.name] = append(byRuntimeName[d.name], d)
		}
		names := make([]string, 0, len(byRuntimeName))
		for name := range byRuntimeName {
			names = append(names, name)
		}
		slices.Sort(names) // 报错输出与 map 迭代顺序无关（可复现）
		for _, name := range names {
			entries := byRuntimeName[name]
			for _, e := range entries[1:] {
				if e.value != entries[0].value {
					violations = append(violations, fmt.Sprintf(
						"运行期内部双写不一致: %s 在 %s 为 %q、在 %s 为 %q（一个键只许一处默认值）",
						name, entries[0].pos, entries[0].value, e.pos, e.value))
				}
			}
		}
		// 与声明表比对重叠子集。
		for _, d := range declared {
			v, ok := Lookup(d.name)
			if !ok || v.Default == "" || d.value == "" {
				continue
			}
			runtimeChecked++
			overlapSeen[d.name] = true
			if d.value == v.Default {
				continue
			}
			if _, known := layerDivergences[d.name]; known {
				divergentRegistered[d.name] = true
				continue
			}
			violations = append(violations, fmt.Sprintf(
				"%s: %s 默认值 %q（声明表为 %q）", d.pos, d.name, d.value, v.Default))
		}
	}
	if runtimeChecked == 0 {
		t.Fatal("运行期配置里没比对到任何声明表变量——漂移锁的运行期半边失效（键名形态变了？）")
	}
	// 重叠覆盖不许静默收缩（登记表与理由见 runtimeOverlapNames）。
	for _, name := range runtimeOverlapNames {
		if overlapSeen[name] {
			continue
		}
		v, _ := Lookup(name)
		violations = append(violations, fmt.Sprintf(
			"重叠覆盖收缩: 运行期不再主张 %s 的默认值（声明表为 %q），比对项被静默摘掉；\n"+
				"  确认这是有意变更后再把它从 runtimeOverlapNames 删掉", name, v.Default))
	}
	// 欠条不许变死账：登记的跨层差异必须**仍然**成立，两侧一旦收敛就要把登记删掉。
	for name, reason := range layerDivergences {
		if !divergentRegistered[name] {
			t.Errorf("已登记的跨层默认值差异 %s 不再成立（两侧已一致，或该键已不在重叠子集里）：\n"+
				"  理由: %s\n  请删掉这条登记，别留死欠条", name, reason)
		}
	}
	t.Logf("漂移锁：声明表 %d 项 / 部署侧 %d 处 / 运行期重叠 %d 项 / 登记差异 %d 条",
		len(EnvVars), checked, runtimeChecked, len(layerDivergences))
	if len(violations) > 0 {
		t.Fatalf("默认值漂移（声明表 %d 项，部署侧 %d 处、运行期 %d 项比对）：\n%s",
			len(EnvVars), checked, runtimeChecked, strings.Join(violations, "\n"))
	}
}

// layerDivergences 已登记的「部署侧默认值 ≠ 运行期默认值」欠条（键 = 环境变量名，值 = 理由）。
// 重叠子集里每一处不相等都必须在这里点名并给出理由，且登记必须仍然成立
// （判据见 TestEnvDefaultsNoDrift）—— 目的是让「有意不同」显式留痕，
// 而不是靠把锁放宽到不比对来蒙过去。
var layerDivergences = map[string]string{
	// 同一个端口号在两层是两回事：部署侧 465 是**生产事实**（隐式 SSL，CD 恒把它注入容器，
	// service/code_service.go 按 Port == 465 分派 sendSMTPS）；运行期 587 是**本地开发兜底**
	// （无 env 时走 smtp.SendMail 的 STARTTLS 路径，部分网络环境会阻断 587 故生产不用它）。
	// 收敛任一侧都是行为变更，不在票5「只扩锁 + 删双写」的范围里。
	"SMTP_PORT": "部署 465=生产隐式 SSL（恒注入）；运行期 587=本地开发 STARTTLS 兜底，两层语义不同",
}

// runtimeOverlapNames 票5 落地时**必须**留在重叠集里的变量名（升序）。
//
// 它是重叠面的**下界**、不是全覆盖清单：新增一个两层都主张默认值的变量不必登记；
// 但已有任何一项从运行期解析结果里掉出去都会判红。要这条，是因为「运行期不再写这个默认值」
// 与「解析形态变了、这一项没认出来」在结果上长得一样 —— 票5 删 redis_pool_size 的双写时，
// 只认 viper.SetDefault 形态的解析器就把这一项静默摘掉了（重叠 16→15 而锁照绿）。
var runtimeOverlapNames = []string{
	"AUTH_COOKIE_NAME",
	"JWT_EXPIRES_HOURS",
	"JWT_REFRESH_EXPIRES_DAYS",
	"LOG_COMPRESS",
	"LOG_FORMAT",
	"LOG_LEVEL",
	"LOG_MAX_AGE_DAYS",
	"LOG_MAX_BACKUPS",
	"LOG_MAX_SIZE_MB",
	"REDIS_DB",
	"REDIS_KEY_PREFIX",
	"REDIS_POOL_SIZE",
	"SMTP_FROM_NAME",
	"SMTP_PORT",
	"STORAGE_DRIVER",
	"TENCENT_SMS_REGION",
}

// runtimeDefault 运行期配置文件里的一处默认值声明。
type runtimeDefault struct {
	name  string // 环境变量名（由 viper 键大写而来）
	value string // 归一成文本的默认值（"fl:" / "20" / "2s" / "true"）
	pos   string // "文件名:行号"，报错定位用
}

// runtimeEnvDefaults 解析运行期配置文件的默认值声明面。
//
// 用 AST 而不是行正则：结构体字段的**注释**里也写着默认值，而且已经和代码不一致过
// （PoolSize 的注释曾写「默认 10」、代码是 20），只有调用面才是事实。
// 键 → 环境变量名按 viper AutomaticEnv 的口径大写（viper.getEnv 就是 strings.ToUpper）。
//
// 只认「首参是键字面量、次参是常量默认值」的读取形态；次参是表达式的（envBoolOr 的
// appEnv == "production"）不认 —— 那不是常量事实，是随环境分支的策略，比对不了。
func runtimeEnvDefaults(t *testing.T, path string) []runtimeDefault {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("解析运行期配置 %s 失败: %v", path, err)
	}
	var out []runtimeDefault
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// 两种宿主形态都要认：`viper.SetDefault(...)` 是选择器，包内的 `positiveInt(...)`
		// 读取回退是裸标识符 —— 只认前者会漏掉 Load 里的全部回退默认值（redis_pool_size
		// 收掉双写后，它是重叠集里唯一活在 Load 的那一项，漏认等于把比对项静默摘掉）。
		var fn string
		switch f := call.Fun.(type) {
		case *ast.Ident:
			fn = f.Name
		case *ast.SelectorExpr:
			fn = f.Sel.Name
		}
		if fn == "" || !runtimeDefaultFn[fn] || len(call.Args) != 2 {
			return true
		}
		key, ok := call.Args[0].(*ast.BasicLit)
		if !ok || key.Kind != token.STRING {
			return true
		}
		name, err := strconv.Unquote(key.Value)
		if err != nil {
			return true
		}
		value, ok := literalDefault(call.Args[1])
		if !ok {
			return true
		}
		out = append(out, runtimeDefault{
			name:  strings.ToUpper(name),
			value: value,
			pos:   fmt.Sprintf("%s:%d", filepath.Base(path), fset.Position(call.Pos()).Line),
		})
		return true
	})
	return out
}

// runtimeDefaultFn 运行期登记默认值的调用面：首参 viper 键、次参默认值。
var runtimeDefaultFn = map[string]bool{
	"SetDefault":       true, // viper.SetDefault：setDefaults 的集中登记
	"positiveInt":      true, // 以下四个是 Load 里的读取回退（非法/非正值时的兜底）
	"nonNegInt":        true,
	"positiveFloat":    true,
	"positiveDuration": true,
}

// literalDefault 把默认值实参归一成可与部署侧逐字比对的文本。认不出形态就交回 ok=false
// （宁可少比一项，也不猜一个值出来判红）。
func literalDefault(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			s, err := strconv.Unquote(e.Value)
			return s, err == nil
		case token.INT:
			return e.Value, true
		case token.FLOAT: // 20.0 与部署侧的 "20" 是同一个值
			f, err := strconv.ParseFloat(e.Value, 64)
			if err != nil {
				return "", false
			}
			return strconv.FormatFloat(f, 'g', -1, 64), true
		}
	case *ast.Ident:
		if e.Name == "true" || e.Name == "false" {
			return e.Name, true
		}
	case *ast.BinaryExpr: // N*time.Second 这类时长字面量
		if e.Op != token.MUL {
			return "", false
		}
		n, ok := e.X.(*ast.BasicLit)
		if !ok || n.Kind != token.INT {
			return "", false
		}
		sel, ok := e.Y.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "time" {
			return "", false
		}
		unit, ok := map[string]time.Duration{
			"Nanosecond":  time.Nanosecond,
			"Microsecond": time.Microsecond,
			"Millisecond": time.Millisecond,
			"Second":      time.Second,
			"Minute":      time.Minute,
			"Hour":        time.Hour,
		}[sel.Sel.Name]
		if !ok {
			return "", false
		}
		v, err := strconv.ParseInt(n.Value, 0, 64)
		if err != nil {
			return "", false
		}
		return formatDuration(time.Duration(v) * unit), true
	}
	return "", false
}

// formatDuration 把时长写成部署侧那种最简形式（5*time.Minute → "5m"、2*time.Second → "2s"）。
// Go 原生 String() 的 "5m0s" 是运行期方言，拿它去比 compose 里的 "5m" 只会假红。
func formatDuration(d time.Duration) string {
	for _, u := range []struct {
		size   time.Duration
		suffix string
	}{
		{time.Hour, "h"}, {time.Minute, "m"}, {time.Second, "s"}, {time.Millisecond, "ms"},
	} {
		if d%u.size == 0 {
			return strconv.FormatInt(int64(d/u.size), 10) + u.suffix
		}
	}
	return d.String()
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
