// 精选域的附件归属写面门禁（第十二波票 4）：featuredImageGate 纯函数表驱动 + 精选写面端到端
// （Create 拒绝外链、编辑未改不动）。简历域的同类门禁留在 internal/service/attachment_gate_test.go。
package featured

import (
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestFeaturedImageGate(t *testing.T) {
	const own = "/static/uploads/featured/a_1.webp"
	const ownR2 = "https://cdn.example.com/featured/b_2.webp"
	const external = "https://evil.example.com/x.png"
	cases := []struct {
		name       string
		oldCover   string
		oldContent string
		newCover   string
		newContent string
		wantErr    bool
	}{
		{"全空放行", "", "", "", "", false},
		{"创建：本站封面放行", "", "", own, "", false},
		{"创建：R2 本站封面放行", "", "", ownR2, "", false},
		{"创建：外链封面拒绝", "", "", external, "", true},
		{"创建：正文外链图拒绝", "", "", "", "![](" + external + ")", true},
		{"创建：正文本站图放行", "", "", "", "![](" + own + ")", false},
		{"编辑未改不动：存量外链封面原样保留放行", external, "", external, "", false},
		{"编辑未改不动：存量外链正文图不动放行", "", "![](" + external + ")", "", "![](" + external + ")", false},
		{"编辑：新增外链正文图拒绝", "", "![](" + external + ")", "", "![](" + external + ")\n![](" + external + "2.png)", true},
		{"编辑：封面换成本站图放行", external, "", own, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := featuredImageGate(c.oldCover, c.oldContent, c.newCover, c.newContent)
			if (err != nil) != c.wantErr {
				t.Fatalf("featuredImageGate() err = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

func TestFeaturedWriteSurfaceGate(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())

	own := "/static/uploads/featured/a_1.webp"
	external := "https://evil.example.com/x.png"

	// 创建面：外链封面 → 拒绝
	if _, err := svc.Create(FeaturedContentInput{Title: "外链封面", Category: "company", CoverImage: external}); err == nil {
		t.Fatal("Create 外链封面应报错")
	}
	// 创建面：本站封面 + 本站正文图 → 放行
	created, err := svc.Create(FeaturedContentInput{Title: "本站封面", Category: "company", CoverImage: own, Content: "正文 ![" + own + "](" + own + ")"})
	if err != nil {
		t.Fatalf("Create 本站图应成功: %v", err)
	}
	// 直接种入一条存量外链数据（绕过写面 = 历史数据形态）
	legacy := model.FeaturedContent{Title: "存量外链", Summary: "s", Content: "![](https://legacy/x.png)", CoverImage: "https://legacy/c.png", Category: "company", Status: 0}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	// 编辑未改不动：只改标题 → 放行
	newTitle := "存量外链改标题"
	if _, err := svc.Update(legacy.ContentID, FeaturedContentUpdateInput{Title: &newTitle}); err != nil {
		t.Fatalf("Update 仅改标题应放行存量外链: %v", err)
	}
	// 编辑：新增另一外链封面 → 拒绝
	badCover := "https://new-evil/x.png"
	if _, err := svc.Update(legacy.ContentID, FeaturedContentUpdateInput{CoverImage: &badCover}); err == nil {
		t.Fatal("Update 新增外链封面应报错")
	}
	// 编辑：封面换成本站 → 放行
	if _, err := svc.Update(created.ContentID, FeaturedContentUpdateInput{CoverImage: &own}); err != nil {
		t.Fatalf("Update 本站封面应放行: %v", err)
	}
}
