<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { notificationApi, type NotificationItem } from '@/api/notification'
import { useNotificationAction } from '@/composables/useNotificationAction'
import { useAuthStore } from '@/stores/auth'
import { formatTime } from '@/utils/format'

/**
 * 通知列表体：数据与交互都在这，供 NotificationPanel 的 popover / panel
 * 两种变体复用，避免两份模板各写一遍。
 *
 * 未读数通过 v-model 回传给外层（popover 变体要用它渲染铃铛徽章）。
 */
const unreadCount = defineModel<number>('unreadCount', { default: 0 })

const router = useRouter()
const authStore = useAuthStore()
const items = ref<NotificationItem[]>([])
const total = ref(0)
const loading = ref(false)
let timer: number | undefined

// 通知动作判定 module：审核通过判定（仅依赖结构化 payload）+ 60s 节流 + refreshUserInfo 同步
const { isProfileApproved, requestThrottledSync, syncIfUnreadApproved } = useNotificationAction({
  refreshUserInfo: () => authStore.refreshUserInfo()
})

async function refresh() {
  loading.value = true
  try {
    const data = await notificationApi.list({ page: 1, page_size: 10 })
    if (data) {
      items.value = data.items || []
      total.value = data.total || 0
      unreadCount.value = data.unread_count || 0
      // 未读审核通过通知 → 同步最新昵称/头像
      await syncIfUnreadApproved(items.value)
    }
  } catch (e) {
    // 静默失败，保留旧数据
  } finally {
    loading.value = false
  }
}

async function refreshUnread() {
  try {
    const data = await notificationApi.unreadCount()
    if (data) {
      unreadCount.value = data.count || 0
      if (unreadCount.value > 0) {
        // 60s 节流同步用户资料（默认窗口，与现状 lastUserSync 一致）
        await requestThrottledSync()
      }
    }
  } catch (e) {
    // 静默失败
  }
}

async function handleClick(item: NotificationItem) {
  if (!item.is_read) {
    item.is_read = true
    unreadCount.value = Math.max(0, unreadCount.value - 1)
    notificationApi.markRead(item.id).catch(() => {})
  }
  if (isProfileApproved(item)) {
    await authStore.refreshUserInfo().catch(() => {})
  }
  if (item.link) {
    router.push(item.link)
  }
}

async function handleMarkAllRead() {
  try {
    await notificationApi.markAllRead()
    items.value.forEach((item) => {
      item.is_read = true
    })
    unreadCount.value = 0
  } catch (e) {
    // 静默失败
  }
}

onMounted(() => {
  refresh()
  timer = window.setInterval(refreshUnread, 30000)
})

onUnmounted(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})

defineExpose({ refresh })
</script>

<template>
  <div class="notification-panel">
    <div class="notification-header">
      <span class="notification-title">通知</span>
      <el-button
        v-if="unreadCount > 0"
        link
        type="primary"
        size="small"
        @click="handleMarkAllRead"
      >
        全部已读
      </el-button>
    </div>

    <div v-if="loading" class="notification-state">加载中...</div>
    <div v-else-if="items.length === 0" class="notification-state">暂无通知</div>
    <div v-else class="notification-list">
      <div
        v-for="item in items"
        :key="item.id"
        class="notification-item"
        :class="{ unread: !item.is_read }"
        @click="handleClick(item)"
      >
        <span v-if="!item.is_read" class="notification-dot"></span>
        <div class="notification-main">
          <div class="notification-item-title">{{ item.title }}</div>
          <div class="notification-item-content">{{ item.content }}</div>
          <div class="notification-item-time">{{ formatTime(item.created_at) }}</div>
        </div>
      </div>
    </div>
    <div v-if="total > items.length" class="notification-footer">
      共 {{ total }} 条通知
    </div>
  </div>
</template>

<style scoped>
.notification-panel {
  display: flex;
  flex-direction: column;
  max-height: 420px;
}

.notification-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--color-border-light, #e2e8f0);
}

.notification-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary, #1e293b);
}

.notification-state {
  padding: 32px 0;
  text-align: center;
  color: var(--color-text-tertiary, #94a3b8);
  font-size: 13px;
}

.notification-list {
  overflow-y: auto;
  max-height: 320px;
}

.notification-item {
  display: flex;
  gap: 8px;
  padding: 10px 4px;
  border-bottom: 1px solid var(--color-border-light, #f1f5f9);
  cursor: pointer;
  transition: background var(--duration-fast, 0.2s) var(--ease-default);
}

.notification-item:hover {
  background: var(--color-bg-page, #f8fafc);
}

.notification-dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--color-primary-500, #2563eb);
}

.notification-main {
  flex: 1;
  min-width: 0;
}

.notification-item-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary, #1e293b);
}

.notification-item.unread .notification-item-title {
  color: var(--color-primary-600, #1d4ed8);
}

.notification-item-content {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-secondary, #475569);
  line-height: 1.5;
  word-break: break-all;
}

.notification-item-time {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-text-tertiary, #94a3b8);
}

.notification-footer {
  padding-top: 8px;
  text-align: center;
  font-size: 12px;
  color: var(--color-text-tertiary, #94a3b8);
}
</style>
