package coerce

import "testing"

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{float64(3.14), 3.14},
		{float32(2.5), 2.5},
		{int(42), 42},
		{int64(100), 100},
		{int32(7), 7},
		{"3.14", 3.14},
		{"42", 42},
		{true, 1},
		{false, 0},
		{"invalid", 0},
		{nil, 0},
		{[]int{1, 2}, 0}, // 不支持的类型
	}
	for _, tt := range tests {
		if got := ToFloat(tt.input); got != tt.want {
			t.Errorf("ToFloat(%v) = %v，期望 %v", tt.input, got, tt.want)
		}
	}
}

func TestClampFloat(t *testing.T) {
	if got := ClampFloat(5, 0, 10); got != 5 {
		t.Errorf("ClampFloat(5,0,10) = %v，期望 5", got)
	}
	if got := ClampFloat(-1, 0, 10); got != 0 {
		t.Errorf("ClampFloat(-1,0,10) = %v，期望 0", got)
	}
	if got := ClampFloat(15, 0, 10); got != 10 {
		t.Errorf("ClampFloat(15,0,10) = %v，期望 10", got)
	}
	if got := ClampFloat(0, 0, 10); got != 0 {
		t.Errorf("ClampFloat(0,0,10) = %v，期望 0", got)
	}
	if got := ClampFloat(10, 0, 10); got != 10 {
		t.Errorf("ClampFloat(10,0,10) = %v，期望 10", got)
	}
}

func TestParseFloat(t *testing.T) {
	if got, err := ParseFloat("3.14"); err != nil || got != 3.14 {
		t.Errorf("ParseFloat('3.14') = %v, err=%v", got, err)
	}
	if got, err := ParseFloat("-5.5"); err != nil || got != -5.5 {
		t.Errorf("ParseFloat('-5.5') = %v, err=%v", got, err)
	}
	for _, s := range []string{"invalid", "", "abc12"} {
		if got, err := ParseFloat(s); err == nil {
			t.Errorf("ParseFloat(%q) 期望报错，得到 %v", s, got)
		}
	}
}

func TestParseInt(t *testing.T) {
	if got, err := ParseInt("42"); err != nil || got != 42 {
		t.Errorf("ParseInt('42') = %v, err=%v", got, err)
	}
	if got, err := ParseInt("-7"); err != nil || got != -7 {
		t.Errorf("ParseInt('-7') = %v, err=%v", got, err)
	}
	for _, s := range []string{"invalid", "", "3.14"} {
		if got, err := ParseInt(s); err == nil {
			t.Errorf("ParseInt(%q) 期望报错，得到 %v", s, got)
		}
	}
}

func TestIntPtr(t *testing.T) {
	p := IntPtr(42)
	if p == nil || *p != 42 {
		t.Errorf("IntPtr(42) 失败")
	}
}

func TestFloatPtr(t *testing.T) {
	p := FloatPtr(3.14)
	if p == nil || *p != 3.14 {
		t.Errorf("FloatPtr(3.14) 失败")
	}
}
