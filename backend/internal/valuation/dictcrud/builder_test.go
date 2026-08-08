// 描述符核心纯逻辑测试：SQL 生成、参数构造、响应构造、描述符校验。
// 锁定的 SQL 与 repository/dict_region.go 迁移前逐字符一致（HTTP/DB 契约不变）。
package dictcrud

import (
	"strings"
	"testing"
)

func TestRegionDescriptor_Validate(t *testing.T) {
	if err := RegionCoefficientDescriptor.Validate(); err != nil {
		t.Fatalf("区域系数描述符非法: %v", err)
	}
}

func TestDescriptorValidate_Errors(t *testing.T) {
	base := RegionCoefficientDescriptor

	cases := []struct {
		name    string
		mutate  func(d *Descriptor)
		wantSub string
	}{
		{
			name:    "create 引用未声明字段",
			mutate:  func(d *Descriptor) { d.Create.Fields = append(d.Create.Fields, "ghost") },
			wantSub: "ghost",
		},
		{
			name:    "update 引用未声明字段",
			mutate:  func(d *Descriptor) { d.Update.Fields = append(d.Update.Fields, "ghost") },
			wantSub: "ghost",
		},
		{
			name: "create required 不在 create 字段集",
			mutate: func(d *Descriptor) {
				d.Create.Fields = []string{"province", "coefficient"}
				d.Create.Required = []string{"city"}
			},
			wantSub: "Required",
		},
		{
			name:    "bind required 不在字段集",
			mutate:  func(d *Descriptor) { d.Update.BindRequired = []string{"ghost"} },
			wantSub: "ghost",
		},
		{
			name:    "upsert 未声明唯一列",
			mutate:  func(d *Descriptor) { d.UniqueColumns = nil },
			wantSub: "UniqueColumns",
		},
		{
			name:    "唯一列不是已声明列",
			mutate:  func(d *Descriptor) { d.UniqueColumns = []string{"ghost"} },
			wantSub: "ghost",
		},
		{
			name: "默认值类型与字段类型不符",
			mutate: func(d *Descriptor) {
				for i := range d.Fields {
					if d.Fields[i].Name == "coefficient" {
						d.Fields[i].Default = "1980"
					}
				}
			},
			wantSub: "Default",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			d.Fields = append([]Field(nil), base.Fields...)
			tc.mutate(&d)
			err := d.Validate()
			if err == nil {
				t.Fatal("期望校验失败, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("错误信息 %q 未包含 %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestBuildInsertSQL_Region 区域系数 create SQL 与迁移前 dict_region.go 逐字符一致。
func TestBuildInsertSQL_Region(t *testing.T) {
	got := BuildInsertSQL(RegionCoefficientDescriptor)
	want := `INSERT INTO region_coefficients (province, city, coefficient) VALUES ($1, $2, $3) ON CONFLICT (province, city) DO UPDATE SET coefficient = EXCLUDED.coefficient RETURNING id`
	if got != want {
		t.Fatalf("insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildUpdateSQL_Region(t *testing.T) {
	got := BuildUpdateSQL(RegionCoefficientDescriptor)
	want := `UPDATE region_coefficients SET coefficient = $2 WHERE id = $1`
	if got != want {
		t.Fatalf("update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildDeleteSQL_Region(t *testing.T) {
	got := BuildDeleteSQL(RegionCoefficientDescriptor)
	want := `DELETE FROM region_coefficients WHERE id = $1`
	if got != want {
		t.Fatalf("delete SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildInsertArgs_Region(t *testing.T) {
	values := map[string]any{"province": "江苏", "city": "苏州", "coefficient": 1.02}
	got := BuildInsertArgs(RegionCoefficientDescriptor, values)
	if len(got) != 3 || got[0] != "江苏" || got[1] != "苏州" || got[2] != 1.02 {
		t.Fatalf("insert args 错误: %v", got)
	}
}

func TestBuildUpdateArgs_Region(t *testing.T) {
	got := BuildUpdateArgs(RegionCoefficientDescriptor, 7, map[string]any{"coefficient": 1.05})
	if len(got) != 2 || got[0] != int64(7) || got[1] != 1.05 {
		t.Fatalf("update args 错误: %v", got)
	}
}

// TestBuildCreateResult 响应形状：id + create 字段（与迁移前 RegionCoefficient 行一致）。
func TestBuildCreateResult_Region(t *testing.T) {
	values := map[string]any{"province": "江苏", "city": "苏州", "coefficient": 0.0}
	got := BuildCreateResult(RegionCoefficientDescriptor, 3, values)
	if got["id"] != int64(3) || got["province"] != "江苏" || got["city"] != "苏州" || got["coefficient"] != 0.0 {
		t.Fatalf("create 响应错误: %v", got)
	}
	if len(got) != 4 {
		t.Fatalf("create 响应应含 id+3 字段, got %d 项: %v", len(got), got)
	}
}

// TestBuildUpdateResult 响应形状：id + update 字段子集（coefficient 单字段）。
func TestBuildUpdateResult_Region(t *testing.T) {
	got := BuildUpdateResult(RegionCoefficientDescriptor, 3, map[string]any{"coefficient": 1.05})
	if len(got) != 2 || got["id"] != int64(3) || got["coefficient"] != 1.05 {
		t.Fatalf("update 响应错误: %v", got)
	}
}

func TestRegistry_Get(t *testing.T) {
	rg := NewRegistry(
		BrandDescriptor, VehicleTypeDescriptor, SeriesDescriptor,
		TonnageDescriptor, MastTypeDescriptor, MastHeightDescriptor,
		BatteryTypeDescriptor, TransmissionTypeDescriptor, EngineTypeDescriptor,
		ConditionRatingDescriptor, RegionCoefficientDescriptor,
		CoefficientConfigDescriptor, OriginalPriceDescriptor,
	)
	d, ok := rg.Get("region_coefficients")
	if !ok {
		t.Fatal("registry 未收录区域系数描述符")
	}
	if d.Path != "region-coefficients" {
		t.Fatalf("Path 错误: %s", d.Path)
	}
	if _, ok := rg.Get("brands"); !ok {
		t.Fatal("brands 已迁移，应在 registry 中")
	}
	if len(rg.All()) != 13 {
		t.Fatalf("registry 应有 13 个描述符, got %d", len(rg.All()))
	}
}

// TestAllDescriptors_Validate 全部描述符注册前校验通过（registry 构造即校验）。
func TestAllDescriptors_Validate(t *testing.T) {
	descriptors := []Descriptor{
		BrandDescriptor, VehicleTypeDescriptor, SeriesDescriptor,
		TonnageDescriptor, MastTypeDescriptor, MastHeightDescriptor,
		BatteryTypeDescriptor, TransmissionTypeDescriptor, EngineTypeDescriptor,
		ConditionRatingDescriptor, RegionCoefficientDescriptor,
		CoefficientConfigDescriptor, OriginalPriceDescriptor,
	}
	for _, d := range descriptors {
		if err := d.Validate(); err != nil {
			t.Errorf("描述符 %s 非法: %v", d.Name, err)
		}
	}
}

// TestBuildInsertSQL_OriginalPrice 原价 create SQL：7 字段复合唯一 DO UPDATE
// 只更新非唯一列（earliest_factory_year/original_price）+ 尾列 updated_at = NOW()。
func TestBuildInsertSQL_OriginalPrice(t *testing.T) {
	got := BuildInsertSQL(OriginalPriceDescriptor)
	want := `INSERT INTO original_prices (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm, earliest_factory_year, original_price) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (brand, vehicle_type, series, tonnage, config_type, mast_type, mast_height_mm) DO UPDATE SET earliest_factory_year = EXCLUDED.earliest_factory_year, original_price = EXCLUDED.original_price, updated_at = NOW() RETURNING id`
	if got != want {
		t.Fatalf("insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildUpdateSQL_OriginalPrice 原价 update SQL：全量字段 + 尾列 updated_at = NOW()。
func TestBuildUpdateSQL_OriginalPrice(t *testing.T) {
	got := BuildUpdateSQL(OriginalPriceDescriptor)
	want := `UPDATE original_prices SET brand = $2, vehicle_type = $3, series = $4, tonnage = $5, config_type = $6, mast_type = $7, mast_height_mm = $8, earliest_factory_year = $9, original_price = $10, updated_at = NOW() WHERE id = $1`
	if got != want {
		t.Fatalf("update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildUpdateKeySQL_Coefficient 系数配置按 key 更新 SQL：
// 尾列 updated_at = NOW() + WHERE key = $1 + RETURNING 整行。
func TestBuildUpdateKeySQL_Coefficient(t *testing.T) {
	got := BuildUpdateKeySQL(CoefficientConfigDescriptor)
	want := `UPDATE coefficient_configs SET value = $2, updated_at = NOW() WHERE key = $1 RETURNING id, key, value, description, updated_at`
	if got != want {
		t.Fatalf("keyed update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

func TestBuildUpdateKeyArgs_Coefficient(t *testing.T) {
	got := BuildUpdateKeyArgs(CoefficientConfigDescriptor, "lambda_electric", map[string]any{"value": 0.15})
	if len(got) != 2 || got[0] != "lambda_electric" || got[1] != 0.15 {
		t.Fatalf("keyed update args 错误: %v", got)
	}
}

// TestBuildUpdateSQL_Condition 车况评级 update SQL：label + base_coefficient（无 rating）。
func TestBuildUpdateSQL_Condition(t *testing.T) {
	got := BuildUpdateSQL(ConditionRatingDescriptor)
	want := `UPDATE condition_ratings SET label = $2, base_coefficient = $3 WHERE id = $1`
	if got != want {
		t.Fatalf("condition update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildUpdateSQL_Simple 品牌 update SQL：k_brand + is_active（无 name 子集）。
func TestBuildUpdateSQL_Simple(t *testing.T) {
	got := BuildUpdateSQL(BrandDescriptor)
	want := `UPDATE brands SET k_brand = $2, is_active = $3 WHERE id = $1`
	if got != want {
		t.Fatalf("brand update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
	got = BuildUpdateSQL(VehicleTypeDescriptor)
	want = `UPDATE vehicle_types SET name = $2, power_type = $3, earliest_factory_year = $4 WHERE id = $1`
	if got != want {
		t.Fatalf("vehicle_type update SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildInsertSQL_DoNothing 规格实体形状：单唯一列 + DO NOTHING。
func TestBuildInsertSQL_DoNothing(t *testing.T) {
	for _, d := range []Descriptor{
		TonnageDescriptor, MastTypeDescriptor, MastHeightDescriptor,
		BatteryTypeDescriptor, TransmissionTypeDescriptor, EngineTypeDescriptor,
	} {
		q := BuildInsertSQL(d)
		if !strings.HasSuffix(q, " DO NOTHING RETURNING id") {
			t.Fatalf("%s 应为 DO NOTHING 结尾: %s", d.Name, q)
		}
		if strings.Contains(q, "DO UPDATE") {
			t.Fatalf("%s 不应含 DO UPDATE: %s", d.Name, q)
		}
	}
	want := `INSERT INTO tonnages (value) VALUES ($1) ON CONFLICT (value) DO NOTHING RETURNING id`
	if got := BuildInsertSQL(TonnageDescriptor); got != want {
		t.Fatalf("tonnages insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildInsertSQL_DoUpdateExcludesUnique 非唯一列才是 SET 目标（brands 形状）。
func TestBuildInsertSQL_DoUpdateExcludesUnique(t *testing.T) {
	got := BuildInsertSQL(BrandDescriptor)
	want := `INSERT INTO brands (name, k_brand, is_active) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE SET k_brand = EXCLUDED.k_brand, is_active = EXCLUDED.is_active RETURNING id`
	if got != want {
		t.Fatalf("brands insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
	got = BuildInsertSQL(VehicleTypeDescriptor)
	want = `INSERT INTO vehicle_types (name, power_type, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (name) DO UPDATE SET power_type = EXCLUDED.power_type, earliest_factory_year = EXCLUDED.earliest_factory_year RETURNING id`
	if got != want {
		t.Fatalf("vehicle_types insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
	got = BuildInsertSQL(SeriesDescriptor)
	want = `INSERT INTO series (brand, name, earliest_factory_year) VALUES ($1, $2, $3) ON CONFLICT (brand, name) DO NOTHING RETURNING id`
	if got != want {
		t.Fatalf("series insert SQL 漂移:\n got: %s\nwant: %s", got, want)
	}
}

// TestBuildCreateResult_ResponseExtra 响应追加字段（original_prices updated_at 零值）。
func TestBuildCreateResult_ResponseExtra(t *testing.T) {
	values := map[string]any{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列",
		"tonnage": 3.0, "config_type": "标准", "mast_type": "标准门架",
		"mast_height_mm": 3000, "earliest_factory_year": 2000, "original_price": 100000.0,
	}
	got := BuildCreateResult(OriginalPriceDescriptor, 1, values)
	if len(got) != 11 {
		t.Fatalf("create 响应应含 id+9 字段+updated_at, got %d 项: %v", len(got), got)
	}
	if got["updated_at"] != "" {
		t.Fatalf("updated_at 应为空字符串（现状响应）, got %v", got["updated_at"])
	}
}

// TestApplyDefaults 零值 → 描述符默认值（vehicle_types 1980 / series、original_prices 2000）。
func TestApplyDefaults(t *testing.T) {
	values := map[string]any{"name": "电动叉车", "power_type": "electric", "earliest_factory_year": 0}
	ApplyDefaults(VehicleTypeDescriptor, values)
	if values["earliest_factory_year"] != 1980 {
		t.Fatalf("vehicle_types 默认值未应用: %v", values)
	}
	values = map[string]any{"brand": "林德", "name": "E系列", "earliest_factory_year": 0}
	ApplyDefaults(SeriesDescriptor, values)
	if values["earliest_factory_year"] != 2000 {
		t.Fatalf("series 默认值未应用: %v", values)
	}
	values = map[string]any{"earliest_factory_year": 0}
	ApplyDefaults(OriginalPriceDescriptor, values)
	if values["earliest_factory_year"] != 2000 {
		t.Fatalf("original_prices 默认值未应用: %v", values)
	}
	values = map[string]any{"name": "电动叉车", "power_type": "electric", "earliest_factory_year": 2005}
	ApplyDefaults(VehicleTypeDescriptor, values)
	if values["earliest_factory_year"] != 2005 {
		t.Fatalf("非零值被默认值覆盖: %v", values)
	}
}
