<template>
  <aside
    class="app-sidebar"
    :class="{
      collapsed: effectiveCollapsed,
      'is-compact': props.density === 'compact'
    }"
  >
    <slot name="top" :collapsed="effectiveCollapsed" />

    <!-- 内部分隔线：工作区入口（#top 插槽）与导航列之间。内缩 --space-4、用通用分隔线色
         （这是「组与组」的分隔，不是外壳边界，所以不参与十字细线那条缝） -->
    <div class="mx-4 h-px shrink-0 bg-line"></div>

    <!-- 导航菜单：每一行都由 AppSidebarItem 渲染（三层嵌套 × 外链/router-link 的 9 份 markup
         收进那一个项级 module，票9 / ADR-0060 §9）；本组件只留分组编排与判定接线。 -->
    <nav class="sidebar-nav">
      <template v-for="item in menuItems" :key="item.key">
        <!-- 有子项的分组 -->
        <template v-if="item.children && item.children.length">
          <div
            v-if="!effectiveCollapsed"
            class="nav-group-label is-accordion"
            :class="{ 'is-active': isGroupActiveLocal(item) }"
            @click="onGroupToggle(item.key)"
          >
            <NavIcon :icon="item.icon" v-if="item.icon" class="nav-group-icon" />
            <span>{{ item.label }}</span>
            <UiIcon name="chevron-down" class="nav-group-arrow" :class="{ expanded: isGroupExpandedLocal(item.key) }" />
          </div>
          <UiTooltip v-else placement="right" :show-after="300">
            <template #content>
              <div class="nav-group-tooltip-title">{{ item.label }}</div>
              <div
                v-for="leaf in flattenLeaves(item)"
                :key="leaf.key"
                class="nav-group-tooltip-item"
                :class="{ active: isRouteActive(leaf) }"
              >
                {{ leaf.label }}
              </div>
            </template>
            <div class="nav-group-icon-only" :class="{ 'is-active': isGroupActiveLocal(item) }">
              <NavIcon :icon="item.icon" />
            </div>
          </UiTooltip>
          <div v-show="isGroupExpandedLocal(item.key)" class="nav-group-children">
            <template v-for="child in item.children" :key="child.key">
              <!-- 二级嵌套：child 自身还有 children（如 题库练习 ┬ 真题练习） -->
              <template v-if="child.children && child.children.length">
                <AppSidebarItem
                  v-if="isNavItemRenderable(child)"
                  :item="child"
                  :collapsed="effectiveCollapsed"
                  :active="isRouteActive(child)"
                  :level="2"
                />
                <div v-else class="nav-group-label nav-sub-group-label" :class="{ 'is-active': isGroupActiveLocal(child) }">
                  <span>{{ child.label }}</span>
                </div>
                <AppSidebarItem
                  v-for="sub in child.children"
                  :key="sub.key"
                  :item="sub"
                  :collapsed="effectiveCollapsed"
                  :active="isRouteActive(sub)"
                  :level="3"
                />
              </template>
              <!-- 叶子 child：目标未就绪时出一行不可点的占位（fallback="inert"） -->
              <AppSidebarItem
                v-else
                :item="child"
                :collapsed="effectiveCollapsed"
                :active="isRouteActive(child)"
                :level="2"
                fallback="inert"
              />
            </template>
          </div>
        </template>

        <!-- 无子项的顶级导航（外链与路由项都在同一行形态里，差别由 AppSidebarItem 判） -->
        <AppSidebarItem
          v-else-if="isNavItemRenderable(item)"
          :item="item"
          :collapsed="effectiveCollapsed"
          :active="isRouteActive(item)"
        />
      </template>
    </nav>

    <!-- 外壳底缝（#1811）：与内容列页脚的上边框是**同一条线**，故满幅（不内缩）、走
         --color-border-seam（与顶栏下沿、侧栏右缘同源）。
         原先这里用 --color-border-light + 内缩：浅色下 #F1F5F9 落在 #F8FAFC 底上几乎同值，
         线等于没画，于是外壳上半部有十字细线、下半部的底边却断掉。 -->
    <div class="h-px shrink-0 bg-[var(--color-border-seam)]"></div>
    <div class="sidebar-footer">
      <div class="footer-row">
        <button class="footer-btn collapse-btn" @click="$emit('toggle-collapse')">
          <!-- 收起/展开是同一按钮上的两个方向：形状变了，所以走形变（ADR-0079 白名单） -->
          <UiIcon :name="effectiveCollapsed ? 'panel-left-open' : 'panel-left-close'" morph class="collapse-icon" />
          <span v-if="!effectiveCollapsed" class="footer-btn-label">收起侧栏</span>
        </button>
      </div>
    </div>

  </aside>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import NavIcon from './NavIcon.vue'
import { computed, reactive, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  isNavRouteActive,
  isGroupExpanded,
  toggleGroupExpanded,
  flattenLeaves,
  isGroupActive,
  type NavItem
} from '@/config/navigation'
import { isNavItemRenderable } from '@/config/navigation'
import AppSidebarItem from '@/components/layout/AppSidebarItem.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

const props = withDefaults(
  defineProps<{
    menuItems: NavItem[]
    collapsed: boolean
    mobileOpen?: boolean
    /**
     * 纵向密度。
     * - `default`：**默认值 = 改造前行为**
     * - `compact`：收紧导航项与用户信息区的纵向间距
     */
    density?: 'default' | 'compact'
  }>(),
  { density: 'default' }
)

defineEmits<{
  'toggle-collapse': []
}>()

// 移动端打开侧边栏时强制展开（显示文字），无视桌面端折叠状态
const effectiveCollapsed = computed(() => props.collapsed && !props.mobileOpen)

const route = useRoute()

