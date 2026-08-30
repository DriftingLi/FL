<template>
  <!-- 企业招聘端：与学员/导师一致的石墨青暗底侧栏 + 紧凑密度 + 内容限宽居中 -->
  <SidebarLayout
    :menu-items="recruiterNav"
    sidebar-theme="dark"
    sidebar-density="compact"
    content-width="narrow"
  />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const recruiterNav = roleNavigation.recruiter
</script>
