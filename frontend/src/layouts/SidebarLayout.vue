<template>
  <div class="sidebar-layout">
    <AppSidebar
      :menu-items="menuItems"
      :collapsed="collapsed"
      :mobile-open="mobileOpen"
      :theme="props.sidebarTheme"
      :density="props.sidebarDensity"
      :class="{ 'sidebar-mobile-open': mobileOpen }"
      @toggle-collapse="handleToggleCollapse"
    >
      <template #top="{ collapsed: topCollapsed }">
        <slot name="top" :collapsed="topCollapsed" />
      </template>
    </AppSidebar>

    <transition name="fade">
      <div v-if="mobileOpen" class="sidebar-overlay" @click="mobileOpen = false"></div>
    </transition>

    <!-- 明暗模式切换（三态：跟随系统 / 浅色 / 深色）。放内容区右上角固定位，
         全站（学员/导师/管理端）共用本布局，一处改动全局生效。 -->
    <button
      class="theme-toggle"
      :aria-label="themeLabel"
      :title="themeLabel"
      @click="themeStore.cycle()"
    >
      <el-icon v-if="themeStore.resolved === 'dark'" :size="18"><Moon /></el-icon>
      <svg v-else-if="themeStore.mode === 'system'" class="theme-half-icon" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M12 3a9 9 0 0 0 0 18V3Z" fill="currentColor" opacity="0.9"/>
        <path d="M12 6a6 6 0 0 0 0 12 8 8 0 0 1 0-12Z" fill="var(--color-bg-page)"/>
        <path d="M12 3a9 9 0 0 1 9 9 9 9 0 0 1-9 9V3Z" fill="none" stroke="currentColor" stroke-width="1.5"/>
        <path d="M16.8 6.2l1.4-1.4M21 12h2M16.8 17.8l1.4 1.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
      <el-icon v-else :size="18"><Sunny /></el-icon>
    </button>

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
      <main class="main-content" :class="{ 'content-narrow': props.contentWidth === 'narrow' }">
        <!-- 内层 router-view + transition：
             App.vue 已用 matched[0]?.path 做 key 锁住外层布局不重挂，
             这里用 fullPath 做 key 让同布局下的子页面也能走 180ms 淡入淡出。
             不用 keep-alive，避免课程章节页/考试页等带副作用的状态被缓存。 -->
        <router-view v-slot="{ Component: Inner, route: r }">
          <transition name="inner-fade" mode="out-in">
            <component :is="Inner" :key="r.fullPath" />
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Operation, Moon, Sunny } from '@element-plus/icons-vue'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import { useThemeStore } from '@/stores/theme'
import type { NavItem } from '@/config/navigation'

const themeStore = useThemeStore()

/** 按钮提示：显示当前态与点击后的下一态 */
const themeLabel = computed(() => {
  if (themeStore.mode === 'system') return '跟随系统（点击切换为浅色）'
  return themeStore.resolved === 'dark' ? '深色模式（点击切换为跟随系统）' : '浅色模式（点击切换为深色）'
})

const props = withDefaults(
  defineProps<{
    menuItems: NavItem[]
    showFooter?: boolean
    /**
     * 内容区宽度。
     * - `full`：**默认值 = 改造前行为**，内容铺满可用宽度
     * - `narrow`：内容限宽 1280px 居中，宽屏下避免行过长
     */
    contentWidth?: 'full' | 'narrow'
    /** 透传给 AppSidebar，不传则沿用其默认值 `dark`（2026-08-31 起统一三端恒深石墨青侧栏） */
    sidebarTheme?: 'light' | 'dark'
    /** 透传给 AppSidebar，不传则沿用其默认值 `default` */
    sidebarDensity?: 'default' | 'compact'
  }>(),
  { showFooter: false, contentWidth: 'full', sidebarTheme: 'dark' }
)

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

/* narrow：内容限宽 1280px 居中。
   router-view + transition(mode="out-in") 同一时刻只渲染一个页面根元素，
   因此 > * 精确命中页面根节点，无需额外包一层 wrapper。 */
.main-content.content-narrow {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.main-content.content-narrow > * {
  width: 100%;
  max-width: 1280px;
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
  transition: background var(--duration-fast) var(--ease-default), box-shadow var(--duration-fast) var(--ease-default);
}

.mobile-fab:hover {
  background: var(--color-bg-page);
}

.mobile-fab:active {
  box-shadow: 0 2px 6px rgba(15, 23, 42, 0.16);
}

/* 明暗模式切换：内容区右上角固定，与左侧移动端菜单按钮不冲突 */
.theme-toggle {
  position: fixed;
  top: var(--space-4);
  right: var(--space-4);
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-full);
  box-shadow: var(--shadow-md);
  cursor: pointer;
  color: var(--color-text-primary);
  z-index: var(--z-sticky);
  transition:
    background var(--duration-fast) var(--ease-default),
    box-shadow var(--duration-fast) var(--ease-default),
    transform var(--duration-tap) var(--ease-default);
}

.theme-toggle:hover {
  background: var(--color-bg-page);
}

.theme-toggle:active {
  transform: scale(0.94);
}

.theme-half-icon {
  width: 18px;
  height: 18px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-normal) var(--ease-default);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 内层子页面过渡：同布局内切换路由时给中间区域一个 180ms 淡入淡出 + 6px 上移。
   比纯 opacity 多一点方向感，进出方向一致（都是向上）所以 out-in 模式下不会打架。 */
.inner-fade-enter-active,
.inner-fade-leave-active {
  transition:
    opacity 180ms var(--ease-default),
    transform 180ms var(--ease-default);
}

.inner-fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.inner-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
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

  /* 移动端加大触控目标 */
  .theme-toggle {
    width: 44px;
    height: 44px;
  }

  .main-content {
    padding: var(--space-4);
    /* 顶部留出浮动按钮的空间，避免内容被遮挡 */
    padding-top: calc(var(--space-4) + 44px);
  }
}
</style>
