<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
/**
 * 管理端标签栏（#1620，ADR-0073）。
 *
 * 位置：**内容列顶部、贴顶栏下沿**（SidebarLayout 的 #content-header 插槽），随顶栏一起固定 ——
 * 侧栏的竖向连续性不被打断，标签栏与它承载的页面同列。
 *
 * 行为：固定页不可关；点击切签；右键出菜单（关闭当前/关闭其他/关闭全部）；
 * 详情页按描述符的归属并入其列表页标签，不重复开签。
 *
 * 固定页不是写死的仪表盘（#1638）：**按当前账号的能力集算**（`adminLandingPage`，
 * 与守卫的能力回落同源）。一个能力位都没有的账号，固定页就是「无管理权限」页 ——
 * 否则固定页会变成一页它看不见的页面，点一次被弹一次。
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useRovingSelection } from '@/composables/useRovingSelection'
import { adminLandingPage, tabOwnerOf, pageTitleOf } from '@/config/navigation'
import { holdsCapability } from '@/utils/authzRuntime'
import { useAuthStore } from '@/stores/auth'
import { useAdminTabsStore } from '@/stores/adminTabs'
import type { AdminTab } from '@/stores/adminTabs'
import type { AuthzCapability } from '@/config/authz'
import type { RouteName } from '@/config/pages'

const route = useRoute()
const router = useRouter()
const tabsStore = useAdminTabsStore()
const authStore = useAuthStore()

/** 本账号的固定页 = 它的落点（能力集变化会自动跟着变）。 */
const pinnedName = computed<RouteName>(() =>
  adminLandingPage((cap: AuthzCapability) =>
    holdsCapability(authStore.userInfo?.role, authStore.capabilities, cap)
  )
)

/**
 * 固定页恒在（不是"访问过它才有"）：首次直接落到别的管理页时，标签栏也必须有一个
 * 保底返回点，否则「关闭全部」之后无处可去。标题取描述符的 nav.label（同一份真源）。
 */
function ensurePinnedTab(): void {
  const existing = tabsStore.tabs.find(t => t.pinned)
  // 能力集变化会让落点换页（超管新授权/收回权限）：把旧的固定页换成新的那一页
  if (existing?.name === pinnedName.value) return
  tabsStore.open({ name: pinnedName.value, title: pageTitleOf(pinnedName.value), pinned: true })
}

// 路由变化 → 开签/激活（含详情页并入所属标签）
watch(
  () => route.fullPath,
  () => {
    ensurePinnedTab()
    const tab = tabsStore.resolveTab(route, tabOwnerOf, pageTitleOf, pinnedName.value)
    if (tab) tabsStore.open(tab)
  },
  { immediate: true }
)

// 落点变化（能力集刷新 / 换账号）→ 固定页随之替换
watch(pinnedName, () => ensurePinnedTab())

function activate(tab: AdminTab): void {
  if (route.name === tab.name) return
  router.push({ name: tab.name })
}

/**
 * 键盘契约（#2158）：页签栏声明了 `role="tab"` 就必须兑现选项卡的键盘行为——
 * 方向键在页签间移动并激活、roving tabindex（整条栏只留一个 Tab 落点）、Delete 关闭当前页签。
 * 实现与 `UiUnderlineTabs` 共用 `useRovingSelection`，同族不会出现两套手感。
 *
 * 关闭按钮挂 `tabindex="-1"`：它是页签内部的次级动作（APG 的 Delete 约定覆盖键盘路径），
 * 留在 Tab 序列里会让「一条页签栏停一次」变成每页签停两次。
 */
const rovingOptions = computed(() =>
  tabsStore.tabs.map((t) => ({ label: t.title, value: t.name as string }))
)
const { activeValue, setBtnRef, onKeydown } = useRovingSelection({
  options: () => rovingOptions.value,
  current: () => tabsStore.activeName ?? '',
  disabled: () => false,
  select: (opt) => {
    if (route.name !== opt.value) router.push({ name: opt.value as RouteName })
  }
})

/** Delete 关闭当前页签；固定页是保底返回点，不可关 */
function closeByKeyboard(tab: AdminTab): void {
  if (tab.pinned) return
  closeTab(tab)
}

function onCommand(command: string, tab: AdminTab): void {
  if (command === 'close') closeTab(tab)
  else if (command === 'others') tabsStore.closeOthers(tab.name)
  else if (command === 'all') {
    const fallback = tabsStore.closeAll()
    if (fallback) router.push({ name: fallback })
  }
  closeMenu()
}

