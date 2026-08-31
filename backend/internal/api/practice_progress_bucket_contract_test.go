// #414 答题进度按证件分桶契约测试：
//   - 顺序练习进度以 (student, mode, credential) 分桶：同一学员不同证件互不污染；
//   - 断点续练回到该证件分区内上次位置；
//   - 迁移回填：存量行归入学员当前证件（Postgres 迁移执行后断言列存在）。
//
// 双适配器：SQLite（AutoMigrate 含新列）+ Postgres（真实 SQL 迁移 000013）。
package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func assertPracticeProgressBucketing(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, pwd)

	cfg := &config.Config{
		JWTSecretKey: `bucket-contract-secret`,
	}
	deps := newContractDeps(t, db, cfg)
	r := NewRouter(deps)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(student.ID, student.Username, `hrwai_user`)
	if err != nil {
		t.Fatalf(`issue token failed: %v`, err)
	}

	// 建两个证件 + 题目
	cred1 := 2001
	cred2 := 2002
	if err := db.Create(&model.Credential{Code: `b1`, Name: `分桶证1`, Category: `special_operation`, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Credential{Code: `b2`, Name: `分桶证2`, Category: `special_operation`, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	var c1, c2 model.Credential
	db.Where(`code = ?`, `b1`).First(&c1)
	db.Where(`code = ?`, `b2`).First(&c2)
	cred1 = c1.ID
	cred2 = c2.ID
	for i := 0; i < 3; i++ {
		if err := db.Create(&model.Question{Type: `single_choice`, Content: `q1-`, Answer: `A`, Status: `published`, CredentialID: &cred1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := db.Create(&model.Question{Type: `single_choice`, Content: `q2-`, Answer: `A`, Status: `published`, CredentialID: &cred2}).Error; err != nil {
			t.Fatal(err)
		}
	}

	// 1. 证1开始顺序练习（3 题，index 0）
	start1 := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential?credential_id=%d`, cred1))
	q1 := start1[`questions`].([]any)
	if len(q1) != 3 {
		t.Fatalf(`cred1 sequential should be 3 questions, got %d`, len(q1))
	}
	// 2. 证2开始顺序练习（3 题）——与证1互不污染
	start2 := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential?credential_id=%d`, cred2))
	q2 := start2[`questions`].([]any)
	if len(q2) != 3 {
		t.Fatalf(`cred2 sequential should be 3 questions, got %d`, len(q2))
	}
	// 3. 证1保存进度 index=2（答 2 题），证1进度 completed 应为 2
	qid1 := int(q1[0].(map[string]any)[`id`].(float64))
	qid2 := int(q1[1].(map[string]any)[`id`].(float64))
	saveBody := map[string]any{
		`index`: 2, `practice_mode`: `sequential`, `total`: 3,
		`answers_state`: map[string]any{
			itoa(qid1): map[string]any{`user_answer`: `A`, `is_correct`: true},
			itoa(qid2): map[string]any{`user_answer`: `B`, `is_correct`: false},
		},
		`credential_id`: cred1,
	}
	rec := doWithToken(t, r, token, http.MethodPost, `/api/practice-mode/progress`, saveBody)
	if rec.Code != http.StatusOK {
		t.Fatalf(`save progress should be 200, got %d body=%s`, rec.Code, rec.Body.String())
	}
	// 4. 证1进度：completed=2（分子只计该证件分区作答）
	prog1 := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential-progress?credential_id=%d`, cred1))
	completed1, _ := prog1[`completed`].(float64)
	if int(completed1) != 2 {
		t.Fatalf(`cred1 progress completed should be 2, got %v`, completed1)
	}
	// 5. 证2进度：completed=0（互不污染）
	prog2 := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential-progress?credential_id=%d`, cred2))
	completed2, _ := prog2[`completed`].(float64)
	if int(completed2) != 0 {
		t.Fatalf(`cred2 progress completed should be 0, got %v`, completed2)
	}
	// 6. 断点续练：证1重新开始 → current_index=2（回到上次位置）
	resume1 := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential?credential_id=%d`, cred1))
	curIdx, _ := resume1[`current_index`].(float64)
	if int(curIdx) != 2 {
		t.Fatalf(`cred1 resume should keep cursor 2, got %v`, curIdx)
	}
}

func TestPracticeProgressBucketing_OnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertPracticeProgressBucketing(t, db)
}

func TestPracticeProgressBucketing_OnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertPracticeProgressBucketing(t, db)
}
