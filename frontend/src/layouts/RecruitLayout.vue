<template>
  <!-- 企业招聘端：与其余三端同一套外壳（默认密度）+ 内容限宽居中 -->
  <SidebarLayout
    :menu-items="recruiterNav"
    content-width="narrow"
  />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const recruiterNav = roleNavigation.recruiter
</script>