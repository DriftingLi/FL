<template>
  <PortalLayout :menu-items="adminNav" hide-footer />
</template>

<script setup lang="ts">
import PortalLayout from './PortalLayout.vue'
import { roleNavigation } from '@/config/navigation'

const adminNav = roleNavigation.admin
</script>
