<template>
  <aside class="app-sidebar" :class="{ collapsed }">
    <!-- 用户信息区 -->
    <div class="sidebar-user">
      <div class="user-avatar-circle">
        {{ (authStore.userInfo?.name || authStore.userInfo?.username || '?').charAt(0) }}
      </div>
      <div v-if="!collapsed" class="user-info">
        <span class="user-name">{{ authStore.userInfo?.name || authStore.userInfo?.username }}</span>
        <span class="role-badge" :class="roleClass">{{ roleLabel }}</span>
      </div>
    </div>

    <!-- 分隔线 -->
    <div class="sidebar-divider"></div>

    <!-- 导航菜单 -->
    <nav class="sidebar-nav">
      <template v-for="item in menuItems" :key="item.key">
        <!-- 有子项的分组 -->
        <template v-if="item.children && item.children.length">
          <div v-if="!collapsed" class="nav-group-label">
            <el-icon v-if="item.icon" class="nav-group-icon"><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </div>
          <el-tooltip v-else :content="item.label" placement="right" :show-after="300">
            <div class="nav-group-icon-only">
              <el-icon><component :is="item.icon" /></el-icon>
            </div>
          </el-tooltip>
          <router-link
            v-for="child in item.children"
            :key="child.key"
            :to="child.path || ''"
            class="nav-item"
            :class="{ active: isRouteActive(child) }"
          >
            <div class="nav-item-icon">
              <el-icon><component :is="child.icon" /></el-icon>
            </div>
            <span v-if="!collapsed" class="nav-item-label">{{ child.label }}</span>
          </router-link>
        </template>

        <!-- 无子项的顶级导航 -->
        <router-link
          v-else-if="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ active: isRouteActive(item) }"
        >
          <div class="nav-item-icon">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <span v-if="!collapsed" class="nav-item-label">{{ item.label }}</span>
        </router-link>
      </template>
    </nav>

    <!-- 底部功能区 -->
    <div class="sidebar-divider"></div>
    <div class="sidebar-footer">
      <button class="footer-btn collapse-btn" @click="$emit('toggle-collapse')">
        <component :is="collapsed ? Expand : Fold" class="collapse-icon" />
        <span v-if="!collapsed" class="footer-btn-label">收起侧栏</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Expand, Fold } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import type { NavItem } from '@/config/navigation'

defineProps<{
  menuItems: NavItem[]
  collapsed: boolean
}>()

defineEmits<{
  'toggle-collapse': []
}>()

const route = useRoute()
const authStore = useAuthStore()

const roleLabel = computed(() => {
  const role = authStore.userInfo?.role
  if (role === 'admin') return '管理员'
  if (role === 'tutor') return '导师'
  if (role === 'student') return '学员'
  return '用户'
})

const roleClass = computed(() => {
  const role = authStore.userInfo?.role
  return role || 'student'
})

function isRouteActive(item: NavItem): boolean {
  const path = item.path
  if (!path) return false
  // exact=true 或根路径仅精确匹配：仪表盘路径恰好是其他菜单的父级
  // （如 /training、/training/tutor），不能走前缀匹配，否则访问任何子路由时仪表盘都会高亮。
  if (path === '/' || item.exact) return route.path === path
  return route.path === path || route.path.startsWith(path + '/')
}
</script>

<style scoped>
.app-sidebar {
  width: var(--sidebar-width);
  background: var(--color-bg-card);
  border-right: 1px solid var(--color-border-light);
  display: flex;
  flex-direction: column;
  transition: width var(--duration-normal) var(--ease-default);
  overflow: hidden;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: var(--z-fixed);
}

.app-sidebar.collapsed {
  width: var(--sidebar-collapsed-width);
}

/* 用户信息区 */
.sidebar-user {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-4) var(--space-3);
  flex-shrink: 0;
}

.app-sidebar.collapsed .sidebar-user {
  justify-content: center;
  padding: var(--space-4) var(--space-2);
}

.user-avatar-circle {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-full);
  background: var(--gradient-brand);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-base);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
  flex-shrink: 0;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.user-name {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.role-badge {
  font-size: 11px;
  font-weight: var(--font-medium);
  padding: 1px 6px;
  border-radius: var(--radius-full);
  width: fit-content;
  white-space: nowrap;
}

.role-badge.student {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

.role-badge.tutor {
  background: #ECFDF5;
  color: #059669;
}

.role-badge.admin {
  background: #F5F3FF;
  color: #7C3AED;
}

/* 分隔线 */
.sidebar-divider {
  height: 1px;
  background: var(--color-border-light);
  margin: 0 var(--space-4);
  flex-shrink: 0;
}

/* 导航菜单 */
.sidebar-nav {
  flex: 1;
  padding: var(--space-2) var(--space-2);
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.nav-group-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-muted);
  padding: var(--space-3) var(--space-3) var(--space-1);
  letter-spacing: 0.03em;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 6px;
}

.nav-group-icon {
  font-size: 14px;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.nav-group-icon-only {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-2);
  color: var(--color-text-muted);
  cursor: default;
}

.nav-group-icon-only .el-icon {
  font-size: 16px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: all var(--duration-fast) var(--ease-default);
  white-space: nowrap;
  position: relative;
  cursor: pointer;
}

.nav-item:hover {
  color: var(--color-primary-600);
  background: var(--color-bg-page);
}

.nav-item.active {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
  font-weight: var(--font-medium);
}

.nav-item.active::before {
  content: '';
  position: absolute;
  left: calc(var(--space-2) * -1);
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 18px;
  background: var(--color-primary-500);
  border-radius: 0 var(--radius-full) var(--radius-full) 0;
}

.app-sidebar.collapsed .nav-item {
  justify-content: center;
  padding: var(--space-3) var(--space-2);
}

.app-sidebar.collapsed .nav-item::before {
  left: 0;
}

.nav-item-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.nav-item-icon .el-icon {
  font-size: 18px;
}

.nav-item-label {
  font-size: var(--text-sm);
  font-weight: var(--font-normal);
}

/* 底部功能区 */
.sidebar-footer {
  padding: var(--space-2) var(--space-2);
  flex-shrink: 0;
}

.footer-btn {
  width: 100%;
  height: 36px;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-3);
  background: transparent;
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-text-tertiary);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
  font-family: var(--font-body);
}

.footer-btn:hover {
  background: var(--color-bg-page);
  color: var(--color-text-secondary);
}

.app-sidebar.collapsed .footer-btn {
  justify-content: center;
  padding: 0;
}

.collapse-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.footer-btn-label {
  font-size: var(--text-sm);
}
</style>