// 侧栏分组折叠：默认全部展开，点击分组标题即可折叠/展开（桌面与移动端一致）
const expandedMap = reactive<Record<string, boolean>>({})

watch(
  () => props.menuItems,
  (items) => {
    for (const item of items) {
      if (item.children?.length && expandedMap[item.key] === undefined) {
        expandedMap[item.key] = true
      }
    }
  },
  { immediate: true, deep: false }
)

// 分组判定三条已抽到 config/navigation.ts（纯函数 + 单测，ADR-0047 §2 / spec #930）：
// 组件只保留响应式状态与事件接线。展开态用 Object.assign 原地写，保持 reactive 引用不变。
function isGroupExpandedLocal(key: string): boolean {
  return isGroupExpanded(expandedMap, key)
}

function onGroupToggle(key: string): void {
  Object.assign(expandedMap, toggleGroupExpanded(expandedMap, key))
}

function isGroupActiveLocal(item: NavItem): boolean {
  return isGroupActive(item, route.name, route.params as Record<string, string | string[] | undefined>)
}

/** 匹配逻辑抽到 config/navigation.ts 的 isNavRouteActive（纯函数，可单测）；跳转目标由 AppSidebarItem 现算 */
function isRouteActive(item: NavItem): boolean {
  return isNavRouteActive(item, route.name, route.params as Record<string, string | string[] | undefined>)
}

</script>

<style scoped>
.app-sidebar {
  width: var(--sidebar-width);
  /* 无缝外壳（#1619 / ADR-0072）：侧栏与顶栏、内容区同底，且**无右边框** ——
     接缝靠「同色」而非分隔线消失；层次由内容区里的卡片承担。 */
  background: var(--color-bg-page);
  /* 十字细线的**竖线**（#1629，改判自 ADR-0072 的「无边框」）：与顶栏左格的右边线相接。
     线色走 --color-border-seam（#1646）—— 与顶栏那两段是**同一条缝**，值必须同源 */
  border-right: 1px solid var(--color-border-seam);
  display: flex;
  flex-direction: column;
  transition: width var(--duration-normal) var(--ease-default);
  overflow: hidden;
  position: fixed;
  /* 从顶栏下沿开始，保持竖向连续（顶栏是棋盘格第一行） */
  top: var(--topbar-height);
  left: 0;
  bottom: 0;
  z-index: var(--z-fixed);
}

.app-sidebar.collapsed {
  width: var(--sidebar-collapsed-width);
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
  /*
   * 隐藏本列的滚动条（#1627，无缝外壳的补漏）：
   * 导航列一溢出，原生滚动条（全局 6px、滑块 --color-border-dark）就**紧贴侧栏右缘** ——
   * 而那条边缘正是外壳的接缝。实测（1920×1080 截图逐像素）它是一条 x=293..299、y=112..904、
   * 色值 #CBD5E1 的竖线，成了整个界面上最显眼的分隔线，把「接缝靠同色消失」直接抹掉。
   * 滚轮 / 触控板 / 键盘 / 拖拽滚动照常（只是不给滑块画出来）；本仓横向滚动条
   * （MarkdownToolbar / UiSegmentTabs / AdminTabBar）已有同样先例。
   */
  scrollbar-width: none;
}

.sidebar-nav::-webkit-scrollbar {
  width: 0;
  height: 0;
}

.nav-group-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-tertiary);
  padding: var(--space-3) var(--space-3) var(--space-1);
  letter-spacing: 0.03em;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 6px;
}

.nav-group-label.is-active {
  color: var(--color-primary-soft-ink);
}

.nav-group-label.is-active .nav-group-icon {
  color: var(--color-primary-soft-ink);
}

.nav-group-label.is-accordion {
  cursor: pointer;
  user-select: none;
}

.nav-group-label.is-accordion:hover {
  color: var(--color-primary-soft-ink);
}

.nav-group-arrow {
  margin-left: auto;
  font-size: var(--text-xs);
  transition: transform var(--duration-fast) var(--ease-default);
}

.nav-group-arrow.expanded {
  transform: rotate(180deg);
}

.nav-group-icon {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  flex-shrink: 0;
}

.nav-group-icon-only {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-2);
  color: var(--color-text-tertiary);
  cursor: default;
}

.nav-group-icon-only.is-active {
  color: var(--color-primary-soft-ink);
  background: var(--color-primary-soft);
  border-radius: var(--radius-md);
}

.nav-group-icon-only .el-icon {
  font-size: var(--text-base);
}

.nav-group-children {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.nav-sub-group-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-tertiary);
  padding: var(--space-2) var(--space-3) var(--space-1) calc(var(--space-3) + 12px);
  white-space: nowrap;
}

.nav-sub-group-label.is-active {
  color: var(--color-primary-soft-ink);
}

.nav-group-tooltip-title {
  font-weight: var(--font-semibold);
  margin-bottom: 4px;
  font-size: var(--text-xs);
}

.nav-group-tooltip-item {
  font-size: var(--text-xs);
  line-height: 1.7;
  opacity: 0.9;
}

.nav-group-tooltip-item.active {
  font-weight: var(--font-semibold);
  opacity: 1;
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

.footer-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.footer-row .footer-btn {
  flex: 1;
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

/* 密度（density）变体
 *
 * 恒深侧栏体系（is-dark / sidebarTheme / --color-bg-sidebar 三件套）已整体退役（#1619 / ADR-0072）：
 * 外壳三块同底后，「侧栏自带一套深色」不再有调用方，留着就是无人走的分支。
 * 现存的变体只剩密度一档。 */

/* compact：收紧纵向间距（原身份区那一档随身份区迁往顶栏而去掉） */
.app-sidebar.is-compact .nav-group-label {
  padding: var(--space-2) var(--space-3) var(--space-1);
}
</style>
