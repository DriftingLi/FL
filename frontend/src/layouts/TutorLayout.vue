<template>
  <SidebarLayout :menu-items="tutorNav" />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'

const tutorNav = roleNavigation.tutor
</script>
