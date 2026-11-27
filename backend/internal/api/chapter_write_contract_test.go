package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

func seedChapterWriteCourse(t *testing.T, db *gorm.DB) (int, int) {
	t.Helper()
	spec := model.Specialty{Code: "chapter-write", Name: "章节写入", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建专业方向失败: %v", err)
	}
	level := model.CourseLevel{Code: "chapter-write-level", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("建课程等级失败: %v", err)
	}
	category := testutil.SeedTrainingCategory(t, db, "章节写入分类")
	course := model.Course{
		Name: "章节写入课程", Status: 1, CourseType: model.CourseTypeIllustrated,
		CategoryID: &category.ID, SpecialtyID: &spec.SpecialtyID, LevelID: &level.LevelID,
		CreatedAt: testutil.Now(),
	}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	chapter := model.Chapter{
		CourseID: course.CourseID, Title: "第一章", Content: "正文", ContentType: "text",
		OrderNum: 1, CreatedAt: testutil.Now(),
	}
	if err := db.Create(&chapter).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	return course.CourseID, chapter.ChapterID
}

func postTutorImage(t *testing.T, router http.Handler, token, queryChapterID, formChapterID string) *httptest.ResponseRecorder {
	t.Helper()
	return postTutorImageRequest(t, router, token, queryChapterID, formChapterID, true)
}

func postTutorImageWithoutFile(t *testing.T, router http.Handler, token, chapterID string) *httptest.ResponseRecorder {
	t.Helper()
	return postTutorImageRequest(t, router, token, chapterID, "", false)
}

