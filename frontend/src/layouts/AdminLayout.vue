<template>
  <SidebarLayout :menu-items="adminNav" />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const adminNav = roleNavigation.admin
</script>
