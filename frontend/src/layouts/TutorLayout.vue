<template>
  <SidebarLayout title="导师工作台" :menu-items="menuItems" />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import { Notebook, EditPen, Finished } from '@element-plus/icons-vue'

const menuItems = [
  { path: '/tutor/courses', label: '我的课程', icon: Notebook },
  { path: '/tutor/question-manage', label: '题库管理', icon: EditPen },
  { path: '/tutor/grading', label: '人工阅卷', icon: Finished }
]
</script>
