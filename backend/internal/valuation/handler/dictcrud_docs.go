// 管理端 CRUD 端点的注解宿主 + 执行体（ADR-0048 片九 / issue #967；具名分派表见 ADR-0056 §11 / #1100）。
//
// 估值管理端写路由由描述符注册表驱动（dictcrud 包，见 ADR-0008）：注解**无法写在循环闭包上**
// （swag 只认函数级注释），故每条路由一个具名方法，注解块放在方法上，方法体一行转发到
// 描述符驱动骨架（先例：internal/api/forum.go 的 AdminGetTopic）。
//
// 第十一波起这些方法不再只是注解壳：路由注册由具名分派表（dictcrud_dispatch.go）指向它们，
// 注解宿主与执行体合成一份。判据仍是**只加注解、不改任何响应形状** —— 方法体恒为一行转发；
// 每个实体的 create/update/delete 均由同一条骨架构造响应（dictcrud.BuildCreateResult /
// BuildUpdateResult / BuildUpdateKeySQL / gin.H{"id"}），故响应形状对全部实体同构：
//
//	{id} ∪ 该操作的声明字段。swag 的 object{} 是字段并集描述（实际字段随描述符变化）。
//
// 注解的 path/method/字段表由描述符派生（dictcrud.RoutePath / SwaggerPath / ResponseFields），
// 与分派表、AllDescriptors() 的全等锁在 dictcrud_docs_lock_test.go —— 改描述符不改注解即红。
//
// 实测（#1100）：37 条注解里 6 条是**幻影** —— 规格族（tonnages / mast_types / mast_heights /
// battery_types / transmission_types / engine_types）的描述符只声明 Create + Delete（无 PUT，
// specs_crud_contract_test.go 的 TestSpecsCrud_NoPutRoute 断言 404），这 6 条 AdminUpdate*
// 注解与域声明表却都写了 PUT。逐条登记在 dictcrud_docs_lock_test.go 的 phantomAnnotations，待另票处置。
package handler

import (
	"github.com/gin-gonic/gin"

	"forklift-training/internal/valuation/dictcrud"
)

// =====================================================
// original-prices（原价记录）
// =====================================================

// AdminCreateOriginalPrice 新增原价记录
// @Summary 新增原价记录
// @Description 新增基准原价记录（brand/vehicle_type/series 必填，original_price>0）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "原价记录字段" example({"brand":"合力","vehicle_type":"电动平衡重式","series":"H3","tonnage":3,"config_type":"无","mast_type":"无","mast_height_mm":0,"earliest_factory_year":2000,"original_price":18.5})
// @Success 200 {object} response.R{data=object{id=integer,brand=string,vehicle_type=string,series=string,tonnage=number,config_type=string,mast_type=string,mast_height_mm=integer,earliest_factory_year=integer,original_price=number,updated_at=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/original-prices [post]
func (h *ConfigHandler) AdminCreateOriginalPrice(c *gin.Context) {
	h.createDict(c, originalPriceDescriptor())
}

// AdminUpdateOriginalPrice 更新原价记录
// @Summary 更新原价记录
// @Description 按 id 更新基准原价记录；响应为 {id} ∪ 更新字段 ∪ {updated_at}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "原价记录 ID"
// @Param body body object true "原价记录字段" example({"brand":"合力","vehicle_type":"电动平衡重式","series":"H3","tonnage":3,"original_price":17.8})
// @Success 200 {object} response.R{data=object{id=integer,brand=string,vehicle_type=string,series=string,tonnage=number,config_type=string,mast_type=string,mast_height_mm=integer,earliest_factory_year=integer,original_price=number,updated_at=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "原价记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/original-prices/{id} [put]
func (h *ConfigHandler) AdminUpdateOriginalPrice(c *gin.Context) {
	h.updateDict(c, originalPriceDescriptor())
}

// AdminDeleteOriginalPrice 删除原价记录
// @Summary 删除原价记录
// @Description 按 id 删除基准原价记录；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "原价记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "原价记录不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/original-prices/{id} [delete]
func (h *ConfigHandler) AdminDeleteOriginalPrice(c *gin.Context) {
	h.deleteDict(c, originalPriceDescriptor())
}

