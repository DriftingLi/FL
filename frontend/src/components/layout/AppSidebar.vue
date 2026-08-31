<template>
  <aside
    class="app-sidebar"
    :class="{
      collapsed: effectiveCollapsed,
      'is-dark': props.theme === 'dark',
      'is-compact': props.density === 'compact'
    }"
  >
    <!-- 用户信息区（含退出登录下拉菜单） -->
    <el-dropdown
      class="sidebar-user-dropdown"
      trigger="click"
      :placement="effectiveCollapsed ? 'right-start' : 'bottom-start'"
      @command="handleUserCommand"
    >
      <div class="sidebar-user" :class="{ 'is-collapsed': effectiveCollapsed }">
        <img
          v-if="authStore.userInfo?.avatar_url"
          :src="String(authStore.userInfo.avatar_url)"
          class="user-avatar-circle user-avatar-img"
          alt="头像"
        />
        <div v-else class="user-avatar-circle">
          {{ (authStore.userInfo?.username || '?').charAt(0) }}
        </div>
        <div v-if="!effectiveCollapsed" class="user-info">
          <span class="user-name">{{ authStore.userInfo?.username }}</span>
          <span class="role-badge" :class="roleClass">{{ roleLabel }}</span>
        </div>
        <el-icon v-if="!effectiveCollapsed" class="user-dropdown-arrow"><ArrowDown /></el-icon>
      </div>
      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="theme">
            <el-icon><Moon /></el-icon>
            <template v-if="themeStore.mode === 'dark'">深色模式（点击切换）</template>
            <template v-else-if="themeStore.mode === 'light'">浅色模式（点击切换）</template>
            <template v-else>跟随系统（点击切换）</template>
          </el-dropdown-item>
          <el-dropdown-item command="logout">
            <el-icon><SwitchButton /></el-icon>退出登录
          </el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>

    <slot name="top" :collapsed="effectiveCollapsed" />

    <!-- 分隔线 -->
    <div class="sidebar-divider"></div>

    <!-- 导航菜单 -->
    <nav class="sidebar-nav">
      <template v-for="item in menuItems" :key="item.key">
        <!-- 有子项的分组 -->
        <template v-if="item.children && item.children.length">
          <div
            v-if="!effectiveCollapsed"
            class="nav-group-label is-accordion"
            :class="{ 'is-active': isGroupActive(item) }"
            @click="onGroupToggle(item.key)"
          >
            <el-icon v-if="item.icon" class="nav-group-icon"><component :is="item.icon" /></el-icon>
            <span>{{ item.label }}</span>
            <el-icon class="nav-group-arrow" :class="{ expanded: isGroupExpanded(item.key) }"><ArrowDown /></el-icon>
          </div>
          <el-tooltip v-else placement="right" :show-after="300">
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
            <div class="nav-group-icon-only" :class="{ 'is-active': isGroupActive(item) }">
              <el-icon><component :is="item.icon" /></el-icon>
            </div>
          </el-tooltip>
          <div v-show="isGroupExpanded(item.key)" class="nav-group-children">
            <template v-for="child in item.children" :key="child.key">
              <!-- 二级嵌套：child 自身还有 children（如 题库练习 ┬ 真题练习） -->
              <template v-if="child.children && child.children.length">
                <a
                  v-if="child.externalUrl"
                  :href="child.externalUrl"
                  target="_blank"
                  rel="noopener"
                  class="nav-item"
                >
                  <div class="nav-item-icon">
                    <el-icon><component :is="child.icon" /></el-icon>
                  </div>
                  <span v-if="!effectiveCollapsed" class="nav-item-label">{{ child.label }}</span>
                </a>
                <router-link
                  v-else-if="child.routeName"
                  :to="itemTo(child)"
                  class="nav-item"
                  :class="{ active: isRouteActive(child) }"
                >
                  <div class="nav-item-icon">
                    <el-icon><component :is="child.icon" /></el-icon>
                  </div>
                  <span v-if="!effectiveCollapsed" class="nav-item-label">{{ child.label }}</span>
                </router-link>
                <div v-else class="nav-group-label nav-sub-group-label" :class="{ 'is-active': isGroupActive(child) }">
                  <span>{{ child.label }}</span>
                </div>
                <template v-for="sub in child.children" :key="sub.key">
                  <a
                    v-if="sub.externalUrl"
                    :href="sub.externalUrl"
                    target="_blank"
                    rel="noopener"
                    class="nav-item nav-sub-item"
                  >
                    <div class="nav-item-icon">
                      <el-icon><component :is="sub.icon" /></el-icon>
                    </div>
                    <span v-if="!effectiveCollapsed" class="nav-item-label">{{ sub.label }}</span>
                  </a>
                  <router-link
                    v-else
                    :to="itemTo(sub)"
                    class="nav-item nav-sub-item"
                    :class="{ active: isRouteActive(sub) }"
                  >
                    <div class="nav-item-icon">
                      <el-icon><component :is="sub.icon" /></el-icon>
                    </div>
                    <span v-if="!effectiveCollapsed" class="nav-item-label">{{ sub.label }}</span>
                  </router-link>
                </template>
              </template>
              <!-- 叶子 child -->
              <a
                v-else-if="child.externalUrl"
                :href="child.externalUrl"
                target="_blank"
                rel="noopener"
                class="nav-item"
              >
                <div class="nav-item-icon">
                  <el-icon><component :is="child.icon" /></el-icon>
                </div>
                <span v-if="!effectiveCollapsed" class="nav-item-label">{{ child.label }}</span>
              </a>
              <router-link
                v-else-if="child.routeName"
                :to="itemTo(child)"
                class="nav-item"
                :class="{ active: isRouteActive(child) }"
              >
                <div class="nav-item-icon">
                  <el-icon><component :is="child.icon" /></el-icon>
                </div>
                <span v-if="!effectiveCollapsed" class="nav-item-label">{{ child.label }}</span>
              </router-link>
              <a
                v-else
                :href="'#'"
                class="nav-item"
                @click.prevent
              >
                <div class="nav-item-icon">
                  <el-icon><component :is="child.icon" /></el-icon>
                </div>
                <span v-if="!effectiveCollapsed" class="nav-item-label">{{ child.label }}</span>
              </a>
            </template>
          </div>
        </template>

        <!-- 外链 -->
        <a
          v-else-if="item.externalUrl"
          :href="item.externalUrl"
          target="_blank"
          rel="noopener"
          class="nav-item"
        >
          <div class="nav-item-icon">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <span v-if="!effectiveCollapsed" class="nav-item-label">{{ item.label }}</span>
        </a>

        <!-- 无子项的顶级导航 -->
        <router-link
          v-else-if="item.routeName"
          :to="itemTo(item)"
          class="nav-item"
          :class="{ active: isRouteActive(item) }"
        >
          <div class="nav-item-icon">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <span v-if="!effectiveCollapsed" class="nav-item-label">{{ item.label }}</span>
        </router-link>
      </template>
    </nav>

    <!-- 底部功能区 -->
    <div class="sidebar-divider"></div>
    <div class="sidebar-footer">
      <div class="footer-row">
        <button class="footer-btn collapse-btn" @click="$emit('toggle-collapse')">
          <component :is="effectiveCollapsed ? Expand : Fold" class="collapse-icon" />
          <span v-if="!effectiveCollapsed" class="footer-btn-label">收起侧栏</span>
        </button>
        <NotificationPanel
          v-if="authStore.isLoggedIn && authStore.userInfo?.role === 'hrwai_user'"
          class="sidebar-notification"
        />
      </div>
    </div>

  </aside>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { Expand, Fold, ArrowDown, SwitchButton, Moon } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { isNavRouteActive, type NavItem } from '@/config/navigation'
