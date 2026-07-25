<template>
  <div class="sidebar-layout">
    <AppSidebar
      :menu-items="menuItems"
      :collapsed="collapsed"
      :mobile-open="mobileOpen"
      :class="{ 'sidebar-mobile-open': mobileOpen }"
      @toggle-collapse="handleToggleCollapse"
    />

    <transition name="fade">
      <div v-if="mobileOpen" class="sidebar-overlay" @click="mobileOpen = false"></div>
    </transition>

    <!-- 移动端浮动菜单按钮（替代原顶栏的 mobile-toggle） -->
    <button
      v-if="!mobileOpen"
      class="mobile-fab"
      aria-label="打开菜单"
      @click="mobileOpen = true"
    >
      <el-icon :size="20"><Operation /></el-icon>
    </button>

    <div class="main-container" :class="{ 'main-collapsed': collapsed }">
      <main class="main-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Operation } from '@element-plus/icons-vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import type { NavItem } from '@/config/navigation'

withDefaults(defineProps<{
  menuItems: NavItem[]
  showFooter?: boolean
}>(), {
  showFooter: false
})

const route = useRoute()

// 折叠状态持久化：同步读取初始值，避免组件重新挂载时 false→true 跳变引发拉伸动画
const collapsed = ref(localStorage.getItem('sidebar-collapsed') === 'true')

// 监听折叠变化持久化
watch(collapsed, (val) => {
  localStorage.setItem('sidebar-collapsed', String(val))
})

const mobileOpen = ref(false)

// 移动端侧边栏打开时，底部按钮关闭侧边栏；桌面端则切换折叠状态
function handleToggleCollapse() {
  if (mobileOpen.value) {
    mobileOpen.value = false
  } else {
    collapsed.value = !collapsed.value
  }
}

// 路由切换时自动关闭移动端侧边栏，避免导航后菜单仍遮挡内容
watch(() => route.path, () => {
  mobileOpen.value = false
})
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

/* 移动端浮动菜单按钮 */
.mobile-fab {
  display: none;
  position: fixed;
  top: var(--space-4);
  left: var(--space-4);
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-full);
  box-shadow: var(--shadow-md, 0 4px 12px rgba(15, 23, 42, 0.12));
  cursor: pointer;
  color: var(--color-text-primary);
  z-index: var(--z-sticky);
  transition: background var(--duration-fast), box-shadow var(--duration-fast);
}

.mobile-fab:hover {
  background: var(--color-bg-page);
}

.mobile-fab:active {
  box-shadow: 0 2px 6px rgba(15, 23, 42, 0.16);
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

  .mobile-fab {
    display: flex;
  }

  .main-content {
    padding: var(--space-4);
    /* 顶部留出浮动按钮的空间，避免内容被遮挡 */
    padding-top: calc(var(--space-4) + 44px);
  }
}
</style>
