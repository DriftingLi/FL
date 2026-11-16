// 本文件：管理端课程代分配的 HTTP 契约（#2597 / 规格 B1、B3、D4、B6）。
//
// 四条 AC 逐条落成一个断言组，且都在**真实路由链**上打（NewRouter，不是自装蓝图）：
//   - 代分配后讲师即可写（认领谓词真的被这条 INSERT 满足）；
//   - 移除后讲师不可写（谓词真的断了，而不是只删了行）；
//   - 重复分配幂等（第二次一行都不写）；
//   - 非管理员 403（tutor.manage 不是讲师能拿到的键）。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// assignFixture 代分配契约的夹具：一位讲师、一门课、一个章节、一位管理员。
type assignFixture struct {
	t        *testing.T
	r        http.Handler
	db       *gorm.DB
	tutorID  int
	courseID int
	chapter  model.Chapter
	tutorTok string
	adminTok string
}

// newAssignFixture 造夹具并装真实路由（NewRouter：管理员能力来自数据层，#1618 段1）。
func newAssignFixture(t *testing.T) *assignFixture {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "assign-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := NewRouter(newContractDeps(t, db, cfg))

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "assign-spec", Name: "代分配", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "assign-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "代分配课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "代分配章", Content: "正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	tutor := testutil.SeedTutor(t, db, "assign_tutor", "x")
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	tutorTok, err := sess.Issue(tutor.TutorID, tutor.Username, "tutor")
	if err != nil {
		t.Fatalf("签讲师 token 失败: %v", err)
	}
	adminTok := adminTokenWithAccount(t, cfg, db, "assign_admin")

	return &assignFixture{
		t: t, r: r, db: db,
		tutorID: tutor.TutorID, courseID: c.CourseID, chapter: ch,
		tutorTok: tutorTok, adminTok: adminTok,
	}
}

// do 发一条请求，返回状态码与响应体。
func (f *assignFixture) do(method, path, token, body string) (int, string) {
	f.t.Helper()
	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	f.r.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// assignCourses 管理员代分配（返回状态码与解出的结果）。
func (f *assignFixture) assignCourses(courseIDs ...int) (int, map[string]any) {
	f.t.Helper()
	body, _ := json.Marshal(map[string]any{"course_ids": courseIDs})
	code, raw := f.do(http.MethodPost, fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID), f.adminTok, string(body))
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw), &env)
	return code, env.Data
}

// removeCourses 管理员移除。
func (f *assignFixture) removeCourses(courseIDs ...int) (int, map[string]any) {
	f.t.Helper()
	body, _ := json.Marshal(map[string]any{"course_ids": courseIDs})
	code, raw := f.do(http.MethodDelete, fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID), f.adminTok, string(body))
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal([]byte(raw), &env)
	return code, env.Data
}

// writeChapter 讲师改自己的章节（认领谓词守在这条路径上）。
func (f *assignFixture) writeChapter() int {
	f.t.Helper()
	code, _ := f.do(http.MethodPut, fmt.Sprintf("/api/tutor/chapter/%d", f.chapter.ChapterID),
		f.tutorTok, `{"title":"改过的标题"}`)
	return code
}

// TestAdminAssignCourseGrantsTutorWrite D4 前半：代分配后讲师即可写。
func TestAdminAssignCourseGrantsTutorWrite(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)

	// 分配前：未认领 → 403
	if code := f.writeChapter(); code != http.StatusForbidden {
		t.Fatalf("未分配时应 403, got %d", code)
	}

	code, data := f.assignCourses(f.courseID)
	if code != http.StatusOK {
		t.Fatalf("代分配应 200, got %d", code)
	}
	if data["applied"] != float64(1) {
		t.Fatalf("首次分配 applied 应为 1, got %v", data["applied"])
	}

	// 分配后：放行
	if code := f.writeChapter(); code != http.StatusOK {
		t.Fatalf("代分配后讲师应可写, got %d", code)
	}
}

