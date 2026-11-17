<template>
  <!-- 导师端：与其余三端同一套外壳（默认密度）+ 内容限宽居中。
       侧栏密度自 ADR-0081 起只剩单档（原 compact 变体已退役），四端观感一致。 -->
  <SidebarLayout
    :menu-items="tutorNav"
    content-width="narrow"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation, filterNavByCapability } from '@/config/navigation'
import { useAuthStore } from '@/stores/auth'
import { holdsCapability } from '@/utils/authzRuntime'

const authStore = useAuthStore()

// 讲师能力自 #2600 起由数据层回答（所挂角色的能力集）：导航按**运行时能力集**过滤 ——
// 管理员把某项权限收回后，该菜单项在下一次能力集刷新时消失，而不是留在侧栏里点进去才 403。
// 判据与守卫同源（holdsCapability），因此「侧栏显示什么」与「守卫放行什么」不会分叉。
const tutorNav = computed(() =>
  filterNavByCapability(roleNavigation.tutor, capability =>
    holdsCapability(authStore.userInfo?.role, authStore.capabilities, capability)
  )
)
</script>
