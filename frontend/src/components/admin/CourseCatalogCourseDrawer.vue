<script setup lang="ts">
// 课程编辑抽屉（全字段 + 章节管理 + 章节对话框）：从 CourseCatalog 拆分，逻辑原样保留。
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { adminApi, type AdminCourseItem, type ChapterPayload } from '@/api/admin'
import type { CatalogDirectionNode, CatalogLevel, CertificateTemplate } from '@/api/training'

const props = defineProps<{
  directions: CatalogDirectionNode[]
  levels: CatalogLevel[]
  certificateTemplates: CertificateTemplate[]
  courseOptions: { course_id: number; name: string }[]
  /** 新增课程时默认选中的方向（当前方向筛选，<=0 视为未选中） */
  defaultSpecialtyId: number | null
  submitting: boolean
}>()

const emit = defineEmits<{ (e: 'saved'): void }>()

const drawerVisible = ref(false)
const courseFormRef = ref<FormInstance | null>(null)
const drawerChapters = ref<{ chapter_id: number; title: string; duration?: number; order_num?: number }[]>([])
const drawerForm = reactive<Record<string, any>>({
  course_id: null,
  name: '',
  specialty_id: null,
  level_id: null,
  status: 1,
  theory_hours: 0,
  practice_hours: 0,
  duration: 0,
  certificate_template_id: null,
  prerequisite_course_ids: [],
  description: ''
})

const courseRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  specialty_id: [{ required: true, message: '请选择专业方向', trigger: 'change' }],
  level_id: [{ required: true, message: '请选择课程等级', trigger: 'change' }]
}

function open(course: AdminCourseItem | null) {
  drawerVisible.value = true
  drawerChapters.value = []
  if (!course) {
    Object.assign(drawerForm, {
      course_id: null,
      name: '',
      specialty_id: props.defaultSpecialtyId !== null && props.defaultSpecialtyId > 0 ? props.defaultSpecialtyId : null,
      level_id: null,
      status: 1,
      theory_hours: 0,
      practice_hours: 0,
      duration: 0,
      certificate_template_id: null,
      prerequisite_course_ids: [],
      description: ''
    })
    return
  }
  Object.assign(drawerForm, {
    course_id: course.course_id,
    name: course.name,
    specialty_id: course.specialty_id,
    level_id: course.level_id,
    status: course.status,
    theory_hours: course.theory_hours || 0,
    practice_hours: course.practice_hours || 0,
    duration: course.duration || 0,
    certificate_template_id: course.certificate_template_id,
    prerequisite_course_ids: course.prerequisite_course_ids || [],
    description: course.description || ''
  })
  void loadDrawerDetail()
}

async function loadDrawerDetail() {
  try {
    const detail = await adminApi.getCourseDetail(drawerForm.course_id)
    if (detail) {
      Object.assign(drawerForm, {
        name: detail.name ?? drawerForm.name,
        specialty_id: detail.specialty_id ?? null,
        level_id: detail.level_id ?? null,
        status: detail.status ?? drawerForm.status,
        theory_hours: detail.theory_hours ?? 0,
        practice_hours: detail.practice_hours ?? 0,
        duration: detail.duration ?? 0,
        certificate_template_id: detail.certificate_template_id ?? null,
        prerequisite_course_ids: detail.prerequisite_course_ids || [],
        description: detail.description ?? ''
      })
      drawerChapters.value = detail.chapters || []
    }
  } catch (error) {
    console.error('加载课程详情失败:', error)
  }
}

async function submitCourse() {
  if (!courseFormRef.value) return
  await courseFormRef.value.validate()
  try {
    const payload = {
      name: drawerForm.name,
      specialty_id: drawerForm.specialty_id,
      level_id: drawerForm.level_id,
      status: drawerForm.status,
      theory_hours: drawerForm.theory_hours,
      practice_hours: drawerForm.practice_hours,
      duration: drawerForm.duration,
      certificate_template_id: drawerForm.certificate_template_id ?? 0,
      prerequisite_course_ids: drawerForm.prerequisite_course_ids,
      description: drawerForm.description
    }
    if (drawerForm.course_id) {
      await adminApi.updateCourse(drawerForm.course_id, payload)
      ElMessage.success('已更新')
    } else {
      await adminApi.createCourse(payload)
      ElMessage.success('已创建')
    }
    drawerVisible.value = false
    emit('saved')
  } catch (error) {
    console.error('保存课程失败:', error)
    /* 错误已由拦截器提示 */
  }
}

