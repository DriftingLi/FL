// ADR-0048 片六（#964）契约测试：本片**新补 / 新改** @Success data 指认的端点，
// 逐条断言信封与 data 顶层 key（片一先例）。
//
// 与既有契约测试的分工：键序 / 可空性由 service 层 DTO 字节锁
// （internal/service/envelope_dto_shape_test.go 的 TestInlineResponseDTOBytes）冻结；
// 本文件锁定「注解声明的类型 == handler 实际渲染的信封形状」—— 注解成为唯一事实源后，
// 这条断言是「指认错类型」的直接护栏。
package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

func newSlice6Env(t *testing.T) (*gin.Engine, *config.Config, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	// 与生产装配同形：文件存储注入本地实现（nil storage 会让学生/讲师文件端点 panic → 500）。
	deps := NewDeps(cfg, db, storage.NewLocalStorage(t.TempDir()), zap.NewNop(), nil)
	return NewRouter(deps), cfg, db
}

// slice6Token 按角色签发测试令牌（能力表见 internal/authz：admin=catalog.manage/question.review，
// tutor=tutor.access/question.author/catalog.author，hrwai_user=student.access/material.read/real_exam.take）。
func slice6Token(t *testing.T, cfg *config.Config, userID int, account, role string) string {
	t.Helper()
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(userID, account, role)
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}

func slice6Data(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) map[string]any {
	t.Helper()
	_, _, raw := unpackData(t, rec, wantCode)
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("data 不是对象: %v (%s)", err, raw)
	}
	return m
}

func slice6AssertKeys(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, want ...string) map[string]any {
	t.Helper()
	m := slice6Data(t, rec, wantCode)
	assertDictKeys(t, m, want)
	return m
}

func slice6AssertNullData(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	_, _, raw := unpackData(t, rec, 200)
	if raw != "null" {
		t.Fatalf("期望 data=null（NoData 端点），got %s", raw)
	}
}

// TestSlice6CatalogEnvelopeKeys 培训目录域：公开/管理端列表 + 字典 CRUD + NoData 端点。
func TestSlice6CatalogEnvelopeKeys(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	admin := slice6Token(t, cfg, 1, "admin1", "admin")

	publicCases := []struct {
		name string
		path string
		want []string
	}{
		{"levels", "/api/levels", []string{"levels"}},
		{"tags", "/api/tags", []string{"tags"}},
		{"credentials", "/api/credentials", []string{"credentials"}},
		{"grouped", "/api/credentials/grouped", []string{"skill_level", "special_operation"}},
		{"catalog tree", "/api/catalog/tree", []string{"specialties"}},
	}
	for _, tc := range publicCases {
		t.Run("public "+tc.name, func(t *testing.T) {
			rec := catalogRequest(t, r, "", "GET", tc.path, "")
			slice6AssertKeys(t, rec, 200, tc.want...)
		})
	}

	adminCases := []struct {
		name string
		path string
		want []string
	}{
		{"admin catalog tree", "/api/admin/catalog/tree", []string{"specialties"}},
		{"certificate templates", "/api/admin/certificate-templates", []string{"certificate_templates"}},
		{"question tags", "/api/admin/question-tags", []string{"tags"}},
		{"admin credentials", "/api/admin/credentials", []string{"credentials"}},
	}
	for _, tc := range adminCases {
		t.Run("admin "+tc.name, func(t *testing.T) {
			rec := catalogRequest(t, r, admin, "GET", tc.path, "")
			slice6AssertKeys(t, rec, 200, tc.want...)
		})
	}

	specialtyKeys := []string{"code", "created_at", "description", "name", "sort_order", "specialty_id", "status"}
	levelKeys := []string{"code", "created_at", "description", "level_id", "name", "sort_order", "status"}
	templateKeys := []string{"code", "created_at", "description", "id", "name", "status", "template_url", "updated_at", "validity_days"}
	tagKeys := []string{"code", "created_at", "description", "id", "name", "sort_order", "status", "updated_at"}

	// 专业方向：创建 201 / 更新 200（SpecialtyDict）
	rec := catalogRequest(t, r, admin, "POST", "/api/admin/specialty", `{"code":"operation","name":"操作","sort_order":1}`)
	m := slice6AssertKeys(t, rec, 201, specialtyKeys...)
	specialtyID := int(m["specialty_id"].(float64))
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/specialty", `{"code":"safety","name":"安全","sort_order":2}`)
	otherSpecialty := slice6AssertKeys(t, rec, 201, specialtyKeys...)
	otherSpecialtyID := int(otherSpecialty["specialty_id"].(float64))
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/specialty/%d", specialtyID), `{"name":"操作二"}`)
	slice6AssertKeys(t, rec, 200, specialtyKeys...)

	// 课程等级：创建 201 / 更新 200（LevelDict）
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/level", `{"code":"L1","name":"入门","sort_order":1}`)
	m = slice6AssertKeys(t, rec, 201, levelKeys...)
	levelID := int(m["level_id"].(float64))
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/level/%d", levelID), `{"name":"入门二"}`)
	slice6AssertKeys(t, rec, 200, levelKeys...)

	// 证书模板：创建 201 / 更新 200（CertificateTemplateDict）
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/certificate-template", `{"code":"CT1","name":"模板","validity_days":365}`)
	m = slice6AssertKeys(t, rec, 201, templateKeys...)
	templateID := int(m["id"].(float64))
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/certificate-template/%d", templateID), `{"name":"模板二"}`)
	slice6AssertKeys(t, rec, 200, templateKeys...)

	// 题库标签：创建/更新不含 question_count（仅列表回填），故键集少一个
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/question-tag", `{"code":"T1","name":"液压"}`)
	m = slice6AssertKeys(t, rec, 201, tagKeys...)
	tagID := int(m["id"].(float64))
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/question-tag/%d", tagID), `{"name":"液压二"}`)
	slice6AssertKeys(t, rec, 200, tagKeys...)

	// 题目打标（QuestionTagsResultDTO）
	question := testutil.SeedQuestion(t, db, "single_choice", "题干", "A")
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/question/%d/tags", question.ID), `{"tag_ids":[]}`)
	slice6AssertKeys(t, rec, 200, "tag_ids")

	// 排序交换：NoData（data=null）
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/specialty/%d/sort", specialtyID), fmt.Sprintf(`{"swap_with":%d}`, otherSpecialtyID))
	slice6AssertNullData(t, rec)
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/level", `{"code":"L2","name":"进阶","sort_order":2}`)
	otherLevel := slice6AssertKeys(t, rec, 201, levelKeys...)
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/level/%d/sort", levelID), fmt.Sprintf(`{"swap_with":%d}`, int(otherLevel["level_id"].(float64))))
	slice6AssertNullData(t, rec)

	// 删除端点：NoData（data=null）
	for _, tc := range []struct {
		name string
		path string
	}{
		{"delete specialty", fmt.Sprintf("/api/admin/specialty/%d", specialtyID)},
		{"delete level", fmt.Sprintf("/api/admin/level/%d", levelID)},
		{"delete certificate template", fmt.Sprintf("/api/admin/certificate-template/%d", templateID)},
		{"delete question tag", fmt.Sprintf("/api/admin/question-tag/%d", tagID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := catalogRequest(t, r, admin, "DELETE", tc.path, "")
			slice6AssertNullData(t, rec)
		})
	}
}

