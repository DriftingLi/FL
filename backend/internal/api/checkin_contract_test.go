// ADR-0028 契约测试：打卡独立蓝图 /api/check-in/*（旧 /api/forum/check-in/* 已删除）。
//
// 覆盖不变式（对应 spec Testing Decisions 清单）：
//   - 首签直记 +5（balance/流水可查）；
//   - 连续第 3/7 天当日合并发阶梯（5+5=10 等，跨档恰触发）；
//   - 同日重复打卡不再发分（幂等，流水只一笔）；
//   - 日历按整月逐日返回 {date, checked, points}；
//   - 旧路由 /api/forum/check-in/* 返回 404。
//
// Main seam：HTTP contract（真实 router + 装配根 -> httptest）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/clock"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// checkInResp POST /api/check-in 响应。
type checkInResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Checked      bool `json:"checked"`
		Streak       int  `json:"streak"`
		Total        int  `json:"total"`
		TodayChecked bool `json:"today_checked"`
		Points       int  `json:"points"`
	} `json:"data"`
}

// calendarResp GET /api/check-in/calendar 响应。
type calendarResp struct {
	Code int `json:"code"`
	Data struct {
		Days []struct {
			Date    string `json:"date"`
			Checked bool   `json:"checked"`
			Points  int    `json:"points"`
		} `json:"days"`
		Streak       int  `json:"streak"`
		Total        int  `json:"total"`
		TodayChecked bool `json:"today_checked"`
	} `json:"data"`
}

func TestCheckInContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	stu := model.HrwaiUser{Account: "checkin_stu", Phone: "13800001001", Username: "打卡学员", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&stu).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "checkin-contract-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(stu.ID), stu.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	doCheckIn := func() checkInResp {
		rec := doWithToken(t, r, token, http.MethodPost, "/api/check-in", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /api/check-in 应 200, got %d %s", rec.Code, rec.Body.String())
		}
		var out checkInResp
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("解析打卡响应失败: %v", err)
		}
		return out
	}
	countLedger := func() int64 {
		var cnt int64
		if err := db.Model(&model.PointsLedger{}).Where("user_id = ? AND reason = ?", stu.ID, "checkin").Count(&cnt).Error; err != nil {
			t.Fatal(err)
		}
		return cnt
	}

	// ===== 1. 模拟昨日、前日已签（真实时钟：以 Asia/Shanghai 今日为锚）=====
	ts := time.Now().In(clock.Location())
	today := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, clock.Location())
	_ = db.Create(&model.ForumCheckIn{UserID: int(stu.ID), CheckDate: today.AddDate(0, 0, -2), CreatedAt: today.AddDate(0, 0, -2)})
	_ = db.Create(&model.ForumCheckIn{UserID: int(stu.ID), CheckDate: today.AddDate(0, 0, -1), CreatedAt: today.AddDate(0, 0, -1)})

	// ===== 2. 今日首签（连续第 3 天）：合并发 5+5=10 =====
	out := doCheckIn()
	if !out.Data.Checked || out.Data.Streak != 3 || out.Data.Total != 3 {
		t.Fatalf("连 3 天首签应 streak=3 total=3, got %+v", out.Data)
	}
	if out.Data.Points != 10 {
		t.Fatalf("第 3 天应合并发 10 分, got %d", out.Data.Points)
	}
	if countLedger() != 1 {
		t.Fatalf("应恰 1 笔打卡流水, got %d", countLedger())
	}
	// balance 到账
	var bal struct {
		Code int `json:"code"`
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	rec := doWithToken(t, r, token, http.MethodGet, "/api/points/balance", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &bal); err != nil {
		t.Fatal(err)
	}
	if bal.Data.Balance != 10 {
		t.Fatalf("打卡后余额应 10, got %d", bal.Data.Balance)
	}

	// ===== 3. 同日重复打卡：不双发 =====
	out2 := doCheckIn()
	if out2.Data.Points != 0 {
		t.Fatalf("重复打卡不应再发分, got %d", out2.Data.Points)
	}
	if countLedger() != 1 {
		t.Fatalf("重复打卡后流水应仍 1 笔, got %d", countLedger())
	}

	// ===== 4. 日历：整月逐日返回，今日 checked 且 points=10 =====
	calURL := fmt.Sprintf("/api/check-in/calendar?year=%d&month=%d", ts.Year(), int(ts.Month()))
	rec = doWithToken(t, r, token, http.MethodGet, calURL, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/check-in/calendar 应 200, got %d", rec.Code)
	}
	var cal calendarResp
	if err := json.Unmarshal(rec.Body.Bytes(), &cal); err != nil {
		t.Fatalf("解析日历失败: %v", err)
	}
	if len(cal.Data.Days) != daysInMonth(ts.Year(), ts.Month()) {
		t.Fatalf("日历应返整月 %d 天, got %d", daysInMonth(ts.Year(), ts.Month()), len(cal.Data.Days))
	}
	todayStr := today.Format("2006-01-02")
	foundToday := false
	for _, d := range cal.Data.Days {
		if d.Date == todayStr {
			foundToday = true
			if !d.Checked || d.Points != 10 {
				t.Fatalf("今日应 checked 且 points=10, got %+v", d)
			}
		}
		if d.Date == today.AddDate(0, 0, -1).Format("2006-01-02") {
			if !d.Checked || d.Points != 0 {
				t.Fatalf("昨日（旧数据无流水）应 checked/points=0, got %+v", d)
			}
		}
	}
	if !foundToday {
		t.Fatalf("日历缺今日 %s", todayStr)
	}

	// ===== 5. 旧路由已删除 =====
	old := doWithToken(t, r, token, http.MethodPost, "/api/forum/check-in", nil)
	if old.Code != http.StatusNotFound {
		t.Fatalf("旧打卡路由应 404, got %d", old.Code)
	}
	oldCal := doWithToken(t, r, token, http.MethodGet, "/api/forum/check-in/calendar", nil)
	if oldCal.Code != http.StatusNotFound {
		t.Fatalf("旧日历路由应 404, got %d", oldCal.Code)
	}
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
