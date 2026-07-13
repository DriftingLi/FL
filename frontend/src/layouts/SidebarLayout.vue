<template>
  <div class="sidebar-layout">
    <AppSidebar
      :menu-items="menuItems"
      :collapsed="collapsed"
      :class="{ 'sidebar-mobile-open': mobileOpen }"
      @toggle-collapse="collapsed = !collapsed"
    />

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
            <el-breadcrumb-item :to="homePath">工作台</el-breadcrumb-item>
            <el-breadcrumb-item v-if="currentLabel">{{ currentLabel }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="topbar-right">
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
        </div>
      </header>

      <main class="main-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessageBox } from 'element-plus'
import { Operation, ArrowDown, User, SwitchButton } from '@element-plus/icons-vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import type { NavItem } from '@/config/navigation'

const props = withDefaults(defineProps<{
  menuItems: NavItem[]
  showFooter?: boolean
}>(), {
  showFooter: false
})

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

// 折叠状态持久化
const collapsed = ref(false)
onMounted(() => {
  const saved = localStorage.getItem('sidebar-collapsed')
  if (saved === 'true') {
    collapsed.value = true
  }
})

// 监听折叠变化持久化
import { watch } from 'vue'
watch(collapsed, (val) => {
  localStorage.setItem('sidebar-collapsed', String(val))
})

const mobileOpen = ref(false)

const homePath = computed(() => {
  const role = authStore.userInfo?.role
  if (role === 'admin') return '/admin/dashboard'
  if (role === 'tutor') return '/training/tutor'
  if (role === 'student') return '/training'
  return '/'
})

const currentLabel = computed(() => {
  return (route.meta?.navLabel as string) || ''
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
.sidebar-layout {
  min-height: 100vh;
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

.sidebar-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(15, 23, 42, 0.5);
  z-index: calc(var(--z-fixed) - 1);
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
  .main-container,
  .main-collapsed {
    margin-left: 0 !important;
  }

  /* 移动端侧边栏：默认隐藏，滑入显示 */
  :deep(.app-sidebar) {
    transform: translateX(-100%);
    transition: transform var(--duration-normal) var(--ease-default), width var(--duration-normal) var(--ease-default) !important;
    width: var(--sidebar-width) !important;
  }

  :deep(.app-sidebar.sidebar-mobile-open) {
    transform: translateX(0) !important;
  }

  .mobile-toggle {
    display: flex;
  }

  .user-name,
  .dropdown-arrow {
    display: none;
  }

  .topbar {
    padding: 0 var(--space-4);
  }

  .main-content {
    padding: var(--space-4);
  }
}
</style>
