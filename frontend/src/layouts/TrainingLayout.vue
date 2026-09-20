<template>
  <!-- 学员端：石墨青暗底侧栏 + 紧凑密度 + 内容限宽居中。
       AdminLayout / TutorLayout 不传这些 prop，走默认值（=改造前行为），零 diff。 -->
  <SidebarLayout
    :menu-items="currentMenuItems"
    sidebar-theme="dark"
    sidebar-density="compact"
    content-width="narrow"
  >
    <template #top="{ collapsed }">
      <div class="flex flex-col gap-2">
        <CredentialSwitcher v-if="!chapterCourseId" :collapsed="collapsed" theme="dark" />
        <!-- 布局级搜索入口（#984）：侧栏顶部一处 + ⌘/Ctrl+K 全工作区可达。
             暗底上必须**显式给底色**：本仓有意不引入 Tailwind preflight（见 tailwind.css 注释），
             裸 <button> 会保留浏览器默认浅底，配浅色文字就成了亮色药丸。
             配色照 CredentialSwitcher 的 dark 分支：白 8% 填充 + 12% 内描边 + 浅色文字。 -->
        <button
          type="button"
          class="flex items-center gap-2 rounded-md bg-white/[0.08] px-2.5 py-1.5 text-[13px] text-white/70 ring-1 ring-white/[0.12] ring-inset transition-colors hover:bg-white/[0.16] hover:text-white"
          :class="collapsed ? 'justify-center' : ''"
          aria-label="全局搜索"
          @click="openSearch"
        >
          <el-icon><Search /></el-icon>
          <template v-if="!collapsed">
            <span>搜索</span>
            <span class="ml-auto rounded border border-white/20 px-1 text-[11px] text-white/60">⌘K</span>
          </template>
        </button>
      </div>
    </template>
  </SidebarLayout>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, watch } from 'vue'
import type { Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import SidebarLayout from './SidebarLayout.vue'
import { href } from '@/config/pages'
import CredentialSwitcher from '@/components/credential/CredentialSwitcher.vue'
import { roleNavigation } from '@/config/navigation'
import { useCourseStore } from '@/stores/course'
import type { NavItem } from '@/config/navigation'

const studentNav = roleNavigation.student
const route = useRoute()
const router = useRouter()
const courseStore = useCourseStore()

/** 布局级搜索入口（#984）：已在搜索页则直接聚焦（自定义事件），否则带 focus=1 跳过去。 */
function openSearch(): void {
  if (route.path === '/training/search') {
    window.dispatchEvent(new Event('focus-global-search'))
    return
  }
  void router.push({ ...href('StudentSearch'), query: { focus: '1' } })
}

/** 事件目标是不是可编辑元素（输入框 / 文本域 / 下拉 / contenteditable）。 */
function isEditableTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el || !el.tagName) return false
  return el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable === true
}

/** ⌘/Ctrl+K 打开搜索（全工作区可达）；卸载时摘掉监听，避免布局切换后残留。 */
function onGlobalKeydown(e: KeyboardEvent): void {
  if (!(e.metaKey || e.ctrlKey) || (e.key !== 'k' && e.key !== 'K')) return
  // 编辑区内的 Ctrl+K 是编辑器 / 浏览器的既有绑定（发帖、搜索框、笔记），不劫持
  if (isEditableTarget(e.target)) return
  e.preventDefault()
  openSearch()
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
})

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
    routeName: 'ChapterView',
    routeParams: { courseId, chapterId: ch.chapter_id },
    icon: createChapterIcon(index + 1)
  }))

  return [
    {
      key: 'back-to-courses',
      label: '返回课程中心',
      routeName: 'CourseList',
      icon: BackIcon
    },
    ...chapterItems
  ]
})
</script>
