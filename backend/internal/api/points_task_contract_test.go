// ADR-0028 契约测试：任务中心行为判定收紧与资料任务进度真实化。
//
// 覆盖不变式（与 spec 对应）：
//   - growth_reply 只统计回复「他人主题」（自问自答不可计分，排除自帖回复）；
//   - 未达成行为时 Claim 返回 400（ErrTaskNotDone 哨兵），todo 任务不可空领；
//   - newbie_profile_* 返回 total=2 且 progress 为已满足子项数（进度条真实化）；
//   - daily_login 需登录落表（MarkDailyLogin）后才能领取。
//
// Main seam：HTTP contract（router -> httptest -> 断言状态码 + 任务形状）。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// tasksTaskResp 任务列表 data.tasks[] 的形状切片。
type tasksTaskSlice struct {
	Code int `json:"code"`
	Data struct {
		Tasks []struct {
			Code     string `json:"code"`
			Status   string `json:"status"`
			Progress int    `json:"progress"`
			Total    int    `json:"total"`
		} `json:"tasks"`
	} `json:"data"`
}

func TestPointsTaskBehaviorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	seedPointsTaskConfigs(t, db)

	now := testutil.Now()
	// 昵称默认 = 登录账号（视为「未完善昵称」；昵称 ≠ 账号才算设置过）
	student := model.HrwaiUser{Account: "task_stu", Phone: "13800001001", Username: "task_stu", Status: 1, CreatedAt: now}
	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}
	other := model.HrwaiUser{Account: "task_other", Phone: "13800001002", Username: "他人", Status: 1, CreatedAt: now}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("创建他人失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "points-task-contract-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
	RegisterPointsRoutes(apiGroup, deps.RouterDeps(), deps.PointsSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	fetchTasks := func() tasksTaskSlice {
		rec := doWithToken(t, r, token, http.MethodGet, "/api/points/tasks", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /points/tasks 应 200, got %d", rec.Code)
		}
		var out tasksTaskSlice
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("解析任务失败: %v", err)
		}
		return out
	}
	statusOf := func(code string) (status string, progress, total int) {
		for _, tk := range fetchTasks().Data.Tasks {
			if tk.Code == code {
				return tk.Status, tk.Progress, tk.Total
			}
		}
		t.Fatalf("任务 %s 不存在", code)
		return "", 0, 0
	}

	// ===== 1. growth_reply 只统计他人主题 =====
	// 学员自己的主题 + 自回 3 条
	ownTopic := model.ForumTopic{UserID: int(student.ID), Title: "自问自答", Content: "求助", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&ownTopic).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := db.Create(&model.ForumReply{TopicID: ownTopic.ID, UserID: int(student.ID), Content: "自答", CreatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if st, pr, tt := statusOf("growth_reply"); st != "todo" || pr != 0 || tt != 3 {
		t.Fatalf("自帖回复不应计入 growth_reply, got status=%s progress=%d total=%d", st, pr, tt)
	}
	// 对他人主题回复 2 条仍未达成，3 条达成
	otherTopic := model.ForumTopic{UserID: int(other.ID), Title: "他人提问", Content: "请教", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&otherTopic).Error; err != nil {
		t.Fatal(err)
	}
	_ = db.Create(&model.ForumReply{TopicID: otherTopic.ID, UserID: int(student.ID), Content: "回1", CreatedAt: now})
	_ = db.Create(&model.ForumReply{TopicID: otherTopic.ID, UserID: int(student.ID), Content: "回2", CreatedAt: now})
	if st, pr, _ := statusOf("growth_reply"); st != "todo" || pr != 2 {
		t.Fatalf("回复他人 2 条应 todo/2, got status=%s progress=%d", st, pr)
	}
	_ = db.Create(&model.ForumReply{TopicID: otherTopic.ID, UserID: int(student.ID), Content: "回3", CreatedAt: now})
	if st, pr, _ := statusOf("growth_reply"); st != "claimable" || pr != 3 {
		t.Fatalf("回复他人 3 条应 claimable/3, got status=%s progress=%d", st, pr)
	}

	// ===== 2. 未达成行为空领 → 400（哨兵 ErrTaskNotDone 文案）=====
	// 清掉他人回复（回滚到未达成）
	if err := db.Where("user_id = ? AND topic_id = ?", student.ID, otherTopic.ID).Delete(&model.ForumReply{}).Error; err != nil {
		t.Fatal(err)
	}
	rec := doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/growth_reply/claim", nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "任务未完成") {
		t.Fatalf("未达成空领应 400「任务未完成」, got %d %s", rec.Code, rec.Body.String())
	}

	// ===== 3. newbie_profile 进度真实化（total=2 + progress=子项数）=====
	if st, pr, tt := statusOf("newbie_profile_basic"); st != "todo" || pr != 0 || tt != 2 {
		t.Fatalf("空资料 basic 应 todo/0/2, got %s/%d/%d", st, pr, tt)
	}
	// 仅头像
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).Update("avatar_url", "https://x/a.png").Error; err != nil {
		t.Fatal(err)
	}
	if st, pr, tt := statusOf("newbie_profile_basic"); st != "todo" || pr != 1 || tt != 2 {
		t.Fatalf("仅头像 basic 应 todo/1/2, got %s/%d/%d", st, pr, tt)
	}
	// 头像 + 昵称（≠ account）→ 达成
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).Update("username", "真昵称").Error; err != nil {
		t.Fatal(err)
	}
	if st, pr, tt := statusOf("newbie_profile_basic"); st != "claimable" || pr != 2 || tt != 2 {
		t.Fatalf("资料齐 basic 应 claimable/2/2, got %s/%d/%d", st, pr, tt)
	}

	// ===== 4. daily_login 登录落表后 claimable；未落表空领 400 =====
	if st, _, _ := statusOf("daily_login"); st != "todo" {
		t.Fatalf("未登录 daily_login 应 todo, got %s", st)
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/daily_login/claim", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未登录空领 daily_login 应 400, got %d", rec.Code)
	}
	deps.PointsSvc.MarkDailyLogin(student.ID)
	if st, pr, tt := statusOf("daily_login"); st != "claimable" || pr != 1 || tt != 1 {
		t.Fatalf("登录后 daily_login 应 claimable/1/1, got %s/%d/%d", st, pr, tt)
	}
	rec = doWithToken(t, r, token, http.MethodPost, "/api/points/tasks/daily_login/claim", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("登录后领取 daily_login 应 200, got %d %s", rec.Code, rec.Body.String())
	}
}
