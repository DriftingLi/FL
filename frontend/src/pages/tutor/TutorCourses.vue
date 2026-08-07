<template>
  <div class="tutor-courses-page">
    <div class="page-header">
      <h2>我的课程</h2>
    </div>

    <div v-loading="loading" class="course-grid">
      <el-empty v-if="!loading && courses.length === 0" description="暂无课程" />

      <div
        v-for="course in courses"
        :key="course.course_id"
        class="course-card"
        @click="goToChapters(course.course_id)"
      >
        <div class="card-cover">
          <img v-if="course.cover_image" :src="course.cover_image" :alt="course.name" class="cover-img" />
          <div v-else class="cover-placeholder">
            <span>{{ course.name.charAt(0) }}</span>
          </div>
        </div>
        <div class="card-body">
          <div class="card-tags">
            <el-tag v-if="levelNameOf(course.level_id)" :type="levelTagType(levelNameOf(course.level_id))" size="small">
              {{ levelNameOf(course.level_id) }}
            </el-tag>
            <el-tag v-if="specialtyNameOf(course.specialty_id)" type="primary" effect="plain" size="small">
              {{ specialtyNameOf(course.specialty_id) }}
            </el-tag>
          </div>
          <h3 class="card-title">{{ course.name }}</h3>
          <p class="card-desc">{{ course.description || '暂无简介' }}</p>
          <div class="card-footer">
            <span>{{ course.chapter_count || 0 }} 个章节</span>
            <el-button type="primary" size="small">
              管理章节 <el-icon><ArrowRight /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="total > pageSize" class="pagination-wrapper">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadCourses"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { tutorApi } from '@/api/tutor'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel } from '@/api/training'

const router = useRouter()
const loading = ref(false)
const courses = ref<{
  course_id: number
  name: string
  cover_image?: string
  description?: string
  chapter_count?: number
  specialty_id?: number | null
  level_id?: number | null
}[]>([])
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const directions = ref<CatalogDirectionNode[]>([])
const levels = ref<CatalogLevel[]>([])

function specialtyNameOf(id?: number | null) {
  if (!id) return ''
  return directions.value.find(d => d.specialty_id === id)?.name || ''
}

function levelNameOf(id?: number | null) {
  if (!id) return ''
  return levels.value.find(l => l.level_id === id)?.name || ''
}

const LEVEL_TAG_TYPES: Record<string, 'success' | 'primary' | 'warning' | 'danger' | 'info'> = {
  入门: 'success',
  进阶: 'primary',
  专项: 'warning',
  认证: 'danger'
}

function levelTagType(name: string) {
  return LEVEL_TAG_TYPES[name] ?? 'info'
}

async function loadCatalog() {
  try {
    const [treeRes, levelsRes] = await Promise.all([
      trainingApi.getCatalogTree(),
      trainingApi.getLevels()
    ])
    if (treeRes.code === 200) {
      directions.value = treeRes.data.specialties || []
    }
    if (levelsRes.code === 200) {
      levels.value = levelsRes.data.levels || []
    }
  } catch (e) {
    // 静默失败：方向/等级标签降级为空
  }
}

async function loadCourses() {
  loading.value = true
  try {
    const res = await tutorApi.getCourses({
      page: currentPage.value,
      page_size: pageSize.value
    })
    if (res.code === 200) {
      courses.value = res.data.courses
      total.value = res.data.total
    }
  } catch (e) {
    console.error('Failed to load courses:', e)
  } finally {
    loading.value = false
  }
}

function goToChapters(courseId: number) {
  router.push(`/training/tutor/course/${courseId}/chapters`)
}

onMounted(() => {
  loadCatalog()
  loadCourses()
})
</script>

<style scoped>
.tutor-courses-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
  margin-bottom: 8px;
}

.course-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.course-card {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  cursor: pointer;
  transition: all 0.3s ease;
}

.course-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

.card-cover {
  height: 160px;
  overflow: hidden;
}

.cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.cover-placeholder span {
  font-size: 48px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: bold;
}

.card-body {
  padding: 16px;
}

.card-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.card-title {
  font-size: 16px;
  color: #303133;
  margin: 8px 0;
  font-weight: 600;
}

.card-desc {
  font-size: 13px;
  color: #909399;
  line-height: 1.5;
  margin-bottom: 12px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  color: #909399;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 24px;
}

@media screen and (max-width: 768px) {
  .course-grid {
    grid-template-columns: 1fr;
  }
}
</style>
