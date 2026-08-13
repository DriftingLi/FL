package service

import (
	"math"
	"testing"
)

// TestDecayAnchor_Aged 年龄>0：锚点 = Kt_adj^(1/age)，曲线从当前残值精确出发。
func TestDecayAnchor_Aged(t *testing.T) {
	ktAdj, kh, kb, age := 0.81, 1.2, 1.0, 5
	got := DecayAnchor(ktAdj, kh, kb, age, 0.12)
	want := math.Pow(ktAdj, 1/float64(age))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("DecayAnchor = %v, want %v", got, want)
	}
}

// TestDecayAnchor_NewVehicle age=0（新车）：锚点 = e^(-λ·Kh/Kb)。
func TestDecayAnchor_NewVehicle(t *testing.T) {
	got := DecayAnchor(1.0, 1.2, 1.0, 0, 0.12)
	want := math.Exp(-0.12 * (1.2 / 1.0))
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("DecayAnchor = %v, want %v", got, want)
	}
}

// TestDecayAnchor_KBrandZero Kb<=0：锚点 = e^(-λ)（无 Kh/Kb 修正项）。
func TestDecayAnchor_KBrandZero(t *testing.T) {
	got := DecayAnchor(0, 1.2, 0, 0, 0.12)
	want := math.Exp(-0.12)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("DecayAnchor = %v, want %v", got, want)
	}
}