// ===== 章节 =====
const chapterDialogVisible = ref(false)
const chapterFormRef = ref<FormInstance | null>(null)
const chapterForm = reactive<{ chapter_id: number | null; title: string; duration: number }>({
  chapter_id: null,
  title: '',
  duration: 0
})

const chapterRules = {
  title: [{ required: true, message: '请输入章节标题', trigger: 'blur' }]
}

function openChapterDialog(ch?: { chapter_id: number; title: string; duration?: number }) {
  chapterForm.chapter_id = ch?.chapter_id ?? null
  chapterForm.title = ch?.title ?? ''
  chapterForm.duration = ch?.duration ?? 0
  chapterDialogVisible.value = true
}

async function submitChapter() {
  if (!chapterFormRef.value) return
  await chapterFormRef.value.validate()
  try {
    const payload = {
      title: chapterForm.title,
      duration: chapterForm.duration
    }
    if (chapterForm.chapter_id) {
      await adminApi.updateChapter(chapterForm.chapter_id, payload)
      ElMessage.success('已更新')
    } else {
      await adminApi.createChapter(drawerForm.course_id, payload)
      ElMessage.success('已创建')
    }
    chapterDialogVisible.value = false
    await loadDrawerDetail()
  } catch (error) {
    console.error('保存章节失败:', error)
    /* 错误已由拦截器提示 */
  }
}

async function handleDeleteChapter(ch: { chapter_id: number }) {
  try {
    await adminApi.deleteChapter(ch.chapter_id)
    ElMessage.success('已删除')
    await loadDrawerDetail()
  } catch (error) {
    console.error('删除章节失败:', error)
    /* 错误已由拦截器提示 */
  }
}

async function moveChapter(ch: { chapter_id: number; order_num?: number }, delta: -1 | 1) {
  const idx = drawerChapters.value.findIndex(c => c.chapter_id === ch.chapter_id)
  const target = drawerChapters.value[idx + delta]
  if (!target) return
  try {
    await adminApi.updateChapter(ch.chapter_id, { order_num: (target.order_num ?? idx + delta) + 0 } as ChapterPayload)
    await adminApi.updateChapter(target.chapter_id, { order_num: (ch.order_num ?? idx) + 0 } as ChapterPayload)
    ElMessage.success('排序已更新')
    await loadDrawerDetail()
  } catch (error) {
    console.error('排序失败:', error)
    /* 错误已由拦截器提示 */
  }
}

defineExpose({ open })
</script>

