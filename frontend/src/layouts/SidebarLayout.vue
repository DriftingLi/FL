<template>
  <div class="sidebar-layout">
    <!-- 顶栏（棋盘格第一行，ADR-0072）：左格与侧栏同宽并联动收缩；中区留给工作区控件。
         汉堡在移动端接管唤出侧栏（原浮动按钮已退役）。 -->
    <AppTopBar :collapsed="collapsed" @open-mobile="mobileOpen = !mobileOpen">
      <template #center>
        <slot name="topbar" :collapsed="collapsed" />
      </template>
    </AppTopBar>

    <AppSidebar
      :menu-items="menuItems"
      :collapsed="collapsed"
      :mobile-open="mobileOpen"
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

    <div class="main-container" :class="{ 'main-collapsed': collapsed }">
      <!-- 内容列顶部插槽（#1620）：管理端在此挂标签栏，其余三端不传即不渲染。
           它必须在 .main-content **之外**（#1633）：内容区有 --space-6 的内边距，插槽留在里面
           就会离顶栏下沿差 24px（线上形态：标签栏浮在内容中间，不像顶栏的第二行）。 -->
      <div v-if="$slots['content-header']" class="content-header">
        <slot name="content-header" />
      </div>

      <main class="main-content" :class="{ 'content-narrow': props.contentWidth === 'narrow' }">
        <!-- 内层 router-view + transition：
             App.vue 已用 matched[0]?.path 做 key 锁住外层布局不重挂，
             这里用 fullPath 做 key 让同布局下的子页面也能走 180ms 淡入淡出。

             keep-alive（#1620）：**只缓存 include 名单里的组件**（名单来自描述符的
             keepAlive: true，当前只有管理端）。名单为空时 keep-alive 等价于直通 ——
             学员/讲师/招聘三端因此保持「一切换即销毁」的既有语义（考试页那类带副作用的状态
             不会被缓存）。缓存页的 key 取**路由名**而不是 fullPath：同一页的不同查询参数
             （翻页、筛选）不该各缓存一份实例。

             **刻意不用 mode="out-in"（#1631）**：out-in 的放行条件是「离场动画完成」的回调
             （BaseTransition 的 leavingHooks.afterLeave），而**根节点不是单个元素**的页面 ——
             管理端「管理员管理」的根是多分支组件 UiAsyncSection（Fragment 根，Vue 会警告
             "renders non-element root node that cannot be animated"）—— 在渲染器里走 Fragment
             分支：deactivate 走的是 move(..., LEAVE)，Fragment 那一支只搬 DOM、不碰过渡钩子，
             回调永不触发 ⇒ state.isLeaving 永久为真、内容列只剩空占位符。
             线上形态：从该页切走那一刻起内容区永久空白（标签栏与侧栏都在），刷新才恢复。
             正确性不该依赖「页面模板恰好单根」这条约定，故改成默认模式 + 只保留进入动画
             （离场即时隐藏，见样式；缓存页的离场本来也没有可播的动画）。 -->
        <router-view v-slot="{ Component: Inner, route: r }">
          <transition name="inner-fade">
            <keep-alive :include="props.keepAliveNames">
              <component :is="Inner" :key="routeKey(r)" />
            </keep-alive>
          </transition>
        </router-view>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import AppTopBar from '@/components/layout/AppTopBar.vue'
import type { NavItem } from '@/config/navigation'

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
    /** 透传给 AppSidebar，不传则沿用其默认值 `default` */
    sidebarDensity?: 'default' | 'compact'
    /**
     * 进 keep-alive 缓存的组件名（#1620）。缺省空数组 = 不缓存任何页面（= 改造前行为），
     * 由管理端布局传入（名单由页面描述符派生，见 config/navigation.ts 的 keepAliveNames）。
     */
    keepAliveNames?: string[]
  }>(),
  { showFooter: false, contentWidth: 'full', keepAliveNames: () => [] }
)

const route = useRoute()

/**
 * 缓存页的 key 取路由名（同名页共用一份缓存实例），未缓存页仍用 fullPath
 * （保持 180ms 淡入淡出对同页不同参数的既有行为）。
 */
function routeKey(r: { name?: unknown; fullPath: string }): string {
  const name = typeof r.name === 'string' ? r.name : ''
  return name && props.keepAliveNames.includes(name) ? name : r.fullPath
}

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
  /* 顶栏恒在视口内（fixed）：内容区整体下移一个顶栏高度（#1619） */
  padding-top: var(--topbar-height);
  transition: margin-left var(--duration-normal) var(--ease-default);
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.main-collapsed {
  margin-left: var(--sidebar-collapsed-width);
}

/* 内容列顶部（当前只有管理端的标签栏）：贴顶栏下沿 + 随页面滚动固定在顶栏之下。
   sticky 而不是 fixed：占位仍在文档流里（内容不会被压在标签栏下面），
   而它的容器 .main-container 没有 overflow，sticky 生效。
   底色与内容区同底，分隔线仍只有顶栏下沿那一条（十字细线），这里不再加第二条。 */
.content-header {
  position: sticky;
  top: var(--topbar-height);
  z-index: var(--z-sticky);
  background: var(--color-bg-page);
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
  /* 从顶栏下沿开始：抽屉打开时顶栏仍可点（汉堡即开关，再点一次收起） */
  top: var(--topbar-height);
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

/* 内层子页面过渡：同布局内切换路由时给中间区域一个 180ms 淡入 + 6px 上移（方向感）。
   离场只做即时隐藏，不播动画（#1631）：缓存页的离场是 KeepAlive 把 DOM 搬进隐藏容器，
   没有可播的动画；留一条 180ms 的 leave 规则反而会让旧页面与新页面在文档流里并存若干帧
   （内容列高度抖一下）。display:none 让旧页立刻让位，新页的淡入照旧。 */
.inner-fade-enter-active {
  transition:
    opacity 180ms var(--ease-default),
    transform 180ms var(--ease-default);
}

.inner-fade-enter-from {
  opacity: 0;
  transform: translateY(6px);
}

.inner-fade-leave-active {
  display: none;
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

  .main-content {
    padding: var(--space-4);
  }
}
</style>
