<template>
  <SidebarLayout :menu-items="adminNav" :keep-alive-names="cachedNames">
    <!-- 标签栏（#1620）：内容列顶部、贴顶栏下沿，随顶栏一起固定 -->
    <template #content-header>
      <AdminTabBar />
    </template>
  </SidebarLayout>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import SidebarLayout from './SidebarLayout.vue'
import AdminTabBar from '@/components/layout/AdminTabBar.vue'
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