import NotificationPanel from '@/components/layout/NotificationPanel.vue'

const props = withDefaults(
  defineProps<{
    menuItems: NavItem[]
    collapsed: boolean
    mobileOpen?: boolean
    /**
     * 侧栏配色。
     * - `light`：浅底，**默认值 = 改造前行为**，tutor / admin 保持原样
     * - `dark`：石墨青暗底（#0C1210），学员端传入
     */
    theme?: 'light' | 'dark'
    /**
     * 纵向密度。
     * - `default`：**默认值 = 改造前行为**
     * - `compact`：收紧导航项与用户信息区的纵向间距
     */
    density?: 'default' | 'compact'
  }>(),
  { theme: 'light', density: 'default' }
)

defineEmits<{
  'toggle-collapse': []
}>()

// 移动端打开侧边栏时强制展开（显示文字），无视桌面端折叠状态
const effectiveCollapsed = computed(() => props.collapsed && !props.mobileOpen)

const route = useRoute()
const router = useRouter()
const themeStore = useThemeStore()
const authStore = useAuthStore()

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

function isGroupExpanded(key: string): boolean {
  return expandedMap[key] !== false
}

function onGroupToggle(key: string): void {
  expandedMap[key] = !isGroupExpanded(key)
}

function flattenLeaves(item: NavItem): NavItem[] {
  const result: NavItem[] = []
  const walk = (node: NavItem) => {
    for (const child of node.children || []) {
      if (child.routeName || child.externalUrl) result.push(child)
      if (child.children?.length) {
        for (const sub of child.children) {
          if (sub.routeName || sub.externalUrl) result.push(sub)
        }
      }
    }
  }
  walk(item)
  return result
}

