<template>
  <div
    class="credential-switcher"
    :class="{ collapsed: collapsed, 'is-compact': props.compact }"
  >
    <div v-if="collapsed" class="collapsed-view">
      <UiTooltip :content="current?.name || '选择证件'" placement="right" :show-after="300">
        <div class="collapsed-icon" @click="switcherVisible = true">
          <UiIcon name="notebook" />
        </div>
      </UiTooltip>
    </div>
    <div v-else class="expanded-view" :class="{ 'is-compact': props.compact }">
      <div v-if="!props.compact" class="switcher-label">当前证件</div>
      <UiSelect
        v-model="selectedId"
        placeholder="请选择证件"
        :size="props.compact ? 'default' : 'small'"
        class="credential-select"
        :aria-label="current ? `当前证件：${current.name}` : '选择证件'"
        :loading="credentialStore.loading"
        :groups="credentialGroups"
        @change="handleChange"
      />
      <div v-if="current && !props.compact" class="current-meta">
        <span class="current-name">{{ current.name }}</span>
        <span class="current-badge" :class="current.category">{{ categoryLabel(current.category) }}</span>
      </div>
    </div>

    <!-- 紧凑档的窄屏后备（≤768 只剩 32px）：顶栏在 375 宽上放不下"证件名 + 箭头"，
         硬留着会把中区挤成 0 并盖住右区的工具按钮（实测中区只剩 48px）。点开是同一个
         dialog 选择器，功能不丢。 -->
    <button
      v-if="props.compact"
      type="button"
      class="compact-icon-btn"
      :aria-label="current ? `当前证件：${current.name}，点击切换` : '切换证件'"
      :title="current?.name || '切换证件'"
      @click="switcherVisible = true"
    >
      <UiIcon name="notebook" :size="18" />
    </button>

    <!-- collapsed 展开为 dialog 选择 -->
    <UiDialog v-model="switcherVisible" title="切换证件" width="380px" append-to-body confirm-text="确定" :confirm-loading="switching" @confirm="switcherVisible = false">
      <UiSelect
        v-model="selectedId"
        placeholder="请选择证件"
        style="width: 100%"
        :groups="credentialGroups"
        @change="handleChange"
      />
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, watch, onMounted, computed } from 'vue'
import { useCredentialStore } from '@/stores/credential'
import type { CredentialDict } from '@/api/credential'
import { ElMessage } from 'element-plus'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

const props = withDefaults(
  defineProps<{
    /**
     * 折叠态（图标 + tooltip）。
     *
     * #1619 起本组件挂在**顶栏中区**（恒展开），故缺省 false；参数保留给未来的窄容器调用方。
     * 原 `theme` prop 与配套的 is-dark 分支已随恒深侧栏退役一并删除（ADR-0072）：
     * 外壳三块同底后不再存在「暗底侧栏」，留着就是无人走的配色分支。
     */
    collapsed?: boolean
    /**
     * 紧凑变体（缺省 false）：只渲染选择器，去掉「当前证件」标签与当前证件 meta 行。
     *
     * 为**顶栏中区**而设（#1619 外壳 / #1629 修）：顶栏恒高 56px，三行内容放不下会溢出到
     * 内容区上方且没有底色 —— 表现为「透明的文字压在卡片上」。
     *
     * 紧凑档的宽度是**显式**的（2026-10-28）：原来只靠 `width:100%`，而它是 flex 项里
     * 一个「百分比宽度 + 内容撑高」的组合，浏览器按 min-content 结算 —— 实测控件只剩
     * 34×24，证件名被裁到 11px，顶栏上于是只有一个悬空的箭头（用户反馈「图标歪了」）。
     * 现在给控件定宽 + 让外层成为 flex 容器（垂直居中由 `align-items` 负责，不再吃
     * inline-block 的基线间隙），名字与箭头各在其位。
     */
    compact?: boolean
  }>(),
  { collapsed: false, compact: false }
)

const credentialStore = useCredentialStore()
const selectedId = ref<number | null>(null)
const switching = ref(false)
const switcherVisible = ref(false)

