<template>
  <!--
    侧栏项级渲染 seam（第十三波 票9 / ADR-0060 §9）：
    三层嵌套 × 外链/router-link 的 9 份 markup 收在这一个 module 里，由 AppSidebar 按层级组合。
    行种类只由 `kind` 一处判定（外链 / 路由项 / 不可点占位），父层不再各写两遍分支。

    样式：`scoped` 不穿子组件，所以 .nav-item / .nav-item-icon / .nav-item-label 一族规则
    随本组件下沉（与 NotificationPanel → NotificationPanelBody 同一手法）。
    `.app-sidebar.collapsed|is-dark|is-compact` 是**祖先**选择器，作用域属性只加在最后一个复合选择器上
    （即本组件根节点），因此这些变体照常命中，声明顺序与特异性与改造前逐字一致。
  -->
  <a
    v-if="kind === 'external'"
    :href="item.externalUrl"
    target="_blank"
    rel="noopener"
    class="nav-item"
    :class="{ 'nav-sub-item': level === 3 }"
  >
    <div class="nav-item-icon">
      <el-icon><component :is="item.icon" /></el-icon>
    </div>
    <span v-if="!collapsed" class="nav-item-label">{{ item.label }}</span>
  </a>
  <router-link
    v-else-if="kind === 'route'"
    :to="to"
    class="nav-item"
    :class="{ 'nav-sub-item': level === 3, active }"
  >
    <div class="nav-item-icon">
      <el-icon><component :is="item.icon" /></el-icon>
    </div>
    <span v-if="!collapsed" class="nav-item-label">{{ item.label }}</span>
  </router-link>
  <a v-else href="#" class="nav-item" :class="{ 'nav-sub-item': level === 3 }" @click.prevent>
    <div class="nav-item-icon">
      <el-icon><component :is="item.icon" /></el-icon>
    </div>
    <span v-if="!collapsed" class="nav-item-label">{{ item.label }}</span>
  </a>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { NavItem } from '@/config/navigation'

const props = withDefaults(
  defineProps<{
    item: NavItem
    /** 侧栏是否收起（AppSidebar 的 effectiveCollapsed）：收起时只出图标，不出文字标签。 */
    collapsed: boolean
    /** 当前路由命中本项（父层用 config/navigation.ts 的 isNavRouteActive 算好传入）。 */
    active?: boolean
    /** 嵌套层级：1 顶级 / 2 分组内 / 3 二级分组内。只有第 3 层带 `nav-sub-item` 缩进类。 */
    level?: 1 | 2 | 3
    /**
     * 既无 `externalUrl` 也无 `routeName` 时的兜底行形态——改造前两处历史形态各留一份，零漂移：
     * - `link`（默认）：照旧渲染 `<router-link>`（二级分组的子项用的就是无条件的 `v-else`）
     * - `inert`：渲染一行不可点的占位（`href="#"` + `@click.prevent`，叶子项用的）
     */
    fallback?: 'link' | 'inert'
  }>(),
  { active: false, level: 1, fallback: 'link' }
)

type RowKind = 'external' | 'route' | 'inert'

const kind = computed<RowKind>(() => {
  const item = props.item
  if (item.externalUrl) return 'external'
  if (item.routeName) return 'route'
  return props.fallback === 'inert' ? 'inert' : 'route'
})

const to = computed(() => ({ name: props.item.routeName, params: props.item.routeParams || {} }))
</script>

<style scoped>
.nav-sub-item {
  padding-left: calc(var(--space-3) + 12px);
}

.app-sidebar.collapsed .nav-sub-item {
  padding-left: var(--space-2);
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
  overflow: hidden;
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
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------------------------------------------------------------------------
 * 主题（theme）与密度（density）变体 —— 同 AppSidebar.vue 的约定：
 * 刻意「追加覆盖」而非改写上面的规则，因此 light + default 分支与改造前逐像素一致；
 * 变体选择器多两个类（.app-sidebar.is-*），特异性天然高于上面的单类规则，无需 !important。
 * ------------------------------------------------------------------------- */

/* dark：石墨青暗底（走 --color-bg-sidebar token，深色模式下自动翻更深 #0B1120） */
.app-sidebar.is-dark .nav-item {
  color: rgba(241, 245, 249, 0.72);
}

.app-sidebar.is-dark .nav-item:hover {
  color: var(--color-primary-300);
  background: rgba(255, 255, 255, 0.06);
}

.app-sidebar.is-dark .nav-item.active {
  color: var(--color-primary-300);
  background: rgba(45, 212, 191, 0.14);
}

/* 激活指示条在暗底上要更亮才看得见 */
.app-sidebar.is-dark .nav-item.active::before {
  background: var(--color-primary-400);
}

/* compact：收紧纵向间距 */
.app-sidebar.is-compact .nav-item {
  padding: 6px var(--space-3);
}
</style>
