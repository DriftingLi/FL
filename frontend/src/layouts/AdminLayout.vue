<template>
  <SidebarLayout
    class="admin-layout"
    :menu-items="adminNav"
    :keep-alive-names="cachedNames"
    :content-header-visible="tabsStore.expanded"
    expand-active-group
  >
    <template #topbar>
      <UiButton
        variant="ghost"
        size="small"
        aria-controls="admin-page-tabs"
        :aria-expanded="tabsStore.expanded"
        @click="tabsStore.expanded = !tabsStore.expanded"
      >
        {{ tabsStore.expanded ? '收起页签' : '展开页签' }}
      </UiButton>
    </template>
    <template #content-header>
      <AdminTabBar />
    </template>
  </SidebarLayout>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import SidebarLayout from './SidebarLayout.vue'
import AdminTabBar from '@/components/layout/AdminTabBar.vue'
import UiButton from '@/components/ui/UiButton.vue'
import { roleNavigation, filterNavByCapability, keepAliveNames } from '@/config/navigation'
import { useAuthStore } from '@/stores/auth'
import { useAdminTabsStore } from '@/stores/adminTabs'
import { holdsCapability } from '@/utils/authzRuntime'

const authStore = useAuthStore()
const tabsStore = useAdminTabsStore()

// 管理端能力由数据层回答（#1618 段1）：导航按**运行时能力集**过滤 —— 超管把某项权限收回后，
// 该菜单项在下一次能力集刷新时消失，而不是留在侧栏里点进去才 403。
const adminNav = computed(() =>
  filterNavByCapability(roleNavigation.admin, capability =>
    holdsCapability(authStore.userInfo?.role, authStore.capabilities, capability)
  )
)

// keep-alive 名单（#1620）：由页面描述符派生（keepAlive: true 的管理端页面），
// 组件名与路由名对齐后 keep-alive 才能命中；名单为空时等价于不缓存。
const cachedNames = keepAliveNames('manage')

// 标签是**登录态的工作集**：登出（或被守卫清登录态）时一并清空，避免下一个人看到上一个人的标签
watch(
  () => authStore.isLoggedIn,
  loggedIn => {
    if (!loggedIn) tabsStore.reset()
  }
)
</script>

<style scoped>
@media screen and (max-width: 768px) {
  .admin-layout :deep(.app-topbar),
  .admin-layout :deep(.app-topbar.is-collapsed) {
    grid-template-columns: minmax(0, 1fr) auto auto;
  }

  .admin-layout :deep(.topbar-left) {
    padding-inline: var(--space-2);
    gap: var(--space-2);
  }
}
</style>
