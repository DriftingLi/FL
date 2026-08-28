<!-- PROTOTYPE — throwaway variant B: 时间分组卡片网格 -->
<template>
  <div class="variant-b">
    <div v-if="items.length === 0" class="empty-wrap">
      <el-empty description="暂无浏览记录" />
    </div>
    <div v-else class="group-stack">
      <div v-for="group in grouped" :key="group.label" class="group-section">
        <div class="group-head">
          <span class="group-dot"></span>
          <span class="group-label">{{ group.label }}</span>
          <span class="group-count">{{ group.items.length }}条</span>
        </div>
        <div class="group-grid">
          <div
            v-for="item in group.items"
            :key="item.id"
            class="history-card"
            :class="{ 'is-deleted': item.deleted }"
            @click="!item.deleted && $emit('select', item.id)"
          >
            <div class="card-top">
              <el-tag size="small" :type="item.deleted ? 'danger' : 'info'" effect="plain">
                {{ item.deleted ? '已删除' : formatRelativeTime(item.viewedAt) }}
              </el-tag>
              <el-button size="small" text type="danger" @click.stop="$emit('remove', item.id)">移除</el-button>
            </div>
            <div class="card-title" :class="{ deleted: item.deleted }">{{ item.title }}</div>
            <div class="card-excerpt">{{ item.excerpt }}</div>
            <div class="card-meta">
              <span>{{ item.author.username }}</span>
              <span class="dot">·</span>
              <span class="meta-stat"><el-icon><View /></el-icon>{{ item.view_count }}</span>
              <span class="meta-stat"><el-icon><ChatDotRound /></el-icon>{{ item.reply_count }}</span>
              <span v-if="item.images_count" class="meta-stat img"><el-icon><Picture /></el-icon>{{ item.images_count }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { View, ChatDotRound, Picture } from '@element-plus/icons-vue'
import { formatRelativeTime } from '@/utils/format'
import type { ForumHistoryItem } from './types'

const props = defineProps<{ items: ForumHistoryItem[] }>()
defineEmits<{ select: [id: number]; remove: [id: number] }>()

function isToday(iso: string): boolean {
  const d = new Date(iso)
  const now = new Date()
  return d.toDateString() === now.toDateString()
}

function isThisWeek(iso: string): boolean {
  const d = new Date(iso).getTime()
  const now = Date.now()
  const diff = now - d
  return diff < 7 * 24 * 3600 * 1000
}

const grouped = computed(() => {
  const groups: Array<{ label: string; items: ForumHistoryItem[] }> = [
    { label: '今天', items: [] },
    { label: '本周', items: [] },
    { label: '更早', items: [] },
  ]
  for (const item of props.items) {
    if (item.deleted) {
      groups[2].items.push(item)
    } else if (isToday(item.viewedAt)) {
      groups[0].items.push(item)
    } else if (isThisWeek(item.viewedAt)) {
      groups[1].items.push(item)
    } else {
      groups[2].items.push(item)
    }
  }
  return groups.filter((g) => g.items.length > 0)
})
</script>

<style scoped>
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.group-stack {
  display: flex;
  flex-direction: column;
  gap: 22px;
}
.group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}
.group-dot {
  width: 8px;
  height: 8px;
  border-radius: 9999px;
  background: var(--color-primary-500);
  box-shadow: 0 0 0 4px var(--color-primary-100);
  flex-shrink: 0;
}
.group-label {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.group-count {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}
.group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.history-card {
  padding: 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: border-color 150ms var(--ease-default), box-shadow 150ms var(--ease-default);
}
.history-card:hover {
  border-color: var(--color-border);
}
.history-card.is-deleted {
  opacity: 0.6;
  cursor: default;
  background: #fafafa;
}
.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.4;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.card-title.deleted {
  color: var(--color-text-muted);
  text-decoration: line-through;
}
.card-excerpt {
  font-size: 12px;
  color: var(--color-text-tertiary);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 8px;
}
.card-meta {
  font-size: 12px;
  color: var(--color-text-tertiary);
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.dot {
  color: var(--color-text-muted);
}
.meta-stat {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.meta-stat.img {
  color: #e6a23c;
}
</style>