// TestAdminRemoveCourseRevokesTutorWrite D4 后半 + B6：移除后讲师不可写，且**不回溯**。
func TestAdminRemoveCourseRevokesTutorWrite(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)

	if code, _ := f.assignCourses(f.courseID); code != http.StatusOK {
		t.Fatalf("代分配应 200, got %d", code)
	}
	if code := f.writeChapter(); code != http.StatusOK {
		t.Fatalf("分配后应可写, got %d", code)
	}

	code, data := f.removeCourses(f.courseID)
	if code != http.StatusOK {
		t.Fatalf("移除应 200, got %d", code)
	}
	if data["applied"] != float64(1) {
		t.Fatalf("移除 applied 应为 1, got %v", data["applied"])
	}

	// 移除后：写被拒（谓词真的断了）
	if code := f.writeChapter(); code != http.StatusForbidden {
		t.Fatalf("移除后应 403, got %d", code)
	}
	// 不回溯：章节作者归属照旧（移除只断今后的编辑权）
	var ch model.Chapter
	if err := f.db.First(&ch, f.chapter.ChapterID).Error; err != nil {
		t.Fatalf("回读章节失败: %v", err)
	}
	if ch.Title != "改过的标题" {
		t.Fatalf("移除认领不得回溯已写内容, got title=%q", ch.Title)
	}
}

// TestAdminAssignCourseIdempotent 重复分配幂等：第二次一行都不写，计入 skipped。
func TestAdminAssignCourseIdempotent(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)

	if code, _ := f.assignCourses(f.courseID); code != http.StatusOK {
		t.Fatalf("首次分配应 200, got %d", code)
	}
	code, data := f.assignCourses(f.courseID)
	if code != http.StatusOK {
		t.Fatalf("重复分配应 200（幂等，不是 409）, got %d", code)
	}
	if data["applied"] != float64(0) || data["skipped"] != float64(1) {
		t.Fatalf("重复分配应 applied=0 skipped=1, got %v", data)
	}

	// 认领行只有一条（不是两次写入）
	var n int64
	if err := f.db.Model(&model.TutorCourseClaim{}).
		Where("tutor_id = ? AND course_id = ?", f.tutorID, f.courseID).Count(&n).Error; err != nil {
		t.Fatalf("查认领行失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("认领行应恰好 1 条, got %d", n)
	}
}

// TestAdminAssignCourseUpgradesSelfClaim B1：讲师自助认领过的课，管理员代分配把来源升级为 admin。
//
// 这条是票内评论拍板的语义：管理员想把「讲师自己认领的」变成「分配给他的」，
// 两者在审计上不同，所以不能返回 409（那会让这个转换永远做不到）。
func TestAdminAssignCourseUpgradesSelfClaim(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)

	// 讲师先自助认领
	if code, _ := f.do(http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", f.courseID), f.tutorTok, ""); code != http.StatusOK {
		t.Fatalf("讲师自助认领应 200, got %d", code)
	}

	code, data := f.assignCourses(f.courseID)
	if code != http.StatusOK {
		t.Fatalf("管理员对已自助认领的课代分配应 200, got %d", code)
	}
	if data["applied"] != float64(1) {
		t.Fatalf("来源升级应计入 applied=1, got %v", data["applied"])
	}

	// 来源真的变成 admin
	var claim model.TutorCourseClaim
	if err := f.db.Where("tutor_id = ? AND course_id = ?", f.tutorID, f.courseID).
		First(&claim).Error; err != nil {
		t.Fatalf("查认领行失败: %v", err)
	}
	if claim.Source != model.ClaimSourceAdmin {
		t.Fatalf("来源应升级为 admin, got %q", claim.Source)
	}
}

