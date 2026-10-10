<script setup lang="ts">
/**
 * 管理端标签栏（#1620，ADR-0073）。
 *
 * 位置：**内容列顶部、贴顶栏下沿**（SidebarLayout 的 #content-header 插槽），随顶栏一起固定 ——
 * 侧栏的竖向连续性不被打断，标签栏与它承载的页面同列。
 *
 * 行为：固定页（仪表盘）不可关；点击切签；右键出菜单（关闭当前/关闭其他/关闭全部）；
 * 详情页按描述符的归属并入其列表页标签，不重复开签。
 */
import { watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Close } from '@element-plus/icons-vue'

import { tabOwnerOf, pageTitleOf } from '@/config/navigation'
import { PINNED_ADMIN_TAB, useAdminTabsStore } from '@/stores/adminTabs'
import type { AdminTab } from '@/stores/adminTabs'
import type { RouteName } from '@/config/pages'

const route = useRoute()
const router = useRouter()
const tabsStore = useAdminTabsStore()

/**
 * 固定页恒在（不是"访问过仪表盘才有"）：首次直接落到别的管理页时，标签栏也必须有一个
 * 保底返回点，否则「关闭全部」之后无处可去。标题取描述符的 nav.label（同一份真源）。
 */
function ensurePinnedTab(): void {
  if (tabsStore.tabs.some(t => t.pinned)) return
  tabsStore.open({ name: PINNED_ADMIN_TAB, title: pageTitleOf(PINNED_ADMIN_TAB), pinned: true })
}

// 路由变化 → 开签/激活（含详情页并入所属标签）
watch(
  () => route.fullPath,
  () => {
    ensurePinnedTab()
    const tab = tabsStore.resolveTab(route, tabOwnerOf)
    if (tab) tabsStore.open(tab)
  },
  { immediate: true }
)

function activate(tab: AdminTab): void {
  if (route.name === tab.name) return
  router.push({ name: tab.name })
}

function onCommand(command: string, tab: AdminTab): void {
  if (command === 'close') closeTab(tab)
  else if (command === 'others') tabsStore.closeOthers(tab.name)
  else if (command === 'all') {
    const fallback = tabsStore.closeAll()
    if (fallback) router.push({ name: fallback })
  }
}

function closeTab(tab: AdminTab): void {
  const fallback = tabsStore.close(tab.name)
  if (fallback) router.push({ name: fallback as RouteName })
}
</script>

<template>
  <div class="admin-tabbar">
    <el-dropdown
      v-for="tab in tabsStore.tabs"
      :key="tab.name"
      trigger="contextmenu"
      placement="bottom-start"
      @command="(cmd: string) => onCommand(cmd, tab)"
    >
      <div
        class="admin-tab"
        :class="{ 'is-active': tabsStore.activeName === tab.name, 'is-pinned': tab.pinned }"
        role="tab"
        :aria-selected="tabsStore.activeName === tab.name"
        @click="activate(tab)"
      >
        <span class="admin-tab-title" :title="tab.title">{{ tab.title }}</span>
        <button
          v-if="!tab.pinned"
          type="button"
          class="admin-tab-close"
          :aria-label="`关闭 ${tab.title}`"
          @click.stop="closeTab(tab)"
        >
          <el-icon :size="12"><Close /></el-icon>
        </button>
      </div>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item v-if="!tab.pinned" command="close">关闭当前</el-dropdown-item>
          <el-dropdown-item :disabled="tabsStore.closable.length < 2" command="others">
            关闭其他
          </el-dropdown-item>
          <el-dropdown-item :disabled="tabsStore.closable.length === 0" command="all">
            关闭全部
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<style scoped>
/* 标签栏与外壳同底（ADR-0072 的无缝口径）：不引入第二档底色，靠标签自身的底与描边表达选中 */
.admin-tabbar {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  height: 40px;
  padding: 0 var(--space-6);
  overflow-x: auto;
  overflow-y: hidden;
  background: var(--color-bg-page);
  scrollbar-width: none;
}

.admin-tabbar::-webkit-scrollbar {
  display: none;
}

.admin-tab {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-shrink: 0;
  max-width: 200px;
  padding: var(--space-1) var(--space-3);
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-default),
    color var(--duration-fast) var(--ease-default);
}

.admin-tab:hover {
  background: var(--color-bg-card);
  color: var(--color-text-primary);
}

.admin-tab.is-active {
  background: var(--color-bg-card);
  border-color: var(--color-border-light);
  color: var(--color-primary-600);
  font-weight: var(--font-medium);
}

.admin-tab-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-tab-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  padding: 0;
  border: none;
  border-radius: var(--radius-full);
  background: transparent;
  color: var(--color-text-tertiary);
  cursor: pointer;
  flex-shrink: 0;
}

.admin-tab-close:hover {
  background: var(--color-bg-page);
  color: var(--color-danger);
}
</style>
