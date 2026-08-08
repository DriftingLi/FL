// 描述符驱动管理端 CRUD 路由工厂（ADR-0008）：
// 一个 Descriptor 声明实体名、路由段、字段、校验与失效标记，
// POST/PUT/DELETE 骨架由同一工厂注册生成，不再逐实体手写。
// 失效 pattern 仍来自 repository 缓存契约单点（PatternsOf），工厂不书写字面量。
package handler

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"forklift-training/internal/valuation/dictcrud"
	"forklift-training/internal/valuation/repository"
	"forklift-training/pkg/response"
)

// dictInvalidationPatterns 描述符实体的写后失效集：
// 契约 pattern（repository.PatternsOf）+ （按描述符）评估结果缓存 pattern。
func dictInvalidationPatterns(d dictcrud.Descriptor) []string {
	patterns := repository.PatternsOf(d.Name)
	if d.InvalidateResult {
		patterns = append(patterns, repository.ResultCachePattern)
	}
	return patterns
}

// registerDictCRUDRoutes 按描述符注册管理端 CRUD 路由：
// Create.Fields 非空 → POST /path；Update.Fields 非空 → PUT /path/:id（UpdateKeyField 时 /:key）；
// Delete=true → DELETE /path/:id。
func (h *ConfigHandler) registerDictCRUDRoutes(group *gin.RouterGroup, reg *dictcrud.Registry) {
	for _, d := range reg.All() {
		d := d
		if len(d.Create.Fields) > 0 {
			group.POST("/"+d.Path, func(c *gin.Context) { h.createDict(c, d) })
		}
		if len(d.Update.Fields) > 0 {
			param := "id"
			if d.UpdateKeyField != "" {
				param = d.UpdateKeyField
			}
			group.PUT("/"+d.Path+"/:"+param, func(c *gin.Context) { h.updateDict(c, d) })
		}
		if d.Delete {
			group.DELETE("/"+d.Path+"/:id", func(c *gin.Context) { h.deleteDict(c, d) })
		}
	}
}

// createDict 描述符驱动创建骨架：
// bind（JSON 语法/字段类型/bind 必填）→ 默认值 → 应用层必填 → 写库 → 缓存失效 → 返回整行。
func (h *ConfigHandler) createDict(c *gin.Context, d dictcrud.Descriptor) {
	values, ok := h.bindDictFields(c, d, d.Create)
	if !ok {
		return
	}
	id, err := h.dictRepo.Create(c.Request.Context(), d, values)
	if err != nil {
		h.logger.Error("新增"+d.EntityLabel+"失败", zap.Error(err))
		response.ServerError(c, "新增"+d.EntityLabel+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), dictInvalidationPatterns(d)...)
	response.Success(c, dictcrud.BuildCreateResult(d, id, values))
}

// updateDict 描述符驱动更新骨架：id 解析 → bind → 写库（ErrNoRows→404）→ 失效 → 返回字段子集。
// UpdateKeyField 非空时走按 key 更新（coefficient_configs，返回完整行）。
func (h *ConfigHandler) updateDict(c *gin.Context, d dictcrud.Descriptor) {
	if d.UpdateKeyField != "" {
		h.updateDictByKey(c, d)
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	values, ok := h.bindDictFields(c, d, d.Update)
	if !ok {
		return
	}
	if err := h.dictRepo.Update(c.Request.Context(), d, id, values); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, d.NotFoundMessage)
			return
		}
		h.logger.Error("更新"+d.EntityLabel+"失败", zap.Error(err))
		response.ServerError(c, "更新"+d.EntityLabel+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), dictInvalidationPatterns(d)...)
	response.Success(c, dictcrud.BuildUpdateResult(d, id, values))
}

