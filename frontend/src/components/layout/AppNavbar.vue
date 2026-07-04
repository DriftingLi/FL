<template>
  <header class="app-navbar">
    <div class="navbar-container">
      <div class="navbar-left">
        <router-link :to="homePath" class="logo-link">
          <div class="logo-icon">
            <svg width="32" height="32" viewBox="0 0 28 28" fill="none">
              <rect width="28" height="28" rx="8" fill="url(#logo-gradient)"/>
              <path d="M8 18L14 8L20 18H8Z" stroke="white" stroke-width="1.5" stroke-linejoin="round" fill="none"/>
              <circle cx="14" cy="15" r="2" fill="white"/>
              <defs>
                <linearGradient id="logo-gradient" x1="0" y1="0" x2="28" y2="28">
                  <stop stop-color="#0EA5E9"/>
                  <stop offset="1" stop-color="#14B8A6"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <span class="logo-text">ForkLift<span class="logo-accent">Pro</span></span>
        </router-link>

        <nav class="desktop-nav">
          <div
            v-for="item in menuItems"
            :key="item.key"
            class="nav-item"
            :class="{ 'has-children': item.children, active: isActive(item) }"
          >
            <router-link v-if="item.path" :to="item.path" class="nav-link">
              <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </router-link>
            <div v-else class="nav-link nav-trigger">
              <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
              <el-icon class="arrow-icon"><ArrowDown /></el-icon>
            </div>

            <div v-if="item.children" class="dropdown-panel">
              <div class="dropdown-grid">
                <router-link
                  v-for="child in item.children"
                  :key="child.key"
                  :to="child.path || ''"
                  class="dropdown-item"
                  :class="{ active: isRouteActive(child.path) }"
                >
                  <div class="dropdown-icon">
                    <el-icon><component :is="child.icon" /></el-icon>
                  </div>
                  <div class="dropdown-info">
                    <span class="dropdown-label">{{ child.label }}</span>
                  </div>
                </router-link>
              </div>
            </div>
          </div>
        </nav>
      </div>

      <div class="navbar-right">
        <button class="icon-btn notification-btn" title="通知">
          <el-icon><Bell /></el-icon>
        </button>

        <el-dropdown @command="handleCommand" trigger="click">
          <div class="user-avatar">
            <div class="avatar-circle">
              {{ (authStore.userInfo?.name || authStore.userInfo?.username || '?').charAt(0) }}
            </div>
            <span class="user-name">{{ authStore.userInfo?.name || authStore.userInfo?.username }}</span>
            <el-icon class="dropdown-arrow"><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="profile">
                <el-icon><User /></el-icon>个人中心
              </el-dropdown-item>
              <el-dropdown-item command="logout" divided>
                <el-icon><SwitchButton /></el-icon>退出登录
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <button
          class="hamburger-btn"
          :class="{ active: mobileOpen }"
          @click="mobileOpen = !mobileOpen"
        >
          <span></span>
          <span></span>
          <span></span>
        </button>
      </div>
    </div>

    <transition name="mobile-fade">
      <div v-if="mobileOpen" class="mobile-overlay" @click="mobileOpen = false">
        <nav class="mobile-nav" @click.stop>
          <div class="mobile-nav-header">
            <span class="mobile-nav-title">菜单</span>
            <button class="mobile-close" @click="mobileOpen = false">
              <el-icon><Close /></el-icon>
            </button>
          </div>

          <div class="mobile-nav-body">
            <template v-for="item in flattenedMobileNav" :key="item.key">
              <router-link
                v-if="item.path"
                :to="item.path"
                class="mobile-nav-item"
                :class="{ active: isRouteActive(item.path), indent: item.isChild }"
                @click="mobileOpen = false"
              >
                <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
                <span>{{ item.label }}</span>
              </router-link>
              <div v-else class="mobile-group-label">
                <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
                <span>{{ item.label }}</span>
              </div>
            </template>
          </div>
        </nav>
      </div>
    </transition>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox } from 'element-plus'
import {
  ArrowDown,
  Bell,
  User,
  SwitchButton,
  Close
} from '@element-plus/icons-vue'
import type { NavItem } from '@/config/navigation'

const props = defineProps<{
  menuItems: NavItem[]
}>()

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const mobileOpen = ref(false)

const homePath = computed(() => {
  const role = authStore.userInfo?.role
  if (role === 'admin') return '/admin/dashboard'
  if (role === 'tutor') return '/training/tutor'
  if (role === 'student') return '/training'
  return '/'
})

function isRouteActive(path?: string): boolean {
  if (!path) return false
  if (path === '/') {
    return route.path === '/'
  }
  return route.path === path || route.path.startsWith(path + '/')
}

function isActive(item: NavItem): boolean {
  if (item.path && isRouteActive(item.path)) return true
  if (item.children) {
    return item.children.some(child => isRouteActive(child.path))
  }
  return false
}

const flattenedMobileNav = computed(() => {
  const result: (NavItem & { isChild?: boolean })[] = []
  props.menuItems.forEach(item => {
    if (item.children) {
      result.push({ ...item, path: undefined })
      item.children.forEach(child => {
        result.push({ ...child, isChild: true })
      })
    } else {
      result.push(item)
    }
  })
  return result
})

