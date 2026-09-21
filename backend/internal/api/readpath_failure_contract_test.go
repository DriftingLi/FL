// 契约测（ADR-0062 票6）：贯穿判据「查不动 ≠ 查得空」。
// seam S1（HTTP 契约层）。故障注入 = 删表 / 关连接池（与 admin_inspection_read_paths_test.go、
// paging_failure_contract_test.go 同族手法），每次只打断被断言的那一条查询。
package api

import (
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

// failureTestStudentToken 造一名学员并签发 access token（本文件三个用例共用）。
func failureTestStudentToken(t *testing.T, db *gorm.DB, cfg *config.Config, account string) string {
	t.Helper()
	pwd, err := service.HashPassword("student123")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	student := testutil.SeedStudent(t, db, account, pwd)
	token, err := securitySession(cfg).Issue(student.ID, student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}

func securitySession(cfg *config.Config) *security.Session {
	return security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
}

func failureTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "readpath-failure-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	return NewRouter(newContractDeps(t, db, cfg)), db, cfg
}

// TestQuestionKnowledgeDBFailureRenders500 考点读面：旧写法把 error 丢给 _ 后照样
// 返回 200 + data:null（「查不到」被答成「这题没有考点」）。
func TestQuestionKnowledgeDBFailureRenders500(t *testing.T) {
	r, db, cfg := failureTestRouter(t)
	token := failureTestStudentToken(t, db, cfg, "knowledge_fail_stu")
	if err := db.Migrator().DropTable(&model.QuestionTag{}); err != nil {
		t.Fatalf("注入故障（删考点表）失败: %v", err)
	}
	rec := doWithToken(t, r, token, http.MethodGet, "/api/questions/1/knowledge", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("考点查询失败必须 500（fail-closed）: got %d %s", rec.Code, rec.Body.String())
	}
}

// TestPracticeStatsDBFailureRenders500 练习统计：旧签名没有 error 出口，DB 抖动时返回
// 「0 题、正确率 0%」的 200，而同页另一侧的 /practice-stats 却 500（同页自相矛盾）。
func TestPracticeStatsDBFailureRenders500(t *testing.T) {
	r, db, cfg := failureTestRouter(t)
	token := failureTestStudentToken(t, db, cfg, "stats_fail_stu")
	if err := db.Migrator().DropTable(&model.QuestionPracticeRecord{}); err != nil {
		t.Fatalf("注入故障（删练习记录表）失败: %v", err)
	}
	rec := doWithToken(t, r, token, http.MethodGet, "/api/practice-mode/stats", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("练习统计查不动必须 500: got %d %s", rec.Code, rec.Body.String())
	}
}

// TestJobListApplyStateFailureRenders500 职位列表的投递状态回填查不动时整条列表必须失败：
// 旧写法 `if err != nil { return }` 会把所有职位回成「可投递」（按钮全绿，点了才 400）。
// 需先有一条开放职位——空列表会在回填前短路，测不到注入的失败。
func TestJobListApplyStateFailureRenders500(t *testing.T) {
	r, db, cfg := failureTestRouter(t)
	token := failureTestStudentToken(t, db, cfg, "apply_state_fail_stu")
	job := model.JobPosting{RecruiterID: 1, Title: "叉车维修技师", Status: "open",
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(), PublishedAt: testutil.Now()}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("建开放职位失败: %v", err)
	}
	if err := db.Migrator().DropTable(&model.JobApplication{}); err != nil {
		t.Fatalf("注入故障（删投递表）失败: %v", err)
	}
	rec := doWithToken(t, r, token, http.MethodGet, "/api/jobs", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("投递状态查不动必须 500，不得把职位全回成「可投递」: got %d %s", rec.Code, rec.Body.String())
	}
}

// TestPublicTagsCountFailureRenders500 学员端标签列表：计数查询失败必须 500，
// 而不是回一份「 QuestionCount 缺失/为零」的标签列表；同路径上「排除来源标记标签」的
// 查询失败也不得被读成「没有要排除的标签」（旧写法 err == nil 才排除）。
// 注入 = 删 question_tag_relation：标签列表与来源标签查询都走 question_tag，只有计数 join 到该表。
func TestPublicTagsCountFailureRenders500(t *testing.T) {
	r, db, _ := failureTestRouter(t)
	tag := model.QuestionTag{Code: "hydraulic", Name: "液压", Status: 1, CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("建标签失败: %v", err)
	}
	if err := db.Migrator().DropTable(&model.QuestionTagRelation{}); err != nil {
		t.Fatalf("注入故障（删标签关系表）失败: %v", err)
	}
	rec := doJSON(t, r, http.MethodGet, "/api/tags", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("标签计数查不动必须 500: got %d %s", rec.Code, rec.Body.String())
	}
}
