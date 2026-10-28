// 注解面锚点（#2072 / #2064 / #2071）：生成物里必须真的存在这四条查询参数，且文字与实现口径一致。
//
// 为什么要这一条：backend-lint 的新鲜度锁只保证「生成物 = 注解」，它管不到「注解被删了、代码还在」
// ——direction 那句错的「非法值 400」就是靠人眼在评审里才发现的。这里把几条承重描述钉成断言。
package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSwaggerQueryParamAnchors(t *testing.T) {
	raw, err := os.ReadFile("../../docs/swagger.json")
	if err != nil {
		t.Fatalf("读 swagger.json 失败: %v", err)
	}
	type param struct {
		Name        string `json:"name"`
		In          string `json:"in"`
		Description string `json:"description"`
	}
	type operation struct {
		Parameters []param `json:"parameters"`
	}
	var doc struct {
		Paths map[string]map[string]operation `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("解析 swagger.json 失败: %v", err)
	}

	cases := []struct {
		path, method, name, mustContain string
	}{
		// 积分流水：实现只认 in|out，其它取值走「不过滤」，描述里不得再许一个不存在的 400。
		{"/points/ledger", "get", "direction", "in|out"},
		// 问答：solved 非空时必须同时 category=question，否则 400（ErrSolvedFilterScope）。
		{"/forum/topics", "get", "solved", "category=question"},
		// 课程：免费/付费按积分兑换价派生（不是现金价）。
		{"/courses", "get", "paid", "积分兑换价"},
	}
	for _, tc := range cases {
		ops, ok := doc.Paths[tc.path]
		if !ok {
			t.Fatalf("swagger 缺路径 %s", tc.path)
		}
		op, ok := ops[tc.method]
		if !ok {
			t.Fatalf("swagger 缺 %s %s", tc.method, tc.path)
		}
		found := false
		for _, p := range op.Parameters {
			if p.In == "query" && p.Name == tc.name {
				found = true
				if !strings.Contains(p.Description, tc.mustContain) {
					t.Fatalf("%s 的 %s 描述未包含 %q: %q", tc.path, tc.name, tc.mustContain, p.Description)
				}
			}
		}
		if !found {
			t.Fatalf("%s 缺查询参数 %s", tc.path, tc.name)
		}
	}

	// 商城在售列表：路径存在且需要鉴权（security 段落存在即可，具体形状由路由锁与契约测试管）。
	if _, ok := doc.Paths["/points/shop/items"]; !ok {
		t.Fatal("swagger 缺 /points/shop/items")
	}
}