// TestSlice6CredentialAndStudentEnvelopeKeys 证件域 + 学员端学习面。
func TestSlice6CredentialAndStudentEnvelopeKeys(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	admin := slice6Token(t, cfg, 1, "admin1", "admin")
	student := testutil.SeedStudent(t, db, "stu1", "hash")
	stuToken := slice6Token(t, cfg, student.ID, student.Account, "hrwai_user")

	// 未选证件：键在、值为 null（CurrentCredentialDTO 的 x-nullable）
	rec := catalogRequest(t, r, admin, "GET", "/api/me/credential", "")
	m := slice6AssertKeys(t, rec, 200, "credential")
	if m["credential"] != nil {
		t.Fatalf("未选证件时期望 credential=null，got %v", m["credential"])
	}

	credKeys := []string{"category", "code", "created_at", "description", "id", "level", "name", "sort_order", "status", "updated_at"}
	rec = catalogRequest(t, r, admin, "POST", "/api/admin/credential", `{"code":"N1","name":"叉车司机","category":"special_operation"}`)
	m = slice6AssertKeys(t, rec, 201, credKeys...)
	credID := int(m["id"].(float64))
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/credential/%d", credID), `{"name":"叉车司机N1"}`)
	slice6AssertKeys(t, rec, 200, credKeys...)

	rec = catalogRequest(t, r, stuToken, "PATCH", "/api/me/credential", fmt.Sprintf(`{"credential_id":%d}`, credID))
	m = slice6AssertKeys(t, rec, 200, "credential")
	if m["credential"] == nil {
		t.Fatal("切换证件后 credential 不应为 null")
	}
	rec = catalogRequest(t, r, stuToken, "GET", "/api/me/credential", "")
	slice6AssertKeys(t, rec, 200, "credential")

	rec = catalogRequest(t, r, admin, "POST", "/api/admin/credential", `{"code":"S1","name":"技能一级","category":"skill_level","level":1}`)
	otherCred := slice6AssertKeys(t, rec, 201, credKeys...)
	rec = catalogRequest(t, r, admin, "PUT", fmt.Sprintf("/api/admin/credential/%d/sort", credID), fmt.Sprintf(`{"swap_with":%d}`, int(otherCred["id"].(float64))))
	slice6AssertNullData(t, rec)
	rec = catalogRequest(t, r, admin, "DELETE", fmt.Sprintf("/api/admin/credential/%d", credID), "")
	if rec.Code != 200 {
		t.Fatalf("删除证件期望 200，got %d: %s", rec.Code, rec.Body.String())
	}

	// 学员学习面
	studentCases := []struct {
		name string
		path string
		want []string
	}{
		{"materials", "/api/materials", []string{"materials", "page", "pages", "total"}},
		{"profile", "/api/student/profile", []string{"course_progress", "student_info", "study_stats"}},
		{"records", "/api/student/records", []string{"page", "pages", "records", "total"}},
		{"study stats", "/api/student/study-stats", []string{"active_days", "data", "days", "labels", "total_minutes"}},
		{"my courses", "/api/student/courses", []string{"continue_learning", "courses"}},
	}
	for _, tc := range studentCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := catalogRequest(t, r, stuToken, "GET", tc.path, "")
			slice6AssertKeys(t, rec, 200, tc.want...)
		})
	}

	// 单课程学习详情：不存在的课程 404（存在路径的形状由 student_courses_contract_test.go 覆盖）
	rec = catalogRequest(t, r, stuToken, "GET", "/api/student/courses/999999", "")
	if rec.Code != 404 {
		t.Fatalf("不存在的课程期望 404，got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSlice6QuestionBankEnvelopeKeys 题库域：注解从零补齐后，data 指认必须与 handler 实际渲染一致。
func TestSlice6QuestionBankEnvelopeKeys(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	admin := slice6Token(t, cfg, 1, "admin1", "admin")
	tutor := testutil.SeedTutor(t, db, "tutor1", "hash")
	tutorToken := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")

	// QuestionDTO 的键集随装配路径略异（均为生成物已表达的**可缺省键**）：
	//   - credential_id 未归属证件时整键不出现（生成物 'credential_id?: number'）；
	//   - tags 在 create/get/update 路径是 null（any 字段持 nil 切片），publish/reject 路径不装配 → 键不出现
	//     （生成物 'tags?: unknown'）。
	questionKeys := []string{
		"answer", "content", "created_at", "created_by", "created_by_type", "explanation", "id",
		"image_url", "options", "reference_answer", "reject_reason", "score", "scoring_criteria",
		"status", "tags", "type", "updated_at",
	}
	questionKeysWithoutTags := []string{
		"answer", "content", "created_at", "created_by", "created_by_type", "explanation", "id",
		"image_url", "options", "reference_answer", "reject_reason", "score", "scoring_criteria",
		"status", "type", "updated_at",
	}

	rec := catalogRequest(t, r, tutorToken, "GET", "/api/question-bank/questions", "")
	slice6AssertKeys(t, rec, 200, "page", "page_size", "questions", "total")

	rec = catalogRequest(t, r, tutorToken, "GET", "/api/question-bank/stats", "")
	slice6AssertKeys(t, rec, 200, "by_status", "by_type", "total")

	rec = catalogRequest(t, r, tutorToken, "POST", "/api/question-bank/questions",
		`{"type":"single_choice","content":"题干","options":{"A":"选项A"},"answer":"A"}`)
	m := slice6AssertKeys(t, rec, 201, questionKeys...)
	questionID := int(m["id"].(float64))

	rec = catalogRequest(t, r, tutorToken, "GET", fmt.Sprintf("/api/question-bank/questions/%d", questionID), "")
	slice6AssertKeys(t, rec, 200, questionKeys...)

	rec = catalogRequest(t, r, tutorToken, "PUT", fmt.Sprintf("/api/question-bank/questions/%d", questionID), `{"content":"新题干"}`)
	slice6AssertKeys(t, rec, 200, questionKeys...)

	// 批量导入（QuestionImportResultDTO；第二条为坏数据，errors 非空）
	rec = catalogRequest(t, r, tutorToken, "POST", "/api/question-bank/questions/batch-import",
		`{"questions":[{"type":"true_false","content":"判断题","answer":"true"},{"type":"","content":"坏数据"}]}`)
	slice6AssertKeys(t, rec, 200, "error_count", "errors", "success_count")

	// 单题发布 / 驳回（QuestionDTO；仅 admin 有 question.review）
	rec = catalogRequest(t, r, admin, "POST", fmt.Sprintf("/api/question-bank/questions/%d/publish", questionID), "")
	slice6AssertKeys(t, rec, 200, questionKeysWithoutTags...)
	rec = catalogRequest(t, r, admin, "POST", fmt.Sprintf("/api/question-bank/questions/%d/reject", questionID), `{"reason":"题干不完整"}`)
	slice6AssertKeys(t, rec, 200, questionKeysWithoutTags...)

	// 批量发布 / 驳回（计数字典）
	rec = catalogRequest(t, r, admin, "POST", "/api/question-bank/questions/batch-publish", fmt.Sprintf(`{"question_ids":[%d]}`, questionID))
	slice6AssertKeys(t, rec, 200, "published_count")
	rec = catalogRequest(t, r, admin, "POST", "/api/question-bank/questions/batch-reject", fmt.Sprintf(`{"question_ids":[%d],"reason":"x"}`, questionID))
	slice6AssertKeys(t, rec, 200, "rejected_count")

	// 删除：NoData
	rec = catalogRequest(t, r, tutorToken, "DELETE", fmt.Sprintf("/api/question-bank/questions/%d", questionID), "")
	slice6AssertNullData(t, rec)
}

// TestSlice6TutorEnvelopeKeys 讲师端：课程 / 章节 / 文件删除的 data 指认。
func TestSlice6TutorEnvelopeKeys(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	tutor := testutil.SeedTutor(t, db, "tutor1", "hash")
	token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")

	course := testutil.SeedCourse(t, db, "课程A")
	chapter := &model.Chapter{CourseID: course.CourseID, Title: "章节1", Content: "内容", ContentType: "text", CreatedAt: testutil.Now()}
	if err := db.Create(chapter).Error; err != nil {
		t.Fatalf("插入章节失败: %v", err)
	}
	chapterFile := &model.ChapterFile{ChapterID: &chapter.ChapterID, FileURL: "/uploads/a.pdf", FileName: "a.pdf", ContentType: "document", CreatedAt: testutil.Now()}
	if err := db.Create(chapterFile).Error; err != nil {
		t.Fatalf("插入章节文件失败: %v", err)
	}

	rec := catalogRequest(t, r, token, "GET", "/api/tutor/courses", "")
	slice6AssertKeys(t, rec, 200, "courses", "page", "pages", "total")

	rec = catalogRequest(t, r, token, "GET", fmt.Sprintf("/api/tutor/course/%d/chapters", course.CourseID), "")
	slice6AssertKeys(t, rec, 200, "chapters", "course")

	// 讲师端章节详情：study_status 是非指针 omitempty 字段且讲师路径不赋值，键不出现
	// （非指针 omitempty 不在本片可空性口径内 —— 见 evidence 的残留缺口一节）。
	chapterKeys := []string{
		"chapter_id", "content", "content_type", "course_id", "created_at", "description", "duration",
		"file_url", "files", "next_chapter_id", "order_num", "previous_chapter_id", "title",
	}
	rec = catalogRequest(t, r, token, "GET", fmt.Sprintf("/api/tutor/chapter/%d", chapter.ChapterID), "")
	slice6AssertKeys(t, rec, 200, chapterKeys...)

	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/tutor/chapter/%d", chapter.ChapterID), `{"title":"章节一"}`)
	slice6AssertKeys(t, rec, 200, "chapter_id", "content", "content_type", "course_id", "created_at", "description", "duration", "file_url", "order_num", "title")

	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/tutor/file/%d", chapterFile.FileID), "")
	slice6AssertKeys(t, rec, 200, "deleted", "file_id")

	rec = catalogRequest(t, r, token, "POST", "/api/tutor/files/batch-delete", `{"file_ids":[999999]}`)
	m := slice6AssertKeys(t, rec, 200, "failed_count", "failed_ids", "success_count")
	if ids, ok := m["failed_ids"].([]any); !ok || len(ids) != 1 {
		t.Fatalf("failed_ids 期望 1 个元素，got %v", m["failed_ids"])
	}
}

// TestSlice6SearchAndRealExamEnvelopeKeys 搜索（联合形状注解取聚合）与真题卷列表（数组 data）。
func TestSlice6SearchAndRealExamEnvelopeKeys(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	student := testutil.SeedStudent(t, db, "stu2", "hash")
	token := slice6Token(t, cfg, student.ID, student.Account, "hrwai_user")

	rec := catalogRequest(t, r, "", "GET", "/api/search?keyword=%E5%8F%89%E8%BD%A6", "")
	slice6AssertKeys(t, rec, 200, "contents", "courses", "keyword", "questions", "topics")

	rec = catalogRequest(t, r, "", "GET", "/api/search?keyword=%E5%8F%89%E8%BD%A6&type=course", "")
	slice6AssertKeys(t, rec, 200, "items", "keyword", "page", "pages", "total", "type")

	rec = catalogRequest(t, r, token, "GET", "/api/real-exam/papers", "")
	_, _, raw := unpackData(t, rec, 200)
	if raw != "[]" {
		t.Fatalf("套卷列表期望空数组 data，got %s", raw)
	}
}
