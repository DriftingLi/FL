<template>
  <div class="ai-assistant-layout">
    <header class="ai-navbar">
      <div class="navbar-container">
        <router-link to="/" class="logo-link">
          <img src="/images/HRWAIlogo.jpg" alt="和润天下" class="logo-img" />
          <div class="logo-text-wrap">
            <span class="logo-text">和润天下</span>
            <span class="logo-sub">AI 助手</span>
          </div>
        </router-link>

        <div class="nav-cta">
          <router-link to="/" class="btn-back">
            <el-icon><ArrowLeft /></el-icon>
            <span>返回官网</span>
          </router-link>

          <el-dropdown @command="handleCommand" trigger="click">
            <div class="user-avatar">
              <div class="avatar-circle">{{ avatarLetter }}</div>
              <span class="user-name">{{ userName }}</span>
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
        </div>

        <button
          class="hamburger"
          :class="{ open: mobileOpen }"
          @click="mobileOpen = !mobileOpen"
          aria-label="菜单"
        >
          <span></span>
          <span></span>
          <span></span>
        </button>
      </div>

      <transition name="mobile-slide">
        <div v-if="mobileOpen" class="mobile-menu">
          <a :href="mainSiteUrl" class="mobile-link" @click="mobileOpen = false">
            <el-icon><ArrowLeft /></el-icon>
            <span>返回官网</span>
          </a>
          <router-link to="/profile" class="mobile-link" @click="mobileOpen = false">
            <el-icon><User /></el-icon>
            <span>个人中心</span>
          </router-link>
          <a href="javascript:void(0)" class="mobile-link" @click="handleCommand('logout')">
            <el-icon><SwitchButton /></el-icon>
            <span>退出登录</span>
          </a>
        </div>
      </transition>
    </header>

    <main class="ai-main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox } from 'element-plus'
import { ArrowLeft, ArrowDown, User, SwitchButton } from '@element-plus/icons-vue'
import { buildSubdomainUrl } from '@/utils/subdomain'

const router = useRouter()
const authStore = useAuthStore()
const mobileOpen = ref(false)
// 跨子域名跳回主域名（router-link to="/" 在当前子域名下会被路由守卫重定向）
const mainSiteUrl = computed(() => buildSubdomainUrl('main', '/'))

const userName = computed(() => authStore.userInfo?.name || authStore.userInfo?.username || '用户')
const avatarLetter = computed(() => userName.value.charAt(0).toUpperCase())

async function handleCommand(command: string) {
  mobileOpen.value = false
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
.ai-assistant-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-page);
  overflow: hidden;
}

.ai-navbar {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  background: var(--color-bg-card);
  border-bottom: 1px solid var(--color-border);
  box-shadow: var(--shadow-xs);
  flex-shrink: 0;
}

.navbar-container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: 0 var(--space-6);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  gap: var(--space-6);
}

.logo-link {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  text-decoration: none;
  flex-shrink: 0;
}

.logo-img {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  object-fit: cover;
}

.logo-text-wrap {
  display: flex;
  flex-direction: column;
  line-height: 1;
}

.logo-text {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-text-primary);
  letter-spacing: -0.025em;
}

.logo-sub {
  font-size: 10px;
  color: var(--color-text-tertiary);
  letter-spacing: 0.15em;
  text-transform: uppercase;
  margin-top: 2px;
}

.nav-cta {
  display: none;
  align-items: center;
  gap: var(--space-4);
  flex-shrink: 0;
}

.btn-back {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  min-height: 40px;
  padding: 8px 16px;
  background: transparent;
  color: var(--color-text-secondary);
  border: 1px solid var(--color-border-dark);
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  text-decoration: none;
  transition: all var(--duration-fast);
}

.btn-back:hover {
  border-color: var(--color-primary-500);
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
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-arrow {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.hamburger {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 5px;
  width: 36px;
  height: 36px;
  background: none;
  border: none;
  cursor: pointer;
  padding: 0;
}

.hamburger span {
  display: block;
  width: 22px;
  height: 2px;
  background: var(--color-text-primary);
  border-radius: 1px;
  transition: all var(--duration-normal);
}

.hamburger.open span:nth-child(1) {
  transform: rotate(45deg) translate(5px, 5px);
}

.hamburger.open span:nth-child(2) {
  opacity: 0;
}

.hamburger.open span:nth-child(3) {
  transform: rotate(-45deg) translate(5px, -5px);
}

.mobile-menu {
  display: flex;
  flex-direction: column;
  background: var(--color-bg-card);
  padding: var(--space-4) var(--space-6);
  border-top: 1px solid var(--color-border);
}

.mobile-link {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  text-decoration: none;
  padding: var(--space-4) 0;
  border-bottom: 1px solid var(--color-border-light);
  cursor: pointer;
}

.mobile-link:last-of-type {
  border-bottom: none;
}

.mobile-link:hover {
  color: var(--color-primary-600);
}

.mobile-slide-enter-active,
.mobile-slide-leave-active {
  transition: opacity var(--duration-normal), transform var(--duration-normal);
}

.mobile-slide-enter-from,
.mobile-slide-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.ai-main {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

@media (min-width: 768px) {
  .nav-cta {
    display: flex;
  }
  .hamburger {
    display: none;
  }
  .mobile-menu {
    display: none !important;
  }
}

@media (max-width: 767px) {
  .navbar-container {
    padding: 0 var(--space-4);
    height: 56px;
  }
  .logo-text {
    font-size: var(--text-base);
  }
}
</style>
