// ADR-0065 批⑤（负数/零 path id 跨域同判）的行为锁。
//
// 开工前这一件事实「路径上的整数 id 不是正整数」有**三个**实现形状：
//   - `pathInt64`（endpoint.go）：`err != nil || v <= 0` ⇒ 400 —— 已经是对的；
//   - `pathInt`（endpoint.go）：只看 `strconv.Atoi` 的 err ⇒ 非数字 400，但 **0 与负数放行**
//     到 service，于是 `GET /course/0` 对外答 404「课程不存在」（拿非法输入冒充不存在的资源），
//     而用户/讲师面因 service 有 `id <= 0` guard 答 400、用的又是另一句文案（「用户 ID 非法」）；
//   - 37 处自定义实现：36 处裸 `strconv.(Atoi|ParseInt)` 读 `c.Param`（散在 10 个文件），另 1 处
//     把路径参数喂给查询侧守卫 `requiredPositiveID`。其中 `question_interaction.go` 那 7 处
//     **整个丢弃解析错误**且无 `<= 0` ⇒ 非数字 id 以 `0` 进 service：6 个端点经
//     `renderOutOfPoolQuestion` 答 404「题目不存在」，`GET /questions/:question_id/knowledge`
//     更答 **200 + 空数组**（下面那三行就是钉这一格的）。
//
// 对比之下，**查询**参数侧早就有单点（`queryIDPtr` / `requiredPositiveID`，helpers.go 的注释自称
// 「id>0 守卫的单点实现」）⇒ 同一类事实在两个入口上一个是单点、一个是三份实现，
// 这正是本波判据要抓的形状。
//
// 两道断言各有分工：
//   - 单元级：钉住 helper 自己的判定表（含 `-1/0/空串`），它不需要数据库，跑得快；
//   - HTTP 级：钉住「与同一端点的非数字档**同码且同文案**」——不硬编码文案，而是拿 `abc` 那一次的
//     响应当基准。基准取自同端点同参数，所以改文案不会误红，而「非法输入被咽成 404 / 咽成 200 空集」必红。
package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestPathIntRejectsNonPositive 解析层的判定表：正整数放行，非数字/0/负数一律 400 且带本端点文案。
func TestPathIntRejectsNonPositive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		raw    string
		wantOK bool
	}{
		{"7", true},
		{"1", true},
		{"0", false},
		{"-1", false},
		{"-5", false},
		{"abc", false},
		{"", false},
		{" 7", false}, // 带空白不是合法 id，Atoi 自己会拒
	} {
		t.Run("raw="+tc.raw, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Params = gin.Params{{Key: "id", Value: tc.raw}}

			v, err := pathInt(c, "id", "ID无效")
			if tc.wantOK {
				if err != nil {
					t.Fatalf("合法值 %q 被拒：%v", tc.raw, err)
				}
				if tc.raw != fmt.Sprint(v) {
					t.Fatalf("合法值 %q 解析成了 %d", tc.raw, v)
				}
				return
			}
			if err == nil {
				t.Fatalf("非正整数 %q 未被拒（v=%d）⇒ 它会继续走到 service，跨域档位就此分叉", tc.raw, v)
			}
			pe, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("%q 的错误类型不是 *ParseError：%T（默认错误面会因此而不是 400）", tc.raw, err)
			}
			if pe.Status != http.StatusBadRequest || pe.Message != "ID无效" {
				t.Fatalf("%q 应为 400 + 本端点文案，实得 %d %q", tc.raw, pe.Status, pe.Message)
			}
		})
	}
}

// nonPositiveFaceCase 一个走 pathInt 的端点（路径含 %s 处放坏的 id）。
type nonPositiveFaceCase struct {
	name    string
	who     string // admin | student（决定用哪枚 token）
	method  string
	pathFmt string
	body    any
}