// TestAdminAssignCourseRequiresTutorManage 非管理员 403（AC 最后一条）。
//
// 两个方向都要测：讲师令牌（有 tutor.access 但没有 tutor.manage）与无令牌（401）。
// 只测 403 而不测 401 会漏掉「守卫顺序反了」——先判能力再判认证会让匿名请求拿到 403。
func TestAdminAssignCourseRequiresTutorManage(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)
	body, _ := json.Marshal(map[string]any{"course_ids": []int{f.courseID}})

	// 讲师：403
	code, _ := f.do(http.MethodPost, fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID), f.tutorTok, string(body))
	if code != http.StatusForbidden {
		t.Fatalf("讲师访问代分配应 403, got %d", code)
	}
	// 匿名：401
	code, _ = f.do(http.MethodPost, fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID), "", string(body))
	if code != http.StatusUnauthorized {
		t.Fatalf("匿名访问代分配应 401, got %d", code)
	}
	// 移除端点同权（别只守一个）
	code, _ = f.do(http.MethodDelete, fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID), f.tutorTok, string(body))
	if code != http.StatusForbidden {
		t.Fatalf("讲师访问移除应 403, got %d", code)
	}
}

// TestAdminAssignCourseEmptyListRejected 空课程列表 400（不是「成功 0 条」）。
func TestAdminAssignCourseEmptyListRejected(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)
	code, _ := f.assignCourses()
	if code != http.StatusBadRequest {
		t.Fatalf("空课程列表应 400, got %d", code)
	}
}

// TestAdminAssignCourseUnknownTutor404 讲师不存在 404（不能给不存在的讲师建认领行）。
func TestAdminAssignCourseUnknownTutor404(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)
	body, _ := json.Marshal(map[string]any{"course_ids": []int{f.courseID}})
	code, _ := f.do(http.MethodPost, "/api/admin/tutor/999999/courses", f.adminTok, string(body))
	if code != http.StatusNotFound {
		t.Fatalf("讲师不存在应 404, got %d", code)
	}
}

// TestAdminListTutorCourses 抽屉的「已分配」数据源：列出该讲师的课程，带来源。
func TestAdminListTutorCourses(t *testing.T) {
	t.Parallel()
	f := newAssignFixture(t)
	path := fmt.Sprintf("/api/admin/tutor/%d/courses", f.tutorID)

	// 未分配：空列表（200，不是 404——讲师存在但没课）
	code, raw := f.do(http.MethodGet, path, f.adminTok, "")
	if code != http.StatusOK {
		t.Fatalf("查已分配应 200, got %d", code)
	}
	var empty struct {
		Data struct {
			CourseIDs []int `json:"course_ids"`
			Items     []any `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &empty); err != nil {
		t.Fatalf("解信封失败: %v", err)
	}
	if len(empty.Data.CourseIDs) != 0 || len(empty.Data.Items) != 0 {
		t.Fatalf("未分配时应为空列表, got %+v", empty.Data)
	}

	// 分配后：出现在列表里，来源是 admin
	if code, _ := f.assignCourses(f.courseID); code != http.StatusOK {
		t.Fatalf("代分配应 200, got %d", code)
	}
	code, raw = f.do(http.MethodGet, path, f.adminTok, "")
	if code != http.StatusOK {
		t.Fatalf("查已分配应 200, got %d", code)
	}
	var got struct {
		Data struct {
			CourseIDs []int `json:"course_ids"`
			Items     []struct {
				CourseID   int    `json:"course_id"`
				CourseName string `json:"course_name"`
				Source     string `json:"source"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("解信封失败: %v", err)
	}
	if len(got.Data.CourseIDs) != 1 || got.Data.CourseIDs[0] != f.courseID {
		t.Fatalf("course_ids 应含 %d, got %v", f.courseID, got.Data.CourseIDs)
	}
	if len(got.Data.Items) != 1 || got.Data.Items[0].Source != model.ClaimSourceAdmin {
		t.Fatalf("items 应带 admin 来源, got %+v", got.Data.Items)
	}

	// 讲师访问同端点 403（与分配同权）
	if code, _ := f.do(http.MethodGet, path, f.tutorTok, ""); code != http.StatusForbidden {
		t.Fatalf("讲师查他人已分配应 403, got %d", code)
	}
}