// =====================================================
// region-coefficients（区域系数）
// =====================================================

// AdminCreateRegionCoefficient 新增区域系数
// @Summary 新增区域系数
// @Description 新增区域系数（province/city 必填）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "区域系数字段" example({"province":"江苏","city":"南京","coefficient":1.02})
// @Success 200 {object} response.R{data=object{id=integer,province=string,city=string,coefficient=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/region-coefficients [post]
func (h *ConfigHandler) AdminCreateRegionCoefficient(c *gin.Context) {
	h.createDict(c, regionCoefficientDescriptor())
}

// AdminUpdateRegionCoefficient 更新区域系数
// @Summary 更新区域系数
// @Description 按 id 更新区域系数（coefficient 单字段子集）；响应为 {id,coefficient}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "区域系数 ID"
// @Param body body object true "区域系数字段" example({"coefficient":1.02})
// @Success 200 {object} response.R{data=object{id=integer,coefficient=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "区域系数不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/region-coefficients/{id} [put]
func (h *ConfigHandler) AdminUpdateRegionCoefficient(c *gin.Context) {
	h.updateDict(c, regionCoefficientDescriptor())
}

// =====================================================
// coefficient-configs（全局系数，按 key 更新）
// =====================================================

// AdminUpdateCoefficientConfig 按 key 更新全局系数
// @Summary 更新全局系数
// @Description 按唯一 key 更新全局系数配置（PUT /:key）；响应为完整行（key/value/description/updated_at/id）。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "系数 key"
// @Param body body object true "系数值" example({"value":0.85})
// @Success 200 {object} response.R{data=object{id=integer,key=string,value=number,description=string,updated_at=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "系数 key 不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/coefficient-configs/{key} [put]
func (h *ConfigHandler) AdminUpdateCoefficientConfig(c *gin.Context) {
	h.updateDict(c, coefficientConfigDescriptor())
}

// =====================================================
// 其余字典实体（brands / vehicle-types / series / tonnages / mast-types /
// mast-heights / battery-types / transmission-types / engine-types /
// condition-ratings）：前端 admin.ts 以 createCrud 统一声明 create/update/remove，
// 故按同一形状逐实体登记（形状同构，仅字段并集不同）。
// =====================================================

// AdminCreateBrand 新增品牌
// @Summary 新增品牌
// @Description 新增品牌（name 必填）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "品牌字段" example({"name":"林德","k_brand":1.1,"is_active":true})
// @Success 200 {object} response.R{data=object{id=integer,name=string,k_brand=number,is_active=boolean}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/brands [post]
func (h *ConfigHandler) AdminCreateBrand(c *gin.Context) { h.createDict(c, brandDescriptor()) }

// AdminUpdateBrand 更新品牌
// @Summary 更新品牌
// @Description 按 id 更新品牌系数与启用态（k_brand 必填，name 不在更新子集）；响应为 {id,k_brand,is_active}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "品牌 ID"
// @Param body body object true "品牌字段" example({"k_brand":1.12,"is_active":false})
// @Success 200 {object} response.R{data=object{id=integer,k_brand=number,is_active=boolean}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "品牌不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/brands/{id} [put]
func (h *ConfigHandler) AdminUpdateBrand(c *gin.Context) { h.updateDict(c, brandDescriptor()) }

// AdminCreateConditionRating 新增车况评级
// @Summary 新增车况评级
// @Description 新增车况评级（rating/label 必填）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "车况评级字段" example({"rating":"A","label":"优","base_coefficient":1.0})
// @Success 200 {object} response.R{data=object{id=integer,rating=string,label=string,base_coefficient=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/condition-ratings [post]
func (h *ConfigHandler) AdminCreateConditionRating(c *gin.Context) {
	h.createDict(c, conditionRatingDescriptor())
}

// AdminUpdateConditionRating 更新车况评级
// @Summary 更新车况评级
// @Description 按 id 更新车况系数（label/base_coefficient 必填，rating 不在更新子集）；响应为 {id,label,base_coefficient}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "车况评级 ID"
// @Param body body object true "车况评级字段" example({"label":"优","base_coefficient":1.02})
// @Success 200 {object} response.R{data=object{id=integer,label=string,base_coefficient=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "车况评级不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/condition-ratings/{id} [put]
func (h *ConfigHandler) AdminUpdateConditionRating(c *gin.Context) {
	h.updateDict(c, conditionRatingDescriptor())
}

