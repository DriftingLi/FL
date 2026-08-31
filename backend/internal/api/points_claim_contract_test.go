// Contract test #409/#410: claim-state determination sunk to SQL + quota config-driven.
// Main seam: HTTP contract layer (router -> httptest -> unwrap envelope assert).
// Three core assertions run on BOTH adapters:
//   - after claim, task list shows claimed;
//   - behavior-done + already-claimed does NOT fall back to claimable;
//   - total_limit=1 newbie task cannot be claimed a second time.
// Postgres adapter is built by the real SQL migrations (NOT model AutoMigrate) and is skipped
// when DATABASE_URL is unset; SQLite adapter is always green (regression lock).
// Also asserts: duplicate claim -> single 400, no second claim row; points_user_progress is
// no longer written (write-only dead state closed, #410).
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/clause"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

type pointsTasksResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Tasks []struct {
			Code     string `json:"code"`
			Status   string `json:"status"`
			Progress int    `json:"progress"`
			Total    int    `json:"total"`
		} `json:"tasks"`
	} `json:"data"`
}

type pointsClaimResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Balance     int    `json:"balance"`
		TotalEarned int    `json:"total_earned"`
		TaskStatus  string `json:"task_status"`
	} `json:"data"`
}

func fetchPointsTasks(t *testing.T, r *gin.Engine, token string) pointsTasksResp {
	t.Helper()
	rec := doWithToken(t, r, token, http.MethodGet, "/api/points/tasks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /points/tasks should be 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp pointsTasksResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse tasks failed: %v", err)
	}
	return resp
}

func taskStatusOf(resp pointsTasksResp, code string) (string, int, int) {
	for _, tk := range resp.Data.Tasks {
		if tk.Code == code {
			return tk.Status, tk.Progress, tk.Total
		}
	}
	return "", 0, 0
}

func seedPointsTaskConfigs(t *testing.T, db *gorm.DB) {
	t.Helper()
	configs := []model.PointsTaskConfig{
		{Code: "daily_checkin", Title: "每日打卡", Group: "daily", Points: 5, DailyLimit: 1, EventType: "check_in", Description: "每日打卡成功"},
		{Code: "daily_quiz", Title: "每日答题 1 次", Group: "daily", Points: 10, DailyLimit: 1, EventType: "question_submit", Description: "每日完成任意练习/模考 1 次"},
		{Code: "daily_browse", Title: "浏览 3 篇帖子", Group: "daily", Points: 5, DailyLimit: 1, EventType: "forum_view", Description: "每日浏览 3 篇帖子"},
		{Code: "newbie_profile_basic", Title: "完善基础资料", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "profile_complete", Description: "上传头像且设置昵称（2/2）"},
		{Code: "newbie_profile_contact", Title: "完善联系资料", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "profile_complete", Description: "填写单位且绑定手机/邮箱（2/2）"},
		{Code: "newbie_credential", Title: "选定目标证件", Group: "newbie", Points: 10, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "credential_onboarding", Description: "完成 onboarding 选定当前证件"},
		{Code: "newbie_first_course", Title: "完成首节课程", Group: "newbie", Points: 20, DailyLimit: 1, TotalLimit: intPtr(1), EventType: "course_complete", Description: "完成首节课程学习"},
		{Code: "growth_post", Title: "发布 1 篇帖子", Group: "growth", Points: 10, DailyLimit: 1, EventType: "topic_create", Description: "每日发布 1 篇帖子"},
		{Code: "growth_reply", Title: "回复 3 次", Group: "growth", Points: 10, DailyLimit: 1, EventType: "reply_create", Description: "每日回复 3 次"},
		{Code: "growth_mock", Title: "完成 1 次模考", Group: "growth", Points: 20, DailyLimit: 1, EventType: "mock_submit", Description: "每日完成 1 次模考"},
	}
	for i := range configs {
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&configs[i]).Error; err != nil {
			t.Fatalf("seed points task config %s failed: %v", configs[i].Code, err)
		}
	}
}

func intPtr(v int) *int { return &v }