function isGroupActive(item: NavItem): boolean {
  if (!item.children?.length) return false
  for (const child of item.children) {
    if (child.children?.length) {
      if (isRouteActive(child)) return true
      for (const sub of child.children) {
        if (isRouteActive(sub)) return true
      }
    } else if (isRouteActive(child)) {
      return true
    }
  }
  return false
}

const roleLabel = computed(() => {
  const role = authStore.userInfo?.role
  if (role === 'admin') return '管理员'
  if (role === 'tutor') return '导师'
  if (role === 'hrwai_user') return '学员'
  if (role === 'recruiter') return '企业'
  return '用户'
})

const roleClass = computed(() => {
  const role = authStore.userInfo?.role
  return role || 'hrwai_user'
})

function itemTo(item: NavItem) {
  return { name: item.routeName, params: item.routeParams || {} }
}

/** 匹配逻辑抽到 config/navigation.ts 的 isNavRouteActive（纯函数，可单测） */
function isRouteActive(item: NavItem): boolean {
  return isNavRouteActive(item, route.name, route.params as Record<string, string | string[] | undefined>)
}

async function handleUserCommand(command: string) {
  if (command === 'theme') {
    themeStore.cycle()
    return
  }
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
      // 用户取消，不做任何操作
    }
  }
}
</script>

<style scoped>
.app-sidebar {
  width: var(--sidebar-width);
  background: var(--color-bg-card);
  border-right: 1px solid var(--color-border-light);
  display: flex;
  flex-direction: column;
  transition: width var(--duration-normal) var(--ease-default);
  overflow: hidden;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: var(--z-fixed);
}

.app-sidebar.collapsed {
  width: var(--sidebar-collapsed-width);
}

/* 用户信息区 */
.sidebar-user-dropdown {
  display: block;
  width: 100%;
  flex-shrink: 0;
}

.sidebar-user {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4) var(--space-4) var(--space-3);
  cursor: pointer;
  border-radius: 0;
  outline: none;
  transition: background var(--duration-fast) var(--ease-default);
}

.sidebar-user:hover,
.sidebar-user:focus-visible {
  background: var(--color-bg-page);
}

.app-sidebar.collapsed .sidebar-user {
  justify-content: center;
  padding: var(--space-4) var(--space-2);
}

.user-avatar-circle {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-full);
  background: var(--gradient-brand);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--text-base);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
  flex-shrink: 0;
}

.user-avatar-img {
  object-fit: cover;
}

.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
  flex: 1;
  min-width: 0;
}

.user-name {
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-dropdown-arrow {
  font-size: 12px;
  color: var(--color-text-tertiary);
  flex-shrink: 0;
  margin-left: auto;
}

.role-badge {
  font-size: 11px;
  font-weight: var(--font-medium);
  padding: 1px 6px;
  border-radius: var(--radius-full);
  width: fit-content;
  white-space: nowrap;
}

.role-badge.student {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

.role-badge.recruiter {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}

.role-badge.tutor {
  background: #ECFDF5;
  color: #059669;
}

.role-badge.admin {
  background: #F5F3FF;
  color: #7C3AED;
}

/* 分隔线 */
.sidebar-divider {
  height: 1px;
  background: var(--color-border-light);
  margin: 0 var(--space-4);
  flex-shrink: 0;
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
}

.nav-group-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-muted);
  padding: var(--space-3) var(--space-3) var(--space-1);
  letter-spacing: 0.03em;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 6px;
}

.nav-group-label.is-active {
  color: var(--color-primary-600);
}

.nav-group-label.is-active .nav-group-icon {
  color: var(--color-primary-600);
}

.nav-group-label.is-accordion {
  cursor: pointer;
  user-select: none;
}

