// 注销即全会话吊销（ADR-0060 票2 / spec #1201 场景 1、2）。
// seam：DELETE /api/auth/account 的 HTTP 契约面 + security.Session 的轮换结果。
// 修复前：硬删除不写吊销标记，而 RotateRefresh 不查用户是否存在——已注销身份最长仍有
// 7 天可用凭证（换出的 access 只过 JWT 校验）。
package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
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

// newDeleteAccountRouter 装配一个带登录态的 DELETE /api/auth/account（黑名单存储可注入）。
func newDeleteAccountRouter(t *testing.T, store security.BlacklistStore) (*gin.Engine, *security.Session, *gorm.DB, int) {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	sess := security.NewSessionWithBlacklistAndRefresh("test-secret", time.Hour, 7*time.Hour,
		security.CookieConfig{Name: "hrwai_token"}, store)
	authSvc := NewService(db, sess, core.NewForumCounter(), "admin", "tutor", "student", zap.NewNop())
	u := model.HrwaiUser{UID: 900001, Account: "gone-soon", Username: "即将注销", Password: "x", Phone: "13900000001", Status: 1}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("播种学员账号失败: %v", err)
	}
	r := gin.New()
	// P2 波 3a：注册真实路由面（/account 带 JWT 中间件），用例自己签 access 带上。
	RegisterRoutes(r.Group("/api"), sess, authSvc, nil, nil, nil, zap.NewNop())
	return r, sess, db, u.ID
}

// 注销成功后，该身份手上的 refresh（含注销前刚轮换出来的那枚）一律换不出新令牌。
func TestDeleteAccount_吊销后旧refresh被拒(t *testing.T) {
	t.Parallel()
	r, sess, db, uid := newDeleteAccountRouter(t, testutil.NewValueBlacklist())
	ctx := context.Background()

	tok, rotated, err := sess.IssuePair(uid, "gone-soon", "hrwai_user")
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

	if rec := testutil.CodeAuthRequest(r, "DELETE", "/api/auth/account", nil, tok); rec.Code != http.StatusOK {
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
	t.Parallel()
	r, sess, db, uid := newDeleteAccountRouter(t, rejectBlacklist{})
	tok, _, err := sess.IssuePair(uid, "gone-soon", core.HrwaiRole)
	if err != nil {
		t.Fatalf("签发 access 失败: %v", err)
	}

	rec := testutil.CodeAuthRequest(r, "DELETE", "/api/auth/account", nil, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("吊销标记写失败时注销应 400，实际 %d", rec.Code)
	}
	var cnt int64
	db.Model(&model.HrwaiUser{}).Where("id = ?", uid).Count(&cnt)
	if cnt != 1 {
		t.Errorf("注销未生效时账号应仍在，实际行数 %d", cnt)
	}
}