// updateDictByKey 按唯一 key 列更新骨架（coefficient_configs）：
// key 参数 → bind → 写库（ErrNoRows→404，消息附 key）→ 失效 → 返回完整行。
func (h *ConfigHandler) updateDictByKey(c *gin.Context, d dictcrud.Descriptor) {
	key := c.Param(d.UpdateKeyField)
	if key == "" {
		response.BadRequest(c, d.UpdateKeyMessage)
		return
	}
	values, ok := h.bindDictFields(c, d, d.Update)
	if !ok {
		return
	}
	row, err := h.dictRepo.UpdateByKey(c.Request.Context(), d, key, values)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			msg := d.NotFoundMessage
			if d.NotFoundWithValue {
				msg += ": " + key
			}
			response.NotFound(c, msg)
			return
		}
		h.logger.Error("更新"+d.EntityLabel+"失败", zap.Error(err))
		response.ServerError(c, "更新"+d.EntityLabel+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), dictInvalidationPatterns(d)...)
	response.Success(c, row)
}

// deleteDict 描述符驱动删除骨架：id 解析 → 删除（ErrNoRows→404）→ 失效 → 返回 {id}。
func (h *ConfigHandler) deleteDict(c *gin.Context, d dictcrud.Descriptor) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 必须为整数")
		return
	}
	if err := h.dictRepo.Delete(c.Request.Context(), d, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, d.NotFoundMessage)
			return
		}
		h.logger.Error("删除"+d.EntityLabel+"失败", zap.Error(err))
		response.ServerError(c, "删除"+d.EntityLabel+"失败")
		return
	}
	h.invalidateCache(c.Request.Context(), dictInvalidationPatterns(d)...)
	response.Success(c, gin.H{"id": id})
}

// bindDictFields 描述符驱动 body 绑定：
// 按 OpSpec.Fields 逐字段解码（JSON 语法错误 → 400 请求体格式错误）；
// 缺失字段落类型零值（与 struct 零值绑定语义一致）；BindRequired 缺失 → 400（复制
// gin validator 的 required 失败消息）；随后应用默认值与应用层必填校验。
func (h *ConfigHandler) bindDictFields(c *gin.Context, d dictcrud.Descriptor, spec dictcrud.OpSpec) (map[string]any, bool) {
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		response.BadRequest(c, "请求体格式错误: "+err.Error())
		return nil, false
	}
	values := make(map[string]any, len(spec.Fields))
	for _, name := range spec.Fields {
		f, ok := d.Field(name)
		if !ok {
			continue
		}
		rawVal, present := raw[name]
		if !present {
			if containsString(spec.BindRequired, name) {
				response.BadRequest(c, "请求体格式错误: Key: '"+f.BindNameOr()+
					"' Error:Field validation for '"+f.BindNameOr()+"' failed on the 'required' tag")
				return nil, false
			}
			values[name] = dictcrud.ZeroValue(f)
			continue
		}
		v, err := decodeDictValue(rawVal, f.Type)
		if err != nil {
			response.BadRequest(c, "请求体格式错误: "+err.Error())
			return nil, false
		}
		values[name] = v
	}
	dictcrud.ApplyDefaults(d, values)
	for _, name := range spec.Required {
		if s, ok := values[name].(string); ok && s == "" {
			response.BadRequest(c, d.RequiredMessage)
			return nil, false
		}
	}
	for _, name := range d.ValidatePositive {
		if !positiveValue(values[name]) {
			response.BadRequest(c, d.PositiveMessage)
			return nil, false
		}
	}
	return values, true
}

// positiveValue 数值字段 > 0 判断（应用层校验，original_prices 的 original_price）。
func positiveValue(v any) bool {
	switch v := v.(type) {
	case float64:
		return v > 0
	case int:
		return v > 0
	}
	return false
}

// decodeDictValue 按字段类型解码单字段 JSON 值。
func decodeDictValue(raw json.RawMessage, t dictcrud.FieldType) (any, error) {
	switch t {
	case dictcrud.FieldString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return s, nil
	case dictcrud.FieldFloat:
		var f float64
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		return f, nil
	case dictcrud.FieldInt:
		var i int
		if err := json.Unmarshal(raw, &i); err != nil {
			return nil, err
		}
		return i, nil
	case dictcrud.FieldBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, errors.New("未知字段类型")
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
