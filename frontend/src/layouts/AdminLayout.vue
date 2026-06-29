<template>
  <SidebarLayout title="管理后台" :menu-items="menuItems" />
</template>

<script setup lang="ts">
import SidebarLayout from './SidebarLayout.vue'
import {
  DataAnalysis, User, Notebook, TrendCharts,
  MagicStick, UserFilled, Calendar, PriceTag
} from '@element-plus/icons-vue'

const menuItems = [
  { path: '/admin/dashboard', label: '数据概览', icon: DataAnalysis },
  { path: '/admin/students', label: '学员管理', icon: User },
  { path: '/admin/tutors', label: '导师管理', icon: UserFilled },
  { path: '/admin/courses', label: '课程管理', icon: Notebook },
  { path: '/admin/statistics', label: '统计分析', icon: TrendCharts },
  { path: '/admin/content-generate', label: '内容生成', icon: MagicStick },
  { path: '/admin/exam-sessions', label: '考试场次', icon: Calendar },
  { path: '/admin/valuation-config', label: '残值配置', icon: PriceTag }
]
</script>
