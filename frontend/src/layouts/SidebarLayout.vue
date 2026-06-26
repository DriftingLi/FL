<template>
  <div class="sidebar-layout">
    <aside
      class="sidebar"
      :class="{ 'sidebar-collapsed': collapsed, 'sidebar-mobile-open': mobileOpen }"
    >
      <div class="sidebar-header">
        <router-link to="/" class="logo-link">
          <div class="logo-icon">
            <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
              <rect width="28" height="28" rx="8" fill="url(#sidebar-logo-gradient)"/>
              <path d="M8 18L14 8L20 18H8Z" stroke="white" stroke-width="1.5" stroke-linejoin="round" fill="none"/>
              <circle cx="14" cy="15" r="2" fill="white"/>
              <defs>
                <linearGradient id="sidebar-logo-gradient" x1="0" y1="0" x2="28" y2="28">
                  <stop stop-color="#3B82F6"/>
                  <stop offset="1" stop-color="#60A5FA"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <span v-if="!collapsed" class="logo-text">{{ title }}</span>
        </router-link>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in menuItems"
          :key="item.path"
          :to="item.path"
          class="sidebar-item"
          :class="{ active: $route.path === item.path }"
        >
          <div class="sidebar-item-icon">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <span v-if="!collapsed" class="sidebar-item-label">{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button class="collapse-btn" @click="collapsed = !collapsed">
          <el-icon :size="18">
            <component :is="collapsed ? 'Expand' : 'Fold'" />
          </el-icon>
        </button>
      </div>
    </aside>

    <transition name="fade">
      <div v-if="mobileOpen" class="sidebar-overlay" @click="mobileOpen = false"></div>
    </transition>

    <div class="main-container" :class="{ 'main-collapsed': collapsed }">
      <header class="topbar">
        <div class="topbar-left">
          <button class="mobile-toggle" @click="mobileOpen = !mobileOpen">
            <el-icon :size="20"><Operation /></el-icon>
          </button>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item
              v-for="(crumb, index) in breadcrumbs"
              :key="index"
              :to="crumb.to"
            >
              {{ crumb.label }}
            </el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="topbar-right">
          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-avatar">
              <div class="avatar-circle">
                {{ (authStore.userInfo.name || authStore.userInfo.username || '?').charAt(0) }}
              </div>
              <span class="user-name">{{ authStore.userInfo.name || authStore.userInfo.username }}</span>
              <el-icon class="dropdown-arrow"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="main-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox } from 'element-plus'
import { Operation, ArrowDown, SwitchButton, Expand, Fold } from '@element-plus/icons-vue'

const props = defineProps({
  title: { type: String, default: '管理后台' },
  menuItems: { type: Array, required: true }
})

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const collapsed = ref(false)
const mobileOpen = ref(false)

const breadcrumbs = computed(() => {
  const crumbs = [{ label: '首页', to: '/' }]
  const currentItem = props.menuItems.find(item => route.path === item.path)
  if (currentItem) {
    crumbs.push({ label: currentItem.label, to: currentItem.path })
  }
  return crumbs
})

async function handleCommand(command) {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      authStore.clearAuthData()
      router.push('/login')
    } catch (e) {}
  }
}
</script>

<style scoped>
.sidebar-layout {
  min-height: 100vh;
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--gradient-sidebar);
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

.sidebar-collapsed {
  width: var(--sidebar-collapsed-width);
}

.sidebar-header {
  height: var(--header-height);
  display: flex;
  align-items: center;
  padding: 0 var(--space-4);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  text-decoration: none;
  overflow: hidden;
}

.logo-icon {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.logo-text {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: white;
  white-space: nowrap;
  letter-spacing: -0.01em;
}

.sidebar-nav {
  flex: 1;
  padding: var(--space-3) var(--space-3);
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.sidebar-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-3);
  border-radius: var(--radius-lg);
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  transition: all var(--duration-fast) var(--ease-default);
  white-space: nowrap;
  position: relative;
  cursor: pointer;
}

.sidebar-item:hover {
  color: rgba(255, 255, 255, 0.9);
  background: rgba(255, 255, 255, 0.06);
}

.sidebar-item.active {
  color: white;
  background: var(--color-bg-sidebar-active);
}

.sidebar-item.active::before {
  content: '';
  position: absolute;
  left: calc(var(--space-3) * -1);
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  background: var(--color-primary-400);
  border-radius: 0 var(--radius-full) var(--radius-full) 0;
}

.sidebar-item-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.sidebar-item-icon .el-icon {
  font-size: 18px;
}

.sidebar-item-label {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
}

.sidebar-footer {
  padding: var(--space-3);
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.collapse-btn {
  width: 100%;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.06);
  border: none;
  border-radius: var(--radius-md);
  color: rgba(255, 255, 255, 0.5);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-default);
}

.collapse-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.8);
}

.main-container {
  margin-left: var(--sidebar-width);
  transition: margin-left var(--duration-normal) var(--ease-default);
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.main-collapsed {
  margin-left: var(--sidebar-collapsed-width);
}

.topbar {
  height: var(--header-height);
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-bottom: 1px solid var(--color-border-light);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--space-6);
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.mobile-toggle {
  display: none;
  width: 40px;
  height: 40px;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--color-text-secondary);
  border-radius: var(--radius-md);
  transition: background var(--duration-fast);
}

.mobile-toggle:hover {
  background: var(--color-bg-page);
}

.topbar-right {
  display: flex;
  align-items: center;
}

.user-avatar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-full);
  transition: background var(--duration-fast) var(--ease-default);
}

.user-avatar:hover {
  background: var(--color-bg-page);
}

.avatar-circle {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-full);
  background: var(--gradient-brand);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  font-family: var(--font-display);
}

.user-name {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
}

.dropdown-arrow {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.main-content {
  background: var(--color-bg-page);
  padding: var(--space-6);
  flex: 1;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-normal) var(--ease-default);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@media screen and (max-width: 768px) {
  .sidebar {
    transform: translateX(-100%);
    width: var(--sidebar-width) !important;
    transition: transform var(--duration-normal) var(--ease-default);
  }

  .sidebar-mobile-open {
    transform: translateX(0) !important;
  }

  .sidebar-collapsed {
    transform: translateX(-100%);
  }

  .sidebar-overlay {
    display: block;
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(15, 23, 42, 0.5);
    z-index: calc(var(--z-fixed) - 1);
  }

  .main-container,
  .main-collapsed {
    margin-left: 0 !important;
  }

  .mobile-toggle {
    display: flex;
  }

  .user-name {
    display: none;
  }

  .dropdown-arrow {
    display: none;
  }

  .topbar {
    padding: 0 var(--space-4);
  }

  .main-content {
    padding: var(--space-4);
  }

  .collapse-btn {
    display: none;
  }
}
</style>
