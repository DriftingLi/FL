<template>
  <SidebarLayout :menu-items="currentMenuItems" />
</template>

<script setup lang="ts">
import { computed, h, watch } from 'vue'
import type { Component } from 'vue'
import { useRoute } from 'vue-router'
import SidebarLayout from './SidebarLayout.vue'
import { roleNavigation } from '@/config/navigation'
import { useCourseStore } from '@/stores/course'
import type { NavItem } from '@/config/navigation'

const studentNav = roleNavigation.student
const route = useRoute()
const courseStore = useCourseStore()

// 章节学习路由下进入"课程章节模式"：侧栏第一项为返回课程中心，其余为章节列表
const chapterCourseId = computed(() => {
  if (route.name === 'ChapterView') {
    return route.params.courseId as string
  }
  return null
})

// 进入章节模式时加载课程章节（store 内部已做缓存与并发去重）
watch(chapterCourseId, (courseId) => {
  if (courseId) courseStore.loadCourse(courseId)
}, { immediate: true })

// 自定义"返回"图标：左上分叉箭头 + 水平短线 + 弯弧向右下铺满图标区
// element-plus 内置图标均无此形态，使用内联 SVG 与图标组件 API 一致
const BackIcon: Component = {
  name: 'BackIcon',
  render() {
    return h(
      'svg',
      {
        xmlns: 'http://www.w3.org/2000/svg',
        viewBox: '0 0 1024 1024',
        'stroke-width': '96',
        stroke: 'currentColor',
        fill: 'none',
        strokeLinecap: 'round',
        strokeLinejoin: 'round'
      },
      [
        // 主体：左上分叉点水平向右 → 二次贝塞尔曲线弯到右下，贴近画布边缘
        h('path', {
          d: 'M 160 512 L 500 512 Q 880 512 880 880'
        }),
        // 分叉型箭头：两条线从尖端点向斜上、斜下伸出（V 形分叉，不是封闭三角）
        h('polyline', {
          points: '320 352, 160 512, 320 672'
        })
      ]
    )
  }
}

// 章节数字图标：用 SVG <text> 渲染序号，跟随父级 currentColor 主题色。
// SVG width/height: 100% 自适应 el-icon 容器；font-size 用 px 在 100×100 viewBox 内足够清晰，
// 侧栏折叠时（仅显示图标）数字仍能直观看到是第几章，避免和考试中心 Document 图标重复。
function createChapterIcon(num: number): Component {
  return {
    name: `ChapterIcon${num}`,
    render() {
      return h(
        'svg',
        {
          xmlns: 'http://www.w3.org/2000/svg',
          viewBox: '0 0 100 100',
          width: '100%',
          height: '100%',
          fill: 'currentColor'
        },
        [
          h('text', {
            x: '50',
            y: '50',
            'text-anchor': 'middle',
            'dominant-baseline': 'central',
            'font-size': '60',
            'font-weight': '600',
            'font-family': 'var(--font-display, system-ui, sans-serif)'
          }, String(num))
        ]
      )
    }
  }
}

const currentMenuItems = computed<NavItem[]>(() => {
  const courseId = chapterCourseId.value
  if (!courseId) return studentNav

  const chapterItems: NavItem[] = courseStore.chapters.map((ch, index) => ({
    key: `chapter-${ch.chapter_id}`,
    label: `${index + 1}. ${ch.title}`,
    path: `/training/course/${courseId}/chapter/${ch.chapter_id}`,
    icon: createChapterIcon(index + 1)
  }))

  return [
    {
      key: 'back-to-courses',
      label: '返回课程中心',
      path: '/training/courses',
      icon: BackIcon
    },
    ...chapterItems
  ]
})
</script>
