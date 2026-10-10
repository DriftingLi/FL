<template>
  <div
    class="credential-switcher"
    :class="{ collapsed: collapsed }"
  >
    <div v-if="collapsed" class="collapsed-view">
      <UiTooltip :content="current?.name || '选择证件'" placement="right" :show-after="300">
        <div class="collapsed-icon" @click="switcherVisible = true">
          <el-icon><Notebook /></el-icon>
        </div>
      </UiTooltip>
    </div>
    <div v-else class="expanded-view">
      <div v-if="!props.compact" class="switcher-label">当前证件</div>
      <el-select
        v-model="selectedId"
        placeholder="请选择证件"
        size="small"
        class="credential-select"
        :loading="credentialStore.loading"
        @change="handleChange"
      >
        <el-option-group label="特种作业上岗证">
          <el-option
            v-for="c in credentialStore.grouped.special_operation"
            :key="c.id"
            :label="c.name"
            :value="c.id"
          />
        </el-option-group>
        <el-option-group label="工程机械维修工">
          <el-option
            v-for="c in credentialStore.grouped.skill_level"
            :key="c.id"
            :label="levelLabel(c)"
            :value="c.id"
          />
        </el-option-group>
      </el-select>
      <div v-if="current && !props.compact" class="current-meta">
        <span class="current-name">{{ current.name }}</span>
        <span class="current-badge" :class="current.category">{{ categoryLabel(current.category) }}</span>
      </div>
    </div>

    <!-- collapsed 展开为 dialog 选择 -->
    <UiDialog v-model="switcherVisible" title="切换证件" width="380px" append-to-body confirm-text="确定" :confirm-loading="switching" @confirm="switcherVisible = false">
      <el-select
        v-model="selectedId"
        placeholder="请选择证件"
        style="width: 100%"
        @change="handleChange"
      >
        <el-option-group label="特种作业上岗证">
          <el-option v-for="c in credentialStore.grouped.special_operation" :key="c.id" :label="c.name" :value="c.id" />
        </el-option-group>
        <el-option-group label="工程机械维修工">
          <el-option v-for="c in credentialStore.grouped.skill_level" :key="c.id" :label="levelLabel(c)" :value="c.id" />
        </el-option-group>
      </el-select>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useCredentialStore } from '@/stores/credential'
import type { CredentialDict } from '@/api/credential'
import { Notebook } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

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
.switcher-label {
  font-size: var(--text-xs);
  color: var(--color-text-muted);
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
  font-size: 10px;
  padding: 1px 5px;
  border-radius: var(--radius-full);
  white-space: nowrap;
}
.current-badge.special_operation {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
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
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

</style>
