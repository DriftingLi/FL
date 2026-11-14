// 本文件：章节审核队列与批量操作的 HTTP 契约（#2499 spec C9/C10）——
// 队列排序与快照预览一致性、批量过审/驳回逐章状态与失败明示、审核结果站内信投递。
// 状态机语义单测在 course 包（chapter_review_queue_test.go），本处只锁 HTTP 面与通知投递。
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestChapterReviewQueueContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "queue-flow", Name: "队列流", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "queue-flow-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "队列流课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	// 三章：两章提交待审（按提交先后），一章留草稿不进队列
	var submitted []model.Chapter
	for i, title := range []string{"待审一章", "待审二章"} {
		ch := model.Chapter{CourseID: c.CourseID, Title: title, Content: "正文" + title,
			ContentType: "text", OrderNum: i + 1, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
		}
		submitted = append(submitted, ch)
	}
	draft := model.Chapter{CourseID: c.CourseID, Title: "未提交章", Content: "草稿",
		ContentType: "text", OrderNum: 3, CreatedAt: testutil.Now()}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("创建草稿章节失败: %v", err)
	}
	// 文件挂到第一章，验证快照预览带清单
	cf := model.ChapterFile{ChapterID: &submitted[0].ChapterID, FileURL: "/static/uploads/chapter/q.pdf",
		FileName: "q.pdf", ContentType: "document", FileSize: 12, CreatedAt: testutil.Now()}
	if err := db.Create(&cf).Error; err != nil {
		t.Fatalf("创建章节文件失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "queue-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))

	sign := func(uid int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(uid, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	tutorTok := sign(11, "tutor_queue", "tutor")
	admin := testutil.SeedAdmin(t, db, "admin_queue", "x")
	adminTok := sign(admin.AdminID, admin.Username, "admin")

	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != "" {
			buf = bytes.NewBufferString(body)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req, _ := http.NewRequest(method, path, buf)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 讲师逐章提交（顺序决定队列排序）
	for i := range submitted {
		if rec := do("POST", "/api/tutor/chapter/"+itoa(submitted[i].ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
			t.Fatalf("提交章节 %d 应 200, got %d: %s", submitted[i].ChapterID, rec.Code, rec.Body.String())
		}
	}

	// C9：队列按提交时间升序，草稿章不入队
	rec := do("GET", "/api/admin/chapter-reviews/pending", adminTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("查队列应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var queue struct {
		Data struct {
			Items []struct {
				ChapterID    int    `json:"chapter_id"`
				ChapterTitle string `json:"chapter_title"`
				CourseName   string `json:"course_name"`
				FileCount    int    `json:"file_count"`
				FrozenAt     string `json:"frozen_at"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatalf("解析队列响应失败: %v", err)
	}
	if queue.Data.Total != 2 || len(queue.Data.Items) != 2 {
		t.Fatalf("队列应有 2 章, got total=%d items=%d", queue.Data.Total, len(queue.Data.Items))
	}
	if queue.Data.Items[0].ChapterID != submitted[0].ChapterID {
		t.Fatalf("队列首条应为先提交的章节 %d, got %d", submitted[0].ChapterID, queue.Data.Items[0].ChapterID)
	}
	if queue.Data.Items[0].CourseName != "队列流课程" || queue.Data.Items[0].FrozenAt == "" {
		t.Fatalf("队列项缺课程名/冻结时刻: %+v", queue.Data.Items[0])
	}
	if queue.Data.Items[0].FileCount != 1 {
		t.Fatalf("第一章文件数 = %d, want 1", queue.Data.Items[0].FileCount)
	}

	// C9：快照预览与队列一致（正文 + 冻结文件清单）
	rec = do("GET", "/api/admin/chapter-reviews/"+itoa(submitted[0].ChapterID)+"/snapshot", adminTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("查快照应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var snap struct {
		Data struct {
			ChapterTitle string `json:"chapter_title"`
			Content      string `json:"content"`
			Files        []struct {
				FileName string `json:"file_name"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("解析快照响应失败: %v", err)
	}
	if snap.Data.Content != "正文待审一章" || snap.Data.ChapterTitle != "待审一章" {
		t.Fatalf("快照内容不一致: %+v", snap.Data)
	}
	if len(snap.Data.Files) != 1 || snap.Data.Files[0].FileName != "q.pdf" {
		t.Fatalf("快照文件清单应含 q.pdf, got %+v", snap.Data.Files)
	}
	// 草稿章无在审版本：快照 409
	if rec := do("GET", "/api/admin/chapter-reviews/"+itoa(draft.ChapterID)+"/snapshot", adminTok, ""); rec.Code != http.StatusConflict {
		t.Fatalf("无在审版本快照应 409, got %d: %s", rec.Code, rec.Body.String())
	}

	// C9：批量过审——逐章落状态
	batchBody := `{"chapter_ids":[` + itoa(submitted[0].ChapterID) + `,` + itoa(submitted[1].ChapterID) + `]}`
	rec = do("POST", "/api/admin/chapter-reviews/batch-approve", adminTok, batchBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("批量过审应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var approveRes struct {
		Data struct {
			ApprovedCount int `json:"approved_count"`
			Failures      []struct {
				ChapterID int    `json:"chapter_id"`
				Reason    string `json:"reason"`
			} `json:"failures"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &approveRes); err != nil {
		t.Fatalf("解析批量过审响应失败: %v", err)
	}
	if approveRes.Data.ApprovedCount != 2 || len(approveRes.Data.Failures) != 0 {
		t.Fatalf("批量过审结果 = %+v, want 2 成功 0 失败", approveRes.Data)
	}
	for i := range submitted {
		var got model.Chapter
		if err := db.First(&got, submitted[i].ChapterID).Error; err != nil {
			t.Fatalf("查章节失败: %v", err)
		}
		if got.ReviewStatus != model.ChapterReviewPublished {
			t.Fatalf("章节 %d 应已发布, got %q", submitted[i].ChapterID, got.ReviewStatus)
		}
	}
	// 队列已空
	rec = do("GET", "/api/admin/chapter-reviews/pending", adminTok, "")
	if err := json.Unmarshal(rec.Body.Bytes(), &queue); err != nil {
		t.Fatalf("解析队列响应失败: %v", err)
	}
	if queue.Data.Total != 0 {
		t.Fatalf("过审后队列应为空, got %d", queue.Data.Total)
	}

	// C10 站内信（#2537 已接通，ADR-0095）：收件人是**章节作者讲师**，落 notifications.tutor_id
	// ——不是 user_id（讲师在 hrwai_users 无行）。这里断言两件事：
	//   ①确实投递了（过审不再静默）；
	//   ②收件人落在 tutor_id 且等于该章节的提交人，user_id 为空（没有误投给撞号的学员）。
	var notices []model.Notification
	if err := db.Where("type = ?", notification.NotifTypeChapterReview).Find(&notices).Error; err != nil {
		t.Fatalf("查站内信失败: %v", err)
	}
	if len(notices) != 2 {
		t.Fatalf("批量过审 2 章应投递 2 条站内信, got %d", len(notices))
	}
	for _, n := range notices {
		if n.UserID != nil {
			t.Fatalf("讲师通知不得落 user_id（会投给撞号的无关学员），got user_id=%d", *n.UserID)
		}
		if n.TutorID == nil || *n.TutorID != 11 {
			t.Fatalf("通知收件人应为章节作者讲师 %d, got %v", 11, n.TutorID)
		}
		if !strings.Contains(n.Title, "通过") {
			t.Fatalf("过审通知标题应含「通过」, got %q", n.Title)
		}
		if !strings.Contains(string(n.Payload), `"published"`) {
			t.Fatalf("过审通知 payload 应携带 review_status=published, got %s", n.Payload)
		}
	}

	// 批量驳回：先让草稿章再进队列，再批量驳回（统一理由）
	if rec := do("POST", "/api/tutor/chapter/"+itoa(draft.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("草稿章提交应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rejectBody := `{"chapter_ids":[` + itoa(draft.ChapterID) + `],"reason":"正文缺少安全须知"}`
	rec = do("POST", "/api/admin/chapter-reviews/batch-reject", adminTok, rejectBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("批量驳回应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var rejectRes struct {
		Data struct {
			RejectedCount int `json:"rejected_count"`
			Failures      []struct {
				ChapterID int `json:"chapter_id"`
			} `json:"failures"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rejectRes); err != nil {
		t.Fatalf("解析批量驳回响应失败: %v", err)
	}
	if rejectRes.Data.RejectedCount != 1 || len(rejectRes.Data.Failures) != 0 {
		t.Fatalf("批量驳回结果 = %+v, want 1 成功 0 失败", rejectRes.Data)
	}
	var rejected model.Chapter
	if err := db.First(&rejected, draft.ChapterID).Error; err != nil {
		t.Fatalf("查驳回章节失败: %v", err)
	}
	if rejected.ReviewStatus != model.ChapterReviewDraft || rejected.RejectReason != "正文缺少安全须知" {
		t.Fatalf("驳回后 = %q/%q, want draft+统一理由", rejected.ReviewStatus, rejected.RejectReason)
	}
	// C10 驳回投递（#2537）：驳回也要发信且带理由——讲师靠理由才知道该改什么。
	var rejectNotices []model.Notification
	if err := db.Where("type = ?", notification.NotifTypeChapterReview).Order("id DESC").Find(&rejectNotices).Error; err != nil {
		t.Fatalf("查驳回站内信失败: %v", err)
	}
	if len(rejectNotices) != 3 {
		t.Fatalf("过审 2 条 + 驳回 1 条应共 3 条站内信, got %d", len(rejectNotices))
	}
	latest := rejectNotices[0]
	if latest.TutorID == nil || *latest.TutorID != 11 {
		t.Fatalf("驳回通知收件人应为讲师 %d, got %v", 11, latest.TutorID)
	}
	if !strings.Contains(latest.Title, "驳回") {
		t.Fatalf("驳回通知标题应含「驳回」, got %q", latest.Title)
	}
	if !strings.Contains(latest.Content, "正文缺少安全须知") {
		t.Fatalf("驳回通知正文应带理由, got %q", latest.Content)
	}
	if !strings.Contains(string(latest.Payload), `"draft"`) {
		t.Fatalf("驳回通知 payload 应携带 review_status=draft, got %s", latest.Payload)
	}

	// 批量驳回空理由 400（不给无理由驳回后门）
	rec = do("POST", "/api/admin/chapter-reviews/batch-reject", adminTok,
		`{"chapter_ids":[`+itoa(draft.ChapterID)+`],"reason":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空理由批量驳回应 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// 权限：讲师不得访问管理端队列
	if rec := do("GET", "/api/admin/chapter-reviews/pending", tutorTok, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("讲师访问队列应 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestChapterReviewQueueBatchPartialFailure HTTP 面：批量里混入无在审版本的章节，
// 其余章节照常处理，失败明细明示（C9「失败明示」，不整体回滚）。
func TestChapterReviewQueueBatchPartialFailure(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "queue-partial", Name: "部分失败", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "queue-partial-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "部分失败课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	pending := model.Chapter{CourseID: c.CourseID, Title: "在审章", Content: "v1", ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}
	noPending := model.Chapter{CourseID: c.CourseID, Title: "无在审章", Content: "v2", ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(&noPending).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "queue-partial-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))
	tutorTok, _ := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(21, "tutor_partial", "tutor")
	admin := testutil.SeedAdmin(t, db, "admin_partial", "x")
	adminTok, _ := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(admin.AdminID, admin.Username, "admin")

	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != "" {
			buf = bytes.NewBufferString(body)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req, _ := http.NewRequest(method, path, buf)
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := do("POST", "/api/tutor/chapter/"+itoa(pending.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("提交应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	batchBody := `{"chapter_ids":[` + itoa(pending.ChapterID) + `,` + itoa(noPending.ChapterID) + `]}`
	rec := do("POST", "/api/admin/chapter-reviews/batch-approve", adminTok, batchBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("批量过审应 200（部分失败不算整体失败）, got %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Data struct {
			ApprovedCount int `json:"approved_count"`
			Failures      []struct {
				ChapterID int    `json:"chapter_id"`
				Reason    string `json:"reason"`
			} `json:"failures"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if res.Data.ApprovedCount != 1 {
		t.Fatalf("成功数 = %d, want 1", res.Data.ApprovedCount)
	}
	if len(res.Data.Failures) != 1 || res.Data.Failures[0].ChapterID != noPending.ChapterID {
		t.Fatalf("失败明细 = %+v, want 只含无在审章 %d", res.Data.Failures, noPending.ChapterID)
	}
	if res.Data.Failures[0].Reason == "" {
		t.Fatal("失败明细应带原因")
	}
}
