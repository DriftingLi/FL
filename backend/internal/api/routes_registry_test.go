package api

import (
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"go.uber.org/zap"

	"forklift-training/internal/testutil"
)

// 域注册表覆盖锁（ADR-0047 §6 / spec #933）：
//  1. 域名非空且唯一；
//  2. 用域注册表构出的路由总数等于登记常量——**新增蓝图却忘记登记会让数量下降**，测试即红；
//     反过来，重复登记会在 gin 注册期直接 panic（同一路径重复注册），由本测试触发暴露。
func TestRouteRegistryCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	seen := map[string]bool{}
	for _, reg := range routeRegistrars {
		if reg.Domain == "" || reg.Register == nil {
			t.Fatalf("域注册项不完整: %+v", reg)
		}
		if seen[reg.Domain] {
			t.Fatalf("域名重复: %s", reg.Domain)
		}
		seen[reg.Domain] = true
	}
	if len(routeRegistrars) == 0 {
		t.Fatal("域注册表为空")
	}

	db := testutil.NewMemoryDB(t)
	r := NewRouter(NewDeps(&config.Config{}, db, nil, zap.NewNop(), nil))
	if got := len(r.Routes()); got != expectedRouteCount {
		t.Fatalf("路由总数 = %d, want %d（新增蓝图请同步登记到 routeRegistrars；若本次是有意增删端点，请更新本常量）", got, expectedRouteCount)
	}
}

// expectedRouteCount 域注册表构出的路由总数基线（2026-09-13，第八波片六）。
const expectedRouteCount = 314
