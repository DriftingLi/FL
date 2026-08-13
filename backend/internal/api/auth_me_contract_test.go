// Ticket #84（issue #84）契约测试：锁定 GET /auth/me 响应 JSON 形状。
// profile 组装收编进 AuthService.GetProfile 前后，响应体必须字节级一致。
// data 为 map 序列化（encoding/json 按键排序），比对完整响应字符串。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestAuthMeContract_ShapeUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "contract-test-secret"

	cases := []struct {
		name     string
		role     string
		username string
		want     string
	}{
		{
			name: "hrwai_user", role: "hrwai_user", username: "alice",
			want: `{"code":200,"message":"success","data":{"account":"acct_alice","avatar_url":"","company":"","email":"","has_password":true,"pending_profile_change":null,"phone":"test_alice","role":"hrwai_user","uid":"1000000000000000001","user_id":1,"username":"alice"}}`,
		},
		{
			name: "tutor", role: "tutor", username: "tutor1",
			want: `{"code":200,"message":"success","data":{"account":"tutor1","name":"tutor1","role":"tutor","user_id":1,"username":"tutor1"}}`,
		},
		{
			name: "admin", role: "admin", username: "admin1",
			want: `{"code":200,"message":"success","data":{"account":"admin1","name":"admin1","role":"admin","user_id":1,"username":"admin1"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			var userID int
			switch tc.role {
			case "hrwai_user":
				userID = testutil.SeedStudent(t, db, tc.username, "hash123").ID
			case "tutor":
				userID = testutil.SeedTutor(t, db, tc.username, "hash123").TutorID
			case "admin":
				userID = testutil.SeedAdmin(t, db, tc.username, "hash123").AdminID
			}

			cfg := &config.Config{
				JWTSecretKey: secret,
				AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
			}
			r := NewRouter(newContractDeps(t, db, cfg))

			token, err := security.NewSession(secret, time.Hour, security.CookieConfig{}).
				Issue(userID, tc.username, tc.role)
			if err != nil {
				t.Fatalf("签发 token 失败: %v", err)
			}

			req, _ := http.NewRequest("GET", "/api/auth/me", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
			}
			if got := w.Body.String(); got != tc.want {
				t.Errorf("响应体与契约不符\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// TestAuthMeContract_EmailPlaceholderPhoneMasked 值级契约：邮箱注册的占位手机号
// （email_ 前缀）在 /auth/me 源头过滤为空串，其余字段与形状不变。
func TestAuthMeContract_EmailPlaceholderPhoneMasked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "contract-test-secret"

	db := testutil.NewMemoryDB(t)
	u := testutil.SeedStudent(t, db, "bob", "hash123")
	if err := db.Model(u).Update("phone", "email_0123456789abcdef0123456789abcdef").Error; err != nil {
		t.Fatalf("更新手机号失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: secret,
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	token, err := security.NewSession(secret, time.Hour, security.CookieConfig{}).
		Issue(u.ID, u.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			Phone    string `json:"phone"`
			Email    string `json:"email"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if env.Data.Phone != "" {
		t.Fatalf("期望占位手机号被过滤为空串, got %q", env.Data.Phone)
	}
	if env.Data.Username != "bob" {
		t.Fatalf("昵称字段不应受影响, got %q", env.Data.Username)
	}
}
