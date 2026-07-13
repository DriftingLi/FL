<template>
  <SidebarLayout :menu-items="studentNav" />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const studentNav = roleNavigation.student
</script>
