package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

// newQueryCtx 构造只带查询串的测试上下文（c.Query 读的就是请求 URL 的查询串）。
func newQueryCtx(query string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/probe?"+query, nil)
	return c
}

// newPathCtx 构造带路径参数的测试上下文（c.Param 读的是路由参数表）。
func newPathCtx(param string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Params = gin.Params{{Key: "id", Value: param}}
	return c
}

func intPtr(v int) *int { return &v }

// TestPathInt 钉住路径整型解析的判定表：非数字、0、负数一律 400（ADR-0065 决策 1），
// 且 400 文案逐字透传调用方给的那句。
func TestPathInt(t *testing.T) {
	cases := []struct {
		name    string
		param   string
		want    int
		wantErr bool
	}{
		{name: "正整数通过", param: "12", want: 12},
		{name: "零是解析层错误", param: "0", wantErr: true},
		{name: "负数是解析层错误", param: "-1", wantErr: true},
		{name: "非数字是解析层错误", param: "abc", wantErr: true},
		{name: "缺失是解析层错误", param: "", wantErr: true},
		{name: "前导空格不放过", param: " 12", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PathInt(newPathCtx(tc.param), "id", "ID无效")
			assertParseResult(t, "PathInt", tc.param, got, err, tc.want, tc.wantErr)
		})
	}
}

// TestPathInt64 与 PathInt 判定逐字相同（两枚是仅有的路径整数解析点）。
func TestPathInt64(t *testing.T) {
	cases := []struct {
		name    string
		param   string
		want    int64
		wantErr bool
	}{
		{name: "正整数通过", param: "12", want: 12},
		{name: "超出 int32 仍通过", param: "9999999999", want: 9999999999},
		{name: "零是解析层错误", param: "0", wantErr: true},
		{name: "负数是解析层错误", param: "-1", wantErr: true},
		{name: "非数字是解析层错误", param: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PathInt64(newPathCtx(tc.param), "id", "ID无效")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("PathInt64(%q) 应报错，却得 %d", tc.param, got)
				}
				assertBadRequest(t, err, "ID无效")
				return
			}
			if err != nil {
				t.Fatalf("PathInt64(%q) 意外报错: %v", tc.param, err)
			}
			if got != tc.want {
				t.Fatalf("PathInt64(%q) = %d, want %d", tc.param, got, tc.want)
			}
		})
	}
}

// TestQueryIntPtr 钉住「任意整数」那一枚：0 与负数**原样透传**，非法/缺失才是 nil。
func TestQueryIntPtr(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  *int
	}{
		{name: "缺失是 nil", query: "", want: nil},
		{name: "正整数通过", query: "min_wrong_count=5", want: intPtr(5)},
		{name: "零透传", query: "min_wrong_count=0", want: intPtr(0)},
		{name: "负数透传", query: "min_wrong_count=-3", want: intPtr(-3)},
		{name: "非数字是 nil", query: "min_wrong_count=abc", want: nil},
		{name: "小数是 nil", query: "min_wrong_count=5.0", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := QueryIntPtr(newQueryCtx(tc.query), "min_wrong_count")
			assertPtr(t, "QueryIntPtr", tc.query, got, tc.want)
		})
	}
}

// TestQueryIDPtr 钉住 ID 型那一枚：非法、缺失、0 与负数**统一 nil**（id>0 守卫的单点）。
func TestQueryIDPtr(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  *int
	}{
		{name: "缺失是 nil", query: "", want: nil},
		{name: "正整数通过", query: "tag_id=7", want: intPtr(7)},
		{name: "零是 nil", query: "tag_id=0", want: nil},
		{name: "负数是 nil", query: "tag_id=-3", want: nil},
		{name: "非数字是 nil", query: "tag_id=abc", want: nil},
		{name: "小数是 nil", query: "tag_id=7.5", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := QueryIDPtr(newQueryCtx(tc.query), "tag_id")
			assertPtr(t, "QueryIDPtr", tc.query, got, tc.want)
		})
	}
}

// TestQueryIntDefault 钉住分页/条数那一枚：缺失或非数字回默认值，0 与负数原样透传（钳制属服务层）。
func TestQueryIntDefault(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int
	}{
		{name: "缺失回默认值", query: "", want: 20},
		{name: "非数字回默认值", query: "page_size=abc", want: 20},
		{name: "正整数通过", query: "page_size=50", want: 50},
		{name: "零透传", query: "page_size=0", want: 0},
		{name: "负数透传", query: "page_size=-4", want: -4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryIntDefault(newQueryCtx(tc.query), "page_size", 20); got != tc.want {
				t.Fatalf("QueryIntDefault(%q, 20) = %d, want %d", tc.query, got, tc.want)
			}
		})
	}
}

// TestPositiveID 钉住纯字符串形态那一枚：非法或 <=0 回 (0,false)，把「缺失 vs 非法」的分流留给调用方。
func TestPositiveID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
		ok   bool
	}{
		{name: "正整数通过", in: "5", want: 5, ok: true},
		{name: "前导零仍通过", in: "007", want: 7, ok: true},
		{name: "空串是缺失", in: "", ok: false},
		{name: "零不通过", in: "0", ok: false},
		{name: "负数不通过", in: "-1", ok: false},
		{name: "尾随字符不通过", in: "12abc", ok: false},
		{name: "小数不通过", in: "1.5", ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PositiveID(tc.in)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("PositiveID(%q) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// assertParseResult 断言 (int, error) 形态助手的结果：wantErr 时必须是 400 ParseError 且文案透传。
func assertParseResult(t *testing.T, fn, in string, got int, err error, want int, wantErr bool) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Fatalf("%s(%q) 应报错，却得 %d", fn, in, got)
		}
		assertBadRequest(t, err, "ID无效")
		return
	}
	if err != nil {
		t.Fatalf("%s(%q) 意外报错: %v", fn, in, err)
	}
	if got != want {
		t.Fatalf("%s(%q) = %d, want %d", fn, in, got, want)
	}
}

// assertBadRequest 断言解析失败落成 400 ParseError 且文案逐字透传。
func assertBadRequest(t *testing.T, err error, msg string) {
	t.Helper()
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("错误应为 *ParseError，got %#v", err)
	}
	if pe.Status != http.StatusBadRequest || pe.Message != msg {
		t.Fatalf("ParseError = {%d,%q}, want {400,%q}", pe.Status, pe.Message, msg)
	}
}

// assertPtr 断言可选整型查询参数的返回值（nil 与值都要逐字对上）。
func assertPtr(t *testing.T, fn, in string, got, want *int) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("%s(%q) = %v, want %v", fn, in, fmtPtr(got), fmtPtr(want))
		}
		return
	}
	if *got != *want {
		t.Fatalf("%s(%q) = %d, want %d", fn, in, *got, *want)
	}
}

func fmtPtr(p *int) string {
	if p == nil {
		return "nil"
	}
	return strconv.Itoa(*p)
}
