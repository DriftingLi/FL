<template>
  <el-dropdown trigger="click" placement="bottom-start" @command="handleCommand">
    <div class="model-selector-trigger">
      <el-icon :size="14"><Cpu /></el-icon>
      <span class="model-label">{{ currentLabel }}</span>
      <el-icon :size="12"><ArrowDown /></el-icon>
    </div>
    <template #dropdown>
      <el-dropdown-menu class="model-dropdown-menu">
        <!-- 管理员配置的模型 -->
        <el-dropdown-item disabled class="dropdown-header">管理员模型</el-dropdown-item>
        <el-dropdown-item
          v-for="m in adminModels"
          :key="'admin-' + m.id"
          :command="{ type: 'admin', id: m.id }"
          :class="{ 'is-selected': isSelected('admin', m.id) }"
        >
          <div class="model-item">
            <span class="model-name">{{ m.name }}</span>
            <span class="model-desc">{{ m.model }}</span>
          </div>
        </el-dropdown-item>

        <!-- 用户自定义模型（仅登录后显示） -->
        <template v-if="isLoggedIn">
          <el-dropdown-item disabled class="dropdown-header divider">我的模型</el-dropdown-item>
          <el-dropdown-item
            v-for="m in userModels"
            :key="'user-' + m.id"
            :command="{ type: 'user', id: m.id }"
            :class="{ 'is-selected': isSelected('user', m.id) }"
          >
            <div class="model-item">
              <span class="model-name">{{ m.name }}</span>
              <span class="model-desc">{{ m.model }}</span>
            </div>
          </el-dropdown-item>
          <el-dropdown-item :command="{ type: 'manage' }" class="manage-btn">
            <el-icon><Setting /></el-icon>
            <span>管理自定义模型</span>
          </el-dropdown-item>
        </template>

        <!-- 临时自定义 -->
        <el-dropdown-item disabled class="dropdown-header divider">临时输入</el-dropdown-item>
        <el-dropdown-item :command="{ type: 'custom' }" :class="{ 'is-selected': isSelected('custom') }">
          <div class="model-item">
            <span class="model-name">使用自定义 API Key</span>
            <span class="model-desc">OpenAI 格式</span>
          </div>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Cpu, ArrowDown, Setting } from '@element-plus/icons-vue'
import { useAIAssistantStore } from '@/stores/aiAssistant'

const store = useAIAssistantStore()

const adminModels = computed(() => store.adminModels)
const userModels = computed(() => store.userModels)
const isLoggedIn = computed(() => store.isLoggedIn)
const selectedModel = computed(() => store.selectedModel)

const currentLabel = computed(() => selectedModel.value?.label || '请选择模型')

const emit = defineEmits<{
  (e: 'manage'): void
  (e: 'custom'): void
}>()

function isSelected(type: string, id?: number): boolean {
  if (!selectedModel.value) return false
  if (selectedModel.value.source !== type) return false
  if (type === 'admin') return selectedModel.value.configId === id
  if (type === 'user') return selectedModel.value.userModelId === id
  if (type === 'custom') return true
  return false
}

function handleCommand(cmd: { type: string; id?: number }) {
  if (cmd.type === 'admin' && cmd.id) {
    store.selectAdminModel(cmd.id)
  } else if (cmd.type === 'user' && cmd.id) {
    store.selectUserModel(cmd.id)
  } else if (cmd.type === 'custom') {
    emit('custom')
  } else if (cmd.type === 'manage') {
    emit('manage')
  }
}
</script>

<style scoped>
.model-selector-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--color-bg-card, #fff);
  border: 1px solid var(--color-border-light, #e2e8f0);
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--color-text-primary, #0f172a);
  transition: all 0.15s ease;
  max-width: 280px;
}

.model-selector-trigger:hover {
  border-color: var(--color-brand-500, #0ea5e9);
}

.model-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.model-dropdown-menu {
  min-width: 280px;
}

.dropdown-header {
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-tertiary, #94a3b8);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  pointer-events: none;
}

.dropdown-header.divider {
  border-top: 1px solid var(--color-border-light, #e2e8f0);
  margin-top: 4px;
  padding-top: 8px;
}

.model-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 0;
}

.model-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary, #0f172a);
}

.model-desc {
  font-size: 11px;
  color: var(--color-text-tertiary, #94a3b8);
}

.is-selected {
  background: var(--color-brand-50, #f0f9ff);
  color: var(--color-brand-600, #0284c7);
}

.manage-btn {
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
  display: flex;
  align-items: center;
  gap: 6px;
}

.manage-btn:hover {
  color: var(--color-brand-600, #0284c7);
}
</style>
