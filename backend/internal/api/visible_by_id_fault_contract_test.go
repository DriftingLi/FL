// 契约测（ADR-0065 决策 7）：可见性判定的「查不动」与「真不可见」在每个消费端点都要分得开。
//
// 缺陷形状：`CourseVisibleByID` 与 `QuestionReadScope.VisibleByID` 都把 `.Error` 丢掉、一律
// fail-closed 返回 false。保守方向没错，错的是**没有把「问不出」这件事交出去**——于是读不动
// question/course 表时，对外与「这道题不在池内」「这门课不存在」逐字同形：404「题目不存在」。
// 调用方会把一次服务端故障当成空态渲染并缓存，故障就此隐身。
//
// 三条断言合起来才是这条改判的证据：① 注入故障 ⇒ 500；② 不注入故障的真池外题 ⇒ 仍 404
// （否则「一律 500」也能骗过第 ① 条）；③ 500 的 message 不带驱动原文（决策 9 在裸 handler
// 一侧的等价落点）。夹具复用 question_scope_w14_contract_test.go 的 poolLeakFixture，
// 它已经把 note / comment / favorite 三条路由都挂上了。
package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// qpath 把题目 id 渲染进路径。本文件不叫它 id——ledger 包里已有同名 helper，
// 而这里的循环变量也叫 id，重名会让「调的是哪个」读不出来。
func qpath(v int) string { return strconv.Itoa(v) }

// TestVisibleByIDFaultIsNotOutOfPool 题目侧：读不动 question 表 ⇒ 500，不是「题目不存在」的 404。
func TestVisibleByIDFaultIsNotOutOfPool(t *testing.T) {
	f := newPoolLeakFixture(t)
	dropTable("question")(t, f.db)

	for _, tc := range []struct {
		name, method, path string
		body               any
		said               string
	}{
		{"评论列表", http.MethodGet, "/api/questions/" + qpath(f.poolQ.ID) + "/comments?page_size=10", nil, "题目不存在"},
		{"写笔记", http.MethodPut, "/api/questions/" + qpath(f.poolQ.ID) + "/note", map[string]any{"content": "我的笔记"}, "题目不存在"},
		{"收藏题目", http.MethodPost, "/api/favorites", map[string]any{"target_type": "question", "target_id": f.poolQ.ID}, "题目不存在或不可收藏"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := doAndBody(t, f, f.studentToken, tc.method, tc.path, tc.body)
			if code != http.StatusInternalServerError {
				t.Fatalf("读不动 question 表被发成了 %d（应 500）：%s", code, body)
			}
			if strings.Contains(body, tc.said) {
				t.Fatalf("故障被冒充成池外事实「%s」：%s", tc.said, body)
			}
			if leakyDriverText(body) {
				t.Fatalf("500 的 message 带着驱动/ORM 原文：%s", body)
			}
		})
	}
}

// TestOutOfPoolQuestionIsStillNotFound 反向半边：没有故障时，池外题必须仍答 404。
// 少了这条，「把所有错误都推给 500」也能骗过上面那把锁——两半同时成立才叫分档。
func TestOutOfPoolQuestionIsStillNotFound(t *testing.T) {
	f := newPoolLeakFixture(t)
	for qid, name := range f.hiddenByID() {
		code, body := doAndBody(t, f, f.studentToken, http.MethodGet,
			"/api/questions/"+qpath(qid)+"/comments?page_size=10", nil)
		if code != http.StatusNotFound {
			t.Fatalf("%s 题的评论读面应 404（池外按不存在，不泄漏存在性），实际 %d：%s", name, code, body)
		}
	}
	// 对照组：池内题照常 200，证明上面那条红不是「这个端点根本答不出 200」。
	if code, body := doAndBody(t, f, f.studentToken, http.MethodGet,
		"/api/questions/"+qpath(f.poolQ.ID)+"/comments?page_size=10", nil); code != http.StatusOK {
		t.Fatalf("池内题应 200，实际 %d：%s", code, body)
	}
}

// TestFaultFacesAllSayFault 逐个端点注故障：凡本批新声明 @Failure 500 的面，都必须真打得出 500。
// 判据 8 的「登记的档必须打得出」在这一半上的形状——文档里写一档而代码出不来，就是本批第一版
// 被双轴评审抓到的那件事（/notes 四个面当时只加了 swagger 行，fallback 仍是 400）。
func TestFaultFacesAllSayFault(t *testing.T) {
	f := newPoolLeakFixture(t)
	dropTable("question")(t, f.db)
	dropTable("note")(t, f.db)
	dropTable("question_comment")(t, f.db)

	for _, tc := range []struct {
		name, method, path string
		body               any
	}{
		{"笔记列表", http.MethodGet, "/api/notes?page_size=10", nil},
		{"新建笔记", http.MethodPost, "/api/notes", map[string]any{"question_id": f.poolQ.ID, "content": "正文"}},
		{"改笔记", http.MethodPut, "/api/notes/1", map[string]any{"content": "正文"}},
		{"删笔记", http.MethodDelete, "/api/notes/1", nil},
		{"删评论", http.MethodDelete, "/api/questions/comments/1", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body := doAndBody(t, f, f.studentToken, tc.method, tc.path, tc.body)
			if code != http.StatusInternalServerError {
				t.Fatalf("本面已在 swagger 声明 500 档，实际却答 %d ⇒ 那一行是对消费方的空头承诺：%s", code, body)
			}
			if leakyDriverText(body) {
				t.Fatalf("500 的 message 带着驱动/ORM 原文：%s", body)
			}
		})
	}
}

// TestFavoriteTargetFactsStillSayThemselves 收藏域被本批改了签名的那条链上，业务事实必须仍各说自己的句子
// （表里每条若被默认面吞成 500，这里就红）。
func TestFavoriteTargetFactsStillSayThemselves(t *testing.T) {
	f := newPoolLeakFixture(t)
	for _, tc := range []struct{ name, want string }{
		{"不支持的类型", "收藏类型仅支持"},
		{"池外课程", "课程不存在或不可收藏"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			switch tc.name {
			case "不支持的类型":
				body = map[string]any{"target_type": "unknown-kind", "target_id": 1}
			default:
				// 未挂载课程：CourseVisibleByID 为假（不是读不动）。
				body = map[string]any{"target_type": "course", "target_id": 999999}
			}
			code, got := doAndBody(t, f, f.studentToken, http.MethodPost, "/api/favorites", body)
			if code != http.StatusBadRequest {
				t.Fatalf("业务事实被默认面吞成了 %d（应 400）：%s", code, got)
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("句子换了（应含「%s」）：%s", tc.want, got)
			}
		})
	}
}
