<template>
  <PortalLayout :menu-items="tutorNav" hide-footer />
</template>

<script setup lang="ts">
import PortalLayout from './PortalLayout.vue'
import { roleNavigation } from '@/config/navigation'

const tutorNav = roleNavigation.tutor
</script>
