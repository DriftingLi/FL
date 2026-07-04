<template>
  <PortalLayout :menu-items="studentNav" hide-footer />
</template>

<script setup lang="ts">
import PortalLayout from './PortalLayout.vue'
import { roleNavigation } from '@/config/navigation'

const studentNav = roleNavigation.student
</script>
