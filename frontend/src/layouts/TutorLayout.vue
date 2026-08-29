<template>
  <!-- 导师端：与学员端一致的石墨青暗底侧栏 + 紧凑密度 + 内容限宽居中。
       AdminLayout 不传这些 prop，走默认值（=改造前行为），管理端零 diff。 -->
  <SidebarLayout
    :menu-items="tutorNav"
    sidebar-theme="dark"
    sidebar-density="compact"
    content-width="narrow"
  />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const tutorNav = roleNavigation.tutor
</script>
