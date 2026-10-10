<template>
  <SidebarLayout :menu-items="adminNav" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation, filterNavByCapability } from '@/config/navigation'
import { useAuthStore } from '@/stores/auth'
import { holdsCapability } from '@/utils/authzRuntime'

const authStore = useAuthStore()

// 管理端能力由数据层回答（#1618 段1）：导航按**运行时能力集**过滤 —— 超管把某项权限收回后，
// 该菜单项在下一次能力集刷新时消失，而不是留在侧栏里点进去才 403。
const adminNav = computed(() =>
  filterNavByCapability(roleNavigation.admin, capability =>
    holdsCapability(authStore.userInfo?.role, authStore.capabilities, capability)
  )
)
</script>