async function handleCommand(command: string) {
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      authStore.clearAuthData()
      router.push('/login')
    } catch (e) {
      // cancelled
    }
  } else if (command === 'profile') {
    router.push('/profile')
  }
}
</script>

<style scoped>
.app-navbar {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  padding: var(--space-3) var(--space-4);
}

.navbar-container {
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: var(--container-page);
  margin: 0 auto;
  padding: 0 var(--space-5);
  height: var(--header-height);
  background: var(--navbar-bg, rgba(255, 255, 255, 0.9));
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--color-border, #E2E8F0);
}

.navbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-8);
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  text-decoration: none;
  flex-shrink: 0;
}

.logo-icon {
  display: flex;
  align-items: center;
}

.logo-text {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  letter-spacing: -0.02em;
}

.logo-accent {
  color: var(--color-primary-500);
}

.desktop-nav {
  display: flex;
  align-items: center;
  gap: var(--space-1);
}

.nav-item {
  position: relative;
}

.nav-link {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: all var(--duration-fast) var(--ease-default);
  white-space: nowrap;
  cursor: pointer;
}

.nav-link:hover,
.nav-item.active > .nav-link {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
}

.nav-trigger .arrow-icon {
  font-size: 12px;
  margin-left: 2px;
  transition: transform var(--duration-fast);
}

.nav-item:hover .arrow-icon {
  transform: rotate(180deg);
}

.dropdown-panel {
  position: absolute;
  top: calc(100% + 8px);
  left: 50%;
  transform: translateX(-50%) translateY(-6px);
  min-width: 200px;
  padding: var(--space-2);
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl);
  opacity: 0;
  visibility: hidden;
  transition: all var(--duration-fast) var(--ease-default);
}

.nav-item.has-children:hover .dropdown-panel,
.nav-item.has-children:focus-within .dropdown-panel {
  opacity: 1;
  visibility: visible;
  transform: translateX(-50%) translateY(0);
}

.dropdown-grid {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-3);
  border-radius: var(--radius-lg);
  text-decoration: none;
  color: var(--color-text-secondary);
  transition: all var(--duration-fast);
}

.dropdown-item:hover,
.dropdown-item.active {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

.dropdown-icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: var(--color-bg-page);
  color: var(--color-primary-500);
}

.dropdown-label {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
}

.navbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.icon-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background var(--duration-fast);
}

.icon-btn:hover {
  background: var(--color-bg-page);
  color: var(--color-primary-600);
}

.user-avatar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-full);
  transition: background var(--duration-fast);
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

.hamburger-btn {
  display: none;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 36px;
  height: 36px;
  background: none;
  border: none;
  cursor: pointer;
  gap: 5px;
  padding: 0;
  border-radius: var(--radius-md);
  transition: background var(--duration-fast);
}

.hamburger-btn:hover {
  background: var(--color-bg-page);
}

.hamburger-btn span {
  display: block;
  width: 18px;
  height: 2px;
  background: var(--color-text-secondary);
  border-radius: 2px;
  transition: all var(--duration-normal) var(--ease-default);
}

.hamburger-btn.active span:nth-child(1) {
  transform: rotate(45deg) translate(5px, 5px);
}

.hamburger-btn.active span:nth-child(2) {
  opacity: 0;
}

.hamburger-btn.active span:nth-child(3) {
  transform: rotate(-45deg) translate(5px, -5px);
}

.mobile-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  z-index: var(--z-modal-backdrop);
  backdrop-filter: blur(4px);
}

.mobile-nav {
  position: absolute;
  top: 0;
  right: 0;
  width: min(320px, 85vw);
  height: 100%;
  background: var(--color-bg-card);
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
}

.mobile-nav-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--color-border-light);
}

.mobile-nav-title {
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
}

.mobile-close {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-md);
  background: var(--color-bg-page);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.mobile-nav-body {
  flex: 1;
  padding: var(--space-4);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.mobile-nav-item,
.mobile-group-label {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-lg);
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: all var(--duration-fast);
}

.mobile-nav-item.active,
.mobile-nav-item:hover {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
}

.mobile-nav-item.indent {
  padding-left: var(--space-8);
}

.mobile-group-label {
  color: var(--color-text-tertiary);
  font-size: var(--text-sm);
  margin-top: var(--space-2);
  pointer-events: none;
}

.mobile-fade-enter-active,
.mobile-fade-leave-active {
  transition: opacity var(--duration-normal);
}

.mobile-fade-enter-from,
.mobile-fade-leave-to {
  opacity: 0;
}

@media screen and (max-width: 1024px) {
  .navbar-left {
    gap: var(--space-4);
  }

  .desktop-nav {
    display: none;
  }

  .hamburger-btn {
    display: flex;
  }

  .notification-btn {
    display: none;
  }

  .user-name,
  .dropdown-arrow {
    display: none;
  }
}

@media screen and (max-width: 480px) {
  .navbar-container {
    padding: 0 var(--space-4);
    height: 60px;
  }

  .logo-text {
    font-size: var(--text-lg);
  }
}
</style>
