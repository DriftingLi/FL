package testutil

import (
	"encoding/json"
	"strings"
	"testing"
)

// MarshalKey 把出口结果序列化，取某个键发出的字面量（"[]" / "{}" 还是 "null"）。
//
// 为什么单独立这条：tag 是一句**关于出口的断言**，而断言可以被任何一次重构悄悄破坏 ——
// 有人把 `out := make([]T, 0, n)` 改成 `var out []T` 时，读源码那把表态锁不会红，
// 只有走真实出口、看发出的是 `[]` 还是 `null` 才会红。刻意不 marshal 零值 DTO：
// 零值切片必然是 nil，那不构成反例。
//
// 已知盲区（改判 JSONArray / JSONB 字段前必读）：本函数只看「这一次出口发出了什么」，
// 分不清「代码保证非 null」与「列默认值恰好是 '[]'」——同一形状的三格 JSONArray
// （recruit_service.go 的脱敏卡）就是例子：重建过的那格恒 `[]`，只加 len==0 守卫的两格
// 在列里存着 JSON `null` 时照样发出 null。
//
// 键缺席即判红（「被加了 omitempty ⇒ nonnil 表态就不成立了」）。
func MarshalKey(t *testing.T, v any, key string) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("响应不是对象: %s", b)
	}
	got, ok := m[key]
	if !ok {
		t.Fatalf("响应里没有键 %q（body=%s）—— 该字段被加了 omitempty？那 nonnil 表态就不成立了", key, b)
	}
	return string(got)
}

// AssertNonNilOutlets 逐键断言登记过的出口都发出**非 null**，并把两件会让判据空转的事判红：
// 表是空的、同一条事实被两张表各自举证。
//
// 判据原本写成「必须等于 `[]`」，那对切片是对的、对映射是错的：`map[K]V` 的恒非 null 形状是 `{}`，
// 于是 4 条 map 字段被判据自己挡在举证之外——一条只认一种形状的锁会把它没覆盖的那一类留在原地。
// 这里断言的是「键在场 + 值不是 null」两件事，改名或被 omitempty 掉都跑不掉。
//
// 参数是**若干张分域表**而不是一张：域包拆出去之后（ADR-0070），每个域在自己的包里声明一张
// `nonnilOutlets<域名>` 表并调本函数——判据本体只有这一份，各域不必各抄一个 runner。
func AssertNonNilOutlets(t *testing.T, tables []map[string]func(t *testing.T) any) {
	t.Helper()
	outlets := map[string]func(t *testing.T) any{}
	owner := map[string]int{}
	for i, tbl := range tables {
		for k, v := range tbl {
			if prev, dup := owner[k]; dup {
				t.Fatalf("%s 被第 %d 张与第 %d 张表各自举证——一条事实一个证据，留一处", k, prev, i)
			}
			owner[k] = i
			outlets[k] = v
		}
	}
	if len(outlets) == 0 {
		t.Fatal("证据表是空的：判据 5 会因此空转，这里必须同步红")
	}
	for key, build := range outlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			if got := MarshalKey(t, build(t), jsonKey); got == "null" {
				t.Errorf("声明 nonnil 的 %s 实际发出 null", key)
			}
		})
	}
}
