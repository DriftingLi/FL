// 本文件：课程认领 HTTP 契约（#2596 / 规格 B3、B5、B6、B7、D1、D5）。
//
// 三层各测一段，避免互相顶替：
//   - 端点面：认领/退出/名单三个端点的状态码与信封（含 409、404 档）；
//   - 谓词面：未认领讲师经 HTTP 打五条写路径全 403（谓词真的接在路由后面）；
//   - 结果面：认领后同一条请求放行（403 不是「永远拒绝」）。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// TestTutorCourseClaimContract 认领端点全流程：未认领写 403 → 认领 200 → 再写 200 →
// 重复认领 409 → 名单含本人 → 退出 200 → 再写 403 → 重复退出 404。
func TestTutorCourseClaimContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "claim-http", Name: "认领", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "claim-http-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "认领课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "第一章", Content: "正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "claim-contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))
	tutor := testutil.SeedTutor(t, db, "tutor_claim", "hash")
	tutorTok := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")

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
	claimPath := fmt.Sprintf("/api/tutor/course/%d/claim", c.CourseID)
	chapterPath := fmt.Sprintf("/api/tutor/chapter/%d", ch.ChapterID)

	// 1. 未认领：写章节 403 且文案可行动（D5「请先认领本课程」）
	rec := do("PUT", chapterPath, tutorTok, `{"title":"未认领改"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("未认领写应 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("请先认领本课程")) {
		t.Fatalf("403 文案应含可行动指引, body = %s", rec.Body.String())
	}

	// 2. 认领成功
	if rec := do("POST", claimPath, tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("认领应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. 认领后同一条写请求放行
	if rec := do("PUT", chapterPath, tutorTok, `{"title":"认领后改"}`); rec.Code != http.StatusOK {
		t.Fatalf("认领后写应 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. 重复认领 409（B5：不静默成功）
	rec = do("POST", claimPath, tutorTok, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("重复认领应 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("你已认领本课程")) {
		t.Fatalf("409 文案应为「你已认领本课程」, body = %s", rec.Body.String())
	}

	// 5. 名单含本人（D1），且是成功信封
	rec = do("GET", fmt.Sprintf("/api/tutor/course/%d/claims", c.CourseID), tutorTok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("名单应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var listEnv struct {
		Data struct {
			Claimants []struct {
				TutorID   int    `json:"tutor_id"`
				TutorName string `json:"tutor_name"`
				Source    string `json:"source"`
			} `json:"claimants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listEnv); err != nil {
		t.Fatalf("解析名单失败: %v, body = %s", err, rec.Body.String())
	}
	if len(listEnv.Data.Claimants) != 1 || listEnv.Data.Claimants[0].TutorID != tutor.TutorID {
		t.Fatalf("名单应含本人, got %+v", listEnv.Data.Claimants)
	}
	if listEnv.Data.Claimants[0].Source != model.ClaimSourceSelf {
		t.Fatalf("自助认领来源应为 self, got %q", listEnv.Data.Claimants[0].Source)
	}

	// 6. 退出成功 → 再写又 403（B6 闭环）
	if rec := do("DELETE", claimPath, tutorTok, ""); rec.Code != http.StatusOK {
		t.Fatalf("退出认领应 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := do("PUT", chapterPath, tutorTok, `{"title":"退出后改"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("退出后写应 403, got %d: %s", rec.Code, rec.Body.String())
	}

	// 7. 章节作者归属仍在（退出不回溯）
	var got model.Chapter
	if err := db.First(&got, ch.ChapterID).Error; err != nil {
		t.Fatalf("读章节失败: %v", err)
	}
	if got.TutorID == nil || *got.TutorID != tutor.TutorID {
		t.Fatalf("退出不回溯：章节归属应仍为 %d, got %v", tutor.TutorID, got.TutorID)
	}

	// 8. 重复退出 404 档（未认领 ≠ 成功）
	if rec := do("DELETE", claimPath, tutorTok, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("重复退出应 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// 9. 不存在的课程认领 404（不是静默建行）
	if rec := do("POST", "/api/tutor/course/999999/claim", tutorTok, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("认领不存在课程应 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestTutorWritePathsAllGuardedByClaim 六条写路径经 HTTP 全被认领谓词拦住（B2）。
//
// 服务层已逐条测过（tutor/claim_test.go + tutorfile/attach_test.go），本处再逐条打 HTTP 一次：
// 路由到 service 的接线也是缺陷高发处（改签名时漏传 tutorID 就会变成「用 0 号讲师查认领」
// 而恰好放行或全拒）。
//
// 批量删除不在本组：它的既有语义是「不能删的进 failed_ids」，未认领不会整批 403——
// 它的取证在服务层（claim_test.go 的 TestWritePathsRejectedWithoutClaim 第 4 条）。
func TestTutorWritePathsAllGuardedByClaim(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "claim-guard", Name: "认领守卫", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "claim-guard-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "守卫课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "守卫章", Content: "正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	chapterFile := model.ChapterFile{ChapterID: &ch.ChapterID, FileURL: "/uploads/guard.pdf",
		FileName: "guard.pdf", ContentType: "document", CreatedAt: testutil.Now()}
	if err := db.Create(&chapterFile).Error; err != nil {
		t.Fatalf("建章节文件失败: %v", err)
	}
	// 文件库文件 + 认领（供 swap 用：swap 需要讲师自己的两个文件）
	fileA := model.TutorFile{TutorID: 1, FileURL: "/uploads/a.mp4", FileName: "a.mp4",
		ContentType: "video", FileSize: 10, ChapterID: &ch.ChapterID, CreatedAt: testutil.Now()}
	if err := db.Create(&fileA).Error; err != nil {
		t.Fatalf("建文件 A 失败: %v", err)
	}
	fileB := model.TutorFile{TutorID: 1, FileURL: "/uploads/b.mp4", FileName: "b.mp4",
		ContentType: "video", FileSize: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&fileB).Error; err != nil {
		t.Fatalf("建文件 B 失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "claim-guard-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))
	// 讲师 id 用 1，与上面文件行的归属对齐
	tutorTok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(1, "tutor_guard", "tutor")
	if err != nil {
		t.Fatalf("签 token 失败: %v", err)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"更新章节", "PUT", fmt.Sprintf("/api/tutor/chapter/%d", ch.ChapterID), `{"title":"x"}`},
		{"提交审核", "POST", fmt.Sprintf("/api/tutor/chapter/%d/submit-review", ch.ChapterID), ""},
		{"删除章节附件", "DELETE", fmt.Sprintf("/api/tutor/file/%d", chapterFile.FileID), ""},
		{"章节侧换挂", "POST", fmt.Sprintf("/api/tutor/chapter/%d/files/swap", ch.ChapterID),
			fmt.Sprintf(`{"old_file_id":%d,"new_file_id":%d}`, fileA.FileID, fileB.FileID)},
		// 挂接/解挂（#2596 补齐 B2 的两条遗漏路径）：文件是自己的、课不是自己能动的。
		{"挂接文件到章节", "POST", fmt.Sprintf("/api/tutor/files/%d/attach", fileB.FileID),
			fmt.Sprintf(`{"chapter_id":%d}`, ch.ChapterID)},
		{"从章节解挂文件", "POST", fmt.Sprintf("/api/tutor/files/%d/detach", fileA.FileID), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer "+tutorTok)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s 未认领应 403, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
			if !bytes.Contains(rec.Body.Bytes(), []byte("请先认领本课程")) {
				t.Fatalf("%s 403 文案应含「请先认领本课程」, body = %s", tc.name, rec.Body.String())
			}
		})
	}

	// 认领后换挂放行（403 不是永远拒绝）：这是唯一需要额外前置的一条（两个文件都得是自己的）
	if err := db.Create(&model.TutorCourseClaim{TutorID: 1, CourseID: c.CourseID,
		Source: model.ClaimSourceSelf, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("造认领行失败: %v", err)
	}
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/tutor/chapter/%d/files/swap", ch.ChapterID),
		bytes.NewBufferString(fmt.Sprintf(`{"old_file_id":%d,"new_file_id":%d}`, fileA.FileID, fileB.FileID)))
	req.Header.Set("Authorization", "Bearer "+tutorTok)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("认领后换挂应 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
