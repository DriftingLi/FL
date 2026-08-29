<script setup lang="ts">
import { ref } from 'vue'
import { Bell } from '@element-plus/icons-vue'
import NotificationPanelBody from './NotificationPanelBody.vue'

/**
 * 通知入口。
 *
 * - `popover`（**默认值 = 改造前行为**）：铃铛按钮 + 点击弹出浮层，tutor / admin 保持原样
 * - `panel`：直接渲染成内嵌卡片，不弹浮层，用于学员端 Dashboard 右栏
 */
const props = withDefaults(
  defineProps<{
    variant?: 'popover' | 'panel'
  }>(),
  { variant: 'popover' }
)

const unreadCount = ref(0)
const bodyRef = ref<InstanceType<typeof NotificationPanelBody> | null>(null)

// 浮层每次展开都要重新拉取（内部还有 30s 轮询，这里只补展开时机）
function onPopoverShow() {
  bodyRef.value?.refresh()
}
</script>

<template>
  <!-- panel：内嵌卡片，无触发按钮 -->
  <div v-if="props.variant === 'panel'" class="notification-panel-card">
    <NotificationPanelBody v-model:unread-count="unreadCount" />
  </div>

  <!-- popover：铃铛 + 浮层 -->
  <el-popover
    v-else
    placement="bottom-end"
    :width="360"
    trigger="click"
    popper-class="notification-popper"
    @show="onPopoverShow"
  >
    <template #reference>
      <button class="icon-btn notification-btn" title="通知">
        <el-badge
          :value="unreadCount"
          :hidden="unreadCount === 0"
          :max="99"
          class="notification-badge"
        >
          <el-icon><Bell /></el-icon>
        </el-badge>
      </button>
    </template>

    <NotificationPanelBody ref="bodyRef" v-model:unread-count="unreadCount" />
  </el-popover>
</template>

<style scoped>
.icon-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-md, 8px);
  background: transparent;
  color: var(--color-text-secondary, #475569);
  cursor: pointer;
  transition: background var(--duration-fast, 0.2s);
  flex-shrink: 0;
}

.icon-btn:hover {
  background: var(--color-bg-page, #f8fafc);
  color: var(--color-primary-600);
}

.notification-btn {
  position: relative;
}

.notification-badge {
  display: flex;
}

/* panel 变体：内嵌卡片外观 */
.notification-panel-card {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  background: var(--color-bg-card);
  padding: var(--space-4);
}
</style>
