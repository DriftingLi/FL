<!-- PROTOTYPE — throwaway variant A: ForumPage 同款 tab 内联列表 -->
<template>
  <div class="variant-a">
    <div v-if="items.length === 0" class="empty-wrap">
      <el-empty description="暂无浏览记录，去论坛看看帖子吧" />
    </div>
    <div v-else class="topic-list">
      <div
        v-for="item in items"
        :key="item.id"
        class="topic-item"
        :class="{ 'is-deleted': item.deleted }"
        @click="!item.deleted && $emit('select', item.id)"
      >
        <div class="topic-author">
          <el-avatar :size="42" :src="item.author.avatar_url || undefined" class="author-avatar">
            {{ authorLetter(item.author.username) }}
          </el-avatar>
        </div>
        <div class="topic-main">
          <div class="topic-title-row">
            <el-tag v-if="item.deleted" size="small" type="danger">已删除</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">已浏览</el-tag>
            <h3 class="topic-title" :class="{ deleted: item.deleted }">{{ item.title }}</h3>
          </div>
          <p class="topic-excerpt">{{ item.excerpt }}</p>
          <div class="topic-meta">
            <span class="author-name">{{ item.author.username }}</span>
            <span class="meta-divider">·</span>
            <span>{{ formatRelativeTime(item.viewedAt) }} 浏览</span>
            <span class="meta-right">
              <span v-if="item.images_count > 0" class="img-mark">
                <el-icon><Picture /></el-icon>
                {{ item.images_count }}
              </span>
              <el-icon><View /></el-icon>
              {{ item.view_count }}
              <el-icon class="reply-icon"><ChatDotRound /></el-icon>
              {{ item.reply_count }}
            </span>
          </div>
        </div>
        <el-button
          size="small"
          text
          type="danger"
          class="remove-btn"
          @click.stop="$emit('remove', item.id)"
        >
          移除
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Picture, View, ChatDotRound } from '@element-plus/icons-vue'
import { formatRelativeTime } from '@/utils/format'
import type { ForumHistoryItem } from './types'

defineProps<{ items: ForumHistoryItem[] }>()
defineEmits<{ select: [id: number]; remove: [id: number] }>()

function authorLetter(name: string) {
  return (name || '?').charAt(0).toUpperCase()
}
</script>

<style scoped>
.empty-wrap {
  padding: 20px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.topic-list {
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  overflow: hidden;
}
.topic-item {
  display: flex;
  gap: 14px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--color-border-light);
  cursor: pointer;
  transition: background 0.2s;
  align-items: center;
}
.topic-item:hover {
  background: var(--color-bg-page);
}
.topic-item:last-child {
  border-bottom: none;
}
.topic-item.is-deleted {
  opacity: 0.6;
  cursor: default;
  background: var(--color-bg-page);
}
.topic-author {
  flex-shrink: 0;
}
.topic-main {
  flex: 1;
  min-width: 0;
}
.topic-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.topic-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.topic-title.deleted {
  color: var(--color-text-tertiary);
  text-decoration: line-through;
}
.topic-excerpt {
  color: var(--color-text-secondary);
  font-size: 13px;
  margin: 6px 0 8px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.topic-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.meta-divider {
  color: var(--color-border-dark);
}
.meta-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}
.reply-icon {
  margin-left: 10px;
}
.img-mark {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-right: 10px;
  color: var(--color-warning);
}
.remove-btn {
  flex-shrink: 0;
}
</style>
