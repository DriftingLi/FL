// #413 顺序练习池口径契约测试：
//   - 带 / 不带 credential_id 返回不同题目集合（当前证件题库池分区）；
//   - sequential-progress 的 pool_total 与开始顺序练习返回的题目数一致（卡片分母口径）；
//   - question-bank/stats 总数按池口径（当前证件分区）。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
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

func fetchMap(t *testing.T, r *gin.Engine, token, path string) map[string]any {
	t.Helper()
	rec := doWithToken(t, r, token, http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf(`GET %s should be 200, got %d body=%s`, path, rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf(`parse envelope failed: %v`, err)
	}
	data, _ := env[`data`].(map[string]any)
	if data == nil {
		t.Fatalf(`data missing for %s: %s`, path, rec.Body.String())
	}
	return data
}

func assertPracticePoolCaliber(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	pwd, _ := service.HashPassword(`student123`)
	student := testutil.SeedStudent(t, db, `stu1`, pwd)
	cfg := &config.Config{
		JWTSecretKey: `pool-contract-secret`,
	}
	r := gin.New()
	api := r.Group(`/api`)
	deps := newContractDeps(t, db, cfg)
	RegisterPracticeModeRoutes(api, deps.RouterDeps(), deps.PracticeModeSvc)
	RegisterQuestionBankRoutes(api, deps.RouterDeps(), deps.QuestionBankSvc, deps.FileSvc)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(student.ID, student.Username, `hrwai_user`)
	if err != nil {
		t.Fatalf(`issue token failed: %v`, err)
	}

	// 用不冲突的证件 ID（1001/1002）：Postgres 适配器跑真实迁移会带基线种子题，
	// 不能假设证件 1/2 为空库。断言用相对口径（集合归属 + pool_total 与抽题数一致）。
	// Postgres 适配器有外键（question.credential_id → credential.id），先建证件行（SQLite 无外键也幂等）。
	if err := db.Create(&model.Credential{Code: `t1001`, Name: `测试证1`, Category: `special_operation`, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Credential{Code: `t1002`, Name: `测试证2`, Category: `special_operation`, Status: 1}).Error; err != nil {
		t.Fatal(err)
	}
	var cred1Row, cred2Row model.Credential
	db.Where(`code = ?`, `t1001`).First(&cred1Row)
	db.Where(`code = ?`, `t1002`).First(&cred2Row)
	cred1 := cred1Row.ID
	cred2 := cred2Row.ID
	for i := 0; i < 2; i++ {
		if err := db.Create(&model.Question{Type: `single_choice`, Content: `q1-`, Answer: `A`, Status: `published`, CredentialID: &cred1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := db.Create(&model.Question{Type: `single_choice`, Content: `q2-`, Answer: `A`, Status: `published`, CredentialID: &cred2}).Error; err != nil {
			t.Fatal(err)
		}
	}

	// 1. 带证件参数：只回该证件分区题目，且集合互斥
	cred1Data := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential?credential_id=%d`, cred1))
	q1 := cred1Data[`questions`].([]any)
	if len(q1) != 2 {
		t.Fatalf(`cred1 sequential should return 2 questions, got %d`, len(q1))
	}
	for _, raw := range q1 {
		q := raw.(map[string]any)
		if id, _ := q[`credential_id`].(float64); id != float64(cred1) {
			t.Fatalf(`cred1 row should belong to cred1, got %v`, q[`credential_id`])
		}
	}
	cred2Data := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential?credential_id=%d`, cred2))
	q2 := cred2Data[`questions`].([]any)
	if len(q2) != 2 {
		t.Fatalf(`cred2 sequential should return 2 questions, got %d`, len(q2))
	}
	// 2. 不带证件参数：返回全部池内题（至少覆盖两个证件，集合大于单证件分区）
	allData := fetchMap(t, r, token, `/api/practice-mode/sequential`)
	qAll := allData[`questions`].([]any)
	if len(qAll) < len(q1)+len(q2) {
		t.Fatalf(`no-credential sequential should cover both credentials, got %d`, len(qAll))
	}
	// 3. 进度返回体 pool_total = 开始练习题目数（卡片分母口径）
	prog := fetchMap(t, r, token, fmt.Sprintf(`/api/practice-mode/sequential-progress?credential_id=%d`, cred1))
	poolTotal, _ := prog[`pool_total`].(float64)
	if int(poolTotal) != len(q1) {
		t.Fatalf(`pool_total should equal started question count %d, got %v`, len(q1), poolTotal)
	}
	// 4. 题库统计总数按池口径（该证件池计数）
	stats := fetchMap(t, r, token, fmt.Sprintf(`/api/question-bank/stats?credential_id=%d`, cred1))
	stTotal, _ := stats[`total`].(float64)
	if int(stTotal) != len(q1) {
		t.Fatalf(`stats total should be pool count for cred1, got %v`, stTotal)
	}
}

func TestPracticePoolContract_CaliberOnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertPracticePoolCaliber(t, db)
}

func TestPracticePoolContract_CaliberOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertPracticePoolCaliber(t, db)
}