// AdminCreateVehicleType 新增车型
// @Summary 新增车型
// @Description 新增车型（name/power_type 必填）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "车型字段" example({"name":"电动平衡重式","power_type":"electric","earliest_factory_year":1980})
// @Success 200 {object} response.R{data=object{id=integer,name=string,power_type=string,earliest_factory_year=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/vehicle-types [post]
func (h *ConfigHandler) AdminCreateVehicleType(c *gin.Context) {
	h.createDict(c, vehicleTypeDescriptor())
}

// AdminUpdateVehicleType 更新车型
// @Summary 更新车型
// @Description 按 id 更新车型（name/power_type 必填）；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "车型 ID"
// @Param body body object true "车型字段" example({"name":"电动平衡重式","power_type":"electric","earliest_factory_year":1980})
// @Success 200 {object} response.R{data=object{id=integer,name=string,power_type=string,earliest_factory_year=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "车型不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/vehicle-types/{id} [put]
func (h *ConfigHandler) AdminUpdateVehicleType(c *gin.Context) {
	h.updateDict(c, vehicleTypeDescriptor())
}

// AdminCreateSeries 新增系列
// @Summary 新增系列
// @Description 新增系列（brand/name 必填，(brand,name) 冲突时 DO NOTHING）；响应为 {id} ∪ 创建字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "系列字段" example({"brand":"林德","name":"H3","earliest_factory_year":2000})
// @Success 200 {object} response.R{data=object{id=integer,brand=string,name=string,earliest_factory_year=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/series [post]
func (h *ConfigHandler) AdminCreateSeries(c *gin.Context) { h.createDict(c, seriesDescriptor()) }

// AdminUpdateSeries 更新系列
// @Summary 更新系列
// @Description 按 id 更新系列（brand/name 必填）；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path integer true "系列 ID"
// @Param body body object true "系列字段" example({"brand":"林德","name":"H3","earliest_factory_year":2000})
// @Success 200 {object} response.R{data=object{id=integer,brand=string,name=string,earliest_factory_year=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "系列不存在"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/series/{id} [put]
func (h *ConfigHandler) AdminUpdateSeries(c *gin.Context) { h.updateDict(c, seriesDescriptor()) }

// AdminCreateTonnage 新增吨位
// @Summary 新增吨位
// @Description 新增吨位（value 必填，唯一列 DO NOTHING）；响应为 {id,value}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "吨位字段" example({"value":3})
// @Success 200 {object} response.R{data=object{id=integer,value=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/tonnages [post]
func (h *ConfigHandler) AdminCreateTonnage(c *gin.Context) { h.createDict(c, tonnageDescriptor()) }

// AdminCreateMastType 新增门架类型
// @Summary 新增门架类型
// @Description 新增门架类型（name 必填，唯一列 DO NOTHING）；响应为 {id,name}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "门架类型字段" example({"name":"两级门架"})
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/mast-types [post]
func (h *ConfigHandler) AdminCreateMastType(c *gin.Context) { h.createDict(c, mastTypeDescriptor()) }

// AdminCreateMastHeight 新增门架高度
// @Summary 新增门架高度
// @Description 新增门架高度（value_mm 必填，唯一列 DO NOTHING）；响应为 {id,value_mm}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "门架高度字段" example({"value_mm":3000})
// @Success 200 {object} response.R{data=object{id=integer,value_mm=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/mast-heights [post]
func (h *ConfigHandler) AdminCreateMastHeight(c *gin.Context) {
	h.createDict(c, mastHeightDescriptor())
}

// AdminCreateBatteryType 新增电池类型
// @Summary 新增电池类型
// @Description 新增电池类型（name 必填，唯一列 DO NOTHING）；响应为 {id,name}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "电池类型字段" example({"name":"磷酸铁锂"})
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/battery-types [post]
func (h *ConfigHandler) AdminCreateBatteryType(c *gin.Context) {
	h.createDict(c, batteryTypeDescriptor())
}

