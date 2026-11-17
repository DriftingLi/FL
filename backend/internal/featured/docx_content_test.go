// 内容精选「导入原始 .docx」的语义锁（2026-11）。
//
// 断言口径与 schedule_test.go 一致：只断言**外部行为** —— 一次写请求之后库里两列是什么、
// 哪些输入该被 400 拒、切成 docx 之后 markdown 正文有没有被清掉。
// 不断言 SQL、不断言内部字段：换实现时本文件应当一字不改。
package featured

import (
	"errors"
	"testing"

	"forklift-training/internal/filestore"
)

// docx 形态的合法 URL（取自 filestore 前缀常量，避免测试里手写第二份路径）。
func docxURL() string {
	return "/static/uploads/" + filestore.FeaturedDocxDirPrefix + "/plan_1700000000000.docx"
}

// TestCreateDocxContentType docx 文章的三列落定：
// content_type=docx、docx_url 原样存、content 清空（正文不落在 Markdown 列）。
func TestCreateDocxContentType(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)

	got, err := svc.Create(FeaturedContentInput{
		Title:       "导入的 Word",
		Category:    "industry",
		Content:     "这段 Markdown 不该被保留",
		ContentType: FeaturedContentTypeDocx,
		DocxURL:     docxURL(),
	})
	if err != nil {
		t.Fatalf("创建 docx 文章失败: %v", err)
	}
	if got.ContentType != FeaturedContentTypeDocx {
		t.Fatalf("DTO content_type = %q，期望 docx", got.ContentType)
	}
	if got.DocxURL != docxURL() {
		t.Fatalf("DTO docx_url = %q，期望 %q", got.DocxURL, docxURL())
	}

	row := loadFeatured(t, db, got.ContentID)
	if row.ContentType != FeaturedContentTypeDocx {
		t.Errorf("库 content_type = %q，期望 docx", row.ContentType)
	}
	if row.Content != "" {
		t.Errorf("库 content = %q，docx 文章应清空 Markdown 正文", row.Content)
	}
	if row.DocxURL != docxURL() {
		t.Errorf("库 docx_url = %q，期望 %q", row.DocxURL, docxURL())
	}
}

// TestCreateMarkdownDropsDocxURL 未点名类型 = markdown（与库层 DEFAULT 同义），
// 且顺手带的 docx_url 不许落库 —— 否则库里会出现「类型说 markdown、文件说 docx」的半截行。
func TestCreateMarkdownDropsDocxURL(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)

	got, err := svc.Create(FeaturedContentInput{Title: "普通文章", Category: "industry", Content: "# 正文"})
	if err != nil {
		t.Fatalf("创建 markdown 文章失败: %v", err)
	}
	if got.ContentType != FeaturedContentTypeMarkdown {
		t.Fatalf("省略类型时 content_type = %q，期望 markdown", got.ContentType)
	}
	row := loadFeatured(t, db, got.ContentID)
	if row.Content != "# 正文" {
		t.Errorf("库 content = %q，期望保留 Markdown 正文", row.Content)
	}
	if row.DocxURL != "" {
		t.Errorf("库 docx_url = %q，markdown 文章应为空", row.DocxURL)
	}
}

// TestCreateDocxRejectsInvalidInputs 四条输入事实各自具名报错（都该落 400，不是 500）。
func TestCreateDocxRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		in   FeaturedContentInput
		want error
	}{
		{
			name: "类型不在两档内",
			in:   FeaturedContentInput{Title: "t", Category: "industry", ContentType: "pdf"},
			want: ErrFeaturedContentTypeInvalid,
		},
		{
			name: "docx 却没带文件",
			in:   FeaturedContentInput{Title: "t", Category: "industry", ContentType: FeaturedContentTypeDocx},
			want: ErrFeaturedDocxURLInvalid,
		},
		{
			name: "docx 带了外站文件",
			in: FeaturedContentInput{Title: "t", Category: "industry",
				ContentType: FeaturedContentTypeDocx, DocxURL: "https://evil.example.com/a.docx"},
			want: ErrFeaturedDocxURLInvalid,
		},
		{
			name: "docx 带了非 docx 子目录的文件",
			in: FeaturedContentInput{Title: "t", Category: "industry",
				ContentType: FeaturedContentTypeDocx,
				DocxURL:     "/static/uploads/featured/cover_1.webp"},
			want: ErrFeaturedDocxURLInvalid,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newFeaturedTestSvc(t)
			_, err := svc.Create(c.in)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v，期望 %v", err, c.want)
			}
		})
	}
}

// TestUpdateSwitchesContentType markdown ⇄ docx 的来回切换：
//   - 切到 docx：content 清空、docx_url 落库；
//   - 切回 markdown：docx_url 清空（文件不进这一列）、正文按新提交的 Markdown 落库。
func TestUpdateSwitchesContentType(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)

	created, err := svc.Create(FeaturedContentInput{Title: "先写 Markdown", Category: "industry", Content: "# 原正文"})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	id := created.ContentID

	toDocx := FeaturedContentTypeDocx
	url := docxURL()
	if _, err := svc.Update(id, FeaturedContentUpdateInput{ContentType: &toDocx, DocxURL: &url}); err != nil {
		t.Fatalf("切到 docx 失败: %v", err)
	}
	row := loadFeatured(t, db, id)
	if row.ContentType != FeaturedContentTypeDocx || row.DocxURL != url || row.Content != "" {
		t.Fatalf("切到 docx 后 content_type=%q docx_url=%q content=%q", row.ContentType, row.DocxURL, row.Content)
	}

	toMarkdown := FeaturedContentTypeMarkdown
	newBody := "# 新正文"
	if _, err := svc.Update(id, FeaturedContentUpdateInput{ContentType: &toMarkdown, Content: &newBody}); err != nil {
		t.Fatalf("切回 markdown 失败: %v", err)
	}
	row = loadFeatured(t, db, id)
	if row.ContentType != FeaturedContentTypeMarkdown {
		t.Errorf("切回后 content_type = %q，期望 markdown", row.ContentType)
	}
	if row.Content != newBody {
		t.Errorf("切回后 content = %q，期望 %q", row.Content, newBody)
	}
	if row.DocxURL != "" {
		t.Errorf("切回后 docx_url = %q，markdown 文章应为空", row.DocxURL)
	}
}

// TestUpdateDocxKeepsExistingFile 存量文件放行（与 featuredImageGate 同一判据）：
// 编辑一篇 docx 文章时只改标题、不带 docx_url，不得报「文档地址无效」。
func TestUpdateDocxKeepsExistingFile(t *testing.T) {
	svc, db := newFeaturedTestSvc(t)

	created, err := svc.Create(FeaturedContentInput{
		Title: "docx 文章", Category: "industry",
		ContentType: FeaturedContentTypeDocx, DocxURL: docxURL(),
	})
	if err != nil {
		t.Fatalf("创建 docx 文章失败: %v", err)
	}
	newTitle := "只改标题"
	if _, err := svc.Update(created.ContentID, FeaturedContentUpdateInput{Title: &newTitle}); err != nil {
		t.Fatalf("只改标题失败（存量 docx 文件应放行）: %v", err)
	}
	row := loadFeatured(t, db, created.ContentID)
	if row.Title != newTitle {
		t.Errorf("title = %q，期望 %q", row.Title, newTitle)
	}
	if row.DocxURL != docxURL() {
		t.Errorf("docx_url = %q，期望原样保留 %q", row.DocxURL, docxURL())
	}
}
