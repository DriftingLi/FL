<template>
  <el-popover
    placement="bottom-end"
    :width="360"
    trigger="click"
    popper-class="notification-popper"
    @show="refresh"
  >
    <template #reference>
      <button class="icon-btn notification-btn" title="通知">
        <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notification-badge">
          <el-icon><Bell /></el-icon>
        </el-badge>
      </button>
    </template>

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
  </el-popover>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { Bell } from '@element-plus/icons-vue'
import { notificationApi, type NotificationItem } from '@/api/notification'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const items = ref<NotificationItem[]>([])
const unreadCount = ref(0)
const total = ref(0)
const loading = ref(false)
let timer: number | undefined
let lastUserSync = 0

// 资料审核通过后，同步最新昵称/头像到本地缓存（昵称/头像修改需管理员审核后才生效）
function isProfileApproved(item: NotificationItem) {
  return item.type === 'profile_review' && item.title.includes('通过')
}

async function syncUserInfoAfterApproval() {
  try {
    await authStore.refreshUserInfo()
  } catch (e) {
    // 静默失败，下次轮询再同步
  }
}

// 节流：未读通知存在时最多每 60 秒同步一次用户资料
async function maybeSyncUserInfo() {
  const now = Date.now()
  if (now - lastUserSync < 60000) return
  lastUserSync = now
  await syncUserInfoAfterApproval()
}

async function refresh() {
  loading.value = true
  try {
    const data = await notificationApi.list({ page: 1, page_size: 10 })
    if (data) {
      items.value = data.items || []
      total.value = data.total || 0
      unreadCount.value = data.unread_count || 0
      if (items.value.some(item => !item.is_read && isProfileApproved(item))) {
        await syncUserInfoAfterApproval()
      }
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
        await maybeSyncUserInfo()
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
    await syncUserInfoAfterApproval()
  }
  if (item.link) {
    router.push(item.link)
  }
}

async function handleMarkAllRead() {
  try {
    await notificationApi.markAllRead()
    items.value.forEach(item => {
      item.is_read = true
    })
    unreadCount.value = 0
  } catch (e) {
    // 静默失败
  }
}

function formatTime(value: string) {
  return dayjs(value).format('YYYY-MM-DD HH:mm')
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
</script>

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
  color: var(--color-primary-600, #1d4ed8);
}

.notification-btn {
  position: relative;
}

.notification-badge {
  display: flex;
}

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
  border-bottom: 1px solid var(--color-border-light, #E2E8F0);
}

.notification-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary, #1E293B);
}

.notification-state {
  padding: 32px 0;
  text-align: center;
  color: var(--color-text-tertiary, #94A3B8);
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
  border-bottom: 1px solid var(--color-border-light, #F1F5F9);
  cursor: pointer;
  transition: background var(--duration-fast, 0.2s);
}

.notification-item:hover {
  background: var(--color-bg-page, #F8FAFC);
}

.notification-dot {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--color-primary-500, #2563EB);
}

.notification-main {
  flex: 1;
  min-width: 0;
}

.notification-item-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary, #1E293B);
}

.notification-item.unread .notification-item-title {
  color: var(--color-primary-600, #1D4ED8);
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
  color: var(--color-text-tertiary, #94A3B8);
}

.notification-footer {
  padding-top: 8px;
  text-align: center;
  font-size: 12px;
  color: var(--color-text-tertiary, #94A3B8);
}
</style>
