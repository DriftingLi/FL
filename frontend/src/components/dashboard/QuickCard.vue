<template>
  <div class="quick-card">
    <div class="card-header">
      <h3 class="card-title">{{ title }}</h3>
      <router-link v-if="moreLink" :to="moreLink" class="card-more">
        查看全部
        <el-icon><ArrowRight /></el-icon>
      </router-link>
    </div>
    <div class="card-body">
      <div v-if="items.length > 0" class="card-list">
        <router-link
          v-for="(item, index) in items.slice(0, maxItems)"
          :key="index"
          :to="item.path || ''"
          class="card-list-item"
        >
          <div class="item-main">
            <span class="item-title">{{ item.title }}</span>
            <span v-if="item.subtitle" class="item-subtitle">{{ item.subtitle }}</span>
          </div>
          <div v-if="item.badge" class="item-badge" :style="item.badgeStyle || {}">
            {{ item.badge }}
          </div>
          <el-icon v-if="item.path" class="item-arrow"><ArrowRight /></el-icon>
        </router-link>
      </div>
      <div v-else class="card-empty">
        <span>{{ emptyText || '暂无数据' }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowRight } from '@element-plus/icons-vue'

export interface QuickCardItem {
  title: string
  subtitle?: string
  badge?: string
  badgeStyle?: Record<string, string>
  path?: string
}

withDefaults(defineProps<{
  title: string
  items: QuickCardItem[]
  moreLink?: string
  maxItems?: number
  emptyText?: string
}>(), {
  maxItems: 5,
  emptyText: '暂无数据'
})
</script>

<style scoped>
.quick-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  border: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-xs);
  overflow: hidden;
  transition: box-shadow var(--duration-normal) var(--ease-default);
}

.quick-card:hover {
  box-shadow: var(--shadow-sm);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border-light);
}

.card-title {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin: 0;
}

.card-more {
  display: flex;
  align-items: center;
  gap: 2px;
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-primary-500);
  text-decoration: none;
  transition: color var(--duration-fast);
}

.card-more:hover {
  color: var(--color-primary-700);
}

.card-more .el-icon {
  font-size: 12px;
}

.card-body {
  padding: var(--space-2) var(--space-5) var(--space-3);
}

.card-list {
  display: flex;
  flex-direction: column;
}

.card-list-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-2);
  border-radius: var(--radius-md);
  text-decoration: none;
  transition: background var(--duration-fast);
  cursor: pointer;
}

.card-list-item:hover {
  background: var(--color-bg-page);
}

.item-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item-title {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-subtitle {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-badge {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  padding: 2px 8px;
  border-radius: var(--radius-full);
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  white-space: nowrap;
  flex-shrink: 0;
}

.item-arrow {
  font-size: 12px;
  color: var(--color-text-muted);
  flex-shrink: 0;
  opacity: 0;
  transition: opacity var(--duration-fast);
}

.card-list-item:hover .item-arrow {
  opacity: 1;
}

.card-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-8) var(--space-4);
  color: var(--color-text-muted);
  font-size: var(--text-sm);
}
</style>
