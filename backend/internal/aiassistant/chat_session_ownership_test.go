package aiassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/filestore"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// 本文件：/chat 写入侧会话归属校验（票 2313）。
//
// 缺陷形态：StreamChat 的持久化分支只判「登录了 + 给了 session_id」，不判会话属于谁
// ⇒ 登录用户 A 传 B 的 session_id 能把消息写进对方会话并刷新 updated_at。
// 修复面：归属判定前置到**第一个 SSE 字节之前**，不命中回 404 信封。

// ownershipPort 固定返回一段答案的模型端口（只关心「有没有被调用」）。
type ownershipPort struct {
	mu     sync.Mutex
	called int
}

func (p *ownershipPort) Complete(string, []*schema.Message, CompleteOptions) (string, error) {
	return "答案", nil
}

func (p *ownershipPort) Stream(_ context.Context, _ ModelSelector, _ []*schema.Message, _ func(string)) (string, *Usage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.called++
	return "答案", nil, nil
}

func (p *ownershipPort) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.called
}

// chatFixture 一套「真 DB + 真 handler」的 /chat 调用装置。
type chatFixture struct {
	svc  *Service
	db   *gorm.DB
	port *ownershipPort
}

func newChatFixture(t *testing.T) *chatFixture {
	t.Helper()
	return newChatFixtureOn(t, testutil.NewFileDB(t))
}

// newChatFixtureOn 用指定 DB 建装置（Postgres 契约测试复用它）。
func newChatFixtureOn(t *testing.T, db *gorm.DB) *chatFixture {
	t.Helper()
	cfgSvc := NewConfigService(db, "test-master-key", zap.NewNop())
	port := &ownershipPort{}
	svc := NewService(db, cfgSvc, filestore.NewFileStore("", nil, zap.NewNop()), "test-master-key", zap.NewNop(), port)
	return &chatFixture{svc: svc, db: db, port: port}
}

func (f *chatFixture) createSession(t *testing.T, userID int, title string) *AIChatSessionDTO {
	t.Helper()
	s, err := f.svc.CreateSession(context.Background(), userID, title, "", FeatureFaultDiagnosis)
	if err != nil {
		t.Fatalf("CreateSession 失败: %v", err)
	}
	return s
}

// post 以指定登录用户（0=未登录）调 /chat，返回记录器。
func (f *chatFixture) post(t *testing.T, userID int, body StreamChatReq) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := newHandler(f.svc)

	r := gin.New()
	// 复刻 middleware.OptionalAuth 的效果：CurrentUserID 读的就是这个 gin context key。
	r.POST("/chat", func(c *gin.Context) {
		if userID > 0 {
			c.Set(string(middleware.CtxUserID), userID)
		}
		h.StreamChat(c)
	})

	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func (f *chatFixture) messageCount(t *testing.T, sessionID int) int64 {
	t.Helper()
	var n int64
	f.db.Model(&model.AIChatMessage{}).Where("session_id = ?", sessionID).Count(&n)
	return n
}

func (f *chatFixture) sessionUpdatedAt(t *testing.T, sessionID int) string {
	t.Helper()
	var s model.AIChatSession
	if err := f.db.Where("id = ?", sessionID).First(&s).Error; err != nil {
		t.Fatalf("读会话失败: %v", err)
	}
	return s.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000")
}

