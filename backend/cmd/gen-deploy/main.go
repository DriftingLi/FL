// Command gen-deploy 再生成部署面产物（ADR-0047 §5 / spec #932 / 票 #1364）——**生成勿改**：
//
//	① deploy/env.defaults       部署默认值：读 internal/deploy.EnvVars 声明表整份覆写
//	② frontend/nginx-host.conf  RGW 直出的投递分档 map：读 service 的类型表，
//	   只覆写文件里两行标记之间的**生成区**，区外内容原样保留（ADR-0066 决策 2 / #1364）
//
// 用法：
//
//	cd backend && go run ./cmd/gen-deploy             # 两份产物全部再生成
//	cd backend && go run ./cmd/gen-deploy -only env   # 只生成 deploy/env.defaults
//	cd backend && go run ./cmd/gen-deploy -only nginx # 只生成 nginx-host.conf 的生成区
//	cd backend && go run ./cmd/gen-deploy -out <path> # 只把 env.defaults 写到显式路径（调试/CI）
//
// 两份产物的同步契约都是「现算结果 == 磁盘内容」字节级全等（codegen.AssertInSync），
// 见 internal/deploy/topology_test.go 与 internal/deploy/nginx_delivery_gen_test.go。
// CI 的新鲜度锁用 `-only nginx`（.github/workflows/ci.yml 的 frontend-check），
// 判据由 main_test.go 钉住：产物名写错必须非零退出，不得「什么都没生成还报绿」。
package main

import (
	"flag"
	"fmt"
	"os"

	"forklift-training/internal/codegen"
	"forklift-training/internal/deploy"
)

// target 一个产物：命令行名字 + 生成器声明。
type target struct {
	name string
	spec codegen.Spec
}

// targets 本命令负责的全部产物（顺序即执行顺序）。
var targets = []target{
	{"env", deploy.EnvDefaultsGen},
	{"nginx", deploy.NginxDeliveryMapGen},
}

// generate 落盘一个产物（测试注入假的 generate 以断言「选了谁、选了几个」）。
func generate(spec codegen.Spec, explicitOut string) error { return codegen.Run(spec, explicitOut) }

func main() {
	if err := run(os.Args[1:], generate); err != nil {
		codegen.Fatal("gen-deploy", err)
	}
}

// run 解析命令行并执行选中的产物生成。
func run(args []string, gen func(codegen.Spec, string) error) error {
	fs := flag.NewFlagSet("gen-deploy", flag.ContinueOnError)
	out := fs.String("out", "", fmt.Sprintf("只生成 env.defaults 到指定路径（默认从当前目录向上定位 %s）", deploy.EnvDefaultsGen.Hint))
	only := fs.String("only", "", "只生成指定产物（env | nginx），默认全部")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// -out 是历史入口：显式路径只对 env.defaults 有意义（另一份产物是就地覆写生成区，
	// 换个路径就等于换了被改的文件），保持 `-out` 语义不变，CI 与调试脚本无需改。
	if *out != "" {
		if *only != "" {
			return fmt.Errorf("-out 与 -only 互斥：-out 已经只生成 env.defaults")
		}
		return gen(deploy.EnvDefaultsGen, *out)
	}

	// 先验名字再执行：写错 -only 取值时绝不出现「什么都没生成、退出码 0」的静默通过。
	if *only != "" {
		t, ok := findTarget(*only)
		if !ok {
			return fmt.Errorf("-only 取值 %q 不是产物名（env | nginx）", *only)
		}
		return gen(t.spec, "")
	}

	for _, t := range targets {
		if err := gen(t.spec, ""); err != nil {
			return err
		}
	}
	return nil
}

// findTarget 按名字取产物声明。
func findTarget(name string) (target, bool) {
	for _, t := range targets {
		if t.name == name {
			return t, true
		}
	}
	return target{}, false
}
