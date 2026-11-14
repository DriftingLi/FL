// 本文件：#2537 讲师通知中心的 HTTP 契约（ADR-0095）——
// 收件人身份空间（讲师落 tutor_id、学员落 user_id、两者不串）、角色分派与 403、
// 章节审核结果投递（含 C7 管理员直接生效）、以及讲师端能真正读到自己的信箱。
//
// 状态机与投递口径的单测在 course / notification 包；本处只锁 HTTP 面与端到端可见性。
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// TestTutorNotificationMailboxContract #2537 端到端：
//  1. 讲师提交章节 → 管理员过审 → 讲师信箱（GET /api/notifications）读到审核结果；
//  2. 学员信箱读不到讲师的信（撞号也不串）；
//  3. admin 调通知端点 403（不是空列表）；
//  4. 讲师把这条标已读后未读数归零。
func TestTutorNotificationMailboxContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "notif-flow", Name: "通知流", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "notif-flow-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "通知流课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "通知章", Content: "正文", ContentType: "text",
		OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "notif-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))

	sign := func(uid int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(uid, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	// 故意让讲师与学员撞同一个数字 id：这正是「两个 id 空间不可互读」要防的场景。
	const sharedID = 11
	tutorTok := sign(sharedID, "tutor_notif", "tutor")
	student := testutil.SeedStudent(t, db, "student_notif", "x")
	studentTok := sign(student.ID, student.Username, "hrwai_user")
	admin := testutil.SeedAdmin(t, db, "admin_notif", "x")
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

	// 1. 讲师提交 → 管理员过审 → 投递
	if rec := do("POST", "/api/tutor/chapter/"+itoa(ch.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("提交审核应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do("POST", "/api/admin/chapter/"+itoa(ch.ChapterID)+"/approve-review", adminTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("过审应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. 讲师信箱读到这条（按 role 分派到 tutor_id 列）
	rec := do("GET", "/api/notifications", tutorTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("讲师查站内信应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tutorBox struct {
		Data struct {
			Items []struct {
				Type      string          `json:"type"`
				Title     string          `json:"title"`
				Content   string          `json:"content"`
				Link      string          `json:"link"`
				IsRead    bool            `json:"is_read"`
				Payload   json.RawMessage `json:"payload"`
				CreatedAt string          `json:"created_at"`
			} `json:"items"`
			Total       int64 `json:"total"`
			UnreadCount int64 `json:"unread_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tutorBox); err != nil {
		t.Fatalf("解析讲师信箱失败: %v", err)
	}
	if tutorBox.Data.Total != 1 || len(tutorBox.Data.Items) != 1 {
		t.Fatalf("讲师信箱应有 1 条, got total=%d items=%d", tutorBox.Data.Total, len(tutorBox.Data.Items))
	}
	item := tutorBox.Data.Items[0]
	if item.Type != "chapter_review" || item.Title != "章节审核通过" {
		t.Fatalf("讲师收到的通知不符: %+v", item)
	}
	if item.Link == "" || item.CreatedAt == "" {
		t.Fatalf("讲师通知应带跳转链接与时间: %+v", item)
	}
	if tutorBox.Data.UnreadCount != 1 {
		t.Fatalf("未读数应为 1, got %d", tutorBox.Data.UnreadCount)
	}

	// 3. 学员信箱读不到讲师的信（撞号也不串）
	rec = do("GET", "/api/notifications", studentTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("学员查站内信应 200, got %d", rec.Code)
	}
	var studentBox struct {
		Data struct {
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &studentBox); err != nil {
		t.Fatalf("解析学员信箱失败: %v", err)
	}
	if studentBox.Data.Total != 0 {
		t.Fatalf("学员不应读到讲师的通知（id 撞号也不串）, got %d", studentBox.Data.Total)
	}

	// 4. admin 没有信箱：403，不是空列表
	rec = do("GET", "/api/notifications", adminTok, "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin 查站内信应 403（区分「没有信箱」与「信箱是空的」）, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. 讲师标记已读 → 未读数归零
	var n model.Notification
	if err := db.Where("tutor_id = ?", sharedID).First(&n).Error; err != nil {
		t.Fatalf("查讲师通知失败: %v", err)
	}
	if rec := do("POST", "/api/notifications/"+itoa(int(n.ID))+"/read", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("标记已读应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do("GET", "/api/notifications/unread-count", tutorTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("查未读数应 200, got %d", rec.Code)
	}
	var unread struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &unread); err != nil {
		t.Fatalf("解析未读数失败: %v", err)
	}
	if unread.Data.Count != 0 {
		t.Fatalf("标记已读后未读数应为 0, got %d", unread.Data.Count)
	}
}

// TestChapterAdminEditNotifiesAuthorContract #2537 / ADR-0095 决策七：
// C7 管理员直接改章节（编辑即发布）也要通知章节作者——否则讲师的内容被静默改掉。
// 同时验证：通知收件人来自 chapter.tutor_id（稳定归属），而不是只活在 pending 槽的 submitted_by。
func TestChapterAdminEditNotifiesAuthorContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "admin-edit", Name: "管理改写", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "admin-edit-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "管理改写课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	// 章节已有稳定归属（讲师 21），但**没有** pending 槽——C7 通知不能依赖 submitted_by。
	const authorID = 21
	ch := model.Chapter{CourseID: c.CourseID, Title: "被管理员改的章", Content: "旧正文",
		ContentType: "text", OrderNum: 1, TutorID: ptr(authorID), CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "admin-edit-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))
	sign := func(uid int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(uid, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	tutorTok := sign(authorID, "tutor_author", "tutor")
	admin := testutil.SeedAdmin(t, db, "admin_edit", "x")
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

	// 管理员直接改章节（C7：编辑即发布）
	body := `{"content":"管理员改后的正文"}`
	if rec := do("PUT", "/api/admin/chapter/"+itoa(ch.ChapterID), adminTok, body); rec.Code != http.StatusOK {
		t.Fatalf("管理员改章节应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 作者讲师收到「已被管理员更新」通知
	rec := do("GET", "/api/notifications", tutorTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("讲师查站内信应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var box struct {
		Data struct {
			Items []struct {
				Type    string `json:"type"`
				Title   string `json:"title"`
				Content string `json:"content"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &box); err != nil {
		t.Fatalf("解析讲师信箱失败: %v", err)
	}
	if box.Data.Total != 1 || len(box.Data.Items) != 1 {
		t.Fatalf("C7 管理员改写应给作者 1 条通知, got total=%d", box.Data.Total)
	}
	got := box.Data.Items[0]
	if got.Type != "chapter_review" || got.Title != "章节已被管理员更新" {
		t.Fatalf("C7 通知类型/标题不符: %+v", got)
	}
	if got.Content == "" {
		t.Fatal("C7 通知应有正文说明")
	}

	// 通知确实落 tutor_id（不是 user_id）：DB 层复核，防「HTTP 面看着对、库里投错列」。
	var n model.Notification
	if err := db.Where("type = ?", "chapter_review").First(&n).Error; err != nil {
		t.Fatalf("查通知失败: %v", err)
	}
	if n.TutorID == nil || *n.TutorID != authorID {
		t.Fatalf("通知应落 tutor_id=%d, got %v", authorID, n.TutorID)
	}
	if n.UserID != nil {
		t.Fatalf("讲师通知不得落 user_id, got %v", *n.UserID)
	}
}
