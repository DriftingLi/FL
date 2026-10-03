package textx

import "testing"

func TestSnippet(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"短于上限原样返回", "短文本", 10, "短文本"},
		{"等于上限原样返回", "正好六个字", 6, "正好六个字"},
		{"超上限按 rune 截断加省略号", "一二三四五六七八", 4, "一二三四…"},
		{"空串", "", 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Snippet(tc.in, tc.n); got != tc.want {
				t.Fatalf("Snippet(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}