.nav-group-label.is-accordion:hover {
  color: var(--color-primary-600);
}

.nav-group-arrow {
  margin-left: auto;
  font-size: 12px;
  transition: transform var(--duration-fast) var(--ease-default);
}

.nav-group-arrow.expanded {
  transform: rotate(180deg);
}

.nav-group-icon {
  font-size: 14px;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.nav-group-icon-only {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-2);
  color: var(--color-text-muted);
  cursor: default;
}

.nav-group-icon-only.is-active {
  color: var(--color-primary-600);
  background: var(--color-primary-50);
  border-radius: var(--radius-md);
}

.nav-group-icon-only .el-icon {
  font-size: 16px;
}

.nav-group-children {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.nav-sub-group-label {
  font-size: var(--text-xs);
  font-weight: var(--font-medium);
  color: var(--color-text-muted);
  padding: var(--space-2) var(--space-3) var(--space-1) calc(var(--space-3) + 12px);
  white-space: nowrap;
}

.nav-sub-group-label.is-active {
  color: var(--color-primary-600);
}

.nav-sub-item {
  padding-left: calc(var(--space-3) + 12px);
}

.app-sidebar.collapsed .nav-sub-item {
  padding-left: var(--space-2);
}

.nav-group-tooltip-title {
  font-weight: var(--font-semibold);
  margin-bottom: 4px;
  font-size: 12px;
}

.nav-group-tooltip-item {
  font-size: 12px;
  line-height: 1.7;
  opacity: 0.9;
}

.nav-group-tooltip-item.active {
  font-weight: var(--font-semibold);
  opacity: 1;
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

.app-sidebar.collapsed .sidebar-notification {
  display: none;
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

/* ---------------------------------------------------------------------------
 * 主题（theme）与密度（density）变体
 *
 * 刻意采用「追加覆盖」而非改写上面的规则：现有声明一行不动，
 * 因此 light + default 分支与改造前逐像素一致（tutor / admin 零 diff）。
 * 变体选择器多一个类，特异性天然高于上面的单类规则，无需 !important。
 * ------------------------------------------------------------------------- */

/* dark：石墨青暗底 */
.app-sidebar.is-dark {
  background: #0c1210;
  border-right-color: rgba(255, 255, 255, 0.08);
}

.app-sidebar.is-dark .sidebar-user:hover,
.app-sidebar.is-dark .sidebar-user:focus-visible {
  background: rgba(255, 255, 255, 0.06);
}

.app-sidebar.is-dark .user-name {
  color: #f1f5f9;
}

.app-sidebar.is-dark .user-dropdown-arrow {
  color: rgba(241, 245, 249, 0.5);
}

/* 角色徽章：tutor / admin 的绿 / 紫在暗底上对比度仍够，只调学员/企业（品牌）色 */
.app-sidebar.is-dark .role-badge.student,
.app-sidebar.is-dark .role-badge.recruiter {
  background: rgba(45, 212, 191, 0.16);
  color: var(--color-primary-300);
}

.app-sidebar.is-dark .sidebar-divider {
  background: rgba(255, 255, 255, 0.08);
}

.app-sidebar.is-dark .nav-group-label,
.app-sidebar.is-dark .nav-group-icon,
.app-sidebar.is-dark .nav-group-icon-only,
.app-sidebar.is-dark .nav-sub-group-label {
  color: rgba(148, 163, 184, 0.85);
}

.app-sidebar.is-dark .nav-group-label.is-active,
.app-sidebar.is-dark .nav-group-label.is-active .nav-group-icon,
.app-sidebar.is-dark .nav-group-label.is-accordion:hover,
.app-sidebar.is-dark .nav-sub-group-label.is-active {
  color: var(--color-primary-300);
}

.app-sidebar.is-dark .nav-group-icon-only.is-active {
  color: var(--color-primary-300);
  background: rgba(45, 212, 191, 0.14);
}

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

.app-sidebar.is-dark .footer-btn {
  color: rgba(148, 163, 184, 0.85);
}

.app-sidebar.is-dark .footer-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #f1f5f9;
}

/* compact：收紧纵向间距 */
.app-sidebar.is-compact .sidebar-user {
  padding: var(--space-3) var(--space-4) var(--space-2);
}

.app-sidebar.is-compact .nav-group-label {
  padding: var(--space-2) var(--space-3) var(--space-1);
}

.app-sidebar.is-compact .nav-item {
  padding: 6px var(--space-3);
}
</style>
