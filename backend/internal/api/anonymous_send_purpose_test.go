package api

import (
	"testing"

	"forklift-training/internal/service"
)

// 本文件：匿名发码口用途白名单的两条锁。
// 白名单是**显式清单**（镜像匿名蓝图上的路由注册），不是用途表属性的投影——
// 所以这里锁的是**单向**不变式，不是「白名单 = 非 RequiresSession 用途」。

// TestAnonymousSendPurposes_SubsetOfSessionFree 单向不变式：
// 匿名发码白名单里的用途一律不得要求已登录会话（漏标属性不会静默多出公网发码口）。
func TestAnonymousSendPurposes_SubsetOfSessionFree(t *testing.T) {
	if len(anonymousSendPurposes) == 0 {
		t.Fatal("匿名发码白名单为空")
	}
	for _, p := range anonymousSendPurposes {
		if service.CodePurposeRequiresSession(p) {
			t.Errorf("用途 %q 要求已登录会话，不得出现在匿名发码白名单里", p)
		}
	}
}

// TestAnonymousSendPurposes_Accepted 白名单里的每个用途都能被 resolvePurpose 接受。
func TestAnonymousSendPurposes_Accepted(t *testing.T) {
	for _, want := range anonymousSendPurposes {
		got, err := resolvePurpose(string(want))
		if err != nil {
			t.Errorf("resolvePurpose(%q) 报错: %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("resolvePurpose(%q) = %q", want, got)
		}
	}
}

// TestAnonymousSendPurposes_RejectsSessionScoped 要求已登录会话的用途（绑定 / 改账号 / 改密码）
// 不得从匿名发码口发出——它们不经 `/auth/<通道>/send-code`，目标也是当前用户自己的账号。
func TestAnonymousSendPurposes_RejectsSessionScoped(t *testing.T) {
	for _, p := range []service.CodePurpose{
		service.CodePurposeBind,
		service.CodePurposeAccountChange,
		service.CodePurposeChangePassword,
	} {
		if _, err := resolvePurpose(string(p)); err == nil {
			t.Errorf("要求会话的用途 %q 不应被匿名发码口接受", p)
		}
	}
}