// nonPositiveFaces 每域点一个代表，但**域要不同**（本批判的是跨域同判）：课程读面 / 管理端课程 /
// 管理端章节 / 目录实体 / 精选内容 / 讲师口令面 / 用户处置面。
// 后两枚（hrwai-users、tutor）今天已经是 400（service 层有 guard），把它们放进来是为了钉住
// 「补齐 pathInt 之后这两域不改变」——没有这两行，本批就是一堆改判而没有反证。
// 末三行是 `question_interaction.go`：那 7 处原本丢弃解析错误，非数字 id 以 0 进 service，
// 对外分别是 404「题目不存在」与 200 + 空数组 —— 本批唯一的改判证据就在这三行里。
var nonPositiveFaces = []nonPositiveFaceCase{
	{name: "GET /course/:course_id", who: "student", method: http.MethodGet, pathFmt: "/api/course/%s"},
	{name: "GET /admin/course/:course_id", who: "admin", method: http.MethodGet, pathFmt: "/api/admin/course/%s"},
	{name: "DELETE /admin/course/:course_id", who: "admin", method: http.MethodDelete, pathFmt: "/api/admin/course/%s"},
	{name: "PUT /admin/chapter/:chapter_id", who: "admin", method: http.MethodPut, pathFmt: "/api/admin/chapter/%s", body: map[string]any{"title": "同判改章"}},
	{name: "PUT /admin/specialty/:id", who: "admin", method: http.MethodPut, pathFmt: "/api/admin/specialty/%s", body: map[string]any{"code": "SAME", "name": "同判方向"}},
	{name: "GET /admin/featured-content/:id", who: "admin", method: http.MethodGet, pathFmt: "/api/admin/featured-content/%s"},
	{name: "PUT /admin/hrwai-users/:id/status", who: "admin", method: http.MethodPut, pathFmt: "/api/admin/hrwai-users/%s/status", body: map[string]any{"status": 0}},
	{name: "PUT /admin/tutor/:id/password", who: "admin", method: http.MethodPut, pathFmt: "/api/admin/tutor/%s/password", body: map[string]any{"password": "validpass123"}},
	{name: "GET /questions/:question_id/comments", who: "student", method: http.MethodGet, pathFmt: "/api/questions/%s/comments"},
	{name: "GET /questions/:question_id/knowledge", who: "student", method: http.MethodGet, pathFmt: "/api/questions/%s/knowledge"},
	{name: "DELETE /questions/comments/:comment_id", who: "student", method: http.MethodDelete, pathFmt: "/api/questions/comments/%s"},
}

// TestNonPositivePathIDMatchesNonNumericFace 同一端点上「0 / 负数」与「非数字」必须同码同文案。
func TestNonPositivePathIDMatchesNonNumericFace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, f := range nonPositiveFaces {
		t.Run(f.name, func(t *testing.T) {
			r, db, adminToken := newAdminContractEnv(t)
			tok := adminToken
			if f.who == "student" {
				tok = tokenFor(t, db, "student")
			}

			// 基准：非数字档（今天就已经是 400，形状不受本批影响）。
			base := doWithToken(t, r, tok, f.method, fmt.Sprintf(f.pathFmt, "not-a-number"), f.body)
			if base.Code != http.StatusBadRequest {
				t.Fatalf("基准档（非数字）在 %s 上不是 400，实得 %d：%s", f.name, base.Code, base.Body.String())
			}
			baseBody := base.Body.String()

			for _, bad := range []string{"0", "-1", "-99999"} {
				rec := doWithToken(t, r, tok, f.method, fmt.Sprintf(f.pathFmt, bad), f.body)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("%s 的 id=%s 实得 %d（基准 400），body=%s ⇒ 非正整数被放行到 service，"+
						"这一域与基准域就此不同判", f.name, bad, rec.Code, rec.Body.String())
				}
				if got := rec.Body.String(); got != baseBody {
					t.Fatalf("%s 的 id=%s 与「非数字」文案不一致：\n  基准 %s\n  实得 %s", f.name, bad, baseBody, got)
				}
			}
		})
	}
}
