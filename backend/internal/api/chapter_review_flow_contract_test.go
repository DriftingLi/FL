// 本文件：章节先审后发 HTTP 契约（#2498 spec C2/C4/C5/C6/C8）——讲师「保存草稿/提交审核」
// 两动作、单 pending 400、TOCTOU（审中改草稿不改在审内容、过审只切快照）、旧版保留、
// 新章三读面 404、管理员写面直接生效（C7）。状态机语义单测在 course 包，本处只锁 HTTP 面。
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

func TestChapterReviewFlowContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "review-flow", Name: "审核流", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "review-flow-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	c := model.Course{Name: "审核流课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "审核流章节", Content: "v1 正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}
	stu := model.HrwaiUser{Account: "acct_review_flow", Phone: "13800000099", Username: "审核流", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&stu).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	// 整机装配（NewRouter）——能力解析源（admincap）由装配根注入，动态角色 admin 的
	// 能力判定依赖它；自组 gin.Engine 会让超管 403（解析源缺失 fail-closed）。
	r := NewRouter(newContractDeps(t, db, cfg))

	sign := func(uid int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(uid, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	tutorTok := sign(1, "tutor_review", "tutor")
	admin := testutil.SeedAdmin(t, db, "admin_review", "x")
	adminTok := sign(admin.AdminID, admin.Username, "admin")
	stuTok := sign(int(stu.ID), stu.Account, "hrwai_user")

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

	// C6：新建章节（从未过审）学员侧完全不出现——直连 404 不泄漏存在性
	if rec := do("GET", "/api/course/"+itoa(c.CourseID)+"/chapter/"+itoa(ch.ChapterID), stuTok, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("未过审章节直连应 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// C2：讲师显式提交审核
	if rec := do("POST", "/api/tutor/chapter/"+itoa(ch.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("提交审核应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// C8：单 pending——重复提交 400
	if rec := do("POST", "/api/tutor/chapter/"+itoa(ch.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("重复提交应 400, got %d: %s", rec.Code, rec.Body.String())
	}

	// C4 TOCTOU：审中改草稿（v2），在审内容仍是 v1
	if rec := do("PUT", "/api/tutor/chapter/"+itoa(ch.ChapterID), tutorTok,
		`{"content":"v2 草稿"}`); rec.Code != http.StatusOK {
		t.Fatalf("审中改草稿应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 过审：切换的是快照（v1），不是 v2 草稿
	if rec := do("POST", "/api/admin/chapter/"+itoa(ch.ChapterID)+"/approve-review", adminTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("过审应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec := do("GET", "/api/course/"+itoa(c.CourseID)+"/chapter/"+itoa(ch.ChapterID), stuTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("过审后学员应可读, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Data.Content != "v1 正文" {
		t.Fatalf("过审应只切快照内容（v1），got %q", body.Data.Content)
	}

	// C5：重审期间旧版照常可见（再改草稿、提交、驳回，学员仍见 v1）
	if rec := do("PUT", "/api/tutor/chapter/"+itoa(ch.ChapterID), tutorTok, `{"content":"v3 草稿"}`); rec.Code != http.StatusOK {
		t.Fatalf("改草稿应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do("POST", "/api/tutor/chapter/"+itoa(ch.ChapterID)+"/submit-review", tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("再提应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// 驳回理由必填（C1）
	if rec := do("POST", "/api/admin/chapter/"+itoa(ch.ChapterID)+"/reject-review", adminTok, `{"reason":""}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("空理由驳回应 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do("POST", "/api/admin/chapter/"+itoa(ch.ChapterID)+"/reject-review", adminTok, `{"reason":"缺安全须知"}`); rec.Code != http.StatusOK {
		t.Fatalf("驳回应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do("GET", "/api/course/"+itoa(c.CourseID)+"/chapter/"+itoa(ch.ChapterID), stuTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("重审被驳回期间旧版应可读, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Data.Content != "v1 正文" {
		t.Fatalf("驳回后学员仍应见旧版 v1, got %q", body.Data.Content)
	}

	// 无在审版本时过审 409（状态前置哨兵）
	if rec := do("POST", "/api/admin/chapter/"+itoa(ch.ChapterID)+"/approve-review", adminTok, ""); rec.Code != http.StatusConflict {
		t.Fatalf("无在审过审应 409, got %d: %s", rec.Code, rec.Body.String())
	}

	// C7：管理员写面直接生效——不进队列、学生立即见新内容
	if rec := do("PUT", "/api/admin/chapter/"+itoa(ch.ChapterID), adminTok,
		`{"content":"v4 管理员直接生效"}`); rec.Code != http.StatusOK {
		t.Fatalf("管理员更新应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = do("GET", "/api/course/"+itoa(c.CourseID)+"/chapter/"+itoa(ch.ChapterID), stuTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("直接生效后学员应可读, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Data.Content != "v4 管理员直接生效" {
		t.Fatalf("管理员写面应直接生效, got %q", body.Data.Content)
	}
}
