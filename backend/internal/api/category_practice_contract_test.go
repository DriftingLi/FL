package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/mockexam"
	"forklift-training/internal/model"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/testutil"
)

func TestCategoryPracticeContract(t *testing.T) {
	assertCategoryPracticeContract(t, testutil.NewMemoryDB(t))
}

func TestCategoryPracticeContractOnPostgres(t *testing.T) {
	assertCategoryPracticeContract(t, testutil.NewPostgresDB(t))
}

func TestCategoryMockExamContractOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	testutil.SetTestGinMode()
	cfg := &config.Config{JWTSecretKey: "category-mock-contract", JWTExpiresHours: 2}
	router := NewRouter(newContractDeps(t, db, cfg))
	owner := failureTestStudentToken(t, db, cfg, "category-mock-student")
	other := failureTestStudentToken(t, db, cfg, "category-mock-other")
	category := testutil.SeedTrainingCategory(t, db, "模考分类")
	empty := testutil.SeedTrainingCategory(t, db, "空模考分类")
	first := testutil.SeedQuestion(t, db, "single_choice", "第一道模考题", "A")
	second := testutil.SeedQuestion(t, db, "single_choice", "第二道模考题", "B")
	if err := db.Model(&model.Question{}).Where("id IN ?", []int{first.ID, second.ID}).
		Updates(map[string]any{"category_id": category.ID, "status": "published"}).Error; err != nil {
		t.Fatal(err)
	}

	for _, categoryID := range []int{0, empty.ID, category.ID} {
		rec := doWithToken(t, router, owner, http.MethodPost, "/api/mock-exam/category-start",
			map[string]any{"category_id": categoryID, "question_count": 2, "duration_minutes": 30})
		if categoryID != category.ID {
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("unavailable category started an exam: %d %s", rec.Code, rec.Body.String())
			}
			continue
		}
		var started struct {
			Data mockexam.MockExamStartDTO `json:"data"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &started) != nil || len(started.Data.Questions) != 2 {
			t.Fatalf("start exam: %d %s", rec.Code, rec.Body.String())
		}
		examPath := fmt.Sprintf("/api/mock-exam/%d", started.Data.MockExamID)
		questionID := started.Data.Questions[0].ID
		answerKey := strconv.Itoa(questionID)
		rec = doWithToken(t, router, owner, http.MethodPost, examPath+"/save",
			map[string]any{"answers": map[string]any{answerKey: "A"}, "remaining_time": 721})
		if rec.Code != http.StatusOK {
			t.Fatalf("save exam: %d %s", rec.Code, rec.Body.String())
		}
		rec = doWithToken(t, router, other, http.MethodGet, examPath+"/resume", nil)
		if rec.Code < http.StatusBadRequest {
			t.Fatalf("another learner resumed the exam: %s", rec.Body.String())
		}
		if err := db.Model(&model.Question{}).Where("id = ?", questionID).
			Updates(map[string]any{"category_id": empty.ID, "status": "draft"}).Error; err != nil {
			t.Fatal(err)
		}
		for _, legacy := range []bool{false, true} {
			if legacy {
				if err := db.Model(&model.MockExam{}).Where("id = ?", started.Data.MockExamID).
					Update("category_id", nil).Error; err != nil {
					t.Fatal(err)
				}
			}
			rec = doWithToken(t, router, owner, http.MethodGet, examPath+"/resume", nil)
			var resumed struct {
				Data mockexam.MockExamResumeDTO `json:"data"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &resumed) != nil ||
				resumed.Data.RemainingTime != 721 || resumed.Data.Answers[answerKey] != "A" ||
				len(resumed.Data.Questions) != 2 || resumed.Data.Questions[0].ID != questionID ||
				resumed.Data.Questions[1].ID != started.Data.Questions[1].ID {
				t.Fatalf("resume exam (legacy=%t): %d %s", legacy, rec.Code, rec.Body.String())
			}
		}
	}
}

func assertCategoryPracticeContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	testutil.SetTestGinMode()
	cfg := &config.Config{JWTSecretKey: "category-practice-contract", JWTExpiresHours: 2}
	router := NewRouter(newContractDeps(t, db, cfg))
	student := failureTestStudentToken(t, db, cfg, "category-practice-student")
	other := failureTestStudentToken(t, db, cfg, "category-practice-other")
	categoryA := testutil.SeedTrainingCategory(t, db, "维修")
	categoryB := testutil.SeedTrainingCategory(t, db, "操作")
	emptyCategory := testutil.SeedTrainingCategory(t, db, "空分类")
	seedQuestion := func(category int, content, status string) int {
		t.Helper()
		question := model.Question{Type: "single_choice", Content: content, Answer: "A", Status: status, CategoryID: &category}
		if err := db.Create(&question).Error; err != nil {
			t.Fatal(err)
		}
		return question.ID
	}
	first := seedQuestion(categoryA.ID, "第一题", "published")
	moved := seedQuestion(categoryA.ID, "将移类的第二题", "published")
	third := seedQuestion(categoryA.ID, "第三题", "published")
	foreign := seedQuestion(categoryB.ID, "另一分类的题", "published")
	_ = seedQuestion(categoryA.ID, "未发布题", "draft")
	start := func(token string, category int, restart bool, want int, mode ...string) practicemode.CategoryPracticeStartDTO {
		t.Helper()
		body := map[string]any{"category_id": category, "mode": "sequential", "restart": restart}
		if len(mode) > 0 {
			body["mode"] = mode[0]
		}
		rec := doWithToken(t, router, token, http.MethodPost, "/api/practice-mode/attempts", body)
		if rec.Code != want {
			t.Fatalf("start HTTP %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
		var payload struct {
			Data practicemode.CategoryPracticeStartDTO `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.Data
	}
	answer := func(token, key string, questionID, want int) {
		t.Helper()
		rec := doWithToken(t, router, token, http.MethodPost, "/api/practice-mode/attempts/answer",
			map[string]any{"attempt_key": key, "question_id": questionID, "user_answer": "A"})
		if rec.Code != want {
			t.Fatalf("answer HTTP %d, want %d: %s", rec.Code, want, rec.Body.String())
		}
	}
	start(student, 0, false, http.StatusBadRequest)
	start(student, emptyCategory.ID, false, http.StatusBadRequest)
	a := start(student, categoryA.ID, false, http.StatusOK)
	b := start(student, categoryB.ID, false, http.StatusOK)
	if a.AttemptKey == "" || a.AttemptKey == b.AttemptKey || len(a.Questions) != 3 ||
		len(b.Questions) != 1 || b.Questions[0].ID != foreign {
		t.Fatalf("category sessions mixed: a=%+v b=%+v", a, b)
	}
	answer(other, a.AttemptKey, first, http.StatusNotFound)
	answer(student, a.AttemptKey, foreign, http.StatusNotFound)
	answer(student, a.AttemptKey, first, http.StatusOK)
	answer(student, a.AttemptKey, first, http.StatusOK)
	if err := db.Model(&model.Question{}).Where("id IN ?", []int{first, moved}).Update("category_id", categoryB.ID).Error; err != nil {
		t.Fatal(err)
	}
	answer(student, a.AttemptKey, moved, http.StatusNotFound)
	newQuestion := seedQuestion(categoryA.ID, "新增题不会插入旧断点", "published")
	resume := start(student, categoryA.ID, false, http.StatusOK)
	if resume.AttemptKey != a.AttemptKey || resume.Completed != 1 || resume.CurrentIndex != 0 ||
		len(resume.Questions) != 1 || resume.Questions[0].ID != third || resume.AnswersState[strconv.Itoa(first)] == nil {
		t.Fatalf("resume changed the saved question set: %+v", resume)
	}
	answer(student, a.AttemptKey, newQuestion, http.StatusNotFound)
	answer(student, a.AttemptKey, third, http.StatusOK)
	start(student, categoryA.ID, false, http.StatusConflict)
	restart := start(student, categoryA.ID, true, http.StatusOK)
	if restart.AttemptKey == a.AttemptKey || restart.Completed != 0 || len(restart.Questions) != 2 {
		t.Fatalf("restart did not create a new attempt: %+v", restart)
	}
	answer(student, a.AttemptKey, first, http.StatusNotFound)
	var records []model.QuestionPracticeRecord
	if err := db.Order("id").Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].QuestionID != first || records[1].QuestionID != third ||
		records[0].CategoryID == nil || *records[0].CategoryID != categoryA.ID ||
		records[1].CategoryID == nil || *records[1].CategoryID != categoryA.ID {
		t.Fatalf("unexpected practice history: %+v", records)
	}
	if err := db.Create(&model.QuestionPracticeRecord{
		StudentID: records[0].StudentID, QuestionID: foreign, UserAnswer: "B", CreatedAt: testutil.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		category string
		total    int
	}{
		{"all", 3},
		{strconv.Itoa(categoryA.ID), 2},
		{strconv.Itoa(categoryB.ID), 0},
		{"unclassified", 1},
	} {
		probe := func(endpoint, totalKey string) {
			t.Helper()
			rec := doWithToken(t, router, student, http.MethodGet, fmt.Sprintf("/api/practice-mode/%s?category_id=%s", endpoint, tc.category), nil)
			data := adminObject(t, rec, http.StatusOK)
			if got := jsonInt(t, data[totalKey]); got != tc.total {
				t.Fatalf("%s (%s): total = %d, want %d", endpoint, tc.category, got, tc.total)
			}
		}
		probe("history", "total")
		probe("stats", "total")
		probe("practice-stats", "total_count")
	}

	wrong := func(key string, questionID int) {
		t.Helper()
		response := doWithToken(t, router, student, http.MethodPost, "/api/practice-mode/attempts/answer",
			map[string]any{"attempt_key": key, "question_id": questionID, "user_answer": "B"})
		if response.Code != http.StatusOK {
			t.Fatalf("submit wrong answer: %d %s", response.Code, response.Body.String())
		}
	}
	wrong(restart.AttemptKey, third)
	if err := db.Model(&model.Question{}).Where("id = ?", third).Update("category_id", categoryB.ID).Error; err != nil {
		t.Fatal(err)
	}
	wrong(start(student, categoryB.ID, true, http.StatusOK).AttemptKey, third)
	for _, category := range []string{"all", strconv.Itoa(categoryA.ID), strconv.Itoa(categoryB.ID), "unclassified"} {
		rec := doWithToken(t, router, student, http.MethodGet, "/api/wrong-questions?category_id="+category, nil)
		data := adminObject(t, rec, http.StatusOK)
		total := jsonInt(t, data["total"])
		if category == "unclassified" && total != 0 || category != "unclassified" && total != 1 {
			t.Fatalf("wrong questions %s: got %d: %s", category, total, rec.Body.String())
		}
	}
	redo := func(category string) {
		t.Helper()
		rec := doWithToken(t, router, student, http.MethodPost,
			fmt.Sprintf("/api/wrong-questions/%d/redo?category_id=%s", third, category), map[string]any{"user_answer": "A"})
		if rec.Code != http.StatusOK {
			t.Fatalf("redo %s: %d %s", category, rec.Code, rec.Body.String())
		}
	}
	redo(strconv.Itoa(categoryA.ID))
	redo("all")
	var redoRecords []model.QuestionPracticeRecord
	if err := db.Where("practice_type = ?", "redo").Order("id").Find(&redoRecords).Error; err != nil {
		t.Fatal(err)
	}
	if len(redoRecords) != 2 || redoRecords[0].CategoryID == nil || *redoRecords[0].CategoryID != categoryA.ID ||
		redoRecords[1].CategoryID == nil || *redoRecords[1].CategoryID != categoryB.ID {
		t.Fatalf("redo did not preserve historical classification: %+v", redoRecords)
	}

	tag := model.QuestionTag{Code: "category_practice_tag", Name: "分类练习考点", Status: 1}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}
	tagMode := string(practicemode.PracticeModeTag(tag.ID))
	legacy := model.PracticeProgress{
		StudentID: records[0].StudentID, PracticeMode: tagMode, Total: 1, CurrentIndex: 1,
		QuestionIDs: model.JSONB(fmt.Sprintf("[%d]", first)), AnswersState: model.JSONB(`{"old":"answer"}`),
	}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	tagAFirst := seedQuestion(categoryA.ID, "标签 A 的第一题", "published")
	for _, id := range []int{tagAFirst, newQuestion, first} {
		if err := db.Exec("INSERT INTO question_tag_relation (question_id, tag_id) VALUES (?, ?)", id, tag.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	tagA := start(student, categoryA.ID, false, http.StatusOK, tagMode)
	tagB := start(student, categoryB.ID, false, http.StatusOK, tagMode)
	if tagA.AttemptKey == tagB.AttemptKey || len(tagA.Questions) != 2 || len(tagB.Questions) != 1 || tagA.Completed != 0 {
		t.Fatalf("tag progress bled across categories or inherited the unknown tag progress: A=%+v B=%+v", tagA, tagB)
	}
	answer(student, tagA.AttemptKey, tagAFirst, http.StatusOK)
	if err := db.Exec("DELETE FROM question_tag_relation WHERE question_id = ? AND tag_id = ?", newQuestion, tag.ID).Error; err != nil {
		t.Fatal(err)
	}
	answer(student, tagA.AttemptKey, newQuestion, http.StatusNotFound)
	start(student, categoryA.ID, false, http.StatusConflict, tagMode)
	var restored model.PracticeProgress
	if err := db.First(&restored, legacy.ID).Error; err != nil || restored.CategoryID != nil || restored.CurrentIndex != 1 {
		t.Fatalf("legacy tag progress did not remain intact: %+v %v", restored, err)
	}
}
