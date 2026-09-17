// 具名分派表（ADR-0056 §11 / issue #1100）：描述符 → 具名方法的执行体表。
//
// 此前路由由 registerDictCRUDRoutes 在循环里注册匿名闭包，而 dictcrud_docs.go 的 37 个
// Admin* 方法只作 swag 注解宿主——全仓零引用、从不注册进路由、更不执行。注解与执行体
// 各写一份，改描述符 Path 或加字段两边静默分叉。
//
// 现在两者合成一份：注解宿主就是执行体，一个描述符的 create/update/delete 各对一个具名方法，
// 由本表（而不是闭包）决定注册哪条路由。全等锁在 dictcrud_docs_lock_test.go：
// 分派表 ↔ AllDescriptors() 一一对应、操作集一致、无孤儿，注解 @Router/@Success 与派生集合全等。
//
// 描述符仍由方法体按名取（descriptorByName，单点在 dictcrud_docs.go 尾部）——两者同源自
// AllDescriptors()，故注解宿主保持「一行转发」形态不变。
package handler

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/valuation/dictcrud"
)

// dictRoute 一个描述符的三条写路由具名方法；nil = 描述符未声明该操作。
type dictRoute struct {
	create gin.HandlerFunc
	update gin.HandlerFunc
	delete gin.HandlerFunc
}

// dictDispatch 具名分派表：描述符名（dictcrud.Descriptor.Name）→ 具名方法。
// 键必须与描述符名逐字一致；漏登记或多登记都在注册期 panic（见 registerDictCRUDRoutes），
// 锁测试再断一次「表 ↔ 描述符 ↔ 注解」三方一致。
func (h *ConfigHandler) dictDispatch() map[string]dictRoute {
	return map[string]dictRoute{
		"original_prices": {
			create: h.AdminCreateOriginalPrice,
			update: h.AdminUpdateOriginalPrice,
			delete: h.AdminDeleteOriginalPrice,
		},
		"region_coefficients": {
			create: h.AdminCreateRegionCoefficient,
			update: h.AdminUpdateRegionCoefficient,
			delete: h.AdminDeleteRegionCoefficient,
		},
		// coefficient_configs 只读 + 按 key 更新（U-only，无 create/delete）。
		"coefficient_configs": {
			update: h.AdminUpdateCoefficientConfig,
		},
		"brands": {
			create: h.AdminCreateBrand,
			update: h.AdminUpdateBrand,
			delete: h.AdminDeleteBrand,
		},
		"vehicle_types": {
			create: h.AdminCreateVehicleType,
			update: h.AdminUpdateVehicleType,
			delete: h.AdminDeleteVehicleType,
		},
		"series": {
			create: h.AdminCreateSeries,
			update: h.AdminUpdateSeries,
			delete: h.AdminDeleteSeries,
		},
		// 规格族（tonnages / mast_types / mast_heights / battery_types / transmission_types /
		// engine_types）描述符只声明 Create + Delete（「单字段唯一列 + C/D」，见 specs_crud_contract_test.go
		// 的 TestSpecsCrud_NoPutRoute），故没有 update 执行体；6 条 AdminUpdate* 注解是幻影
		// （见 dictcrud_docs_lock_test.go 的 phantomAnnotations）。
		"tonnages": {
			create: h.AdminCreateTonnage,
			delete: h.AdminDeleteTonnage,
		},
		"mast_types": {
			create: h.AdminCreateMastType,
			delete: h.AdminDeleteMastType,
		},
		"mast_heights": {
			create: h.AdminCreateMastHeight,
			delete: h.AdminDeleteMastHeight,
		},
		"battery_types": {
			create: h.AdminCreateBatteryType,
			delete: h.AdminDeleteBatteryType,
		},
		"transmission_types": {
			create: h.AdminCreateTransmissionType,
			delete: h.AdminDeleteTransmissionType,
		},
		"engine_types": {
			create: h.AdminCreateEngineType,
			delete: h.AdminDeleteEngineType,
		},
		"condition_ratings": {
			create: h.AdminCreateConditionRating,
			update: h.AdminUpdateConditionRating,
			delete: h.AdminDeleteConditionRating,
		},
	}
}

// registerDictCRUDRoutes 按描述符注册管理端 CRUD 路由，处理体取自具名分派表：
// Create.Fields 非空 → POST /<Path>；Update.Fields 非空 → PUT /<Path>/:<param>；Delete=true → DELETE /<Path>/:id。
//
// 注册期 fail-closed：描述符声明了操作而表里没有具名方法 → panic（漏登记是编程错误，
// 不能静默少注册一条路由）；表里登记了未声明的描述符 → panic。
func (h *ConfigHandler) registerDictCRUDRoutes(group *gin.RouterGroup, reg *dictcrud.Registry) {
	table := h.dictDispatch()
	for _, d := range reg.All() {
		routes, ok := table[d.Name]
		if !ok {
			panic("dictcrud: 描述符 " + d.Name + " 未在具名分派表（dictcrud_dispatch.go）中登记")
		}
		if len(d.Create.Fields) > 0 {
			group.POST("/"+dictcrud.RoutePath(d, dictcrud.OpCreate), requireDictRoute(d, dictcrud.OpCreate, routes.create))
		}
		if len(d.Update.Fields) > 0 {
			group.PUT("/"+dictcrud.RoutePath(d, dictcrud.OpUpdate), requireDictRoute(d, dictcrud.OpUpdate, routes.update))
		}
		if d.Delete {
			group.DELETE("/"+dictcrud.RoutePath(d, dictcrud.OpDelete), requireDictRoute(d, dictcrud.OpDelete, routes.delete))
		}
	}
	for name := range table {
		if _, ok := reg.Get(name); !ok {
			panic("dictcrud: 具名分派表登记了未声明的描述符 " + name + "（dictcrud.AllDescriptors() 漏了它？）")
		}
	}
}

// requireDictRoute 描述符声明了该操作就必须有具名方法（缺项 → panic，不静默少注册路由）。
func requireDictRoute(d dictcrud.Descriptor, op dictcrud.Op, fn gin.HandlerFunc) gin.HandlerFunc {
	if fn == nil {
		panic("dictcrud: 描述符 " + d.Name + " 的 " + op.Method() + " 路由没有具名方法（dictcrud_dispatch.go 漏登记）")
	}
	return fn
}