// AdminCreateTransmissionType 新增传动系统类型
// @Summary 新增传动系统类型
// @Description 新增传动系统类型（name 必填，唯一列 DO NOTHING）；响应为 {id,name}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "传动系统字段" example({"name":"手波"})
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/transmission-types [post]
func (h *ConfigHandler) AdminCreateTransmissionType(c *gin.Context) {
	h.createDict(c, transmissionTypeDescriptor())
}

// AdminCreateEngineType 新增发动机类型
// @Summary 新增发动机类型
// @Description 新增发动机类型（name 必填，唯一列 DO NOTHING）；响应为 {id,name}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object true "发动机类型字段" example({"name":"国产发动机"})
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 500 {object} response.R "服务器内部错误"
// @Router /valuation/admin/engine-types [post]
func (h *ConfigHandler) AdminCreateEngineType(c *gin.Context) {
	h.createDict(c, engineTypeDescriptor())
}

// AdminUpdateTonnage 更新吨位
// @Summary 更新吨位
// @Description value；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,value=number}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/tonnages/{id} [put]
func (h *ConfigHandler) AdminUpdateTonnage(c *gin.Context) { h.updateDict(c, tonnageDescriptor()) }

// AdminUpdateMastType 更新门架类型
// @Summary 更新门架类型
// @Description name；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/mast-types/{id} [put]
func (h *ConfigHandler) AdminUpdateMastType(c *gin.Context) { h.updateDict(c, mastTypeDescriptor()) }

// AdminUpdateMastHeight 更新门架高度
// @Summary 更新门架高度
// @Description value_mm；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,value_mm=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/mast-heights/{id} [put]
func (h *ConfigHandler) AdminUpdateMastHeight(c *gin.Context) {
	h.updateDict(c, mastHeightDescriptor())
}

// AdminUpdateBatteryType 更新电池类型
// @Summary 更新电池类型
// @Description name；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/battery-types/{id} [put]
func (h *ConfigHandler) AdminUpdateBatteryType(c *gin.Context) {
	h.updateDict(c, batteryTypeDescriptor())
}

// AdminUpdateTransmissionType 更新传动系统类型
// @Summary 更新传动系统类型
// @Description name；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/transmission-types/{id} [put]
func (h *ConfigHandler) AdminUpdateTransmissionType(c *gin.Context) {
	h.updateDict(c, transmissionTypeDescriptor())
}

// AdminUpdateEngineType 更新发动机类型
// @Summary 更新发动机类型
// @Description name；响应为 {id} ∪ 更新字段。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Param body body object true "字段"
// @Success 200 {object} response.R{data=object{id=integer,name=string}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/engine-types/{id} [put]
func (h *ConfigHandler) AdminUpdateEngineType(c *gin.Context) {
	h.updateDict(c, engineTypeDescriptor())
}

// AdminDeleteBrand 删除品牌
// @Summary 删除品牌
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/brands/{id} [delete]
func (h *ConfigHandler) AdminDeleteBrand(c *gin.Context) { h.deleteDict(c, brandDescriptor()) }

// AdminDeleteVehicleType 删除车型
// @Summary 删除车型
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/vehicle-types/{id} [delete]
func (h *ConfigHandler) AdminDeleteVehicleType(c *gin.Context) {
	h.deleteDict(c, vehicleTypeDescriptor())
}

// AdminDeleteSeries 删除系列
// @Summary 删除系列
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/series/{id} [delete]
func (h *ConfigHandler) AdminDeleteSeries(c *gin.Context) { h.deleteDict(c, seriesDescriptor()) }

// AdminDeleteTonnage 删除吨位
// @Summary 删除吨位
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/tonnages/{id} [delete]
func (h *ConfigHandler) AdminDeleteTonnage(c *gin.Context) { h.deleteDict(c, tonnageDescriptor()) }

// AdminDeleteMastType 删除门架类型
// @Summary 删除门架类型
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/mast-types/{id} [delete]
func (h *ConfigHandler) AdminDeleteMastType(c *gin.Context) { h.deleteDict(c, mastTypeDescriptor()) }

