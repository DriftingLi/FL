// Package api #981 回归：题目按 id 直取的读路径必须过题库池口径。
//
// 背景（ADR-0049 决策 4 的前置）：GET /api/question-bank/questions/:id 与
// POST /api/practice-mode/submit 都只按主键取题，学员可读到 draft 与源标记真题题的
// 题干 / 选项 / 答案 / 解析。本测试钉住修复后的口径：
// 学员 = 池口径（published + 排源标记真题 + 当前证件），导师编辑路径不受影响。
package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestQuestionByIdReadPathEnforcesPool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "pool-read-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterQuestionBankRoutes(api, deps.RouterDeps(), deps.QuestionBankSvc, deps.FileSvc)
	RegisterPracticeModeRoutes(api, deps.RouterDeps(), deps.PracticeModeSvc)

	credA, credB := 1, 2
	student := model.HrwaiUser{Account: "pool_read_user", Phone: "13800000777", Username: "学员", Status: 1, CurrentCredentialID: &credA, CreatedAt: testutil.Now()}
	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("建学员失败: %v", err)
	}
	tutor := model.Tutor{Username: "pool_read_tutor", Password: "x", Name: "讲师", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&tutor).Error; err != nil {
		t.Fatalf("建导师失败: %v", err)
	}

	mkQ := func(content, status string, cred *int) model.Question {
		q := model.Question{
			Type: "single_choice", Content: content, Answer: "A", Explanation: "解析:" + content,
			Status: status, CredentialID: cred,
			CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
		}
		if err := db.Create(&q).Error; err != nil {
			t.Fatalf("建题失败: %v", err)
		}
		return q
	}
	poolQ := mkQ("池内题", "published", &credA)
	draftQ := mkQ("草稿题", "draft", &credA)
	otherCredQ := mkQ("别的证件的题", "published", &credB)
	sourceQ := mkQ("真题题", "published", &credA)
	tag := model.QuestionTag{Code: "SRC981", Name: "真题", IsSourceTag: true, Status: 1, CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("建标签失败: %v", err)
	}
	if err := db.Create(&model.QuestionTagRelation{QuestionID: sourceQ.ID, TagID: tag.ID, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("建标签关系失败: %v", err)
	}

	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	stuToken, _ := sess.Issue(student.ID, student.Account, "hrwai_user")
	tutToken, _ := sess.Issue(tutor.TutorID, tutor.Username, "tutor")

	get := func(token string, id int) int {
		rec := doWithToken(t, r, token, http.MethodGet, fmt.Sprintf("/api/question-bank/questions/%d", id), nil)
		return rec.Code
	}

	// 学员：池内可读；草稿 / 源标记真题题 / 非当前证件一律不可见（404，不是 403）
	if code := get(stuToken, poolQ.ID); code != http.StatusOK {
		t.Fatalf("学员读池内题应 200, got %d", code)
	}
	for name, id := range map[string]int{"draft": draftQ.ID, "source": sourceQ.ID, "otherCred": otherCredQ.ID} {
		if code := get(stuToken, id); code != http.StatusNotFound {
			t.Fatalf("学员读 %s 应 404, got %d", name, code)
		}
	}
	// 导师：编辑路径读 draft 不受影响
	if code := get(tutToken, draftQ.ID); code != http.StatusOK {
		t.Fatalf("导师读 draft 应 200（编辑路径不得回归）, got %d", code)
	}

	// 提交路径同理：源标记真题题不得经 /practice-mode/submit 换到答案与解析
	submit := func(token string, id int) (int, string) {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/practice-mode/submit", map[string]any{"question_id": id, "user_answer": "A"})
		return rec.Code, rec.Body.String()
	}
	if code, body := submit(stuToken, sourceQ.ID); code == http.StatusOK {
		t.Fatalf("源标记真题题不得经提交路径作答: %d %s", code, body)
	}
	if code, body := submit(stuToken, poolQ.ID); code != http.StatusOK {
		t.Fatalf("池内题应可提交, got %d %s", code, body)
	}
}
