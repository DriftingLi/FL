<template>
  <div
    class="credential-switcher"
    :class="{ collapsed: collapsed, 'is-dark': props.theme === 'dark' }"
  >
    <div v-if="collapsed" class="collapsed-view">
      <el-tooltip :content="current?.name || '选择证件'" placement="right" :show-after="300">
        <div class="collapsed-icon" @click="switcherVisible = true">
          <el-icon><Notebook /></el-icon>
        </div>
      </el-tooltip>
    </div>
    <div v-else class="expanded-view">
      <div class="switcher-label">当前证件</div>
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
      <div v-if="current" class="current-meta">
        <span class="current-name">{{ current.name }}</span>
        <span class="current-badge" :class="current.category">{{ categoryLabel(current.category) }}</span>
      </div>
    </div>

    <!-- collapsed 展开为 dialog 选择 -->
    <el-dialog v-model="switcherVisible" title="切换证件" width="380px" append-to-body>
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
      <template #footer>
        <el-button @click="switcherVisible = false">取消</el-button>
        <el-button type="primary" :loading="switching" @click="switcherVisible = false">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useCredentialStore } from '@/stores/credential'
import type { CredentialDict } from '@/api/credential'
import { Notebook } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const props = withDefaults(
  defineProps<{
    collapsed: boolean
    /**
     * 配色，需与所在侧栏的 theme 保持一致。
     * - `light`：**默认值 = 改造前行为**
     * - `dark`：适配石墨青暗底侧栏
     */
    theme?: 'light' | 'dark'
  }>(),
  { theme: 'light' }
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
      // 全局刷新由各页面经 useCredentialRefetch watch store 变化完成（#387 单点）
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
  background: #ECFDF5;
  color: #059669;
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

/* dark：适配石墨青暗底侧栏（Theme 由外层 TrainingLayout 与 AppSidebar 同步传入）。
   追加覆盖，light 分支零改动。
   下拉面板（el-select-dropdown）被 teleport 到 body，不在此适配 —— 保持浅色浮层。 */
.credential-switcher.is-dark .switcher-label {
  color: rgba(148, 163, 184, 0.85);
}

.credential-switcher.is-dark .current-name {
  color: #f1f5f9;
}

.credential-switcher.is-dark .current-badge.special_operation {
  background: rgba(45, 212, 191, 0.16);
  color: var(--color-primary-300);
}

.credential-switcher.is-dark .collapsed-icon {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(241, 245, 249, 0.8);
}

.credential-switcher.is-dark .collapsed-icon:hover {
  background: rgba(45, 212, 191, 0.18);
  color: var(--color-primary-300);
}

.credential-switcher.is-dark :deep(.el-select__wrapper) {
  background-color: rgba(255, 255, 255, 0.08);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12) inset;
  color: rgba(241, 245, 249, 0.9);
}

.credential-switcher.is-dark :deep(.el-select__placeholder) {
  color: rgba(148, 163, 184, 0.7);
}
</style>
