<template>
  <div class="forum-history-panel">
    <div v-if="items.length === 0" class="empty-wrap">
      <el-empty description="暂无浏览记录，去论坛看看帖子吧" />
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
      <div class="panel-footer">
        <el-button size="small" type="danger" plain @click="$emit('clear')">清空浏览记录</el-button>
        <span class="footer-hint">最多保留 {{ maxHistory }} 条 · 重复浏览自动置顶</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { View, ChatDotRound, Picture } from '@element-plus/icons-vue'
import { formatRelativeTime } from '@/utils/format'
import { getMaxHistory } from '@/utils/forumHistory'
import type { ForumHistoryItem } from '@/utils/forumHistory'

const props = defineProps<{ items: ForumHistoryItem[] }>()
defineEmits<{ select: [id: number]; remove: [id: number]; clear: [] }>()

const maxHistory = getMaxHistory()

function isToday(iso: string): boolean {
  const d = new Date(iso)
  const now = new Date()
  return d.toDateString() === now.toDateString()
}

function isThisWeek(iso: string): boolean {
  const d = new Date(iso).getTime()
  const now = Date.now()
  return now - d < 7 * 24 * 3600 * 1000
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
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
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
  background: var(--color-primary-500, #409eff);
  box-shadow: 0 0 0 4px var(--color-primary-100, #ecf5ff);
  flex-shrink: 0;
}
.group-label {
  font-size: 15px;
  font-weight: 700;
  color: var(--color-text-primary, #303133);
}
.group-count {
  font-size: 12px;
  color: var(--color-text-muted, #909399);
  background: var(--color-bg-page, #f5f7fa);
  border: 1px solid var(--color-border-light, #ebeef5);
  padding: 2px 8px;
  border-radius: 9999px;
}
.group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.history-card {
  padding: 14px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 12px;
  cursor: pointer;
  transition: border-color 150ms ease, box-shadow 150ms ease;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
}
.history-card:hover {
  border-color: #dcdfe6;
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
  color: #303133;
  line-height: 1.4;
  margin-bottom: 6px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.card-title.deleted {
  color: #909399;
  text-decoration: line-through;
}
.card-excerpt {
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  margin-bottom: 8px;
}
.card-meta {
  font-size: 12px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.dot {
  color: #c0c4cc;
}
.meta-stat {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.meta-stat.img {
  color: #e6a23c;
}
.panel-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 4px;
}
.footer-hint {
  font-size: 12px;
  color: #c0c4cc;
}
</style>
