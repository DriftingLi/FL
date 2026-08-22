// 错题重做契约测试：POST /api/wrong-questions/:id/redo 返回 typed SubmitResultDTO
// （练习/错题重做共用装配，spec #294/#300）。冻结 JSON key 形状与练习记录落库口径。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestWrongQuestionRedoContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	student := model.HrwaiUser{Account: "acct_wq", Phone: "13800000003", Username: "错题学员", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}
	q := testutil.SeedQuestion(t, db, "single_choice", "液压安全阀作用", "A")
	wq := model.WrongQuestion{StudentID: int(student.ID), QuestionID: q.ID, WrongCount: 2, LastWrongAt: testutil.Now(), CreatedAt: testutil.Now()}
	if err := db.Create(&wq).Error; err != nil {
		t.Fatalf("创建错题失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterWrongQuestionRoutes(api, deps.RouterDeps(), deps.WrongQuestionSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"user_answer": "A"})
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/wrong-questions/%d/redo", q.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("redo 应 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Code != http.StatusOK {
		t.Fatalf("信封不符: raw=%s err=%v", w.Body.String(), err)
	}
	// typed 形状冻结：key 与 SubmitResultDTO 一致（is_removed 已随 map 出参移除）。
	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 应为对象: %v", err)
	}
	for _, key := range []string{"question_id", "correct_answer", "explanation"} {
		if _, ok := data[key]; !ok {
			t.Errorf("缺少契约 key %q: %v", key, data)
		}
	}
	if v, ok := data["is_correct"].(bool); !ok || !v {
		t.Errorf("答对应 is_correct=true, got %v", data["is_correct"])
	}
	if _, exists := data["is_removed"]; exists {
		t.Error("map 出参时代的 is_removed 不应再出现")
	}
	// 重做结果落练习记录（PracticeType=redo）
	var cnt int64
	db.Model(&model.QuestionPracticeRecord{}).
		Where("student_id = ? AND question_id = ? AND practice_type = ? AND is_correct = ?", student.ID, q.ID, "redo", true).
		Count(&cnt)
	if cnt != 1 {
		t.Fatalf("应恰好落一条 redo 练习记录, got %d", cnt)
	}
	// 答对置 is_redone
	var after model.WrongQuestion
	db.First(&after, "id = ?", wq.ID)
	if !after.IsRedone {
		t.Fatal("答对后错题应标记 is_redone=true")
	}
}
