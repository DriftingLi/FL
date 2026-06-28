// Package service 实现核心业务逻辑
// 本文件：时间衰减系数 Kt 的单元测试
// 注意：Kt 计算依赖 CoefficientProvider 实时查询 coefficient_configs 表
// 测试用例需连接真实 PostgreSQL；DB 不可用时自动跳过
package service

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/valuation/model"
	"forklift-training/internal/valuation/repository"
)

// newTestProvider 构造测试用 CoefficientProvider
// DB 不可用时返回 nil + skip 标记
func newTestProvider(t *testing.T) (*CoefficientProvider, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = "postgresql://luohao:123456@localhost:5432/forklift_valuation?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("跳过测试：DB 不可用: %v", err)
	}
	// 简单 ping
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("跳过测试：DB ping 失败: %v", err)
	}
	dictRepo := repository.NewDictionaryRepository(pool)
	return NewCoefficientProvider(dictRepo), pool
}

// TestCalcKTime_BoundaryCases 覆盖 Kt 各典型年限与两种动力类型
// 期望值与开发方案表一致：
//   电动 λ=0.12：t=1→0.887 / t=3→0.698 / t=5→0.549 / t=8→0.382 / t=10→0.301
//   内燃 λ=0.10：t=1→0.905 / t=3→0.741 / t=5→0.607 / t=8→0.449 / t=10→0.368
func TestCalcKTime_BoundaryCases(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()

	type testCase struct {
		name     string
		pt       model.PowerType
		years    int
		expected float64
	}
	cases := []testCase{
		// 电动
		{"electric_1y", model.PowerTypeElectric, 1, 0.887},
		{"electric_3y", model.PowerTypeElectric, 3, 0.698},
		{"electric_5y", model.PowerTypeElectric, 5, 0.549},
		{"electric_8y", model.PowerTypeElectric, 8, 0.382},
		{"electric_10y", model.PowerTypeElectric, 10, 0.301},
		// 内燃
		{"combustion_1y", model.PowerTypeCombustion, 1, 0.905},
		{"combustion_3y", model.PowerTypeCombustion, 3, 0.741},
		{"combustion_5y", model.PowerTypeCombustion, 5, 0.607},
		{"combustion_8y", model.PowerTypeCombustion, 8, 0.449},
		{"combustion_10y", model.PowerTypeCombustion, 10, 0.368},
	}

	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := CalcKTime(ctx, c.pt, 2025-c.years, 2025, provider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(res.KTime-c.expected) > 0.005 {
				t.Errorf("KTime = %.4f, want ≈ %.4f", res.KTime, c.expected)
			}
		})
	}
}

// TestCalcKTime_ZeroYear 边界：t=0 时 Kt=1
func TestCalcKTime_ZeroYear(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()

	ctx := context.Background()
	res, err := CalcKTime(ctx, model.PowerTypeElectric, 2025, 2025, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(res.KTime-1.0) > 1e-9 {
		t.Errorf("KTime at t=0 = %.6f, want 1.0", res.KTime)
	}
	if res.Age != 0 {
		t.Errorf("Age = %d, want 0", res.Age)
	}
}

// TestCalcKTime_NegativeYear 异常：成交年份早于出厂年份
func TestCalcKTime_NegativeYear(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()

	ctx := context.Background()
	_, err := CalcKTime(ctx, model.PowerTypeElectric, 2025, 2024, provider)
	if err != model.ErrInvalidYear {
		t.Errorf("expected ErrInvalidYear, got %v", err)
	}
}

// TestCalcKTime_UnknownPowerType 异常：未知动力类型
// 重构后 Kt 直接接收 PowerType，由上层 vehicle_types 派生
// 未知类型在 Kt 内部返回错误（不再使用 model.ErrInvalidForkliftType）
func TestCalcKTime_UnknownPowerType(t *testing.T) {
	provider, pool := newTestProvider(t)
	defer pool.Close()

	ctx := context.Background()
	_, err := CalcKTime(ctx, model.PowerType("hybrid"), 2020, 2025, provider)
	if err == nil {
		t.Error("expected error for unknown power type, got nil")
	}
}