func assertPointsClaimStateMachine(t *testing.T, db *gorm.DB) {
	gin.SetMode(gin.TestMode)
	seedPointsTaskConfigs(t, db)
	pwd, _ := service.HashPassword("student123")
	student := testutil.SeedStudent(t, db, "stu1", pwd)
	cfg := &config.Config{
		JWTSecretKey:          "points-claim-contract-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	token, err := sess.Issue(student.ID, student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("issue student token failed: %v", err)
	}

	// 1. no behavior yet -> todo
	resp := fetchPointsTasks(t, r, token)
	if st, _, _ := taskStatusOf(resp, "daily_checkin"); st != "todo" {
		t.Fatalf("initial daily_checkin should be todo, got %q", st)
	}
	// 2. behavior done -> claimable
	ts := time.Now().In(clock.Location())
	todayStart := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, clock.Location())
	if err := db.Create(&model.ForumCheckIn{UserID: student.ID, CheckDate: todayStart}).Error; err != nil {
		t.Fatalf("seed checkin failed: %v", err)
	}
	resp = fetchPointsTasks(t, r, token)
	if st, _, _ := taskStatusOf(resp, "daily_checkin"); st != "claimable" {
		t.Fatalf("after behavior daily_checkin should be claimable, got %q", st)
	}
	// 3. claim -> 200, task_status claimed, +5 points
	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/daily_checkin/claim", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("first claim should be 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var cr pointsClaimResp
	if err := json.Unmarshal(rec.Body.Bytes(), &cr); err != nil {
		t.Fatalf("parse claim failed: %v", err)
	}
	if cr.Data.TaskStatus != "claimed" || cr.Data.Balance != 5 {
		t.Fatalf("claim result unexpected: %+v", cr.Data)
	}
	// 4. tasks again -> still claimed (behavior done + claimed does not fall back)
	resp = fetchPointsTasks(t, r, token)
	if st, pr, tt := taskStatusOf(resp, "daily_checkin"); st != "claimed" || pr != 1 || tt != 1 {
		t.Fatalf("after claim daily_checkin should be claimed 1/1, got %q %d/%d", st, pr, tt)
	}
	// 5. duplicate claim -> single 400, no second claim row
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/daily_checkin/claim", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("duplicate claim should be 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "今日已领取") {
		t.Fatalf("duplicate daily claim message should contain 今日已领取, got %s", rec.Body.String())
	}
	var dailyRows int64
	if err := db.Model(&model.PointsTaskClaim{}).Where("user_id = ? AND task_code = ?", student.ID, "daily_checkin").Count(&dailyRows).Error; err != nil {
		t.Fatalf("count daily claims failed: %v", err)
	}
	if dailyRows != 1 {
		t.Fatalf("daily_checkin claim rows should be exactly 1, got %d", dailyRows)
	}
	// 6. points_user_progress no longer written
	var progressRows int64
	if err := db.Model(&model.PointsUserProgress{}).Where("user_id = ?", student.ID).Count(&progressRows).Error; err != nil {
		t.Fatalf("count progress rows failed: %v", err)
	}
	if progressRows != 0 {
		t.Fatalf("points_user_progress should have no rows, got %d", progressRows)
	}
	// 7. newbie_credential (total_limit=1): behavior -> claimable; claim once; lifetime exhausted
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).UpdateColumn("current_credential_id", 1).Error; err != nil {
		t.Fatalf("seed credential behavior failed: %v", err)
	}
	resp = fetchPointsTasks(t, r, token)
	if st, _, _ := taskStatusOf(resp, "newbie_credential"); st != "claimable" {
		t.Fatalf("newbie_credential should be claimable after behavior, got %q", st)
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/newbie_credential/claim", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("newbie first claim should be 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/newbie_credential/claim", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("newbie duplicate claim should be 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "已领取") {
		t.Fatalf("newbie duplicate message should contain 已领取, got %s", rec.Body.String())
	}
	var newbieRows int64
	if err := db.Model(&model.PointsTaskClaim{}).Where("user_id = ? AND task_code = ?", student.ID, "newbie_credential").Count(&newbieRows).Error; err != nil {
		t.Fatalf("count newbie claims failed: %v", err)
	}
	if newbieRows != 1 {
		t.Fatalf("newbie_credential claim rows should be exactly 1, got %d", newbieRows)
	}
	resp = fetchPointsTasks(t, r, token)
	if st, pr, tt := taskStatusOf(resp, "newbie_credential"); st != "claimed" || pr != 1 || tt != 1 {
		t.Fatalf("newbie_credential should stay claimed 1/1, got %q %d/%d", st, pr, tt)
	}
}

func TestPointsClaimContract_StateMachineOnSqlite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	assertPointsClaimStateMachine(t, db)
}

func TestPointsClaimContract_StateMachineOnPostgres(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	assertPointsClaimStateMachine(t, db)
}
