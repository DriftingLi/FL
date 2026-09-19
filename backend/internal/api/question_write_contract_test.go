// Package api 第十二波票 6 契约：题库 typed 写面——
//   - 写面拒收 status 通道（状态迁移只经显式动作）；
//   - 字段类型不符即 400（不再静默落零值）；
//   - 新增「提交审核」端点（draft→pending）；
//   - 审核不变式：讲师改已发布题内容回 pending，管理员改动即时生效。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func newQuestionWriteEnv(t *testing.T) (*gin.Engine, *config.Config, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "qwrite-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterQuestionBankRoutes(api, deps.RouterDeps(), deps.QuestionBankSvc, deps.FileSvc)
	return r, cfg, db
}

// qwriteIssue 按角色签发 token（tutor 走 CapQuestionAuthor，admin 走讲师+审核能力）。
func qwriteIssue(t *testing.T, cfg *config.Config, role string) string {
	t.Helper()
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	tok, err := sess.Issue(1001, "qwrite_"+role, role)
	if err != nil {
		t.Fatalf("签发 %s token 失败: %v", role, err)
	}
	return tok
}

func TestQuestionWriteSurfaceRejectsStatus(t *testing.T) {
	r, cfg, _ := newQuestionWriteEnv(t)
	tutor := qwriteIssue(t, cfg, "tutor")

	body := map[string]any{"type": "single_choice", "content": "带 status 的创建", "options": map[string]string{"A": "甲", "B": "乙"}, "answer": "A", "status": "published"}
	rec := doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("创建携带 status 应 400, got %d %s", rec.Code, rec.Body.String())
	}

	// 正常创建：typed 入参、固定 pending
	delete(body, "status")
	rec = doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("合法创建应 201, got %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.Status != "pending" {
		t.Fatalf("新题应固定入 pending, got %q", created.Data.Status)
	}
	qURL := "/api/question-bank/questions/" + strconv.Itoa(created.Data.ID)

	// 更新携带 status 同样拒收
	rec = doWithToken(t, r, tutor, http.MethodPut, qURL, map[string]any{"status": "draft"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("更新携带 status 应 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuestionWriteTypedFieldMismatchFails(t *testing.T) {
	r, cfg, _ := newQuestionWriteEnv(t)
	tutor := qwriteIssue(t, cfg, "tutor")
	// score 传字符串：typed 绑定必须拒绝（旧 map 面会静默落零值）
	rec := doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions",
		map[string]any{"type": "single_choice", "content": "分值类型不符", "options": map[string]string{"A": "甲"}, "answer": "A", "score": "abc"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("score 类型不符应 400, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestQuestionSubmitAction(t *testing.T) {
	r, cfg, db := newQuestionWriteEnv(t)
	tutor := qwriteIssue(t, cfg, "tutor")
	q := model.Question{Type: "single_choice", Content: "待提交题", Answer: "A", Status: "draft", CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&q).Error; err != nil {
		t.Fatal(err)
	}
	rec := doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions/"+strconv.Itoa(q.ID)+"/submit", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("draft 提交应 200, got %d %s", rec.Code, rec.Body.String())
	}
	// 再提交：非 draft → 400（状态前置哨兵）
	rec = doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions/"+strconv.Itoa(q.ID)+"/submit", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("pending 再提交应 400, got %d", rec.Code)
	}
	rec = doWithToken(t, r, tutor, http.MethodPost, "/api/question-bank/questions/999999/submit", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在提交应 404, got %d", rec.Code)
	}
}

func TestQuestionReviewInvariantOverHTTP(t *testing.T) {
	r, cfg, db := newQuestionWriteEnv(t)
	tutor := qwriteIssue(t, cfg, "tutor")
	admin := qwriteIssue(t, cfg, "admin")

	statusOf := func(id int) string {
		var row model.Question
		if err := db.First(&row, id).Error; err != nil {
			t.Fatal(err)
		}
		return row.Status
	}
	mkPublished := func(content string) int {
		q := model.Question{Type: "single_choice", Content: content, Answer: "A", Status: "published", CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
		if err := db.Create(&q).Error; err != nil {
			t.Fatal(err)
		}
		return q.ID
	}

	// 讲师改已发布题内容 → 回 pending
	q1 := mkPublished("讲师会改的题")
	if rec := doWithToken(t, r, tutor, http.MethodPut, "/api/question-bank/questions/"+strconv.Itoa(q1), map[string]any{"content": "改后的题干"}); rec.Code != http.StatusOK {
		t.Fatalf("讲师更新应 200, got %d %s", rec.Code, rec.Body.String())
	}
	if got := statusOf(q1); got != "pending" {
		t.Fatalf("讲师改内容后应回 pending, got %q", got)
	}
	// 讲师重提交相同内容 → 编辑未改不动，留在池内
	q2 := mkPublished("原样重交的题")
	if rec := doWithToken(t, r, tutor, http.MethodPut, "/api/question-bank/questions/"+strconv.Itoa(q2), map[string]any{"content": "原样重交的题"}); rec.Code != http.StatusOK {
		t.Fatalf("原样更新应 200, got %d", rec.Code)
	}
	if got := statusOf(q2); got != "published" {
		t.Fatalf("未改内容不得触发重审, got %q", got)
	}
	// 管理员改已发布题内容 → 即时生效保持 published
	q3 := mkPublished("管理员改错字的题")
	if rec := doWithToken(t, r, admin, http.MethodPut, "/api/question-bank/questions/"+strconv.Itoa(q3), map[string]any{"content": "改了错字"}); rec.Code != http.StatusOK {
		t.Fatalf("管理员更新应 200, got %d %s", rec.Code, rec.Body.String())
	}
	if got := statusOf(q3); got != "published" {
		t.Fatalf("管理员改动应即时生效, got %q", got)
	}
}
