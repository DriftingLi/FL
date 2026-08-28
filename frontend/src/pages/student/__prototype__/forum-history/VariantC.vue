<!-- PROTOTYPE — throwaway variant C: 紧凑清单 + 工具栏 -->
<template>
  <div class="variant-c">
    <div class="toolbar">
      <span class="toolbar-count">共 {{ items.length }} 条浏览记录</span>
      <span class="toolbar-hint">点击标题回到帖子 · MRU 去重 · 最多 50 条</span>
      <el-button size="small" type="danger" plain :disabled="items.length === 0" @click="$emit('clear')">一键清空</el-button>
    </div>
    <div v-if="items.length === 0" class="empty-wrap">
      <el-empty description="暂无浏览记录" />
    </div>
    <div v-else class="compact-list">
      <div
        v-for="item in items"
        :key="item.id"
        class="compact-row"
        :class="{ 'is-deleted': item.deleted }"
      >
        <div class="row-index">#{{ item.id }}</div>
        <div class="row-main" @click="!item.deleted && $emit('select', item.id)">
          <div class="row-title">
            <span :class="{ deleted: item.deleted }">{{ item.title }}</span>
            <el-tag v-if="item.deleted" size="small" type="danger" class="row-tag">已删除</el-tag>
            <el-tag v-else size="small" type="info" effect="plain" class="row-tag">{{ formatRelativeTime(item.viewedAt) }}</el-tag>
          </div>
          <div class="row-sub">
            <span>{{ item.author.username }}</span>
            <span class="dot">·</span>
            <span>浏览 {{ item.view_count }} · 回复 {{ item.reply_count }}</span>
            <span v-if="item.images_count" class="dot">·</span>
            <span v-if="item.images_count" class="img-text">{{ item.images_count }} 图</span>
          </div>
        </div>
        <div class="row-actions">
          <el-button size="small" text type="primary" :disabled="!!item.deleted" @click="$emit('select', item.id)">查看</el-button>
          <el-button size="small" text type="danger" @click="$emit('remove', item.id)">移除</el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatRelativeTime } from '@/utils/format'
import type { ForumHistoryItem } from './types'

defineProps<{ items: ForumHistoryItem[] }>()
defineEmits<{ select: [id: number]; remove: [id: number]; clear: [] }>()
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.toolbar-count {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}
.toolbar-hint {
  font-size: 12px;
  color: var(--color-text-tertiary);
  margin-right: auto;
}
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.compact-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.compact-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  transition: border-color 150ms var(--ease-default);
}
.compact-row:hover {
  border-color: var(--color-border);
}
.compact-row.is-deleted {
  opacity: 0.55;
  background: #fafafa;
}
.row-index {
  font-size: 12px;
  font-weight: 700;
  color: var(--color-text-muted);
  min-width: 42px;
}
.row-main {
  flex: 1;
  min-width: 0;
  cursor: pointer;
}
.row-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.row-title .deleted {
  color: var(--color-text-muted);
  text-decoration: line-through;
}
.row-tag {
  flex-shrink: 0;
}
.row-sub {
  font-size: 12px;
  color: var(--color-text-tertiary);
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  flex-wrap: wrap;
}
.dot {
  color: var(--color-text-muted);
}
.img-text {
  color: #e6a23c;
}
.row-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
</style>
