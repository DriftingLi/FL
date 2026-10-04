package valuation

import (
	"reflect"
	"testing"
)

// 派生单点自己的用例：响应字段表 = 响应构造规则（builder.go）的形状视图。
// 锁的另一半（37 条注解与它全等）在 handler/dictcrud_docs_lock_test.go。

func fields(pairs ...string) []DictResponseField {
	out := make([]DictResponseField, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, DictResponseField{Name: pairs[i], Type: pairs[i+1]})
	}
	return out
}

func TestResponseFields(t *testing.T) {
	cases := []struct {
		name string
		d    DictDescriptor
		op   DictOp
		want []DictResponseField
	}{
		{
			name: "brands/create：id + 创建字段（声明序）",
			d:    BrandDescriptor,
			op:   DictOpCreate,
			want: fields("id", "integer", "name", "string", "k_brand", "number", "is_active", "boolean"),
		},
		{
			name: "brands/update：id + 更新子集（非对称子集不含 name）",
			d:    BrandDescriptor,
			op:   DictOpUpdate,
			want: fields("id", "integer", "k_brand", "number", "is_active", "boolean"),
		},
		{
			name: "brands/delete：恒为 {id}",
			d:    BrandDescriptor,
			op:   DictOpDelete,
			want: fields("id", "integer"),
		},
		{
			name: "original_prices/create：尾部追加 ResponseExtra(updated_at)",
			d:    OriginalPriceDescriptor,
			op:   DictOpCreate,
			want: fields("id", "integer", "brand", "string", "vehicle_type", "string", "series", "string",
				"tonnage", "number", "config_type", "string", "mast_type", "string", "mast_height_mm", "integer",
				"earliest_factory_year", "integer", "original_price", "number", "updated_at", "string"),
		},
		{
			name: "coefficient_configs/update：全行响应（ResponseColumns，id 前置）",
			d:    CoefficientConfigDescriptor,
			op:   DictOpUpdate,
			want: fields("id", "integer", "key", "string", "value", "number", "description", "string", "updated_at", "string"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DictResponseFields(tc.d, tc.op); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DictResponseFields = %v，期望 %v", got, tc.want)
			}
		})
	}
}

// 路由形态与注解形态共用同一个参数名派生：UpdateKeyField 改了就两边一起改。
func TestRoutePathParamDerivedFromDescriptor(t *testing.T) {
	if got := DictRoutePath(CoefficientConfigDescriptor, DictOpUpdate); got != "coefficient-configs/:key" {
		t.Fatalf("DictRoutePath(keyed update) = %q", got)
	}
	if got := DictSwaggerPath(CoefficientConfigDescriptor, DictOpUpdate); got != "coefficient-configs/{key}" {
		t.Fatalf("DictSwaggerPath(keyed update) = %q", got)
	}
	if got := DictRoutePath(BrandDescriptor, DictOpUpdate); got != "brands/:id" {
		t.Fatalf("DictRoutePath(update) = %q", got)
	}
	if got := DictSwaggerPath(BrandDescriptor, DictOpCreate); got != "brands" {
		t.Fatalf("DictSwaggerPath(create) = %q", got)
	}
}