// AdminDeleteMastHeight 删除门架高度
// @Summary 删除门架高度
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/mast-heights/{id} [delete]
func (h *ConfigHandler) AdminDeleteMastHeight(c *gin.Context) {
	h.deleteDict(c, mastHeightDescriptor())
}

// AdminDeleteBatteryType 删除电池类型
// @Summary 删除电池类型
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/battery-types/{id} [delete]
func (h *ConfigHandler) AdminDeleteBatteryType(c *gin.Context) {
	h.deleteDict(c, batteryTypeDescriptor())
}

// AdminDeleteTransmissionType 删除传动系统类型
// @Summary 删除传动系统类型
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/transmission-types/{id} [delete]
func (h *ConfigHandler) AdminDeleteTransmissionType(c *gin.Context) {
	h.deleteDict(c, transmissionTypeDescriptor())
}

// AdminDeleteEngineType 删除发动机类型
// @Summary 删除发动机类型
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/engine-types/{id} [delete]
func (h *ConfigHandler) AdminDeleteEngineType(c *gin.Context) {
	h.deleteDict(c, engineTypeDescriptor())
}

// AdminDeleteConditionRating 删除车况评级
// @Summary 删除车况评级
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/condition-ratings/{id} [delete]
func (h *ConfigHandler) AdminDeleteConditionRating(c *gin.Context) {
	h.deleteDict(c, conditionRatingDescriptor())
}

// AdminDeleteRegionCoefficient 删除区域系数
// @Summary 删除区域系数
// @Description 按 id 删除；响应固定为 {id}。需管理员（主体系 JWT + role=admin）。
// @Tags 估值-管理端
// @Security BearerAuth
// @Param id path integer true "记录 ID"
// @Success 200 {object} response.R{data=object{id=integer}} "success"
// @Failure 400 {object} response.R "参数错误"
// @Failure 403 {object} response.R "无权限"
// @Failure 404 {object} response.R "记录不存在"
// @Router /valuation/admin/region-coefficients/{id} [delete]
func (h *ConfigHandler) AdminDeleteRegionCoefficient(c *gin.Context) {
	h.deleteDict(c, regionCoefficientDescriptor())
}

// originalPriceDescriptor / regionCoefficientDescriptor / coefficientConfigDescriptor /
// brandDescriptor / ... 按名取描述符的单点：注解壳与路由注册共用同一份描述符列表。
//
// 查不到即 panic（fail-closed）：名字打错或描述符被删时返回零值 Descriptor{} 会让
// 「无字段、无路由」静默成立（注解照发、路由少注册），到运行期才以 404/空响应暴露。
// 注册期纪律同 requireDictRoute（dictcrud_dispatch.go）——描述符缺失必须在启动前炸。
func descriptorByName(name string) dictcrud.Descriptor {
	for _, d := range dictcrud.AllDescriptors() {
		if d.Name == name {
			return d
		}
	}
	panic("字典描述符不存在: " + name + "（名字打错或描述符被删？）")
}

func originalPriceDescriptor() dictcrud.Descriptor { return descriptorByName("original_prices") }

func regionCoefficientDescriptor() dictcrud.Descriptor {
	return descriptorByName("region_coefficients")
}

func coefficientConfigDescriptor() dictcrud.Descriptor {
	return descriptorByName("coefficient_configs")
}

func brandDescriptor() dictcrud.Descriptor { return descriptorByName("brands") }

func vehicleTypeDescriptor() dictcrud.Descriptor { return descriptorByName("vehicle_types") }

func seriesDescriptor() dictcrud.Descriptor { return descriptorByName("series") }

func tonnageDescriptor() dictcrud.Descriptor { return descriptorByName("tonnages") }

func mastTypeDescriptor() dictcrud.Descriptor { return descriptorByName("mast_types") }

func mastHeightDescriptor() dictcrud.Descriptor { return descriptorByName("mast_heights") }

func batteryTypeDescriptor() dictcrud.Descriptor { return descriptorByName("battery_types") }

func transmissionTypeDescriptor() dictcrud.Descriptor { return descriptorByName("transmission_types") }

func engineTypeDescriptor() dictcrud.Descriptor { return descriptorByName("engine_types") }

func conditionRatingDescriptor() dictcrud.Descriptor { return descriptorByName("condition_ratings") }
