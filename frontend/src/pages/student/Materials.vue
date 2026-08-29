<template>
  <div class="materials-page">
    <div class="page-header">
      <h2>学习资料</h2>
    </div>

    <div class="filter-bar">
      <el-select
        v-model="courseFilter"
        placeholder="全部课程"
        clearable
        filterable
        style="width: 280px"
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

    <div v-loading="loading" class="material-list">
      <template v-if="materials.length > 0">
        <div v-for="item in materials" :key="item.file_id" class="material-item">
          <div class="item-icon" :style="{ background: typeConfig(item.content_type).bg, color: typeConfig(item.content_type).color }">
            <el-icon :size="20"><component :is="typeConfig(item.content_type).icon" /></el-icon>
          </div>
          <div class="item-main">
            <span class="item-name">{{ item.file_name }}</span>
            <span class="item-meta">
              {{ item.course_name }}<template v-if="item.chapter_title"> · {{ item.chapter_title }}</template>
            </span>
          </div>
          <div class="item-side">
            <span class="item-info">{{ formatSize(item.file_size) }} · {{ formatLocaleDateTime(item.created_at || '') }}</span>
            <el-button type="primary" link size="small" @click="download(item)">
              <el-icon><Download /></el-icon>
              下载
            </el-button>
          </div>
        </div>
      </template>
      <UiEmptyState v-else-if="!loading" description="暂无学习资料" />
    </div>

    <div class="pagination-wrapper" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadMaterials"
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
import UiEmptyState from '@/components/ui/UiEmptyState.vue'

const loading = ref(false)
const materials = ref<MaterialItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const courseFilter = ref<number | undefined>(undefined)
const courses = ref<CourseSummary[]>([])

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

async function loadMaterials() {
  loading.value = true
  try {
    const res = await materialApi.list({
      course_id: courseFilter.value,
      page: currentPage.value,
      page_size: pageSize.value
    })
    materials.value = res.materials || []
    total.value = res.total || 0
  } catch (e) {
    console.error('加载学习资料失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
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

<style scoped>
.materials-page {
  padding: 20px;
  max-width: 960px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 12px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}

.filter-bar {
  margin-bottom: 16px;
}

.material-list {
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-height: 200px;
}

.material-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--color-border-light);
}

.material-item:last-child {
  border-bottom: none;
}

.item-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.item-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.item-name {
  font-size: 15px;
  font-weight: 500;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-meta {
  font-size: 12px;
  color: var(--color-text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-side {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.item-info {
  font-size: 12px;
  color: var(--color-text-tertiary);
  white-space: nowrap;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
