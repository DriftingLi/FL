package dictcrud

import (
	"reflect"
	"testing"
)

// 派生单点自己的用例：响应字段表 = 响应构造规则（builder.go）的形状视图。
// 锁的另一半（37 条注解与它全等）在 handler/dictcrud_docs_lock_test.go。

func fields(pairs ...string) []ResponseField {
	out := make([]ResponseField, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, ResponseField{Name: pairs[i], Type: pairs[i+1]})
	}
	return out
}

func TestResponseFields(t *testing.T) {
	cases := []struct {
		name string
		d    Descriptor
		op   Op
		want []ResponseField
	}{
		{
			name: "brands/create：id + 创建字段（声明序）",
			d:    BrandDescriptor,
			op:   OpCreate,
			want: fields("id", "integer", "name", "string", "k_brand", "number", "is_active", "boolean"),
		},
		{
			name: "brands/update：id + 更新子集（非对称子集不含 name）",
			d:    BrandDescriptor,
			op:   OpUpdate,
			want: fields("id", "integer", "k_brand", "number", "is_active", "boolean"),
		},
		{
			name: "brands/delete：恒为 {id}",
			d:    BrandDescriptor,
			op:   OpDelete,
			want: fields("id", "integer"),
		},
		{
			name: "original_prices/create：尾部追加 ResponseExtra(updated_at)",
			d:    OriginalPriceDescriptor,
			op:   OpCreate,
			want: fields("id", "integer", "brand", "string", "vehicle_type", "string", "series", "string",
				"tonnage", "number", "config_type", "string", "mast_type", "string", "mast_height_mm", "integer",
				"earliest_factory_year", "integer", "original_price", "number", "updated_at", "string"),
		},
		{
			name: "coefficient_configs/update：全行响应（ResponseColumns，id 前置）",
			d:    CoefficientConfigDescriptor,
			op:   OpUpdate,
			want: fields("id", "integer", "key", "string", "value", "number", "description", "string", "updated_at", "string"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResponseFields(tc.d, tc.op); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ResponseFields = %v，期望 %v", got, tc.want)
			}
		})
	}
}

// 路由形态与注解形态共用同一个参数名派生：UpdateKeyField 改了就两边一起改。
func TestRoutePathParamDerivedFromDescriptor(t *testing.T) {
	if got := RoutePath(CoefficientConfigDescriptor, OpUpdate); got != "coefficient-configs/:key" {
		t.Fatalf("RoutePath(keyed update) = %q", got)
	}
	if got := SwaggerPath(CoefficientConfigDescriptor, OpUpdate); got != "coefficient-configs/{key}" {
		t.Fatalf("SwaggerPath(keyed update) = %q", got)
	}
	if got := RoutePath(BrandDescriptor, OpUpdate); got != "brands/:id" {
		t.Fatalf("RoutePath(update) = %q", got)
	}
	if got := SwaggerPath(BrandDescriptor, OpCreate); got != "brands" {
		t.Fatalf("SwaggerPath(create) = %q", got)
	}
}