<template>
  <!-- 课程编辑抽屉（全字段 + 章节管理） -->
  <el-drawer v-model="drawerVisible" :title="drawerForm.course_id ? '编辑课程' : '新增课程'" size="560px">
    <el-form ref="courseFormRef" :model="drawerForm" :rules="courseRules" label-width="96px">
      <el-form-item label="课程名称" prop="name">
        <el-input v-model="drawerForm.name" placeholder="课程名称" maxlength="50" />
      </el-form-item>
      <el-form-item label="专业方向" prop="specialty_id">
        <el-select v-model="drawerForm.specialty_id" placeholder="必选" style="width: 100%">
          <el-option v-for="d in directions" :key="d.specialty_id" :label="d.name" :value="d.specialty_id" />
        </el-select>
      </el-form-item>
      <el-form-item label="课程等级" prop="level_id">
        <el-select v-model="drawerForm.level_id" placeholder="必选" style="width: 100%">
          <el-option v-for="l in levels" :key="l.level_id" :label="l.name" :value="l.level_id" />
        </el-select>
      </el-form-item>
      <el-form-item label="学时">
        <div class="cc-hours">
          <el-input-number v-model="drawerForm.theory_hours" :min="0" :max="999" controls-position="right" />
          <span class="cc-hours-unit">理论</span>
          <el-input-number v-model="drawerForm.practice_hours" :min="0" :max="999" controls-position="right" />
          <span class="cc-hours-unit">实操</span>
        </div>
      </el-form-item>
      <el-form-item label="课程时长">
        <el-input-number v-model="drawerForm.duration" :min="0" :step="10" controls-position="right" />
        <span class="cc-hours-unit">分钟</span>
      </el-form-item>
      <el-form-item label="证书模板">
        <el-select v-model="drawerForm.certificate_template_id" clearable placeholder="不关联" style="width: 100%">
          <el-option v-for="t in certificateTemplates" :key="t.id" :label="t.name" :value="t.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="前置课程">
        <el-select
          v-model="drawerForm.prerequisite_course_ids"
          multiple
          filterable
          collapse-tags
          placeholder="选择需先完成的课程"
          style="width: 100%"
        >
          <el-option
            v-for="c in courseOptions"
            :key="c.course_id"
            :label="c.name"
            :value="c.course_id"
            :disabled="c.course_id === drawerForm.course_id"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="课程简介">
        <el-input v-model="drawerForm.description" type="textarea" :rows="2" maxlength="500" />
      </el-form-item>
      <el-form-item label="上架状态">
        <el-radio-group v-model="drawerForm.status">
          <el-radio :value="1">上架</el-radio>
          <el-radio :value="0">下架</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>

    <div class="cc-chapters" v-if="drawerForm.course_id">
      <div class="cc-chapters-head">
        章节管理
        <el-button size="small" type="primary" plain @click="openChapterDialog()">新增章节</el-button>
      </div>
      <div v-for="(ch, i) in drawerChapters" :key="ch.chapter_id" class="cc-chapter-row">
        <span class="cc-chapter-idx">{{ i + 1 }}</span>
        <span class="cc-chapter-title">{{ ch.title }}</span>
        <el-link :underline="'never'" @click="openChapterDialog(ch)">编辑</el-link>
        <el-link :underline="'never'" @click="moveChapter(ch, -1)">上移</el-link>
        <el-link :underline="'never'" @click="moveChapter(ch, 1)">下移</el-link>
        <el-popconfirm title="确定删除该章节？" @confirm="handleDeleteChapter(ch)">
          <template #reference>
            <el-link type="danger" :underline="'never'">删除</el-link>
          </template>
        </el-popconfirm>
      </div>
      <div v-if="drawerChapters.length === 0" class="cc-chapters-empty">暂无章节</div>
    </div>

    <template #footer>
      <el-button @click="drawerVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitCourse">保存</el-button>
    </template>
  </el-drawer>

  <!-- 章节对话框 -->
  <el-dialog v-model="chapterDialogVisible" :title="chapterForm.chapter_id ? '编辑章节' : '新增章节'" width="520px" destroy-on-close>
    <el-form ref="chapterFormRef" :model="chapterForm" :rules="chapterRules" label-width="90px">
      <el-form-item label="章节标题" prop="title">
        <el-input v-model="chapterForm.title" placeholder="章节标题" maxlength="100" />
      </el-form-item>
      <el-form-item label="时长(分钟)">
        <el-input-number v-model="chapterForm.duration" :min="0" :max="9999" style="width: 100%" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="chapterDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitChapter">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.cc-hours {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cc-hours-unit {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.cc-chapters {
  margin-top: var(--space-4);
  border-top: 1px solid var(--color-border-light);
  padding-top: var(--space-3);
}

.cc-chapters-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  margin-bottom: var(--space-2);
}

.cc-chapter-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-1);
  border-bottom: 1px dashed var(--color-border-light);
  font-size: var(--text-sm);
}

.cc-chapter-idx {
  width: 20px;
  color: var(--color-text-tertiary);
}

.cc-chapter-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-chapters-empty {
  color: var(--color-text-tertiary);
  font-size: var(--text-xs);
  padding: var(--space-3) 0;
}

@media screen and (max-width: 768px) {
  .cc-hours {
    flex-wrap: wrap;
  }

  .cc-hours :deep(.el-input-number) {
    flex: 1;
    min-width: 90px;
  }
}
</style>
