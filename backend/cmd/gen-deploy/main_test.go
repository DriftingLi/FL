// 本文件钉住 gen-deploy 的命令行契约 —— CI 的新鲜度锁直接调 `go run ./cmd/gen-deploy -only nginx`
// （.github/workflows/ci.yml 的 frontend-check），所以「产物名写错 ⇒ 必须非零退出」这条要能被证伪：
// 若 run() 变成「认不出名字就什么都不做、返回 nil」，那条锁会在产物根本没再生的情况下照绿。
package main

import (
	"testing"

	"forklift-training/internal/codegen"
)

// fakeGen 记录每次被选中的产物（断言「选了谁、选了几个」，不碰磁盘）。
type fakeGen struct {
	calls []string
	err   error
}

func (f *fakeGen) run(spec codegen.Spec, out string) error {
	name := spec.Hint
	if out != "" {
		name += "@" + out
	}
	f.calls = append(f.calls, name)
	return f.err
}

func TestRunSelectsTargets(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"无参数：两份产物全部生成", nil, []string{"deploy/env.defaults", "frontend/nginx-host.conf"}},
		{"-only env：只生成默认值文件", []string{"-only", "env"}, []string{"deploy/env.defaults"}},
		// CI 用的就是这一条 —— 拼错这个取值等于把新鲜度锁变成空跑。
		{"-only nginx：只覆写 nginx 生成区", []string{"-only", "nginx"}, []string{"frontend/nginx-host.conf"}},
		{"-out：显式路径只作用于 env.defaults", []string{"-out", "/tmp/env.defaults"}, []string{"deploy/env.defaults@/tmp/env.defaults"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gen := &fakeGen{}
			if err := run(c.args, gen.run); err != nil {
				t.Fatalf("run(%v) 期望成功，实得 %v", c.args, err)
			}
			if len(gen.calls) != len(c.want) {
				t.Fatalf("选中的产物数 %d ≠ 期望 %d：%v", len(gen.calls), len(c.want), gen.calls)
			}
			for i, w := range c.want {
				if gen.calls[i] != w {
					t.Errorf("第 %d 个产物 = %q，期望 %q", i+1, gen.calls[i], w)
				}
			}
		})
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"-only 取值不是产物名（静默空跑就是假绿）", []string{"-only", "nginxz"}},
		{"-only 传空产物名以外的值", []string{"-only", "env,nginx"}},
		{"-out 与 -only 互斥", []string{"-out", "/tmp/x", "-only", "env"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gen := &fakeGen{}
			if err := run(c.args, gen.run); err == nil {
				t.Fatalf("run(%v) 期望报错，实得 nil（且选中了 %v）", c.args, gen.calls)
			}
			if len(gen.calls) != 0 {
				t.Errorf("参数非法时仍执行了生成：%v", gen.calls)
			}
		})
	}
}

// TestFindTargetMatchesSpecs 产物名与生成器声明不许各自漂移：
// -only 的可选值必须恰好是 targets 里登记的两份产物。
func TestFindTargetMatchesSpecs(t *testing.T) {
	for _, name := range []string{"env", "nginx"} {
		if _, ok := findTarget(name); !ok {
			t.Errorf("-only %s 认不出来：CI/文档里的用法会直接失败", name)
		}
	}
	if len(targets) != 2 {
		t.Errorf("产物数变成 %d：-only 的取值面与本文件头部的用法注释、以及 ci.yml 的调用点都要同步", len(targets))
	}
}
