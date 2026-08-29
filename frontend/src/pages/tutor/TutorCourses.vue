<template>
  <div class="tutor-courses-page">
    <div class="page-header">
      <h2>我的课程</h2>
      <el-select v-model="credentialId" placeholder="全部证件" clearable style="width: 180px" @change="loadCourses">
        <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
    </div>

    <div class="course-layout">
      <!-- 左栏：双卡片（专业方向 / 课程等级），与学员端课程中心共享筛选 module -->
      <aside class="cc-sidebar">
        <FacetCard title="专业方向">
          <FacetItem
            :active="specialtyId === null"
            name="全部课程"
            :count="totalAll"
            @select="selectDirection(null)"
          />
          <FacetItem
            v-for="d in directions"
            :key="d.specialty_id"
            :active="specialtyId === d.specialty_id"
            :name="d.name"
            :count="countOfDirection(d.specialty_id)"
            @select="selectDirection(d.specialty_id)"
          />
        </FacetCard>

        <FacetCard title="课程等级">
          <FacetItem
            :active="levelId === null"
            name="全部等级"
            :count="scopedTotal"
            @select="selectLevel(null)"
          />
          <FacetItem
            v-for="l in levels"
            :key="l.level_id"
            :active="levelId === l.level_id"
            :name="l.name"
            :count="countOfLevel(l.level_id)"
            @select="selectLevel(l.level_id)"
          />
        </FacetCard>
      </aside>

      <!-- 右栏：课程网格 -->
      <main class="cc-main">
        <div v-loading="loading" class="course-grid">
          <el-empty v-if="!loading && courses.length === 0" description="该方向/等级下暂无课程" />

          <CourseCard
            v-for="course in courses"
            :key="course.course_id"
            :name="course.name"
            :description="course.description"
            :cover-image="course.cover_image"
            :specialty-id="course.specialty_id"
            @click="goToChapters(course.course_id)"
          >
            <template #tags>
              <div class="card-tags">
                <el-tag
                  v-if="levelNameOf(course.level_id)"
                  :type="levelTagType(levelNameOf(course.level_id))"
                  size="small"
                >
                  {{ levelNameOf(course.level_id) }}
                </el-tag>
                <el-tag
                  v-if="specialtyNameOf(course.specialty_id)"
                  type="primary"
                  effect="plain"
                  size="small"
                >
                  {{ specialtyNameOf(course.specialty_id) }}
                </el-tag>
                <el-tag v-if="(course as any).credential_id" size="small" effect="plain">{{ credentials.find(c => c.id === (course as any).credential_id)?.name || '' }}</el-tag>
              </div>
            </template>
            <template #meta>
              <div class="card-footer">
                <span>{{ course.chapter_count || 0 }} 个章节</span>
                <el-button type="primary" size="small">
                  管理章节 <el-icon><ArrowRight /></el-icon>
                </el-button>
              </div>
            </template>
          </CourseCard>
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
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { tutorApi, type TutorCourse } from '@/api/tutor'
import { trainingApi } from '@/api/training'
import { credentialApi, type CredentialDict } from '@/api/credential'
import { levelTagType } from '@/constants/level'
import { useCourseCatalog, treeCatalogAdapter } from '@/composables/useCourseCatalog'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCard from '@/components/catalog/CourseCard.vue'

const router = useRouter()
const credentials = ref<CredentialDict[]>([])
const credentialId = ref<number | null>(null)
const loading = ref(false)
const courses = ref<TutorCourse[]>([])
const currentPage = ref(1)
const pageSize = ref(12)
const total = ref(0)

const {
  directions,
  levels,
  specialtyId,
  levelId,
  totalAll,
  scopedTotal,
  countOfDirection,
  countOfLevel,
  selectDirection,
  selectLevel,
  fetchCatalog,
  levelNameOf,
  specialtyNameOf
} = useCourseCatalog({
  adapter: treeCatalogAdapter(() => trainingApi.getCatalogTree()),
  onSelect: () => {
    currentPage.value = 1
    loadCourses()
  }
})

async function loadCredentials() {
  try {
    const data = await credentialApi.listCredentials()
    credentials.value = data.credentials || []
  } catch {}
}

async function loadCourses() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (credentialId.value !== null) (params as any).credential_id = credentialId.value
    if (specialtyId.value !== null) params.specialty_id = specialtyId.value
    if (levelId.value !== null) params.level_id = levelId.value
    const res = await tutorApi.getCourses(params)
    courses.value = res.courses
    total.value = res.total
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
  fetchCatalog()
  loadCourses()
  loadCredentials()
})
</script>

<style scoped>
.tutor-courses-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
  margin-bottom: 8px;
}

.course-layout {
  display: flex;
  align-items: flex-start;
  gap: var(--space-5);
}

/* ===== 左栏 ===== */
.cc-sidebar {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* ===== 右栏 ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}

.course-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: var(--space-4);
}

.card-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: var(--space-1);
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: var(--space-5);
}

@media (max-width: 900px) {
  .course-layout {
    flex-direction: column;
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
  }
}
</style>
