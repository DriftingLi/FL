<template>
  <div class="mx-auto max-w-[960px] p-5">
    <div class="mb-3">
      <h2 class="text-[22px] text-ink">学习资料</h2>
    </div>

    <div class="mb-4">
      <el-select
        v-model="courseFilter"
        placeholder="全部课程"
        clearable
        filterable
        class="w-[280px]"
        @change="handleFilterChange"
      >
        <el-option
          v-for="course in courses"
          :key="course.course_id"
          :label="course.name"
          :value="course.course_id"
        />
      </el-select>
    </div>

    <div class="min-h-[200px] rounded-card bg-panel shadow-card">
      <UiErrorState
        v-if="loadError"
        title="资料加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />

      <UiSkeleton v-else-if="loading" variant="list" :count="5" />

      <template v-else-if="materials.length > 0">
        <div
          v-for="(item, i) in materials"
          :key="item.file_id"
          class="stagger-in flex items-center gap-3.5 border-b border-line px-5 py-3.5 last:border-b-0"
          :style="staggerStyle(i)"
        >
          <div
            class="flex size-11 shrink-0 items-center justify-center rounded-[8px]"
            :style="{ background: typeConfig(item.content_type).bg, color: typeConfig(item.content_type).color }"
          >
            <el-icon :size="20"><component :is="typeConfig(item.content_type).icon" /></el-icon>
          </div>
          <div class="flex min-w-0 flex-1 flex-col gap-1">
            <span class="truncate text-[15px] font-medium text-ink">{{ item.file_name }}</span>
            <span class="truncate text-xs text-ink-3">
              {{ item.course_name }}<template v-if="item.chapter_title"> · {{ item.chapter_title }}</template>
            </span>
          </div>
          <div class="flex shrink-0 items-center gap-3">
            <span class="whitespace-nowrap text-xs text-ink-3">{{ formatSize(item.file_size) }} · {{ formatLocaleDateTime(item.created_at || '') }}</span>
            <UiButton variant="primary" link size="small" @click="download(item)">
              <el-icon><Download /></el-icon>
              下载
            </UiButton>
          </div>
        </div>
      </template>
      <UiEmptyState v-else description="暂无学习资料" />
    </div>

    <div class="mt-4 flex justify-center" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Document, VideoCamera, Picture, Download } from '@element-plus/icons-vue'
import { materialApi, type MaterialItem } from '@/api/material'
import { courseApi, type CourseSummary } from '@/api/course'
import { resolveFileUrl } from '@/utils/fileUrl'
import { formatLocaleDateTime } from '@/utils/format'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'

const materials = ref<MaterialItem[]>([])
const courseFilter = ref<number | undefined>(undefined)
const courses = ref<CourseSummary[]>([])

// 三态 + 分页三件套收编（#388）
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page: currentPage,
  pageSize,
  total,
  run: loadMaterials,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await materialApi.list({
    course_id: courseFilter.value,
    page: currentPage.value,
    page_size: pageSize.value
  })
  materials.value = res.materials || []
  total.value = res.total || 0
})

const staggerStyle = useStagger()

const TYPE_CONFIG: Record<string, { label: string; icon: any; color: string; bg: string }> = {
  document: { label: '文档', icon: Document, color: 'var(--color-primary-500)', bg: 'var(--color-primary-50)' },
  video: { label: '视频', icon: VideoCamera, color: 'var(--color-danger)', bg: 'var(--color-danger-light)' },
  ppt: { label: 'PPT', icon: Document, color: 'var(--color-warning)', bg: 'var(--color-warning-light)' },
  image: { label: '图片', icon: Picture, color: 'var(--color-success)', bg: 'var(--color-success-light)' }
}

function typeConfig(contentType?: string) {
  return TYPE_CONFIG[contentType || 'document'] || TYPE_CONFIG.document
}

function formatSize(bytes?: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)}MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)}GB`
}

function handleFilterChange() {
  currentPage.value = 1
  loadMaterials()
}

async function loadCourses() {
  try {
    // 课程筛选选项（页大小取大值覆盖全部课程）
    const res = await courseApi.getCourses({ page: 1, page_size: 100 })
    courses.value = res.courses || []
  } catch (e) {
    console.error('加载课程列表失败:', e)
  }
}

function download(item: MaterialItem) {
  window.open(resolveFileUrl(item.file_url), '_blank')
}

onMounted(() => {
  loadCourses()
  loadMaterials()
})
</script>
