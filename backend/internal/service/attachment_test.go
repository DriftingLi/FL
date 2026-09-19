// 附件归属单 module 的表驱动测试（第十二波票 4）：
// 只测三个导出函数的外部行为——本站判定、URL→key、multipart 读取。
package service

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"
)

func TestIsSiteAttachmentURL(t *testing.T) {
	const prefix = ForumImageDirPrefix // "images/forum"
	cases := []struct {
		name string
		u    string
		want bool
	}{
		{"空串", "", false},
		{"纯空白", "   ", false},
		{"local 形态", "/static/uploads/images/forum/a_1.webp", true},
		{"local 带 query 尾巴", "/static/uploads/images/forum/a.webp?v=2", true},
		{"R2 https 任意域名", "https://cdn.example.com/images/forum/a_1.webp", true},
		{"R2 http", "http://cdn.example.com/images/forum/a_1.webp", true},
		{"R2 带 query 尾巴", "https://cdn.example.com/images/forum/a.webp?x=1", true},
		// 现行立场：R2 形态接受任意域名（含可疑路径），收紧属另立裁定——本用例钉死语义不因收编而变。
		{"任意域名含前缀路径（现行判本站）", "https://evil/x/images/forum/a.png", true},
		{"根相对非 /static/uploads（判非本站）", "/images/forum/a.png", false},
		{"前缀在 URL 起点（无域名段，idx=0 判非本站）", "images/forum/a.png", false},
		{"异前缀", "/static/uploads/featured/a.png", false},
		{"仅目录名相似", "https://cdn.example.com/images/forumx/a.png", false},
		{"缺尾部斜杠", "https://cdn.example.com/images/forum", false},
		{"大小写不同（敏感，判非本站）", "https://cdn.example.com/Images/Forum/a.png", false},
		{"LOCAL 前缀大小写不同", "/STATIC/uploads/images/forum/a.png", false},
		{"非 http scheme 的绝对 URL", "ftp://cdn.example.com/images/forum/a.png", false},
		{"首尾空白被容忍", "  https://cdn.example.com/images/forum/a.png  ", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsSiteAttachmentURL(c.u, prefix); got != c.want {
				t.Fatalf("IsSiteAttachmentURL(%q, %q) = %v, want %v", c.u, prefix, got, c.want)
			}
		})
	}
}

func TestAttachmentKey(t *testing.T) {
	cases := []struct {
		name   string
		u      string
		prefix string
		want   string
	}{
		{"local 论坛图", "/static/uploads/images/forum/a_1.webp", ForumImageDirPrefix, "images/forum/a_1.webp"},
		{"R2 论坛图", "https://cdn.example.com/images/forum/a_1.webp", ForumImageDirPrefix, "images/forum/a_1.webp"},
		{"直接给 key", "images/forum/a_1.webp", ForumImageDirPrefix, "images/forum/a_1.webp"},
		{"未命中前缀", "/static/uploads/featured/a.png", ForumImageDirPrefix, ""},
		{"投稿文件 local", "/static/uploads/contributions/n.pdf", ContributionFileDirPrefix, "contributions/n.pdf"},
		{"投稿文件 R2", "https://cdn.example.com/contributions/n.pdf", ContributionFileDirPrefix, "contributions/n.pdf"},
		{"精选封面", "https://cdn.example.com/featured/cover.png", FeaturedImageDirPrefix, "featured/cover.png"},
		{"简历工作照", "/static/uploads/resumes/images/p_1.webp", ResumeImageDirPrefix, "resumes/images/p_1.webp"},
		{"异前缀不误提（resumes 不吞 resumes/images）", "/static/uploads/resumes/a.pdf", ResumeImageDirPrefix, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AttachmentKey(c.u, c.prefix); got != c.want {
				t.Fatalf("AttachmentKey(%q, %q) = %q, want %q", c.u, c.prefix, got, c.want)
			}
		})
	}
}

func TestReadMultipartFile(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("attachment-bytes")
	if _, err := fw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, "/upload", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	_, file, err := req.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadMultipartFile(file)
	if err != nil {
		t.Fatalf("ReadMultipartFile: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("ReadMultipartFile = %q, want %q", got, payload)
	}
}
