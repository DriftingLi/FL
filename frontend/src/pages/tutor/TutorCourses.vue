<template>
  <div class="tutor-courses-page">
    <div class="page-header">
      <h2>我的课程</h2>
      <el-button type="primary" @click="openCourseDialog()">新增课程</el-button>
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
            <el-button size="small" @click.stop="openCourseDialog(course)">编辑</el-button>
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

    <!-- 课程编辑（新建/修改，方向/等级必填） -->
    <el-dialog v-model="courseDialogVisible" :title="courseForm.course_id ? '编辑课程' : '新增课程'" width="560px" destroy-on-close>
      <el-form ref="courseFormRef" :model="courseForm" :rules="courseRules" label-width="96px">
        <el-form-item label="课程名称" prop="name">
          <el-input v-model="courseForm.name" placeholder="课程名称" maxlength="50" />
        </el-form-item>
        <el-form-item label="专业方向" prop="specialty_id">
          <el-select v-model="courseForm.specialty_id" placeholder="必选" style="width: 100%">
            <el-option v-for="d in directions" :key="d.specialty_id" :label="d.name" :value="d.specialty_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="课程等级" prop="level_id">
          <el-select v-model="courseForm.level_id" placeholder="必选" style="width: 100%">
            <el-option v-for="l in levels" :key="l.level_id" :label="l.name" :value="l.level_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="学时">
          <div class="tc-hours">
            <el-input-number v-model="courseForm.theory_hours" :min="0" :max="999" controls-position="right" />
            <span class="tc-hours-unit">理论</span>
            <el-input-number v-model="courseForm.practice_hours" :min="0" :max="999" controls-position="right" />
            <span class="tc-hours-unit">实操</span>
          </div>
        </el-form-item>
        <el-form-item label="证书模板">
          <el-select v-model="courseForm.certificate_template_id" clearable placeholder="不关联" style="width: 100%">
            <el-option v-for="t in certificateTemplates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="课程简介">
          <el-input v-model="courseForm.description" type="textarea" :rows="2" maxlength="500" />
        </el-form-item>
        <el-form-item label="上架状态">
          <el-radio-group v-model="courseForm.status">
            <el-radio :value="1">上架</el-radio>
            <el-radio :value="0">下架</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="courseDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCourse">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { tutorApi, type TutorCourse } from '@/api/tutor'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel, type CertificateTemplate } from '@/api/training'
import { levelTagType } from '@/constants/level'

const router = useRouter()
const loading = ref(false)
const courses = ref<TutorCourse[]>([])
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

// ===== 课程新建/编辑（方向/等级必填，与管理端同校验） =====
const courseDialogVisible = ref(false)
const submitting = ref(false)
const courseFormRef = ref<FormInstance | null>(null)
const certificateTemplates = ref<CertificateTemplate[]>([])
const courseForm = reactive<Record<string, any>>({
  course_id: null,
  name: '',
  specialty_id: null,
  level_id: null,
  theory_hours: 0,
  practice_hours: 0,
  certificate_template_id: null,
  description: '',
  status: 1
})

const courseRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  specialty_id: [{ required: true, message: '请选择专业方向', trigger: 'change' }],
  level_id: [{ required: true, message: '请选择课程等级', trigger: 'change' }]
}

async function loadCertificateTemplates() {
  try {
    const res = await trainingApi.getCertificateTemplates()
    if (res.code === 200) {
      certificateTemplates.value = res.data.certificate_templates || []
    }
  } catch (e) {
    // 静默失败：证书下拉降级为空
  }
}

function openCourseDialog(course?: (typeof courses.value)[number] | null) {
  if (!course) {
    Object.assign(courseForm, {
      course_id: null,
      name: '',
      specialty_id: null,
      level_id: null,
      theory_hours: 0,
      practice_hours: 0,
      certificate_template_id: null,
      description: '',
      status: 1
    })
  } else {
    Object.assign(courseForm, {
      course_id: course.course_id,
      name: course.name,
      specialty_id: course.specialty_id,
      level_id: course.level_id,
      theory_hours: course.theory_hours || 0,
      practice_hours: course.practice_hours || 0,
      certificate_template_id: course.certificate_template_id,
      description: course.description || '',
      status: course.status ?? 1
    })
  }
  courseDialogVisible.value = true
}

async function submitCourse() {
  if (!courseFormRef.value) return
  await courseFormRef.value.validate()
  submitting.value = true
  try {
    const payload = {
      name: courseForm.name,
      specialty_id: courseForm.specialty_id,
      level_id: courseForm.level_id,
      theory_hours: courseForm.theory_hours,
      practice_hours: courseForm.practice_hours,
      certificate_template_id: courseForm.certificate_template_id ?? 0,
      description: courseForm.description,
      status: courseForm.status
    }
    if (courseForm.course_id) {
      const res = await tutorApi.updateCourse(courseForm.course_id, payload)
      if (res.code === 200) ElMessage.success('已更新')
    } else {
      const res = await tutorApi.createCourse(payload)
      if (res.code === 201) ElMessage.success('已创建')
    }
    courseDialogVisible.value = false
    await loadCourses()
  } catch (e) {
    console.error('保存课程失败:', e)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadCatalog()
  loadCourses()
  loadCertificateTemplates()
})
</script>

<style scoped>
.tutor-courses-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
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

.tc-hours {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tc-hours-unit {
  font-size: 12px;
  color: var(--color-text-tertiary);
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
