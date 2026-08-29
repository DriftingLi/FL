<template>
  <div>
    <UiPageHeader
      :title="pageTitle"
      :subtitle="chapters.length > 0 ? `共 ${chapters.length} 个章节` : undefined"
      back
      @back="goBack"
    >
      <template #meta>
        <p class="text-sm text-ink-3">点击章节进入编辑，可修改正文、上传与预览多种类型内容</p>
      </template>
    </UiPageHeader>

    <UiErrorState
      v-if="loadError"
      title="章节加载失败"
      description="未能获取该课程的章节列表，请检查网络后重试。"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <UiSkeleton v-else-if="loading" variant="list" :count="5" />

    <UiEmptyState
      v-else-if="chapters.length === 0"
      title="暂无章节"
      description="该课程还没有任何章节，可返回课程列表继续其他操作。"
      action-text="返回课程列表"
      @action="goBack"
    />

    <div v-else class="flex flex-col gap-3">
      <UiCard
        v-for="(chapter, index) in chapters"
        :key="chapter.chapter_id"
        variant="interactive"
        class="group flex items-center gap-4"
        @click="goToEdit(chapter.chapter_id)"
      >
        <span
          class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-ui-50 text-[13px] font-semibold text-ui-600"
        >
          {{ String(index + 1).padStart(2, '0') }}
        </span>

        <div class="min-w-0 flex-1">
          <p class="truncate text-[15px] font-medium text-ink">{{ chapter.title }}</p>
          <div class="mt-2 flex flex-wrap items-center gap-3">
            <UiTag :tone="contentTypeTone(chapter.content_type || '')">
              {{ contentTypeLabel(chapter.content_type || '') }}
            </UiTag>
            <span v-if="chapter.duration" class="inline-flex items-center gap-1 text-xs text-ink-3">
              <el-icon><Timer /></el-icon> {{ chapter.duration }}分钟
            </span>
            <span v-if="getFileCount(chapter) > 0" class="inline-flex items-center gap-1 text-xs text-ink-3">
              <el-icon><Document /></el-icon> {{ getFileCount(chapter) }} 个文件
            </span>
            <span v-if="chapter.content" class="inline-flex items-center gap-1 text-xs text-ok">
              <el-icon><EditPen /></el-icon> 含正文
            </span>
          </div>
        </div>

        <el-icon class="shrink-0 text-ink-muted transition-colors duration-150 group-hover:text-ui-600 max-sm:hidden">
          <ArrowRight />
        </el-icon>
      </UiCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Timer, Document, EditPen } from '@element-plus/icons-vue'
import { tutorApi, type TutorChapter, type TutorCourse } from '@/api/tutor'
import UiCard from '@/components/ui/UiCard.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'

type ChapterRow = TutorChapter & { content_type?: string; files?: unknown[] }
type ContentTone = 'brand' | 'success' | 'warning' | 'danger' | 'neutral'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const loadError = ref(false)
const retrying = ref(false)
const courseInfo = ref<TutorCourse | null>(null)
const chapters = ref<ChapterRow[]>([])

const pageTitle = computed(() =>
  courseInfo.value?.name ? `${courseInfo.value.name} · 章节管理` : '章节管理'
)

/** 内容类型 → 语义色调。视频不再用 danger（红色=告警语义），改用 brand 石墨青。 */
const CONTENT_TONE: Record<string, ContentTone> = {
  text: 'neutral',
  document: 'brand',
  ppt: 'warning',
  video: 'brand',
  image: 'success'
}

const CONTENT_LABEL: Record<string, string> = {
  text: '图文',
  document: '文档',
  ppt: 'PPT',
  video: '视频',
  image: '图片'
}

function contentTypeTone(contentType: string): ContentTone {
  return CONTENT_TONE[contentType] || 'neutral'
}

function contentTypeLabel(contentType: string): string {
  return CONTENT_LABEL[contentType] || contentType || '未设置'
}

function getFileCount(chapter: { files?: unknown[] }): number {
  return (chapter.files || []).length
}

async function loadChapters() {
  loading.value = true
  loadError.value = false
  try {
    const courseId = Number(route.params.id)
    const res = await tutorApi.getCourseChapters(courseId)
    courseInfo.value = res.course ?? null
    chapters.value = res.chapters || []
  } catch (e) {
    loadError.value = true
    console.error('Failed to load chapters:', e)
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}

async function handleRetry() {
  retrying.value = true
  try {
    await loadChapters()
  } finally {
    retrying.value = false
  }
}

function goBack() {
  router.push({ name: 'TutorCourses' })
}

function goToEdit(chapterId: number) {
  const courseId = route.params.id
  router.push({ name: 'TutorChapterEdit', params: { courseId, chapterId } })
}

onMounted(() => {
  loadChapters()
})
</script>
