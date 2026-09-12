package middleware

// 审计日志 IP 口径（ticket #888）：写进审计表的是服务端认定的客户端 IP，
// 不是请求头里自称的那个。
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// newAuditRouter 构造带审计中间件的路由：审计只对 admin/tutor 的写请求落库，
// 因此先用一个中间件把身份放进上下文（等价于 JWTAuth 之后的位置）。
func newAuditRouter(t *testing.T, trusted []string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	svc := service.NewAuditService(db)
	r := gin.New()
	if err := r.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies(%v) 失败: %v", trusted, err)
	}
	r.Use(func(c *gin.Context) {
		c.Set(string(CtxUserID), 7)
		c.Set(string(CtxUserRole), "admin")
		c.Next()
	})
	r.Use(AuditLog(svc, zap.NewNop()))
	r.POST("/api/admin/thing", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r, db
}

// latestAuditIP 发起一次写请求并返回审计表最新一条记录的 IP。
func latestAuditIP(t *testing.T, r *gin.Engine, db *gorm.DB, remoteAddr, xff string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/thing", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("写请求应成功: got %d, body %s", w.Code, w.Body.String())
	}
	var last model.AuditLog
	if err := db.Order("id DESC").First(&last).Error; err != nil {
		t.Fatalf("审计日志应落库: %v", err)
	}
	return last.IP
}

func TestAuditLog_RecordsRealClientIPNotSpoofedHeader(t *testing.T) {
	r, db := newAuditRouter(t, []string{"192.0.2.10"})
	got := latestAuditIP(t, r, db, "203.0.113.7:51820", "114.114.114.114")
	if got != "203.0.113.7" {
		t.Errorf("审计日志应记录真实客户端 IP: got %q, want %q", got, "203.0.113.7")
	}
}

func TestAuditLog_RecordsClientIPForwardedByTrustedProxy(t *testing.T) {
	r, db := newAuditRouter(t, []string{"192.0.2.10"})
	got := latestAuditIP(t, r, db, "192.0.2.10:41234", "114.114.114.114, 198.51.100.9")
	if got != "198.51.100.9" {
		t.Errorf("审计日志应记录代理转发的真实客户端 IP: got %q, want %q", got, "198.51.100.9")
	}
}
