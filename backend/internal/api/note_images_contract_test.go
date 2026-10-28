// 契约测试 #2069：笔记图片附件（多图读写 + 悬空清理沿用既有策略）。
//
// 守的边界：
//   - 带图创建 → 回显地址与顺序原样；列表也带回；
//   - 更新**整组替换**（换图 / 传空数组清空）；
//   - 超数量 / 外链 / 非图片扩展名 / 站内但文件不存在 ⇒ 400（与论坛图片通道同一套判据）；
//   - **纯文本笔记行为一字不动**（不带 images 的老客户端照常工作，回显多一个空集）；
//   - 两种形态都覆盖：独立笔记（/api/notes）与题目笔记（/api/questions/{id}/note）。
//
// 「删除笔记后旧图进入可清理集」那一条在 internal/forum/image_service_test.go 里锁（清理引用集
// 是那一侧的事，笔记域只负责让行消失）。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/forum"
	"forklift-training/internal/note"
	"forklift-training/internal/questioninteraction"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// noteImageEnv 笔记图片契约的装配（学员 token + 三条蓝图）。
type noteImageEnv struct {
	r     *gin.Engine
	token string
}

func newNoteImageEnv(t *testing.T, db *gorm.DB) *noteImageEnv {
	t.Helper()
	testutil.SetTestGinMode()
	pwd, _ := core.HashPassword("student123")
	stu := seedStudent(t, db, "stuNoteImg", pwd)

	cfg := &config.Config{JWTSecretKey: "note-image-secret", JWTExpiresHours: 2, AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	deps := newContractDepsWithStorage(t, db, cfg, storage.NewLocalStorage(t.TempDir()))
	r := gin.New()
	api := r.Group("/api")
	forum.RegisterRoutes(api, deps.RouterDeps().Session, deps.ForumSvc, deps.ForumModSvc, deps.ForumImageSvc)
	note.RegisterRoutes(api, deps.RouterDeps().Session, deps.RouterDeps().CredentialScope, deps.NoteSvc)
	questioninteraction.RegisterRoutes(api, deps.RouterDeps().Session, deps.RouterDeps().CredentialScope,
		deps.QuestionCommentSvc, deps.NoteSvc, deps.QuestionKnowledgeSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: "hrwai_token"}).
		Issue(int(stu.ID), stu.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return &noteImageEnv{r: r, token: token}
}

// uploadForumImage 走论坛图片通道真上传一张 PNG（#2069：笔记不新建上传通道）。
func (e *noteImageEnv) uploadForumImage(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "note-shot.png")
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := fw.Write(pngBytes); err != nil {
		t.Fatalf("写 multipart 失败: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/forum/upload-image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+e.token)
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("上传图片失败: %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Data.URL == "" {
		t.Fatalf("上传未返回 url: %s", w.Body.String())
	}
	return env.Data.URL
}

// noteDTO 独立笔记的返回形状（本用例关心的两格）。
type noteDTO struct {
	ID      int      `json:"id"`
	Content string   `json:"content"`
	Images  []string `json:"images"`
}

// createNote 建一条独立笔记，返回状态码与解出的 DTO。
func (e *noteImageEnv) createNote(t *testing.T, body map[string]any) (int, noteDTO) {
	t.Helper()
	rec := doWithToken(t, e.r, e.token, http.MethodPost, "/api/notes", body)
	var env struct {
		Data noteDTO `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Data
}

func assertNoteImageContract(t *testing.T, db *gorm.DB) {
	e := newNoteImageEnv(t, db)
	img1 := e.uploadForumImage(t)
	img2 := e.uploadForumImage(t)

	// ===== 1. 带图创建：回显地址与顺序原样 =====
	code, created := e.createNote(t, map[string]any{"content": "第三章的液压图", "images": []string{img1, img2}})
	if code != http.StatusCreated {
		t.Fatalf("带图建笔记应 201, got %d", code)
	}
	if len(created.Images) != 2 || created.Images[0] != img1 || created.Images[1] != img2 {
		t.Fatalf("图片应原样回显且保序, got %+v", created.Images)
	}

	// ===== 2. 纯文本回归：不带 images 照常工作，回显是空集（不是 null）=====
	code, plain := e.createNote(t, map[string]any{"content": "纯文本笔记"})
	if code != http.StatusCreated {
		t.Fatalf("纯文本笔记应 201, got %d", code)
	}
	if plain.Images == nil || len(plain.Images) != 0 {
		t.Fatalf("不带图时 images 应是空集（不是 null）, got %+v", plain.Images)
	}

	// ===== 3. 列表带回图片 =====
	rec := doWithToken(t, e.r, e.token, http.MethodGet, "/api/notes?scope=standalone", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("列笔记应 200, got %d", rec.Code)
	}
	var listEnv struct {
		Data struct {
			Items []noteDTO `json:"items"`
			Total int64     `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &listEnv)
	var listed noteDTO
	for _, it := range listEnv.Data.Items {
		if it.ID == created.ID {
			listed = it
		}
	}
	if len(listed.Images) != 2 {
		t.Fatalf("列表应带回 2 张图, got %+v (total=%d)", listed.Images, listEnv.Data.Total)
	}

	// ===== 4. 更新：整组替换 =====
	rec = doWithToken(t, e.r, e.token, http.MethodPut, "/api/notes/"+itoa(created.ID),
		map[string]any{"content": "只留一张", "images": []string{img2}})
	if rec.Code != http.StatusOK {
		t.Fatalf("改笔记应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var updated noteDTO
	{
		var env struct {
			Data noteDTO `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		updated = env.Data
	}
	if len(updated.Images) != 1 || updated.Images[0] != img2 {
		t.Fatalf("更新应整组替换成 [img2], got %+v", updated.Images)
	}

	// 空数组 = 清空图片（与正文「传什么就是什么」同语义）
	rec = doWithToken(t, e.r, e.token, http.MethodPut, "/api/notes/"+itoa(created.ID),
		map[string]any{"content": "清空图片", "images": []string{}})
	if rec.Code != http.StatusOK {
		t.Fatalf("清空图片应 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"images":[]`) {
		t.Fatalf("清空后应发 [], got %s", rec.Body.String())
	}

	// ===== 5. 四条输入校验（与论坛图片通道同一套判据）=====
	cases := []struct {
		name   string
		images []string
	}{
		{"超过 3 张", []string{img1, img2, img1, img2}},
		{"外链", []string{"https://evil.example.com/a.png"}},
		{"非图片扩展名", []string{"/static/uploads/images/forum/not-image.pdf"}},
		{"站内但没有这个文件", []string{"/static/uploads/images/forum/ghost_1.png"}},
	}
	for _, c := range cases {
		if code, _ := e.createNote(t, map[string]any{"content": "非法图片", "images": c.images}); code != http.StatusBadRequest {
			t.Fatalf("%s 应 400, got %d", c.name, code)
		}
	}

	// ===== 6. 题目笔记形态（/api/questions/{id}/note）同样带图 =====
	q := testutil.SeedQuestion(t, db, "single_choice", "液压泵异响的判断", "A")
	rec = doWithToken(t, e.r, e.token, http.MethodPut, "/api/questions/"+itoa(q.ID)+"/note",
		map[string]any{"content": "这题的图我截了", "images": []string{img1}})
	if rec.Code != http.StatusOK {
		t.Fatalf("题目笔记带图应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var qNote struct {
		Data struct {
			Content    string   `json:"content"`
			Images     []string `json:"images"`
			QuestionID *int     `json:"question_id"`
			UserID     int      `json:"user_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &qNote); err != nil {
		t.Fatalf("解析题目笔记失败: %v", err)
	}
	if len(qNote.Data.Images) != 1 || qNote.Data.Images[0] != img1 {
		t.Fatalf("题目笔记应回显图片, got %+v", qNote.Data.Images)
	}
	// 响应只增不减：既有五个键一个不少
	if qNote.Data.Content != "这题的图我截了" || qNote.Data.QuestionID == nil || *qNote.Data.QuestionID != q.ID || qNote.Data.UserID == 0 {
		t.Fatalf("题目笔记响应应保留既有键, got %+v", qNote.Data)
	}
	// 读回来也带图
	rec = doWithToken(t, e.r, e.token, http.MethodGet, "/api/questions/"+itoa(q.ID)+"/note", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"images":["`) {
		t.Fatalf("读题目笔记应带回图片, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestNoteImagesContract_OnSqlite(t *testing.T) {
	assertNoteImageContract(t, testutil.NewMemoryDB(t))
}

func TestNoteImagesContract_OnPostgres(t *testing.T) {
	assertNoteImageContract(t, testutil.NewPostgresDB(t))
}
