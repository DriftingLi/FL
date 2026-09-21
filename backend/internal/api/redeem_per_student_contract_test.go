// 契约测（ADR-0062 票1）：兑换是「每人每 SKU」一个事件，占坑键必须含主体。
// seam S1（HTTP 契约层）：两个学员兑换同一门付费课程，各自成功、各自只被自己那次扣费。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

const redeemCoursePrice = 100

func TestTwoStudentsRedeemSameCourse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "redeem-per-student-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	price := redeemCoursePrice
	cred := 1
	course := model.Course{Name: "叉车液压系统维修", CredentialID: &cred, PointsPrice: &price, Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建付费课程失败: %v", err)
	}
	redeemURL := "/api/points/shop/course/" + strconv.Itoa(course.CourseID) + "/redeem"

	tokenOf := func(account string) string {
		pwd, err := service.HashPassword("student123")
		if err != nil {
			t.Fatalf("hash password failed: %v", err)
		}
		stu := testutil.SeedStudent(t, db, account, pwd)
		if err := db.Model(&model.HrwaiUser{}).Where("id = ?", stu.ID).
			Update("points_balance", 5*redeemCoursePrice).Error; err != nil {
			t.Fatalf("预置余额失败: %v", err)
		}
		sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{
			Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure,
		})
		token, err := sess.Issue(stu.ID, stu.Username, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return token
	}

	tokenA, tokenB := tokenOf("redeem_stu_a"), tokenOf("redeem_stu_b")

	recA := doWithToken(t, r, tokenA, http.MethodPost, redeemURL, nil)
	if recA.Code != http.StatusOK {
		t.Fatalf("甲兑换付费课程应 200, got %d body=%s", recA.Code, recA.Body.String())
	}
	if got, want := redeemBalance(t, recA), 4*redeemCoursePrice; got != want {
		t.Fatalf("甲兑换后余额 = %d, 期望 %d（扣一次课价）", got, want)
	}

	recB := doWithToken(t, r, tokenB, http.MethodPost, redeemURL, nil)
	if recB.Code != http.StatusOK {
		t.Fatalf("乙兑换同一门课应 200（兑换按人成立）, got %d body=%s", recB.Code, recB.Body.String())
	}
	if got, want := redeemBalance(t, recB), 4*redeemCoursePrice; got != want {
		t.Fatalf("乙兑换后余额 = %d, 期望 %d（各自只被自己那次扣费）", got, want)
	}
}

// redeemBalance 从响应信封读 data.balance——只经外部契约，不查库。
func redeemBalance(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	var envelope struct {
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("解析兑换响应失败: %v body=%s", err, rec.Body.String())
	}
	return envelope.Data.Balance
}
