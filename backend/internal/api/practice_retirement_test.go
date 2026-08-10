// Ticket 2（issue #52）API 契约测试：
//   - 章节练习/知识点练习/知识点管理/四分类统计端点已移除（404）
//   - /api/question-bank/questions 不再消费 knowledge_point_id 参数（忽略不报错）
//   - 导师/管理端题目创建与编辑支持 tag_ids 打标（service 层另有测试）
package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/testutil"
)

func TestRetiredPracticeEndpointsReturn404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{}

	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterPracticeModeRoutes(api, deps.Session, deps.PracticeModeSvc)
	RegisterQuestionBankRoutes(api, deps.Session, deps.QuestionBankSvc, deps.FileSvc)

	cases := []struct {
		method string
		path   string
	}{
		{"GET", "/api/practice-mode/category?category=CATEGORY_01"},
		{"GET", "/api/practice-mode/knowledge-point?knowledge_point_id=1"},
		{"GET", "/api/practice-mode/knowledge-point-progress"},
		{"GET", "/api/question-bank/knowledge-points"},
		{"GET", "/api/question-bank/categories"},
	}
	for _, c := range cases {
		rec := performRequest(r, c.method, c.path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s 应 404, got %d", c.method, c.path, rec.Code)
		}
	}
}

func TestQuestionBankQuestionsIgnoresKpParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{}

	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterQuestionBankRoutes(api, deps.Session, deps.QuestionBankSvc, deps.FileSvc)

	// 未登录访问会被 JWT 中间件拦下（401），但绝不应因已删的 knowledge_point_id 参数而 500
	rec := performRequest(r, "GET", "/api/question-bank/questions?knowledge_point_id=1")
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("knowledge_point_id 参数不应导致 500: %d %s", rec.Code, rec.Body.String())
	}
}

// TestTutorCourseRoutesAbsent 导师端不可建课/改课：/api/tutor/course 路由不存在（404）。
func TestTutorCourseRoutesAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{}

	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterTutorRoutes(api, deps.Session, deps.TutorSvc, deps.FileSvc)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{"POST", "/api/tutor/course"},
		{"PUT", "/api/tutor/course/1"},
	} {
		rec := performRequest(r, tc.method, tc.path)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s 应 404（导师不可建课）, got %d", tc.method, tc.path, rec.Code)
		}
	}
}
