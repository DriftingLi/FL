// ReadSpec 读面 shape-lock 测试（ADR-0013 候选 1 门禁）：
// 锁定每个简单单表读由 ReadSpec 生成的 SELECT SQL 与深化前手写 SQL 字节级一致。
// SQL 的列顺序/列名/WHERE/ORDER BY 一旦漂移，JSON 响应 shape（key 集合、字段顺序、
// 筛选/排序语义）即漂移——本测试把这些锁成常量，深化后不可复现漂移。
package repository

import (
	"encoding/json"
	"testing"
)

// TestReadSpec_SQLBytesMatchLegacy 每个简单读的生成 SQL 与深化前手写 SQL 一致。
func TestReadSpec_SQLBytesMatchLegacy(t *testing.T) {
	cases := []struct {
		name string
		spec ReadSpec
		want string
	}{
		{"brands:list", readSpecBrandsList, "SELECT id, name, k_brand, is_active FROM brands ORDER BY k_brand DESC, name ASC"},
		{"brands:get", readSpecBrandsGet, "SELECT id, name, k_brand, is_active FROM brands WHERE name = $1"},
		{"condition:list", readSpecConditionList, "SELECT id, rating, label, base_coefficient FROM condition_ratings ORDER BY base_coefficient DESC"},
		{"condition:get", readSpecConditionGet, "SELECT id, rating, label, base_coefficient FROM condition_ratings WHERE rating = $1"},
		{"vt:list", readSpecVtList, "SELECT id, name, power_type, earliest_factory_year FROM vehicle_types ORDER BY id ASC"},
		{"vt:get", readSpecVtGet, "SELECT id, name, power_type, earliest_factory_year FROM vehicle_types WHERE name = $1"},
		{"series:list", readSpecSeriesList, "SELECT id, brand, name, earliest_factory_year FROM series ORDER BY id ASC"},
		{"tonnages:list", readSpecTonnagesList, "SELECT id, value FROM tonnages ORDER BY value ASC"},
		{"mast_types:list", readSpecMastTypesList, "SELECT id, name FROM mast_types ORDER BY id ASC"},
		{"mast_heights:list", readSpecMastHeightsList, "SELECT id, value_mm FROM mast_heights ORDER BY value_mm ASC"},
		{"battery_types:list", readSpecBatteryTypesList, "SELECT id, name FROM battery_types ORDER BY id ASC"},
		{"transmission_types:list", readSpecTransmissionTypesList, "SELECT id, name FROM transmission_types ORDER BY id ASC"},
		{"engine_types:list", readSpecEngineTypesList, "SELECT id, name FROM engine_types ORDER BY id ASC"},
		{"region:list", readSpecRegionList, "SELECT id, province, city, coefficient FROM region_coefficients ORDER BY id ASC"},
		{"region:provinces", readSpecRegionProvinces, "SELECT DISTINCT province FROM region_coefficients ORDER BY province ASC"},
		{"region:cities", readSpecRegionCities, "SELECT city FROM region_coefficients WHERE province = $1 ORDER BY city ASC"},
		{"region:get", readSpecRegionGet, "SELECT id, province, city, coefficient FROM region_coefficients WHERE province = $1 AND city = $2"},
		{"coef:list", readSpecCoefList, "SELECT id, key, value, description, updated_at FROM coefficient_configs ORDER BY key ASC"},
		{"coef:get", readSpecCoefGet, "SELECT id, key, value, description, updated_at FROM coefficient_configs WHERE key = $1"},
	}
	for _, c := range cases {
		if got := c.spec.selectSQL(); got != c.want {
			t.Errorf("%s SQL 漂移\n got: %s\nwant: %s", c.name, got, c.want)
		}
	}
}

// TestReadSpec_ScanShapeBytesMatch 反射扫描产出的 JSON 与 typed struct 自然序列化一致
// （shape-lock：字段名集合、顺序、零值语义深化前后不变）。
func TestReadSpec_ScanShapeBytesMatch(t *testing.T) {
	// 伪造一个按列顺序吐值的 Scanner，验证反射 scan 到 Brand 后 JSON 与期望一致。
	fake := &fakeScanner{values: []any{int64(1), "林德", 1.15, true}}
	got, err := scanInto[Brand](readSpecBrandsList, fake)
	if err != nil {
		t.Fatalf("scan 失败: %v", err)
	}
	gotJSON, _ := json.Marshal(got)
	want := `{"id":1,"name":"林德","k_brand":1.15,"is_active":true}`
	if string(gotJSON) != want {
		t.Errorf("Brand scan JSON 漂移\n got: %s\nwant: %s", gotJSON, want)
	}
}

// fakeScanner 实现 Scan(dest ...any) error，按 values 顺序写入。
type fakeScanner struct {
	values []any
}

func (f *fakeScanner) Scan(dest ...any) error {
	for i, d := range dest {
		if i >= len(f.values) {
			break
		}
		switch dp := d.(type) {
		case *int64:
			*dp = f.values[i].(int64)
		case *int:
			*dp = f.values[i].(int)
		case *string:
			*dp = f.values[i].(string)
		case *float64:
			*dp = f.values[i].(float64)
		case *bool:
			*dp = f.values[i].(bool)
		}
	}
	return nil
}
