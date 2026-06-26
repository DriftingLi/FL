<!--
  动态部件状态表（Tesla 极简风：扁平、白底、4px 圆角）
  - 状态列用 el-table 的 #default scoped slot 渲染 el-select，确保响应式更新
-->
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ITEM_STATUS_OPTIONS } from '@/utils/valuationConstants'
import type { ItemStatus } from '@/types/valuation/evaluation'
import type { PartConfigList, PartItemDef } from '@/types/valuation/condition'

interface Props {
  configs: PartConfigList | null
  /** 双向绑定的状态映射：item_code → status */
  modelValue: Record<string, ItemStatus | undefined>
  /** 是否禁用（只读模式） */
  readonly?: boolean
}

const props = withDefaults(defineProps<Props>(), { readonly: false })
const emit = defineEmits<{ 'update:modelValue': [v: Record<string, ItemStatus | undefined>] }>()

/** el-collapse v-model：默认展开规则复刻源 default-active-key 语义（总数 ≤ 4 全展开，否则全收起） */
const activeNames = ref<string[]>([])

watch(
  () => props.configs,
  (list) => {
    if (list && list.length > 0) {
      activeNames.value = list.length <= 4 ? list.map((c) => c.category_code) : []
    }
  },
  { immediate: true }
)

/** 更新单条状态：emit 出去让父组件 v-model 接住 */
function updateStatus(itemCode: string, val: ItemStatus) {
  emit('update:modelValue', { ...props.modelValue, [itemCode]: val })
}

/** el-select onChange 桥接：narrow 守卫到 ItemStatus */
function onStatusSelectChange(itemCode: string, v: unknown) {
  if (v === 'normal' || v === 'slight_wear' || v === 'need_repair' || v === 'need_replace') {
    updateStatus(itemCode, v)
  }
}

/** a-select 的下拉选项（与常量保持同步） */
const statusSelectOptions = ITEM_STATUS_OPTIONS.map((o) => ({ value: o.value, label: o.label }))

/** 状态 select 的 value */
function statusValueOf(itemCode: string): ItemStatus | undefined {
  return props.modelValue[itemCode]
}

/** 类别摘要映射：避免模板中多次调用导致重复遍历 */
const categorySummaryMap = computed(() => {
  const map: Record<string, { total: number; filled: number }> = {}
  for (const c of props.configs ?? []) {
    let filled = 0
    for (const it of c.items) {
      if (props.modelValue[it.item_code]) {
        filled++
      }
    }
    map[c.category_code] = { total: c.items.length, filled }
  }
  return map
})

/** 状态值 → 是否已填 */
function isFilled(itemCode: string): boolean {
  return !!props.modelValue[itemCode]
}
</script>

<template>
  <div class="condition-table" v-if="configs && configs.length">
    <el-collapse v-model="activeNames" class="condition-collapse">
      <el-collapse-item
        v-for="cat in configs"
        :key="cat.category_code"
        :name="cat.category_code"
        class="category-block"
      >
        <template #title>
          <div class="category-header">
            <span class="category-name">{{ cat.category_name }}</span>
            <span
              class="category-meta"
              :class="{
                'is-complete': categorySummaryMap[cat.category_code]?.filled === categorySummaryMap[cat.category_code]?.total
              }"
            >
              {{ categorySummaryMap[cat.category_code]?.filled ?? 0 }} / {{ categorySummaryMap[cat.category_code]?.total ?? 0 }}
            </span>
          </div>
        </template>

        <el-table :data="cat.items" row-key="item_code" size="default">
          <el-table-column prop="item_name" label="部件条目" />
          <el-table-column label="状态" width="280">
            <template #default="{ row }">
              <el-select
                :model-value="statusValueOf((row as PartItemDef).item_code)"
                :disabled="readonly === true"
                :class="['status-select', { 'is-filled': isFilled((row as PartItemDef).item_code) }]"
                placeholder="请选择"
                @change="(v: unknown) => onStatusSelectChange((row as PartItemDef).item_code, v)"
              >
                <el-option
                  v-for="o in statusSelectOptions"
                  :key="o.value"
                  :value="o.value"
                  :label="o.label"
                />
              </el-select>
            </template>
          </el-table-column>
        </el-table>
      </el-collapse-item>
    </el-collapse>
  </div>

  <el-empty v-else description="暂无部件配置" />
</template>

<style scoped>
.condition-table {
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
}
.category-block {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  margin-bottom: var(--sp-3);
}
.category-block :deep(.el-collapse-item__header) {
  padding: 0 var(--sp-5);
  align-items: center;
  border-bottom: 1px solid var(--color-border);
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
}
.category-block :deep(.el-collapse-item__wrap) {
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}
.category-block :deep(.el-collapse-item__content) {
  padding: 0;
}
.category-block :deep(.el-table) {
  border-radius: 0 0 var(--radius-lg) var(--radius-lg);
}
.category-header {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  width: 100%;
}
.category-name {
  font-size: var(--fs-base);
  font-weight: var(--fw-medium);
  color: var(--color-text);
}
.category-meta {
  font-size: var(--fs-xs);
  font-family: var(--font-mono);
  color: var(--color-text-tertiary);
  background: var(--color-bg-muted);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
  font-weight: var(--fw-medium);
}
.category-meta.is-complete {
  color: var(--color-primary);
  background: rgba(62, 106, 225, 0.08);
}
.status-select {
  width: 240px;
}

/* ===== 移动端适配 ===== */
@media (max-width: 768px) {
  .status-select {
    width: 100%;
  }
  .category-block :deep(.el-collapse-item__header) {
    padding: 12px 16px;
  }
}
</style>
