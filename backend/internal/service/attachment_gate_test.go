// 附件归属写面门禁的行为测试（第十二波票 4）：
// featuredImageGate 纯函数表驱动 + 精选/简历两域的写面端到端（Create 拒绝外链、编辑未改不动）。
package service

import (
	"encoding/json"
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
	svc := NewFeaturedService(db, nil, zap.NewNop())

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

func TestJobCardAttachmentOwnershipGate(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewJobCardService(db, nil, zap.NewNop())

	const own = "/static/uploads/resumes/images/p_1.webp"
	const external = "https://evil.example.com/p.jpg"
	photosPtr := func(v any) *json.RawMessage {
		b, _ := json.Marshal(v)
		rm := json.RawMessage(b)
		return &rm
	}

	// 创建面：外链工作照 → 拒绝
	if _, err := svc.Upsert(101, JobCardInput{Photos: photosPtr([]string{external})}); err == nil {
		t.Fatal("创建面外链工作照应报错")
	}
	// 创建面：本站工作照 → 放行
	if _, err := svc.Upsert(101, JobCardInput{Photos: photosPtr([]string{own})}); err != nil {
		t.Fatalf("创建面本站工作照应放行: %v", err)
	}
	// 存量外链直接种库（门禁开启前的历史数据）
	if err := db.Create(&model.JobCard{UserID: 102, Photos: model.JSONB([]byte(`["` + external + `"]`))}).Error; err != nil {
		t.Fatal(err)
	}
	// 编辑未改不动：重提交含存量外链 → 放行
	if _, err := svc.Upsert(102, JobCardInput{Photos: photosPtr([]string{external})}); err != nil {
		t.Fatalf("编辑未改不动应放行存量外链: %v", err)
	}
	// 编辑：在存量外链之外新增一条外链 → 拒绝
	if _, err := svc.Upsert(102, JobCardInput{Photos: photosPtr([]string{external, "https://evil2/x.jpg"})}); err == nil {
		t.Fatal("新增外链工作照应报错")
	}
	// 证书原图同判据：新增外链 image_urls → 拒绝
	certs := photosPtr([]any{map[string]any{"cert_no": "C1", "image_urls": []string{external}}})
	if _, err := svc.Upsert(101, JobCardInput{ResumeCertifications: certs}); err == nil {
		t.Fatal("证书新增外链原图应报错")
	}
	// 证书本站原图 → 放行
	certsOwn := photosPtr([]any{map[string]any{"cert_no": "C1", "image_urls": []string{own}}})
	if _, err := svc.Upsert(101, JobCardInput{ResumeCertifications: certsOwn}); err != nil {
		t.Fatalf("证书本站原图应放行: %v", err)
	}
}
