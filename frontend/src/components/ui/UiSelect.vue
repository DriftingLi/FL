<script lang="ts">
export interface UiSelectOption {
  label: string
  value: string | number
}

/** 分组形态：渲染成 `el-option-group`（存量 8 处，`options` 平铺表达不了）。 */
export interface UiSelectGroup {
  label: string
  options: UiSelectOption[]
}
</script>

<script setup lang="ts">
/**
 * 下拉选择：包 `el-select`，把选项收成数组，避免每处手写 el-option。
 * 圆角与聚焦环同样由 element-overrides.css 统一处理。
 *
 * 三种选项形态（#1981 批次 0）：
 *   - `options`：平铺（原有形态，零改动）
 *   - `groups` ：分组 —— 渲染成 `el-option-group` 套 `el-option`；与 `options` 互斥，`groups` 优先
 *   - 自定义    ：`#options` 插槽，原样落到 `el-select` 里（远程搜索、自定义模板这类）
 */
/**
 * 值可以是单值（默认）或数组（`multiple`）。
 * 写成联合而不是两个组件：`multiple` 与单选是**同一意图**（从数据源选），
 * 登记表「一意图一形态」，拆成 `UiMultiSelect` 就是第二种写法了。
 */
/** `null` 也要收：EP 的 clearable 清空态在存量里被写成 `ref<number | null>(null)`。 */
const value = defineModel<string | number | null | Array<string | number> | undefined>({ default: undefined })

withDefaults(
  defineProps<{
    options?: UiSelectOption[]
    /** 分组选项；给了它就用它，忽略 `options` */
    groups?: UiSelectGroup[]
    placeholder?: string
    size?: 'small' | 'default' | 'large'
    disabled?: boolean
    clearable?: boolean
    /** 输入即筛选（EP 的 filterable） */
    filterable?: boolean
    /** 多选（EP 的 multiple）；值为数组 */
    multiple?: boolean
    /** 多选时把已选项折叠成一行（EP 的 collapse-tags） */
    collapseTags?: boolean
  }>(),
  {
    options: () => [],
    groups: undefined,
    size: 'default',
    disabled: false,
    clearable: false,
    filterable: false,
    multiple: false,
    collapseTags: false
  }
)
</script>

<template>
  <el-select
    v-model="value"
    :placeholder="placeholder"
    :size="size"
    :disabled="disabled"
    :clearable="clearable"
    :filterable="filterable"
    :multiple="multiple"
    :collapse-tags="collapseTags"
  >
    <slot name="options">
      <el-option-group
        v-for="group in groups ?? []"
        :key="group.label"
        :label="group.label"
      >
        <el-option
          v-for="opt in group.options"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-option-group>
      <el-option
        v-for="opt in groups ? [] : options"
        :key="opt.value"
        :label="opt.label"
        :value="opt.value"
      />
    </slot>
  </el-select>
</template>
