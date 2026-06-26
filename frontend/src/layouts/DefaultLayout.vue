<template>
  <div class="default-layout">
    <header class="navbar">
      <div class="navbar-inner">
        <div class="navbar-left">
          <router-link to="/" class="logo-link">
            <div class="logo-icon">
              <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
                <rect width="28" height="28" rx="8" fill="url(#logo-gradient)"/>
                <path d="M8 18L14 8L20 18H8Z" stroke="white" stroke-width="1.5" stroke-linejoin="round" fill="none"/>
                <circle cx="14" cy="15" r="2" fill="white"/>
                <defs>
                  <linearGradient id="logo-gradient" x1="0" y1="0" x2="28" y2="28">
                    <stop stop-color="#1E40AF"/>
                    <stop offset="1" stop-color="#2563EB"/>
                  </linearGradient>
                </defs>
              </svg>
            </div>
            <span class="logo-text">ForkLift<span class="logo-accent">Pro</span></span>
          </router-link>
          <nav class="desktop-nav">
            <router-link
              v-for="item in menuItems"
              :key="item.path"
              :to="item.path"
              class="nav-link"
              :class="{ active: $route.path === item.path }"
            >
              <el-icon><component :is="item.icon" /></el-icon>
              <span>{{ item.label }}</span>
            </router-link>
          </nav>
        </div>

        <div class="navbar-right">
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
            :class="{ active: mobileMenuOpen }"
            @click="mobileMenuOpen = !mobileMenuOpen"
          >
            <span></span>
            <span></span>
            <span></span>
          </button>
        </div>
      </div>
    </header>

    <transition name="slide-down">
      <div v-if="mobileMenuOpen" class="mobile-overlay" @click="mobileMenuOpen = false">
        <nav class="mobile-nav" @click.stop>
          <router-link
            v-for="item in menuItems"
            :key="item.path"
            :to="item.path"
            class="mobile-nav-item"
            :class="{ active: $route.path === item.path }"
            @click="mobileMenuOpen = false"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
          </router-link>
        </nav>
      </div>
    </transition>

    <main class="main-content">
      <div class="content-wrapper">
        <router-view />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import {
  HomeFilled, Notebook, MagicStick, SetUp, EditPen,
  User, SwitchButton, ArrowDown, Document
} from '@element-plus/icons-vue'

const authStore = useAuthStore()
const router = useRouter()
const mobileMenuOpen = ref(false)

const menuItems = [
  { path: '/', label: '首页', icon: HomeFilled },
  { path: '/courses', label: '课程中心', icon: Notebook },
  { path: '/question-bank', label: '题库练习', icon: EditPen },
  { path: '/level-exam', label: '考试中心', icon: Document },
  { path: '/ai-generate', label: 'AI助手', icon: MagicStick },
  { path: '/practice', label: '虚拟实操', icon: SetUp },
  { path: '/valuation', label: '残值评估', icon: DataAnalysis },
  { path: '/profile', label: '个人中心', icon: User }
]

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
  } else if (command === 'profile') {
    router.push('/profile')
  }
}
</script>

<style scoped>
.default-layout {
  min-height: 100vh;
  background: var(--color-bg-page);
  display: flex;
  flex-direction: column;
}

.navbar {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  padding: var(--space-3) var(--space-4);
}

.navbar-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: var(--container-2xl);
  margin: 0 auto;
  padding: 0 var(--space-5);
  height: var(--header-height);
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-sm);
  border: 1px solid rgba(226, 232, 240, 0.6);
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
  position: relative;
}

.nav-link:hover {
  color: var(--color-primary-500);
  background: var(--color-primary-50);
}

.nav-link.active {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
}

.nav-link.active::after {
  content: '';
  position: absolute;
  bottom: -2px;
  left: 50%;
  transform: translateX(-50%);
  width: 16px;
  height: 2px;
  background: var(--color-primary-500);
  border-radius: var(--radius-full);
}

.nav-link .el-icon {
  font-size: 16px;
}

.navbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
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
  transition: transform var(--duration-fast);
}

.hamburger-btn {
  display: none;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  width: 40px;
  height: 40px;
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
  width: 20px;
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
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(15, 23, 42, 0.4);
  z-index: var(--z-modal-backdrop);
  backdrop-filter: blur(4px);
}

.mobile-nav {
  position: absolute;
  top: 0;
  right: 0;
  width: 280px;
  height: 100%;
  background: var(--color-bg-card);
  padding: var(--space-6) var(--space-4);
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.mobile-nav-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-radius: var(--radius-lg);
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: all var(--duration-fast) var(--ease-default);
}

.mobile-nav-item:hover,
.mobile-nav-item.active {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
}

.mobile-nav-item .el-icon {
  font-size: 20px;
}

.main-content {
  padding: var(--space-6) var(--space-4);
  flex: 1;
}

.content-wrapper {
  max-width: var(--container-2xl);
  margin: 0 auto;
}

.slide-down-enter-active,
.slide-down-leave-active {
  transition: opacity var(--duration-normal) var(--ease-default);
}

.slide-down-enter-from,
.slide-down-leave-to {
  opacity: 0;
}

@media screen and (max-width: 768px) {
  .navbar {
    padding: var(--space-2) var(--space-3);
  }

  .navbar-inner {
    padding: 0 var(--space-4);
    border-radius: var(--radius-lg);
  }

  .navbar-left {
    gap: var(--space-4);
  }

  .logo-text {
    font-size: var(--text-lg);
  }

  .desktop-nav {
    display: none;
  }

  .hamburger-btn {
    display: flex;
  }

  .user-name {
    display: none;
  }

  .dropdown-arrow {
    display: none;
  }

  .main-content {
    padding: var(--space-4) var(--space-3);
  }
}

@media screen and (max-width: 480px) {
  .logo-text {
    font-size: var(--text-base);
  }

  .navbar-inner {
    padding: 0 var(--space-3);
  }

  .main-content {
    padding: var(--space-3) var(--space-2);
  }
}
</style>