func postTutorImageRequest(t *testing.T, router http.Handler, token, queryChapterID, formChapterID string, withFile bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if withFile {
		part, err := writer.CreateFormFile("file", "a.png")
		if err != nil {
			t.Fatalf("构造图片上传失败: %v", err)
		}
		if _, err := part.Write([]byte{0x89, 'P', 'N', 'G'}); err != nil {
			t.Fatalf("写图片内容失败: %v", err)
		}
	}
	if formChapterID != "" {
		if err := writer.WriteField("chapter_id", formChapterID); err != nil {
			t.Fatalf("写章节参数失败: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("结束 multipart 失败: %v", err)
	}
	path := "/api/tutor/upload-image"
	if queryChapterID != "" {
		path += "?chapter_id=" + queryChapterID
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doTutorJSON(t *testing.T, router http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertTutorImageError(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("图片上传应返回 %d，实际 %d: %s", status, rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("错误响应不是统一信封: %v: %s", err, rec.Body.String())
	}
	if envelope.Code != status || envelope.Data != nil {
		t.Fatalf("错误信封码或 data 不符: %s", rec.Body.String())
	}
}

func TestTutorUploadImageRequiresChapterWriteClaim(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "chapter-image-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	router := NewRouter(newContractDepsWithStorage(t, db, cfg, storage.NewLocalStorage(t.TempDir())))
	tutor := testutil.SeedTutor(t, db, "chapter_image_tutor", "x")
	token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")
	courseID, chapterID := seedChapterWriteCourse(t, db)

	cases := []struct {
		name       string
		queryID    string
		formID     string
		wantStatus int
	}{
		{name: "缺少章节ID", wantStatus: http.StatusBadRequest},
		{name: "非法章节ID", queryID: "abc", wantStatus: http.StatusBadRequest},
		{name: "零章节ID", formID: "0", wantStatus: http.StatusBadRequest},
		{name: "负章节ID", queryID: "-1", wantStatus: http.StatusBadRequest},
		{name: "章节不存在", queryID: "999999", wantStatus: http.StatusNotFound},
		{name: "章节未认领", queryID: fmt.Sprint(chapterID), wantStatus: http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := postTutorImage(t, router, token, tc.queryID, tc.formID)
			assertTutorImageError(t, rec, tc.wantStatus)
		})
	}
	t.Run("无文件也先回答章节不存在", func(t *testing.T) {
		assertTutorImageError(t, postTutorImageWithoutFile(t, router, token, "999999"), http.StatusNotFound)
	})
	t.Run("无文件也先回答未认领", func(t *testing.T) {
		assertTutorImageError(t, postTutorImageWithoutFile(t, router, token, fmt.Sprint(chapterID)), http.StatusForbidden)
	})

	claim := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), nil)
	claim.Header.Set("Authorization", "Bearer "+token)
	claimRec := httptest.NewRecorder()
	router.ServeHTTP(claimRec, claim)
	if claimRec.Code != http.StatusOK {
		t.Fatalf("认领课程应 200，实际 %d: %s", claimRec.Code, claimRec.Body.String())
	}

	t.Run("有效章节缺少文件仍返回Vditor错误", func(t *testing.T) {
		rec := postTutorImageWithoutFile(t, router, token, fmt.Sprint(chapterID))
		if rec.Code != http.StatusOK {
			t.Fatalf("缺少文件应保持 HTTP 200，实际 %d: %s", rec.Code, rec.Body.String())
		}
		var shape struct {
			Msg  string `json:"msg"`
			Code int    `json:"code"`
			Data struct {
				ErrFiles []string          `json:"errFiles"`
				SuccMap  map[string]string `json:"succMap"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &shape); err != nil {
			t.Fatalf("缺文件响应不是 Vditor 信封: %v: %s", err, rec.Body.String())
		}
		if shape.Msg != "未找到上传文件" || shape.Code != 1 || len(shape.Data.ErrFiles) != 0 || len(shape.Data.SuccMap) != 0 {
			t.Fatalf("Vditor 缺文件错误信封不符: %s", rec.Body.String())
		}
		want, _ := json.Marshal(legacyVditorErrorEnvelope("未找到上传文件", []string{}))
		if !bytes.Equal(rec.Body.Bytes(), want) {
			t.Fatalf("Vditor 缺文件错误形状漂移\n got: %s\nwant: %s", rec.Body.Bytes(), want)
		}
	})

	for _, tc := range []struct {
		name    string
		queryID string
		formID  string
	}{
		{name: "query传参成功", queryID: fmt.Sprint(chapterID)},
		{name: "form传参成功", formID: fmt.Sprint(chapterID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := postTutorImage(t, router, token, tc.queryID, tc.formID)
			if rec.Code != http.StatusOK {
				t.Fatalf("认领后图片上传应 200，实际 %d: %s", rec.Code, rec.Body.String())
			}
			var shape struct {
				Msg  string `json:"msg"`
				Code int    `json:"code"`
				Data struct {
					ErrFiles []string          `json:"errFiles"`
					SuccMap  map[string]string `json:"succMap"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &shape); err != nil {
				t.Fatalf("成功响应不是 Vditor 信封: %v: %s", err, rec.Body.String())
			}
			if shape.Msg != "" || shape.Code != 0 || len(shape.Data.ErrFiles) != 0 {
				t.Fatalf("Vditor 成功信封不符: %s", rec.Body.String())
			}
			url, ok := shape.Data.SuccMap["a.png"]
			if !ok || url == "" {
				t.Fatalf("succMap 应包含 a.png 的 URL: %s", rec.Body.String())
			}
			want, _ := json.Marshal(legacyVditorSuccessEnvelope("a.png", url))
			if !bytes.Equal(rec.Body.Bytes(), want) {
				t.Fatalf("Vditor 成功形状漂移\n got: %s\nwant: %s", rec.Body.Bytes(), want)
			}
		})
	}

	var chapter model.Chapter
	if err := db.First(&chapter, chapterID).Error; err != nil {
		t.Fatalf("读章节失败: %v", err)
	}
	if chapter.TutorID == nil || *chapter.TutorID != tutor.TutorID {
		t.Fatalf("图片写入应首次落定章节归属为 %d，实际 %v", tutor.TutorID, chapter.TutorID)
	}
}

func TestChapterWriteAttachmentMutationsReturnToDraft(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "chapter-attachment-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	router := NewRouter(newContractDeps(t, db, cfg))
	tutor := testutil.SeedTutor(t, db, "chapter_attachment_tutor", "x")
	token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")
	courseID, chapterID := seedChapterWriteCourse(t, db)
	rec := doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("认领课程应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	oldFile := model.TutorFile{TutorID: tutor.TutorID, FileURL: "/static/uploads/tutor-files/old.mp4",
		FileName: "old.mp4", ContentType: "video", FileSize: 10, CreatedAt: testutil.Now()}
	newFile := model.TutorFile{TutorID: tutor.TutorID, FileURL: "/static/uploads/tutor-files/new.mp4",
		FileName: "new.mp4", ContentType: "video", FileSize: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&oldFile).Error; err != nil {
		t.Fatalf("建旧文件失败: %v", err)
	}
	if err := db.Create(&newFile).Error; err != nil {
		t.Fatalf("建新文件失败: %v", err)
	}
	markPublished := func() {
		t.Helper()
		if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
			Update("review_status", model.ChapterReviewPublished).Error; err != nil {
			t.Fatalf("设置已发布状态失败: %v", err)
		}
	}
	assertDraft := func(step string) {
		t.Helper()
		var chapter model.Chapter
		if err := db.First(&chapter, chapterID).Error; err != nil {
			t.Fatalf("%s后读章节失败: %v", step, err)
		}
		if chapter.ReviewStatus != model.ChapterReviewDraft {
			t.Fatalf("%s应回草稿，实际 %q", step, chapter.ReviewStatus)
		}
		if chapter.TutorID == nil || *chapter.TutorID != tutor.TutorID {
			t.Fatalf("%s应落定或保留章节归属，实际 %v", step, chapter.TutorID)
		}
	}

	rec = doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/files/%d/attach", oldFile.FileID),
		fmt.Sprintf(`{"chapter_id":%d}`, chapterID))
	if rec.Code != http.StatusOK {
		t.Fatalf("挂接应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	assertDraft("挂接")
	var gotOld model.TutorFile
	if err := db.First(&gotOld, oldFile.FileID).Error; err != nil || gotOld.ChapterID == nil || *gotOld.ChapterID != chapterID {
		t.Fatalf("挂接后旧文件应指向本章，got=%+v err=%v", gotOld, err)
	}

	markPublished()
	rec = doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/chapter/%d/files/swap", chapterID),
		fmt.Sprintf(`{"old_file_id":%d,"new_file_id":%d}`, oldFile.FileID, newFile.FileID))
	if rec.Code != http.StatusOK {
		t.Fatalf("换挂应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	assertDraft("换挂")
	var gotNew model.TutorFile
	if err := db.First(&gotOld, oldFile.FileID).Error; err != nil {
		t.Fatalf("重读旧文件失败: %v", err)
	}
	if err := db.First(&gotNew, newFile.FileID).Error; err != nil {
		t.Fatalf("重读新文件失败: %v", err)
	}
	if gotOld.ChapterID != nil || gotNew.ChapterID == nil || *gotNew.ChapterID != chapterID {
		t.Fatalf("换挂结果不符，old=%+v new=%+v", gotOld, gotNew)
	}

	markPublished()
	rec = doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/files/%d/detach", newFile.FileID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("解挂应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	assertDraft("解挂")
	if err := db.First(&gotNew, newFile.FileID).Error; err != nil || gotNew.ChapterID != nil {
		t.Fatalf("解挂后新文件应回库，got=%+v err=%v", gotNew, err)
	}
}

func TestChapterWriteAttachFileRequiresClaimBeforeIdempotency(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "chapter-attach-claim-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	router := NewRouter(newContractDeps(t, db, cfg))
	tutor := testutil.SeedTutor(t, db, "chapter_attach_claim_tutor", "x")
	token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")
	courseID, chapterID := seedChapterWriteCourse(t, db)
	rec := doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("认领课程应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	file := model.TutorFile{TutorID: tutor.TutorID, FileURL: "/static/uploads/tutor-files/attached.mp4",
		FileName: "attached.mp4", ContentType: "video", FileSize: 10, CreatedAt: testutil.Now()}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("建文件失败: %v", err)
	}
	path := fmt.Sprintf("/api/tutor/files/%d/attach", file.FileID)
	body := fmt.Sprintf(`{"chapter_id":%d}`, chapterID)
	rec = doTutorJSON(t, router, token, http.MethodPost, path, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("认领后挂接应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	rec = doTutorJSON(t, router, token, http.MethodDelete, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("退出认领应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	rec = doTutorJSON(t, router, token, http.MethodPost, path, body)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("未认领讲师重复挂同章应 403，实际 %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChapterWriteOwnershipDraftAndReviewSnapshot(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "chapter-state-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	router := NewRouter(newContractDeps(t, db, cfg))
	tutorA := testutil.SeedTutor(t, db, "chapter_state_a", "x")
	tutorB := testutil.SeedTutor(t, db, "chapter_state_b", "x")
	tokenA := slice6Token(t, cfg, tutorA.TutorID, tutorA.Username, "tutor")
	tokenB := slice6Token(t, cfg, tutorB.TutorID, tutorB.Username, "tutor")
	courseID, chapterID := seedChapterWriteCourse(t, db)
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).
		Update("review_status", model.ChapterReviewPublished).Error; err != nil {
		t.Fatalf("设置已发布状态失败: %v", err)
	}
	for _, token := range []string{tokenA, tokenB} {
		rec := doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), "")
		if rec.Code != http.StatusOK {
			t.Fatalf("共认课程应 200，实际 %d: %s", rec.Code, rec.Body.String())
		}
	}

	chapterPath := fmt.Sprintf("/api/tutor/chapter/%d", chapterID)
	rec := doTutorJSON(t, router, tokenA, http.MethodPut, chapterPath, `{"title":"只改元数据"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("元数据写入应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var chapter model.Chapter
	if err := db.First(&chapter, chapterID).Error; err != nil {
		t.Fatalf("读章节失败: %v", err)
	}
	if chapter.ReviewStatus != model.ChapterReviewPublished {
		t.Fatalf("纯元数据不得降级，实际 %q", chapter.ReviewStatus)
	}
	if chapter.TutorID == nil || *chapter.TutorID != tutorA.TutorID {
		t.Fatalf("首次写入应落定归属为讲师 A，实际 %v", chapter.TutorID)
	}

	rec = doTutorJSON(t, router, tokenB, http.MethodPut, chapterPath, `{"content":"待审正文"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("正文写入应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if err := db.First(&chapter, chapterID).Error; err != nil {
		t.Fatalf("重读章节失败: %v", err)
	}
	if chapter.ReviewStatus != model.ChapterReviewDraft {
		t.Fatalf("正文变化应回草稿，实际 %q", chapter.ReviewStatus)
	}
	if chapter.TutorID == nil || *chapter.TutorID != tutorA.TutorID {
		t.Fatalf("后到讲师不得改写首作者归属，实际 %v", chapter.TutorID)
	}

	rec = doTutorJSON(t, router, tokenB, http.MethodPost, fmt.Sprintf("/api/tutor/chapter/%d/submit-review", chapterID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("提审应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	var pending model.ChapterContentSlot
	if err := db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).First(&pending).Error; err != nil {
		t.Fatalf("读在审快照失败: %v", err)
	}
	if pending.Content != "待审正文" || pending.SubmittedBy != tutorB.TutorID {
		t.Fatalf("在审快照应冻结正文并记录提交人，实际 %+v", pending)
	}

	rec = doTutorJSON(t, router, tokenB, http.MethodPut, chapterPath, `{"content":"审中草稿"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("审中草稿写入应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if err := db.First(&chapter, chapterID).Error; err != nil {
		t.Fatalf("重读审中章节失败: %v", err)
	}
	if chapter.ReviewStatus != model.ChapterReviewPending || chapter.Content != "审中草稿" {
		t.Fatalf("审中只应改 working 草稿且保持 pending，实际 %q/%q", chapter.ReviewStatus, chapter.Content)
	}
	if err := db.Where("slot = ? AND chapter_id = ?", model.ChapterReviewPending, chapterID).First(&pending).Error; err != nil {
		t.Fatalf("重读在审快照失败: %v", err)
	}
	if pending.Content != "待审正文" {
		t.Fatalf("审中草稿不得污染在审快照，实际 %q", pending.Content)
	}
	if chapter.TutorID == nil || *chapter.TutorID != tutorA.TutorID {
		t.Fatalf("提审与后续写入均不得改写首作者归属，实际 %v", chapter.TutorID)
	}
}
