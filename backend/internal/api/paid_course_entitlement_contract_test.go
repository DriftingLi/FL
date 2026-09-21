// 契约测（ADR-0062 票2）：兑换写下的权益必须有读面消费——付费课程的章节正文与幻灯片
// 在学员未兑换时按「不存在」返回，兑换后同一请求即 200。
// 旧形状：全仓只有真题卷域读 user_entitlement，课程的闸门只剩前端内存里抹掉的 points_price
// ⇒ 后端对付费课程零门禁（直连读面即可白看），而刷新后已付费的学员反被拦回「请先兑换」。
package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestPaidCourseEntitlementGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "paid-gate-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	price := 100
	spec := model.Specialty{Code: "paid-gate", Name: "付费门禁", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建专业方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "paid-gate-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建课程等级失败: %v", err)
	}
	course := model.Course{Name: "叉车电气诊断", Status: 1, PointsPrice: &price,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建付费课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "万用表使用", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	pwd, _ := service.HashPassword("student123")
	student := testutil.SeedStudent(t, db, "paid_gate_stu", pwd)
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).UpdateColumn("points_balance", 500).Error; err != nil {
		t.Fatalf("预置余额失败: %v", err)
	}
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(student.ID, student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	chapterURL := fmt.Sprintf("/api/course/%d/chapter/%d", course.CourseID, ch.ChapterID)
	slidesURL := fmt.Sprintf("/api/chapter/%d/slides", ch.ChapterID)

	// 未兑换：两条内容读面都按「不存在」（不泄漏「这门课要钱」之外的信息）。
	// 断言必须命中信封 404 —— 路由写错时的 gin 裸 404 同样状态码，会伪装成通过。
	for _, u := range []string{chapterURL, slidesURL} {
		rec := doWithToken(t, r, token, http.MethodGet, u, nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":404`) {
			t.Fatalf("未兑换读付费内容应命中信封 404: %s got %d %s", u, rec.Code, rec.Body.String())
		}
	}

	if rec := doWithToken(t, r, token, http.MethodPost,
		fmt.Sprintf("/api/points/shop/course/%d/redeem", course.CourseID), nil); rec.Code != http.StatusOK {
		t.Fatalf("兑换付费课程应 200, got %d %s", rec.Code, rec.Body.String())
	}

	// 兑换后：权益即成为读面依据（服务端事实，不靠前端内存标记）
	for _, u := range []string{chapterURL, slidesURL} {
		if rec := doWithToken(t, r, token, http.MethodGet, u, nil); rec.Code != http.StatusOK {
			t.Fatalf("兑换后读该章节内容应 200: %s got %d %s", u, rec.Code, rec.Body.String())
		}
	}
}
