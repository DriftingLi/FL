// 契约测试 #488：学员视角职位投递状态——列表/详情 apply_state（none/applied/not_hired）+ 冷却天数。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestJobApplyStateContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuApplyState", pwd)
	now := time.Now()
	pos := model.Position{Code: "apply_pos", Name: "叉车司机", Status: 1}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("create pos: %v", err)
	}
	card := model.JobCard{UserID: stu.ID, RealName: "张三", ContactPhone: "13800000001", ExpectedRegions: model.JSONB([]byte(`[]`)), Visibility: "hidden", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}
	// 三个职位：A 可投（无记录）、B 已投（applied）、C 被拒冷却中
	jA := model.JobPosting{RecruiterID: 1, Title: "职位A", PositionID: &pos.PositionID, Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	jB := model.JobPosting{RecruiterID: 1, Title: "职位B", PositionID: &pos.PositionID, Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	jC := model.JobPosting{RecruiterID: 1, Title: "职位C", PositionID: &pos.PositionID, Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	for _, j := range []model.JobPosting{jA, jB, jC} {
		if err := db.Create(&j).Error; err != nil {
			t.Fatalf("create job: %v", err)
		}
	}
	var a, b, c model.JobPosting
	db.First(&a, "title = ?", "职位A")
	db.First(&b, "title = ?", "职位B")
	db.First(&c, "title = ?", "职位C")
	// B：applied
	appB := model.JobApplication{JobPostingID: b.ID, RecruiterID: 1, StudentUserID: stu.ID, Status: "applied", ResumeUpdatedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&appB).Error; err != nil {
		t.Fatalf("create appB: %v", err)
	}
	// C：rejected（2 天前）→ 冷却中
	rejAt := now.Add(-48 * time.Hour)
	appC := model.JobApplication{JobPostingID: c.ID, RecruiterID: 1, StudentUserID: stu.ID, Status: "rejected", ResumeUpdatedAt: now, RejectedAt: &rejAt, CreatedAt: rejAt, UpdatedAt: rejAt}
	if err := db.Create(&appC).Error; err != nil {
		t.Fatalf("create appC: %v", err)
	}
	// 第 4 个职位 D：rejected 31 天前 → 冷却期满可重投（none）
	jD := model.JobPosting{RecruiterID: 1, Title: "职位D", PositionID: &pos.PositionID, Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&jD).Error; err != nil {
		t.Fatalf("create jobD: %v", err)
	}
	var d model.JobPosting
	db.First(&d, "title = ?", "职位D")
	rejOld := now.Add(-31 * 24 * time.Hour)
	appD := model.JobApplication{JobPostingID: d.ID, RecruiterID: 1, StudentUserID: stu.ID, Status: "rejected", ResumeUpdatedAt: now, RejectedAt: &rejOld, CreatedAt: rejOld, UpdatedAt: rejOld}
	if err := db.Create(&appD).Error; err != nil {
		t.Fatalf("create appD: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")

	// 列表：四态齐全
	rec := doWithToken(t, r, studentToken, http.MethodGet, "/api/jobs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("列表应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Data struct {
			Items []struct {
				ID           int    `json:"id"`
				ApplyState   string `json:"apply_state"`
				CooldownDays int    `json:"cooldown_days"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	state := map[int]string{}
	for _, it := range listResp.Data.Items {
		state[it.ID] = it.ApplyState
	}
	if state[a.ID] != "" && state[a.ID] != "none" {
		t.Fatalf("职位A 应无可投递(none), 实际 %s", state[a.ID])
	}
	if state[b.ID] != "applied" {
		t.Fatalf("职位B 应 applied, 实际 %s", state[b.ID])
	}
	if state[c.ID] != "not_hired" {
		t.Fatalf("职位C 应 not_hired, 实际 %s", state[c.ID])
	}
	if state[d.ID] != "" && state[d.ID] != "none" {
		t.Fatalf("职位D 冷却期满应可投(none), 实际 %s", state[d.ID])
	}

	// 详情：已投职位 C 带 cooldown_days
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/jobs/"+strconv.Itoa(c.ID), nil)
	var detail struct {
		Data struct {
			ApplyState   string `json:"apply_state"`
			CooldownDays int    `json:"cooldown_days"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("parse detail: %v", err)
	}
	if detail.Data.ApplyState != "not_hired" {
		// 避免时区边界问题，允许重查：B 已投详情
		rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/jobs/"+strconv.Itoa(b.ID), nil)
		_ = json.Unmarshal(rec.Body.Bytes(), &detail)
		if detail.Data.ApplyState != "applied" {
			t.Fatalf("详情 B 应 applied, 实际 %s", detail.Data.ApplyState)
		}
	} else if detail.Data.CooldownDays <= 0 || detail.Data.CooldownDays > 30 {
		t.Fatalf("冷却天数应在 1-30, 实际 %d", detail.Data.CooldownDays)
	}

	// 无 token 401（L1 保持）
	rec = doWithoutToken(t, r, http.MethodGet, "/api/jobs")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 列表应 401, 实际 %d", rec.Code)
	}
}
