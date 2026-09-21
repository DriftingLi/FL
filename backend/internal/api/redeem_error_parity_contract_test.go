// 契约测（ADR-0062 票9）：同一批积分哨兵在三个兑换面上必须同一个码。
// seam S1（HTTP 契约层）。旧形状：/points/* 走 pointsErrStatus（积分不足/已兑换 → 400），
// 而 /real-exam/papers/:id/redeem 挂 errStatusAll(404) ⇒ 同一事件两处不同码，
// 消费端只能靠 message 文案猜语义（前端据此弹过「已解锁本卷」的成功提示）。
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestRedeemErrorParityAcrossSurfaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "redeem-parity-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token := failureTestStudentToken(t, db, cfg, "redeem_parity_stu")

	paper := model.RealExamPaper{CredentialID: 1, Title: "叉车维修真题卷一", Status: 1,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&paper).Error; err != nil {
		t.Fatalf("建真题卷失败: %v", err)
	}
	course := model.Course{Name: "液压系统", CredentialID: intPtr(1), PointsPrice: intPtr(100), Status: 1,
		CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建付费课程失败: %v", err)
	}

	// 同一学员、同样余额不足（新学员余额 0），两个兑换面必须给同一个状态码。
	recPaper := doWithToken(t, r, token, http.MethodPost, "/api/real-exam/papers/1/redeem", nil)
	recCourse := doWithToken(t, r, token, http.MethodPost, "/api/points/shop/course/1/redeem", nil)

	if recCourse.Code != http.StatusBadRequest {
		t.Fatalf("课程兑换的积分族错误应 400（pointsErrStatus 现状）, got %d %s", recCourse.Code, recCourse.Body.String())
	}
	if recPaper.Code != recCourse.Code {
		t.Fatalf("同一事件两处不同码：real-exam=%d points=%d（%s）",
			recPaper.Code, recCourse.Code, recPaper.Body.String())
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recPaper.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if env.Message == "" {
		t.Fatal("积分不足的响应文案不得为空")
	}
}
