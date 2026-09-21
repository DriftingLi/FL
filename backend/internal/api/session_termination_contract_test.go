// 注销即全会话吊销（ADR-0060 票2 / spec #1201 场景 1、2）。
// seam：DELETE /api/auth/account 的 HTTP 契约面 + security.Session 的轮换结果。
// 修复前：硬删除不写吊销标记，而 RotateRefresh 不查用户是否存在——已注销身份最长仍有
// 7 天可用凭证（换出的 access 只过 JWT 校验）。
package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// rejectBlacklist 任何写入都失败——注入「吊销标记写不进去」的故障。
type rejectBlacklist struct{}

func (rejectBlacklist) Get(context.Context, string) (string, error) {
	return "", errors.New("blacklist down")
}

func (rejectBlacklist) Set(context.Context, string, string, time.Duration) error {
	return errors.New("blacklist down")
}

func (rejectBlacklist) PutIfAbsent(context.Context, string, string, time.Duration) (bool, error) {
	return false, errors.New("blacklist down")
}

// valBlacklist 保留写入值的内存黑名单存储：用户级吊销标记的值是时间戳，
// auth_refresh_test.go 的 memBlacklist 恒写 "1"（只用于按 key 存在性判定），
// 用它测吊销标记会把标记读成 1970 年、判成「未吊销」。
type valBlacklist struct{ m map[string]string }

func newValBlacklist() *valBlacklist { return &valBlacklist{m: map[string]string{}} }

func (s *valBlacklist) Get(_ context.Context, key string) (string, error) {
	v, ok := s.m[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

func (s *valBlacklist) Set(_ context.Context, key, value string, _ time.Duration) error {
	s.m[key] = value
	return nil
}

func (s *valBlacklist) PutIfAbsent(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	if _, ok := s.m[key]; ok {
		return false, nil
	}
	s.m[key] = value
	return true, nil
}

// newDeleteAccountRouter 装配一个带登录态的 DELETE /api/auth/account（黑名单存储可注入）。
func newDeleteAccountRouter(t *testing.T, store security.BlacklistStore) (*gin.Engine, *security.Session, *gorm.DB, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, store)
	authSvc := service.NewAuthService(db, sess, service.NewForumCounter(), "admin", "tutor", "student", zap.NewNop())
	u := model.HrwaiUser{UID: 900001, Account: "gone-soon", Username: "即将注销", Password: "x", Phone: "13900000001", Status: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("播种学员账号失败: %v", err)
	}
	h := NewAuthHandler(sess, authSvc, nil, nil, nil, zap.NewNop())
	r := gin.New()
	g := r.Group("/api/auth", func(c *gin.Context) {
		c.Set(string(middleware.CtxUserID), u.ID)
		c.Next()
	})
	g.DELETE("/account", h.DeleteAccount)
	return r, sess, db, u.ID
}

// 注销成功后，该身份手上的 refresh（含注销前刚轮换出来的那枚）一律换不出新令牌。
func TestDeleteAccount_吊销后旧refresh被拒(t *testing.T) {
	r, sess, db, uid := newDeleteAccountRouter(t, newValBlacklist())
	ctx := context.Background()

	_, rotated, err := sess.IssuePair(uid, "gone-soon", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 refresh 失败: %v", err)
	}
	// 模拟客户端处于会话中段：先轮换一次，手上剩下 rotated
	_, liveRefresh, err := sess.RotateRefresh(ctx, rotated)
	if err != nil {
		t.Fatalf("注销前应可正常轮换: %v", err)
	}
	if liveRefresh == "" {
		t.Fatal("轮换未返回新 refresh")
	}

	if rec := performRequest(r, "DELETE", "/api/auth/account"); rec.Code != http.StatusOK {
		t.Fatalf("注销应 200，实际 %d", rec.Code)
	}
	var cnt int64
	db.Model(&model.HrwaiUser{}).Where("id = ?", uid).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("注销后账号应已删除，实际行数 %d", cnt)
	}

	if _, _, err := sess.RotateRefresh(ctx, liveRefresh); !errors.Is(err, security.ErrInvalidRefresh) {
		t.Errorf("注销后旧 refresh 应报 ErrInvalidRefresh（端点侧映射为 401），实际: %v", err)
	}
}

// 吊销标记写不进去 ⇒ 注销整体不生效，账号仍在（不留「资料已删、凭证仍活」的半成品）。
func TestDeleteAccount_吊销写失败则整体不生效(t *testing.T) {
	r, _, db, uid := newDeleteAccountRouter(t, rejectBlacklist{})

	rec := performRequest(r, "DELETE", "/api/auth/account")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("吊销标记写失败时注销应 400，实际 %d", rec.Code)
	}
	var cnt int64
	db.Model(&model.HrwaiUser{}).Where("id = ?", uid).Count(&cnt)
	if cnt != 1 {
		t.Errorf("注销未生效时账号应仍在，实际行数 %d", cnt)
	}
}
