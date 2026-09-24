// 行为锁（ADR-0065 决策 4 步 2）：「发起交换申请」的三档必须**互相分辨得开**。
//
// 缺陷形状：旧 handler 对任何 error 都答 `response.BadRequest(c, err.Error())`，而 service 侧
// 两处 `First()` 又不区分「行不存在」与「查不动」——于是 hrwai_users / recruiter_users 读不动时
// 对外发的却是「学员不存在」「招聘者不存在」：调用方按 400 处理（改参数、放弃重试），真因在服务端。
//
// 另有一条本波自己造出来的口径要钉住：`ErrStudentNotFound` / `ErrRecruiterNotFound` 各自在两个
// 端点落**两个码**。那不是「同一事实按端点分家」（批③ 收掉的那种），而是两件事实——「被请求的
// 那个资源没有」(404) 与「请求带来的引用指向不存在的行」(400)。载体合一不推出档位合一；没有这组
// 「两半同时成立」的断言，下一波把它顺手统一掉不会有任何测试变红。
//
// 判据一律复用台账那三份（dropTable 注故障 / leakyDriverText 认驱动原文 / unpackData 拆信封），
// 本文件不重写第二份——批③ 评审就是按这条抓过我自己另起的一份。
package api

import (
	"net/http"
	"testing"
	"time"

	"forklift-training/internal/security"
)

// TestContactCreate_RowLookupFaultIsNotNotFound 两处行查询的「查不动」都要落 500，且不得借用
// 相邻那一档的句子。表驱动是必要的：只测学员那半边时，把招聘者那半边的 ErrRecordNotFound 分档
// 整个删掉、全包仍然绿（本批实测）。
func TestContactCreate_RowLookupFaultIsNotNotFound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table string
		said  string
	}{
		{"学员那一行读不动", "hrwai_users", "学员不存在"},
		{"招聘者那一行读不动", "recruiter_users", "招聘者不存在"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newContactCreateEnv(t)
			dropTable(tc.table)(t, e.db)

			_, msg, _ := unpackData(t,
				doWithToken(t, e.router, e.recruiterTok, http.MethodPost, "/api/recruit/contact-requests",
					map[string]any{"student_user_id": e.studentID, "message": "想聊聊岗位"}),
				http.StatusInternalServerError)
			if msg == tc.said {
				t.Fatalf("查不动被发成了「%s」，调用方会按参数错误放弃重试", msg)
			}
			if leakyDriverText(msg) {
				t.Fatalf("500 的 message 里带着驱动/ORM 原文：%q", msg)
			}
		})
	}
}

// TestContactCreate_BadReferenceIsNotMissingResource 同一枚哨兵、两个端点、两件事实。
// 400 那一半的字节另由保形锁钉着；本锁要的是**两半同时成立**这一格。
func TestContactCreate_BadReferenceIsNotMissingResource(t *testing.T) {
	t.Run("学员不存在：档案面 404、发起面 400", func(t *testing.T) {
		e := newContactCreateEnv(t)

		// 被请求的资源就是那个学员本身 ⇒ 404。用一枚指向不存在学员的合法会话打它，让 service
		// 的那次 First 去答——中间件只读 claim，不参与这一档。
		sess := security.NewSession(contactCreateSecret, time.Hour, security.CookieConfig{Name: "hrwai_token"})
		tok, err := sess.Issue(e.missingID, "ghost", "hrwai_user")
		if err != nil {
			t.Fatalf("签发会话失败: %v", err)
		}
		_, msg, _ := unpackData(t,
			doWithToken(t, e.router, tok, http.MethodGet, "/api/student/profile", nil),
			http.StatusNotFound)
		if msg != "学员不存在" {
			t.Fatalf("档案面那一句换了：%q", msg)
		}

		// 发起申请时它是请求带来的引用 ⇒ 400（被请求的资源是「申请集合」，它在）。
		_, msg, _ = unpackData(t,
			doWithToken(t, e.router, e.recruiterTok, http.MethodPost, "/api/recruit/contact-requests",
				map[string]any{"student_user_id": e.missingID, "message": "想聊聊岗位"}),
			http.StatusBadRequest)
		if msg != "学员不存在" {
			t.Fatalf("同一枚哨兵在发起面换了句：%q", msg)
		}
	})

	t.Run("招聘者不存在：管理面 404、发起面 400", func(t *testing.T) {
		e := newContactCreateEnv(t)

		_, msg, _ := unpackData(t,
			doWithToken(t, e.router, e.adminTok, http.MethodPut,
				"/api/admin/recruiters/999999/status", map[string]any{"status": 0}),
			http.StatusNotFound)
		if msg != "招聘者不存在" {
			t.Fatalf("管理面那一句换了：%q", msg)
		}

		// 发起面用 e.vanishedTok：登录之后行被删，会话仍认（撤销不住在这张表上）。
		_, msg, _ = unpackData(t,
			doWithToken(t, e.router, e.vanishedTok, http.MethodPost, "/api/recruit/contact-requests",
				map[string]any{"student_user_id": e.studentID, "message": "想聊聊岗位"}),
			http.StatusBadRequest)
		if msg != "招聘者不存在" {
			t.Fatalf("同一枚哨兵在发起面换了句：%q", msg)
		}
	})
}

// TestContactCreate_FactTableIsNineDistinctFacts 表的**自证**（不管行为，行为由上面两把锁管）：
// 九条事实、每条有自己那句、互不相同。抓的是「两条事实共用一句」那种漏法——档位全对、码全对，
// 客户端却分不出是哪一件。装配**漏项**不在这里抓：漏一条会掉进 500 默认面，由行为面当场判红
// （日限那条就是这么发现的）。
func TestContactCreate_FactTableIsNineDistinctFacts(t *testing.T) {
	seen := map[string]string{}
	for _, sent := range contactCreateFacts400 {
		text := sent.Error()
		if text == "" {
			t.Fatalf("哨兵 %v 没有对外句子，落到 400 面会发空 message", sent)
		}
		if prev, dup := seen[text]; dup {
			t.Fatalf("两条事实共用一句 %q：%s 与 %s", text, prev, sent)
		}
		seen[text] = text
	}
	if len(contactCreateFacts400) != 9 {
		t.Fatalf("本端点的业务事实应为 9 条，实际 %d 条：%v", len(contactCreateFacts400), contactCreateFacts400)
	}
}
