// 收藏条目的缺省态形状锁（#2067）：三种语义各归其位，生成面必须与运行一致。
//
//   - company / region：x-optional ⇒ 只有职位条目有这两个键（其余类型**键缺席**，不是空串）；
//   - tags：nonnil ⇒ 键恒在、恒为数组（非题目类型也发 []，不是 null、也不是缺席）。
package favorite

import (
	"encoding/json"
	"testing"
)

func TestFavoriteDTOShapeLock(t *testing.T) {
	keysOf := func(v any) map[string]bool {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal 失败: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal 失败: %v", err)
		}
		out := make(map[string]bool, len(m))
		for k := range m {
			out[k] = true
		}
		return out
	}

	course := FavoriteDTO{FavoriteID: 1, TargetType: FavoriteTargetCourse, TargetID: 2, Title: "课程", CreatedAt: "", Tags: []FavoriteTagDTO{}}
	ck := keysOf(course)
	if ck["company"] || ck["region"] {
		t.Fatalf("非职位条目不该有 company/region 键: %+v", ck)
	}
	if !ck["tags"] {
		t.Fatalf("tags 应恒在（nonnil 表态）: %+v", ck)
	}

	jobDTO := FavoriteDTO{FavoriteID: 2, TargetType: FavoriteTargetJob, TargetID: 3, Title: "职位", CreatedAt: "",
		Company: "测试企业", Region: "上海", Tags: []FavoriteTagDTO{}}
	jk := keysOf(jobDTO)
	if !jk["company"] || !jk["region"] {
		t.Fatalf("职位条目应带 company/region: %+v", jk)
	}

	question := FavoriteDTO{FavoriteID: 3, TargetType: FavoriteTargetQuestion, TargetID: 4, Title: "题干", CreatedAt: "",
		Tags: []FavoriteTagDTO{{TagID: 7, TagName: "法规"}}}
	qk := keysOf(question)
	if !qk["tags"] || qk["company"] || qk["region"] {
		t.Fatalf("题目条目应只有 tags: %+v", qk)
	}
}
