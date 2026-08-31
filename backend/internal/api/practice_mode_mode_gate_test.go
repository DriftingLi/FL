// Package api #386 回归：全局搜索 question 分区走题库池口径（排真题题）、
// practice-mode 进度读写入口对未知 mode 返回 400、模考历史卷来源字段。
package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// TestSearchQuestionExcludesSourceTagged 搜索 question 分区走题库池口径：
// 来源标记标签的真题题退出搜索；公共池未打标真题题仍可搜。
func TestSearchQuestionExcludesSourceTagged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	catalogSvc := service.NewTrainingCatalogService(db, nil)
	qsvc := service.NewQuestionBankService(db, nil, nil)

	srcTag, _ := catalogSvc.CreateQuestionTag(service.QuestionTagInput{Code: "real_exam", Name: "真题"})
	if err := db.Model(&model.QuestionTag{}).Where("id = ?", srcTag.ID).Update("is_source_tag", true).Error; err != nil {
		t.Fatalf("置 source 标签失败: %v", err)
	}
	mk := func(tagIDs []int, content string) {
		if _, err := qsvc.CreateQuestion(map[string]any{
			"type": "single_choice", "content": content, "options": []string{"A", "B"}, "answer": "A",
			"status": "published", "tag_ids": tagIDs,
		}, nil, "tutor"); err != nil {
			t.Fatalf("建题失败: %v", err)
		}
	}
	mk([]int{srcTag.ID}, "液压系统真题关键词")
	mk(nil, "液压系统普通关键词")
	mk(nil, "液压系统真题卷关键词") // 去重折叠进公共池的未打标真题题

	svc := service.NewSearchService(db, nil)
	items, err := svc.Search("液压系统", service.SearchTypeQuestion, 1, 20)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	page := items.(*service.SearchPageDTO)
	if page.Total != 2 {
		t.Fatalf("应命中 2 条（排真题题）, got %d: %+v", page.Total, page.Items)
	}
	seen := map[string]bool{}
	for _, it := range page.Items {
		seen[it.Title] = true
	}
	if seen["液压系统真题关键词"] {
		t.Fatal("来源标记真题题不应出现在搜索结果")
	}
	if !seen["液压系统普通关键词"] || !seen["液压系统真题卷关键词"] {
		t.Fatal("普通题与未打标真题卷题应可搜到")
	}

	// 聚合路径（type 缺省各分区 top5）同样过滤
	all, err := svc.Search("液压系统", "", 1, 20)
	if err != nil {
		t.Fatalf("聚合搜索失败: %v", err)
	}
	allDTO := all.(*service.SearchAllDTO)
	for _, it := range allDTO.Questions.Items {
		if it.Title == "液压系统真题关键词" {
			t.Fatal("聚合搜索不应出现来源标记真题题")
		}
	}
}

// TestPracticeModeUnknownModeRejected400 进度读写入口对未知 mode 返回 400；
// 合法三形态（sequential / tag:<id> / paper:<id>）不受影响。
func TestPracticeModeUnknownModeRejected400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "mode-gate-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterPracticeModeRoutes(api, deps.RouterDeps(), deps.PracticeModeSvc)

	student := model.HrwaiUser{Account: "mode_gate_user", Phone: "13800000999", Username: "学员", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("create student: %v", err)
	}
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	token, _ := sess.Issue(student.ID, student.Account, "hrwai_user")

	// 未知 mode：写进度 400（typo "sequental" / 非法 "tag:abc" / "free"）
	for _, mode := range []string{"sequental", "tag:abc", "free", "paper:"} {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/practice-mode/progress", map[string]any{"index": 1, "practice_mode": mode, "total": 3})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("写进度未知 mode %q 应 400, got %d %s", mode, rec.Code, rec.Body.String())
		}
		rec = doWithToken(t, r, token, http.MethodGet, "/api/practice-mode/progress?mode="+mode, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("读进度未知 mode %q 应 400, got %d %s", mode, rec.Code, rec.Body.String())
		}
	}

	// 合法三形态不受影响（空 mode 默认 sequential）
	for _, mode := range []string{"sequential", "tag:5", "paper:7", ""} {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/practice-mode/progress", map[string]any{"index": 1, "practice_mode": mode, "total": 3})
		if rec.Code != http.StatusOK {
			t.Fatalf("写进度合法 mode %q 应 200, got %d %s", mode, rec.Code, rec.Body.String())
		}
		query := "/api/practice-mode/progress"
		if mode != "" {
			query += "?mode=" + mode
		}
		rec = doWithToken(t, r, token, http.MethodGet, query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("读进度合法 mode %q 应 200, got %d %s", mode, rec.Code, rec.Body.String())
		}
	}

	// 进度行只落合法 mode，无 typo 孤儿行
	var cnt int64
	if err := db.Model(&model.PracticeProgress{}).Where("student_id = ?", student.ID).Count(&cnt).Error; err != nil || cnt != 3 {
		t.Fatalf("应恰落 3 行合法进度, got %d err=%v", cnt, err)
	}
}

// TestMockExamHistoryCarriesPaperID 模考历史响应补卷来源：按卷开考的历史条目
// 携带 paper_id；随机模考（无卷）不出现该字段（向后兼容）。
func TestMockExamHistoryCarriesPaperID(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := service.NewMockExamService(db, nil, nil)
	student := testutil.SeedStudent(t, db, "paper_src_student", "x")

	now := time.Now()
	paperID := 42
	withPaper := model.MockExam{StudentID: student.ID, Status: "submitted", StartTime: &now, SubmitTime: &now, CreatedAt: now, PaperID: &paperID, Duration: 90}
	random := model.MockExam{StudentID: student.ID, Status: "submitted", StartTime: &now, SubmitTime: &now, CreatedAt: now, Duration: 90}
	for _, m := range []model.MockExam{withPaper, random} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("插入模拟考试失败: %v", err)
		}
	}

	got := svc.GetHistory(student.ID, 1, 10)
	if got.Total != 2 {
		t.Fatalf("应有 2 条历史, got %d", got.Total)
	}
	paperFound, randomFound := false, false
	for _, e := range got.Exams {
		if e.PaperID != nil {
			paperFound = true
			if *e.PaperID != paperID {
				t.Fatalf("卷来源应为 %d, got %d", paperID, *e.PaperID)
			}
		} else {
			randomFound = true
		}
	}
	if !paperFound || !randomFound {
		t.Fatalf("应同时含按卷与随机记录: paper=%v random=%v", paperFound, randomFound)
	}
}