// TestChatRejectsForeignSession 核心用例：A 拿 B 的 session_id ⇒ 404 信封，
// 且 B 的会话「消息条数不变、updated_at 不变、模型端口没被调用」。
//
// 前两条刻意用查库断言而不是只看 HTTP 码：这个缺陷的表现就是「写进去了但读侧看不见」，
// 只断言 404 会在「先回 404、后又偷偷写了」的实现上误绿。
func TestChatRejectsForeignSession(t *testing.T) {
	f := newChatFixture(t)
	bSession := f.createSession(t, 2001, "B 的会话")
	beforeCount := f.messageCount(t, bSession.ID)
	beforeUpdated := f.sessionUpdatedAt(t, bSession.ID)

	w := f.post(t, 2002, StreamChatReq{
		SessionID: bSession.ID,
		Messages:  []StreamMessage{{Role: "user", Content: "A 想蹭写"}},
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("越权写他人会话应回 404，得到 %d：%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "会话不存在") {
		t.Fatalf("404 文案应与既有「会话不存在」一致，得到：%s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "event:") {
		t.Fatalf("前置失败不应写出任何 SSE 帧，得到：%s", w.Body.String())
	}
	if got := f.messageCount(t, bSession.ID); got != beforeCount {
		t.Fatalf("B 的会话消息条数变了：%d → %d（越权写成立）", beforeCount, got)
	}
	if got := f.sessionUpdatedAt(t, bSession.ID); got != beforeUpdated {
		t.Fatalf("B 的会话 updated_at 被刷新：%s → %s", beforeUpdated, got)
	}
	if n := f.port.callCount(); n != 0 {
		t.Fatalf("归属校验失败时不该调用模型端口，实际调用 %d 次", n)
	}
}

// TestChatRejectsMissingSession 不存在的 session_id 同样前置拒绝，
// 且不把数据库约束名/SQLSTATE 透给客户端。
func TestChatRejectsMissingSession(t *testing.T) {
	f := newChatFixture(t)

	w := f.post(t, 2001, StreamChatReq{
		SessionID: 999999,
		Messages:  []StreamMessage{{Role: "user", Content: "不存在的会话"}},
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在的会话应回 404，得到 %d：%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, leak := range []string{"foreign key", "SQLSTATE", "ai_chat_messages_session_id_fkey", "pq:"} {
		if strings.Contains(body, leak) {
			t.Fatalf("响应泄漏了数据库原文（%s）：%s", leak, body)
		}
	}
	if n := f.port.callCount(); n != 0 {
		t.Fatalf("会话不存在时不该调用模型端口，实际调用 %d 次", n)
	}
}

// TestChatOwnSessionStillPersists 自己的会话行为不变：正常流、落库两行、updated_at 刷新。
func TestChatOwnSessionStillPersists(t *testing.T) {
	f := newChatFixture(t)
	mine := f.createSession(t, 2001, "自己的会话")
	beforeUpdated := f.sessionUpdatedAt(t, mine.ID)

	w := f.post(t, 2001, StreamChatReq{
		SessionID: mine.ID,
		Messages:  []StreamMessage{{Role: "user", Content: "我自己的问题"}},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("自己的会话应正常流式返回 200，得到 %d：%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "event: done") {
		t.Fatalf("应流完并发出 done 事件：%s", w.Body.String())
	}
	if got := f.messageCount(t, mine.ID); got != 2 {
		t.Fatalf("自己的会话应落库两行（user + assistant），得到 %d", got)
	}
	if got := f.sessionUpdatedAt(t, mine.ID); got == beforeUpdated {
		t.Fatalf("自己的会话 updated_at 应被刷新，仍是 %s", got)
	}
}

// TestChatAnonymousAndZeroSessionNoPersist 未登录 / session_id=0 行为不变：不落库、不拦。
func TestChatAnonymousAndZeroSessionNoPersist(t *testing.T) {
	cases := []struct {
		name   string
		userID int
	}{
		{"未登录", 0},
		{"登录但 session_id=0", 2001},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newChatFixture(t)
			w := f.post(t, tc.userID, StreamChatReq{
				Messages: []StreamMessage{{Role: "user", Content: "免登录试用"}},
			})
			if w.Code != http.StatusOK {
				t.Fatalf("应正常流式返回 200，得到 %d：%s", w.Code, w.Body.String())
			}
			var total int64
			f.db.Model(&model.AIChatMessage{}).Count(&total)
			if total != 0 {
				t.Fatalf("未指定会话时不该落库任何消息，实际 %d 条", total)
			}
		})
	}
}

// TestChatMissingSessionDoesNotLeakDBErrorOnPostgres 票 2313 的 Postgres 契约测试面。
//
// 为什么必须单独一条跑 PG：外键只在 Postgres 生效，SQLite 面下把消息写进不存在的会话
// 会静默成功（err = nil）—— 也就是说，若把归属校验漏掉，「不泄漏数据库原文」这条
// 在 SQLite 上永远绿。本地无 DATABASE_URL 时本用例干净 skip，首跑在 CI。
func TestChatMissingSessionDoesNotLeakDBErrorOnPostgres(t *testing.T) {
	f := newChatFixtureOn(t, testutil.NewPostgresDB(t))

	w := f.post(t, 2001, StreamChatReq{
		SessionID: 999999,
		Messages:  []StreamMessage{{Role: "user", Content: "不存在的会话"}},
	})

	body := w.Body.String()
	for _, leak := range []string{"foreign key", "SQLSTATE", "ai_chat_messages_session_id_fkey", "pq:"} {
		if strings.Contains(body, leak) {
			t.Fatalf("Postgres 面响应泄漏了数据库原文（%s）：%s", leak, body)
		}
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("不存在的会话应回 404（不是 200 带 error 帧），得到 %d：%s", w.Code, body)
	}
}

// TestChatPersistFailureDoesNotLeakDBError 落库阶段失败时也不外发数据库原文。
//
// 走的是「归属校验通过、但写入阶段出错」这条路径：把 messages 表改名让 Create 必然失败。
// 期望 SSE 的 error 文案是面向用户的话，且不含驱动原文。
func TestChatPersistFailureDoesNotLeakDBError(t *testing.T) {
	f := newChatFixture(t)
	mine := f.createSession(t, 2001, "自己的会话")
	if err := f.db.Migrator().RenameTable("ai_chat_messages", "ai_chat_messages_broken"); err != nil {
		t.Fatalf("改表名失败: %v", err)
	}
	t.Cleanup(func() {
		_ = f.db.Migrator().RenameTable("ai_chat_messages_broken", "ai_chat_messages")
	})

	w := f.post(t, 2001, StreamChatReq{
		SessionID: mine.ID,
		Messages:  []StreamMessage{{Role: "user", Content: "落库会失败"}},
	})

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Fatalf("落库失败应发出 error 事件：%s", body)
	}
	for _, leak := range []string{"no such table", "SQL logic error", "sqlite", "UNIQUE constraint"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
			t.Fatalf("SSE 错误泄漏了数据库原文（%s）：%s", leak, body)
		}
	}
}