/* ---------------- 右键菜单（自建，不用 el-dropdown） ----------------
 *
 * 为什么不用 EP：`el-dropdown` 会把默认插槽的第一个元素**改写成触发器**——
 * 实测渲染出 `role="button"` + `aria-haspopup="menu"` + 它自己的 `tabindex`，
 * 把我们挂的 `role="tab"` 整个盖掉。选项卡语义与它的触发器语义天然冲突，
 * 两者不能住在同一个元素上，所以菜单自己实现（三项，样式与 EP 菜单同值）。
 */
const menu = ref<{ tab: AdminTab; x: number; y: number } | null>(null)

function openMenu(event: MouseEvent, tab: AdminTab): void {
  menu.value = { tab, x: event.clientX, y: event.clientY }
}

function closeMenu(): void {
  menu.value = null
}

function onMenuKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') closeMenu()
}

watch(menu, (open) => {
  if (typeof document === 'undefined') return
  if (open) document.addEventListener('keydown', onMenuKeydown)
  else document.removeEventListener('keydown', onMenuKeydown)
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') document.removeEventListener('keydown', onMenuKeydown)
})

function closeTab(tab: AdminTab): void {
  const fallback = tabsStore.close(tab.name)
  if (fallback) router.push({ name: fallback as RouteName })
}
</script>

<template>
  <div class="admin-tabbar" role="tablist" aria-label="已打开的页面">
    <div
      v-for="(tab, i) in tabsStore.tabs"
      :key="tab.name"
      :ref="(el) => setBtnRef(el, i)"
      class="admin-tab"
      :class="{ 'is-active': tabsStore.activeName === tab.name, 'is-pinned': tab.pinned }"
      role="tab"
      :aria-selected="tabsStore.activeName === tab.name"
      :tabindex="activeValue === tab.name ? 0 : -1"
      @click="activate(tab)"
      @keydown="onKeydown($event, i)"
      @keydown.delete.prevent="closeByKeyboard(tab)"
      @contextmenu.prevent="openMenu($event, tab)"
    >
      <span class="admin-tab-title" :title="tab.title">{{ tab.title }}</span>
      <button
        v-if="!tab.pinned"
        type="button"
        class="admin-tab-close"
        tabindex="-1"
        :aria-label="`关闭 ${tab.title}`"
        @click.stop="closeTab(tab)"
      >
        <UiIcon name="x" :size="12" />
      </button>
    </div>

    <!-- 遮罩只负责「点外面关掉」；菜单挂 body 是因为标签栏 overflow-x:auto 会裁掉它 -->
    <div v-if="menu" class="admin-tabbar-mask" @click="closeMenu" @contextmenu.prevent="closeMenu" />
    <Teleport to="body">
      <div
        v-if="menu"
        class="admin-tabbar-menu"
        role="menu"
        :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      >
        <button
          v-if="!menu.tab.pinned"
          type="button"
          role="menuitem"
          class="admin-tabbar-menu-item"
          @click="onCommand('close', menu.tab)"
        >
          关闭当前
        </button>
        <button
          type="button"
          role="menuitem"
          class="admin-tabbar-menu-item"
          :disabled="tabsStore.closable.length < 2"
          @click="onCommand('others', menu.tab)"
        >
          关闭其他
        </button>
        <button
          type="button"
          role="menuitem"
          class="admin-tabbar-menu-item"
          :disabled="tabsStore.closable.length === 0"
          @click="onCommand('all', menu.tab)"
        >
          关闭全部
        </button>
      </div>
    </Teleport>
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
  color: var(--color-primary-soft-ink);
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

/* 右键菜单（自建，见脚本里的说明）：样式与 element-overrides 的 EP 菜单同值 */
.admin-tabbar-mask {
  position: fixed;
  inset: 0;
  z-index: 2000;
}

.admin-tabbar-menu {
  position: fixed;
  z-index: 2001;
  display: flex;
  flex-direction: column;
  min-width: 132px;
  padding: var(--space-1);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  background: var(--color-bg-card);
  box-shadow: var(--shadow-lg);
}

.admin-tabbar-menu-item {
  padding: var(--space-1) var(--space-3);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-primary);
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}

.admin-tabbar-menu-item:hover:not(:disabled) {
  background: var(--color-bg-page);
}

.admin-tabbar-menu-item:disabled {
  /* 置灰靠透明度而不是更浅的字色：ink-muted 只有 2.45:1，是 check-a11y 的禁入档 */
  color: var(--color-text-tertiary);
  opacity: 0.6;
  cursor: not-allowed;
}

.admin-tabbar-menu-item:focus-visible {
  outline: 2px solid var(--color-primary-500);
  outline-offset: -2px;
}
</style>