const current = computed(() => credentialStore.current)

/**
 * 分组选项（特种作业 / 技能等级两族）：模板里原来把这段表达式抄了两遍（紧凑档与折叠档的
 * dialog），改一处漏一处就会让两条入口给出不同的选项。收成具名派生，单点。
 */
const credentialGroups = computed(() => [
  {
    label: '特种作业上岗证',
    options: credentialStore.grouped.special_operation.map((c) => ({ label: c.name, value: c.id }))
  },
  {
    label: '工程机械维修工',
    options: credentialStore.grouped.skill_level.map((c) => ({ label: levelLabel(c), value: c.id }))
  }
])

function categoryLabel(cat: string) {
  return cat === 'special_operation' ? '特种作业' : '技能等级'
}

function levelLabel(c: CredentialDict) {
  if (c.category === 'skill_level' && c.level) return `${c.name}`
  return c.name
}

function handleChange(val: number) {
  if (!val || val === current.value?.id) return
  switching.value = true
  credentialStore
    .switchTo(val)
    .then(() => {
      ElMessage.success('已切换证件')
      switcherVisible.value = false
      // 受证件过滤页面的失效刷新由 useAsyncPage 内聚 watch store 变化完成（#604 单点）
    })
    .catch((e: any) => {
      ElMessage.error(e?.message || '切换失败')
      selectedId.value = current.value?.id || null
    })
    .finally(() => {
      switching.value = false
    })
}

watch(
  () => credentialStore.current?.id,
  (id) => {
    selectedId.value = id || null
  },
  { immediate: true }
)

onMounted(async () => {
  if (!credentialStore.grouped.special_operation.length && !credentialStore.grouped.skill_level.length) {
    await credentialStore.loadGrouped().catch(() => {})
  }
  if (!credentialStore.initialized) {
    await credentialStore.loadCurrent().catch(() => {})
  }
  selectedId.value = credentialStore.current?.id || null
})
</script>

<style scoped>
.credential-switcher {
  padding: var(--space-3) var(--space-4);
  flex-shrink: 0;
}
.credential-switcher.collapsed {
  padding: var(--space-3) var(--space-2);
  display: flex;
  justify-content: center;
}
/* 紧凑档：顶栏中区，控件本体不要再带上下内边距（那会让它比铃铛高出 3px 且不上不下） */
.credential-switcher.is-compact {
  padding: 0;
  display: flex;
  align-items: center;
  min-width: 0;
}
.expanded-view {
  display: flex;
  flex-direction: column;
}
/* flex 行：子项被块化，不再按 baseline 落位（原来控件比容器低 3.5px） */
.expanded-view.is-compact {
  flex-direction: row;
  align-items: center;
  min-width: 0;
}
.expanded-view.is-compact .credential-select {
  width: 224px;
  max-width: 100%;
  min-width: 0;
}
.expanded-view.is-compact :deep(.el-select__wrapper) {
  min-height: 32px;
}
/* 窄屏后备按钮：默认不出现，只在 ≤768 顶替选择器 */
.compact-icon-btn {
  display: none;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
}
.compact-icon-btn:hover {
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
}
@media screen and (max-width: 768px) {
  .credential-switcher.is-compact .expanded-view {
    display: none;
  }
  .credential-switcher.is-compact .compact-icon-btn {
    display: flex;
  }
}
.switcher-label {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  margin-bottom: 6px;
  letter-spacing: 0.03em;
}
.credential-select {
  width: 100%;
}
.current-meta {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.current-name {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 140px;
}
.current-badge {
  font-size: var(--text-xs);
  padding: 1px 5px;
  border-radius: var(--radius-full);
  white-space: nowrap;
}
.current-badge.special_operation {
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
}
.current-badge.skill_level {
  background: var(--color-success-light);
  color: var(--color-success-strong);
}
.collapsed-view {
  display: flex;
  justify-content: center;
}
.collapsed-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: var(--color-bg-page);
  color: var(--color-text-secondary);
  cursor: pointer;
}
.collapsed-icon:hover {
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
}

</style>
