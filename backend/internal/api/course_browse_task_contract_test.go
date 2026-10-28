// 契约测试 #2065：任务中心「浏览课程」三件套 + 默认分支收口。
//
// 守的边界（三件套缺一件就是一个空口任务，票面的硬坑）：
//   - **只加配置行、没实现行为判定的任务码 ⇒ 不可领**（GetTasks 报 todo、Claim 400「任务未完成」）：
//     收口前 default 分支是「无行为前置 ⇒ 直接可领」，于是漏写 case 的任务点一下白拿分；
//   - 上报两次只记一次（recorded true → false），进度 1/3；三门不同课 → 3/3 claimable；
//   - 昨日浏览不计入今日（跨日重置：把今天的行挪到昨天，进度归零、任务退回 todo）；
//   - 未上报时 todo/0/3 不可领；领取后 +5、重复领取 400；
//   - 不可见课程（未发布 / 未挂载）上报 404 且**不落事实行**（不给任意 id 造事实的机会）。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/points"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// browseTaskRow 任务中心单行里本票关心的那几格。
type browseTaskRow struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Points   int    `json:"points"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
}

func fetchBrowseTasks(t *testing.T, r *gin.Engine, token string) map[string]browseTaskRow {
	t.Helper()
	rec := doWithToken(t, r, token, http.MethodGet, "/api/points/tasks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /points/tasks 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Tasks []browseTaskRow `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析任务列表失败: %v", err)
	}
	out := make(map[string]browseTaskRow, len(env.Data.Tasks))
	for _, tk := range env.Data.Tasks {
		out[tk.Code] = tk
	}
	return out
}

// browseOf 取某个任务行，不存在即判红（配置缺失与判定缺失是两件事，别混）。
func browseOf(t *testing.T, tasks map[string]browseTaskRow, code string) browseTaskRow {
	t.Helper()
	row, ok := tasks[code]
	if !ok {
		t.Fatalf("任务 %s 不在任务中心列表里（配置行丢了？）", code)
	}
	return row
}

