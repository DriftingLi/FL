<template>
  <!-- 导师端：与其余三端同一套外壳（默认密度）+ 内容限宽居中。
       侧栏密度自 ADR-0081 起只剩单档（原 compact 变体已退役），四端观感一致。 -->
  <SidebarLayout
    :menu-items="tutorNav"
    content-width="narrow"
  />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const tutorNav = roleNavigation.tutor
</script>