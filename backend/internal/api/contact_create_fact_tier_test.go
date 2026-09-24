// 行为锁（ADR-0065 决策 4 步 2）：「发起交换申请」的三档必须**互相分辨得开**。
//
// 缺陷形状：旧 handler 对任何 error 都答 `response.BadRequest(c, err.Error())`，而 service 侧
// 两处 `First()` 又不区分「行不存在」与「查不动」——于是 hrwai_users 读不动时对外发的是
// 「学员不存在」，调用方按 400 处理（改参数、放弃重试），而真因是服务端故障。
//
// 另有一条本波自己造出来的口径要钉住：`ErrStudentNotFound` 这一枚哨兵在两个端点落**两个码**，
// 那不是「同一事实按端点分家」（批③ 收掉的那种），而是两件不同的事实——
// 「被请求的那个资源没有」(404) 与「body 里的引用指向不存在的行」(400)。载体合一不推出档位合一。
// 没有这条断言，下一波很可能把它「顺手统一」。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"forklift-training/internal/security"
)

// TestContactCreate_StudentLookupFaultIsNotNotFound 「查不动」不得冒充「学员不存在」。
func TestContactCreate_StudentLookupFaultIsNotNotFound(t *testing.T) {
	e := newContactCreateEnv(t)
	if err := e.db.Exec("DROP TABLE hrwai_users").Error; err != nil {
		t.Fatalf("注入故障（删 hrwai_users）失败: %v", err)
	}

	got := e.postCreate(t, e.recruiterTok, map[string]any{"student_user_id": e.studentID, "message": "想聊聊岗位"})
	if strings.Contains(got, "学员不存在") {
		t.Fatalf("读不到 hrwai_users 被发成了「学员不存在」，调用方会按参数错误放弃重试：%s", got)
	}
	if code := bodyCode(t, got); code != http.StatusInternalServerError {
		t.Fatalf("查不动应 500，实际 %s", got)
	}
	// 决策 9：5xx 不外发驱动原文。
	for _, leak := range []string{"no such table", "SQLITE", "gorm", "SELECT", "record not found"} {
		if strings.Contains(got, leak) {
			t.Fatalf("500 的 message 里带着驱动原文 %q：%s", leak, got)
		}
	}
}

// TestContactCreate_BadReferenceIsNotNotFound 同一枚 ErrStudentNotFound 在两端的档位是**两件事实**：
// 本端点坏引用 400，档案端点被请求的资源缺席 404。
func TestContactCreate_BadReferenceIsNotNotFound(t *testing.T) {
	e := newContactCreateEnv(t)

	// 面 1：POST /api/recruit/contact-requests，body 里的 student_user_id 指向不存在的行 ⇒ 400。
	got := e.postCreate(t, e.recruiterTok, map[string]any{"student_user_id": e.missingID, "message": "想聊聊岗位"})
	if code := bodyCode(t, got); code != http.StatusBadRequest {
		t.Fatalf("坏引用应 400（被请求的资源是「申请集合」，它在），实际 %s", got)
	}
	if !strings.Contains(got, "学员不存在") {
		t.Fatalf("档位对但句子丢了：应仍说「学员不存在」，实际 %s", got)
	}

	// 面 2：GET /api/student/profile，被请求的资源就是那个学员 ⇒ 404。
	// 用一枚指向不存在学员的合法会话打它，让 service 的那次 First 去答（绕开中间件的 401 分档）。
	sess := security.NewSession(contactCreateSecret, time.Hour, security.CookieConfig{Name: "hrwai_token"})
	tok, err := sess.Issue(e.missingID, "ghost", "hrwai_user")
	if err != nil {
		t.Fatalf("签发会话失败: %v", err)
	}
	rec := doWithToken(t, e.router, tok, http.MethodGet, "/api/student/profile", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("档案端点的「学员不存在」应 404（那才是被请求的资源缺席），实际 %d %s", rec.Code, rec.Body.String())
	}
}

// TestContactCreate_TableCoversEveryBusinessFact 表的自证：九条业务事实各说一件自己的句子，
// 且句子互不相同——「装配漏项」由行为面抓（日限那条就是这么发现的），这里抓的是另一种漏法：
// 两条事实共用一句 ⇒ 客户端分不出是哪一件。
func TestContactCreate_TableCoversEveryBusinessFact(t *testing.T) {
	seen := map[string]string{}
	for _, sent := range contactCreateFacts400 {
		text := sent.Error()
		if text == "" {
			t.Fatalf("哨兵 %v 没有对外句子，落到 400 面会发空 message", sent)
		}
		if prev, dup := seen[text]; dup {
			t.Fatalf("两条事实共用一句 %q：%s 与 %s", text, prev, sent)
		}
		seen[text] = sent.Error()
	}
	if len(contactCreateFacts400) != 9 {
		t.Fatalf("本端点的业务事实应为 9 条，实际 %d 条：%v", len(contactCreateFacts400), contactCreateFacts400)
	}
}

// bodyCode 从统一信封里读 code（测试断言整个字节形状时用 contactBody 字面量，这里只关心档位）。
func bodyCode(t *testing.T, body string) int {
	t.Helper()
	var r struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("响应不是合法信封: %v\n%s", err, body)
	}
	return r.Code
}