func assertCourseBrowseTaskContract(t *testing.T, db *gorm.DB) {
	testutil.SetTestGinMode()
	// 生产面 10 行种子（PG 由迁移建出）＋本票新增的一行（SQLite 面补种，两侧断言同一份事实）
	seedPointsTaskConfigs(t, db)
	mustCreate(t, db, &model.PointsTaskConfig{
		Code: "browse_course", Title: "浏览 3 门课程", Group: "daily", Points: 5, DailyLimit: 1,
		EventType: "course_browse", Description: "每日浏览 3 门课程",
	})
	// 只有配置行、没有 taskProgressFor case 的任务（防「空口任务」回归）
	mustCreate(t, db, &model.PointsTaskConfig{
		Code: "future_task_without_case", Title: "未实现判定的任务", Group: "daily", Points: 99,
		DailyLimit: 1, EventType: "future", Description: "占位（测试用）",
	})

	pwd, _ := core.HashPassword("student123")
	stu := seedStudent(t, db, "stuBrowse", pwd)

	// 可见课程：已发布 + 已挂载（专业方向与等级都非空，ADR-0058）。
	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "browse-spec", Name: "浏览测试方向", SortOrder: 1, Status: 1, CreatedAt: testutil.Now()}
	mustCreate(t, db, &spec)
	lv := model.CourseLevel{Code: "browse-level", Name: "浏览测试等级", SortOrder: 1, Status: 1, CreatedAt: testutil.Now()}
	mustCreate(t, db, &lv)
	mkCourse := func(name string, status int16) *model.Course {
		// Status 带 gorm default:1 ⇒ 传零值会被 GORM 当成「没给」而落成 1（草稿课就发布出去了）。
		// 先按 1 建、再显式改回目标值，绕开这个零值陷阱。
		c := model.Course{Name: name, Status: 1, SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
		mustCreate(t, db, &c)
		if status != 1 {
			if err := db.Model(&model.Course{}).Where("course_id = ?", c.CourseID).Update("status", status).Error; err != nil {
				t.Fatalf("改课程状态失败: %v", err)
			}
		}
		return &c
	}
	c1, c2, c3, c4 := mkCourse("浏览课程一", 1), mkCourse("浏览课程二", 1), mkCourse("浏览课程三", 1), mkCourse("浏览课程四", 1)
	unpublished := mkCourse("草稿课程", 0)

	cfg := &config.Config{JWTSecretKey: "course-browse-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	course.RegisterRoutes(apiGroup, deps.RouterDeps().Session, deps.RouterDeps().CredentialScope, deps.CourseSvc)
	points.RegisterRoutes(apiGroup, deps.RouterDeps().Session, deps.PointsSvc)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(stu.ID), stu.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	report := func(courseID int) (int, bool) {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/course/"+itoa(courseID)+"/browse", nil)
		if rec.Code != http.StatusOK {
			return rec.Code, false
		}
		var env struct {
			Data struct {
				Recorded bool `json:"recorded"`
			} `json:"data"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &env)
		return rec.Code, env.Data.Recorded
	}
	claim := func(code string) int {
		return doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/"+code+"/claim", nil).Code
	}

	today := clock.DayKey(clock.Now())
	yesterday := clock.DayKey(clock.Now().AddDate(0, 0, -1))

	// ===== 1. 未上报：todo / 0/3，且不可领 =====
	row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_course")
	if row.Status != "todo" || row.Progress != 0 || row.Total != 3 {
		t.Fatalf("未上报应是 todo/0/3, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	if row.Points != 5 || row.Title != "浏览 3 门课程" {
		t.Fatalf("分值与文案应与配置一致（5 分 / 浏览 3 门课程）, got %d 分 / %q", row.Points, row.Title)
	}
	if code := claim("browse_course"); code != http.StatusBadRequest {
		t.Fatalf("未达成领取应 400, got %d", code)
	}

	// ===== 2. 昨日浏览不算今日（跨日口径的事实侧）=====
	mustCreate(t, db, &model.CourseView{UserID: int(stu.ID), ViewDate: yesterday, CourseID: c1.CourseID, CreatedAt: testutil.Now()})
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_course"); row.Progress != 0 {
		t.Fatalf("昨天浏览过的课程不该计入今天, got progress=%d", row.Progress)
	}

	// ===== 3. 上报两次只记一次 =====
	code, recorded := report(c1.CourseID)
	if code != http.StatusOK || !recorded {
		t.Fatalf("首次上报应 200/recorded=true, got %d/%v", code, recorded)
	}
	code, recorded = report(c1.CourseID)
	if code != http.StatusOK || recorded {
		t.Fatalf("同日重复上报应 200/recorded=false（幂等，不是错误）, got %d/%v", code, recorded)
	}
	var viewRows int64
	db.Model(&model.CourseView{}).Where("user_id = ? AND view_date = ?", stu.ID, today).Count(&viewRows)
	if viewRows != 1 {
		t.Fatalf("同一天同一门课只该有一行, got %d", viewRows)
	}
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_course"); row.Progress != 1 || row.Status != "todo" {
		t.Fatalf("浏览 1 门应是 todo/1/3, got %s/%d", row.Status, row.Progress)
	}

	// ===== 4. 三门不同课 → claimable；第四门不把进度顶过 3 =====
	_, _ = report(c2.CourseID)
	_, _ = report(c3.CourseID)
	row = browseOf(t, fetchBrowseTasks(t, r, token), "browse_course")
	if row.Status != "claimable" || row.Progress != 3 {
		t.Fatalf("浏览 3 门应 claimable/3/3, got %s/%d/%d", row.Status, row.Progress, row.Total)
	}
	_, _ = report(c4.CourseID)
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_course"); row.Progress != 3 {
		t.Fatalf("进度封顶 3/3（与浏览帖子同口径）, got %d", row.Progress)
	}

	// ===== 5. 跨日重置：把今天的行挪到昨天 ⇒ 今日一条都不算 =====
	if err := db.Where("user_id = ? AND view_date = ? AND course_id = ?", stu.ID, yesterday, c1.CourseID).
		Delete(&model.CourseView{}).Error; err != nil {
		t.Fatalf("清昨日探针行失败: %v", err)
	}
	if err := db.Model(&model.CourseView{}).Where("user_id = ? AND view_date = ?", stu.ID, today).
		Update("view_date", yesterday).Error; err != nil {
		t.Fatalf("把今日行挪到昨天失败: %v", err)
	}
	if row := browseOf(t, fetchBrowseTasks(t, r, token), "browse_course"); row.Progress != 0 || row.Status != "todo" {
		t.Fatalf("跨日应重置为 todo/0/3, got %s/%d", row.Status, row.Progress)
	}
	if code := claim("browse_course"); code != http.StatusBadRequest {
		t.Fatalf("跨日后未重新达成应 400, got %d", code)
	}

	// ===== 6. 重新达成 → 领取 +5；重复领取 400 =====
	_, _ = report(c1.CourseID)
	_, _ = report(c2.CourseID)
	_, _ = report(c3.CourseID)
	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/browse_course/claim", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("达成后领取应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var claimEnv struct {
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &claimEnv)
	if claimEnv.Data.Balance != 5 {
		t.Fatalf("领取后余额应为 5, got %d", claimEnv.Data.Balance)
	}
	if code := claim("browse_course"); code != http.StatusBadRequest {
		t.Fatalf("重复领取应 400, got %d", code)
	}

	// ===== 7. 不可见课程：404 且不落事实行 =====
	if code, _ := report(unpublished.CourseID); code != http.StatusNotFound {
		t.Fatalf("未发布课程上报应 404, got %d", code)
	}
	db.Model(&model.CourseView{}).Where("user_id = ? AND course_id = ?", stu.ID, unpublished.CourseID).Count(&viewRows)
	if viewRows != 0 {
		t.Fatalf("不可见课程不该落浏览行, got %d 行", viewRows)
	}
	if code, _ := report(999999); code != http.StatusNotFound {
		t.Fatalf("不存在课程上报应 404, got %d", code)
	}

	// ===== 8. 默认分支收口：只有配置行、没有 case ⇒ 不可领 =====
	row = browseOf(t, fetchBrowseTasks(t, r, token), "future_task_without_case")
	if row.Status != "todo" {
		t.Fatalf("未实现判定的任务应报 todo（防「配置行即空口任务」）, got %s", row.Status)
	}
	if code := claim("future_task_without_case"); code != http.StatusBadRequest {
		t.Fatalf("未实现判定的任务领取应 400, got %d", code)
	}
}

// mustCreate 建一行，失败即判红（夹具建不起来时不该继续断言）。
func mustCreate(t *testing.T, db *gorm.DB, v any) {
	t.Helper()
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(v).Error; err != nil {
		t.Fatalf("夹具建行失败（%T）: %v", v, err)
	}
}

func TestCourseBrowseTaskContract_OnSqlite(t *testing.T) {
	assertCourseBrowseTaskContract(t, testutil.NewMemoryDB(t))
}

func TestCourseBrowseTaskContract_OnPostgres(t *testing.T) {
	assertCourseBrowseTaskContract(t, testutil.NewPostgresDB(t))
}
