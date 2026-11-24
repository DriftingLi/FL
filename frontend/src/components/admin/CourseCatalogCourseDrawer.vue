<script setup lang="ts">
// 课程编辑抽屉（全字段 + 章节管理 + 章节对话框）：从 CourseCatalog 拆分，逻辑原样保留。
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { adminApi, type AdminCourseItem, type ChapterPayload } from '@/api/admin'
import { useConfirm } from '@/composables/useConfirm'
import type { CertificateTemplate, TrainingCategoryDTO } from '@/api/training'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiRadioGroup from '@/components/ui/UiRadioGroup.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

const props = defineProps<{
  certificateTemplates: CertificateTemplate[]
  courseOptions: { course_id: number; name: string }[]
  categories: TrainingCategoryDTO[]
  defaultCategoryId: number | null
  submitting: boolean
}>()

const emit = defineEmits<{ (e: 'saved'): void }>()

const selectedCategory = ref<TrainingCategoryDTO | null>(null)
const categoryOptions = computed(() => {
  const options = props.categories
    .filter(category => (category.status ?? 1) !== 0)
    .map(category => ({ label: category.name, value: category.id }))
  const current = selectedCategory.value
  if (current && !options.some(option => option.value === current.id)) {
    options.push({
      label: `${current.name}（已停用，保留当前归属）`,
      value: current.id
    })
  }
  return options
})

const drawerVisible = ref(false)
const courseFormRef = ref<FormInstance | null>(null)
const drawerChapters = ref<{ chapter_id: number; title: string; duration?: number; order_num?: number }[]>([])
const drawerForm = reactive<Record<string, any>>({
  course_id: null,
  name: '',
  category_id: null,
  // 课程主类型（#2760）：video | illustrated。创建必选、编辑默认回填原值。
  // 默认值不预设（null）：让管理员真的选一次，而不是悄悄落一个「没选过」的类型。
  course_type: null,
  status: 1,
  theory_hours: 0,
  practice_hours: 0,
  duration: 0,
  points_price: null,
  certificate_template_id: null,
  prerequisite_course_ids: [],
  description: '',
  is_hot: false,
  is_featured: false
})

const courseRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  category_id: [{ required: true, message: '请选择课程分类', trigger: 'change' }],
  // 类型必填（#2760）：公开目录的分流全靠这一格，缺了访客就找不到这门课。
  course_type: [{ required: true, message: '请选择课程类型', trigger: 'change' }],
  // 积分校验口径与后端一致（#2447）：≥1 的整数为定价，留空 = 免费（落 NULL）。
  points_price: [{
    validator: (_rule: unknown, value: unknown, callback: (e?: Error) => void) => {
      if (value === null || value === undefined || value === '') return callback()
      if (Number.isInteger(value) && (value as number) >= 1) return callback()
      return callback(new Error('积分价须为 ≥1 的整数，留空=免费'))
    },
    trigger: 'change'
  }]
}

function open(course: AdminCourseItem | null) {
  drawerVisible.value = true
  drawerChapters.value = []
  selectedCategory.value = null
  if (!course) {
    Object.assign(drawerForm, {
      course_id: null,
      name: '',
      category_id: props.defaultCategoryId,
      course_type: null,
      status: 1,
      theory_hours: 0,
      practice_hours: 0,
      duration: 0,
      points_price: null,
      certificate_template_id: null,
      prerequisite_course_ids: [],
      description: '',
      is_hot: false,
      is_featured: false
    })
    return
  }
  selectedCategory.value = course.category
    ?? props.categories.find(category => category.id === course.category_id)
    ?? null
  Object.assign(drawerForm, {
    course_id: course.course_id,
    name: course.name,
    category_id: course.category_id ?? null,
    course_type: (course as any).course_type ?? null,
    status: course.status,
    theory_hours: course.theory_hours || 0,
    practice_hours: course.practice_hours || 0,
    duration: course.duration || 0,
    points_price: course.points_price ?? null,
    certificate_template_id: course.certificate_template_id,
    prerequisite_course_ids: course.prerequisite_course_ids || [],
    description: course.description || '',
    is_hot: !!(course as any).is_hot,
    is_featured: !!(course as any).is_featured
  })
  void loadDrawerDetail()
}

async function loadDrawerDetail() {
  try {
    const id = drawerForm.course_id
    const detail = await adminApi.getCourseDetail(id)
    if (detail && drawerForm.course_id === id) {
      selectedCategory.value = detail.category
        ?? props.categories.find(category => category.id === detail.category_id)
        ?? selectedCategory.value
      Object.assign(drawerForm, {
        name: detail.name ?? drawerForm.name,
        category_id: detail.category_id ?? null,
        course_type: (detail as any).course_type ?? drawerForm.course_type,
        status: detail.status ?? drawerForm.status,
        theory_hours: detail.theory_hours ?? 0,
        practice_hours: detail.practice_hours ?? 0,
        duration: detail.duration ?? 0,
        points_price: detail.points_price ?? null,
        certificate_template_id: detail.certificate_template_id ?? null,
        prerequisite_course_ids: detail.prerequisite_course_ids || [],
        description: detail.description ?? '',
        is_hot: !!(detail as any).is_hot,
        is_featured: !!(detail as any).is_featured
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
      category_id: drawerForm.category_id,
      course_type: drawerForm.course_type,
      status: drawerForm.status,
      theory_hours: drawerForm.theory_hours,
      practice_hours: drawerForm.practice_hours,
      duration: drawerForm.duration,
      // 空输入 = null = 显式清空（免费）；键恒携带，后端靠它与「未携带=保留」分家（#2447）
      points_price: drawerForm.points_price ?? null,
      certificate_template_id: drawerForm.certificate_template_id ?? 0,
      prerequisite_course_ids: drawerForm.prerequisite_course_ids,
      description: drawerForm.description,
      is_hot: drawerForm.is_hot,
      is_featured: drawerForm.is_featured
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
  // #2498 C7：管理员写面直接生效（不走审核队列），改前确认。
  if (chapterForm.chapter_id) {
    await useConfirm().confirm(
      '管理员保存将直接生效并同步已发布版本（不走审核队列），确认保存？',
      '直接生效确认'
    )
  }
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
      <el-form-item label="课程分类" prop="category_id">
        <UiSelect v-model="drawerForm.category_id" placeholder="请选择课程分类" class="w-full" :options="categoryOptions" />
      </el-form-item>
      <el-form-item label="课程名称" prop="name">
        <UiInput v-model="drawerForm.name" placeholder="课程名称" :maxlength="50" />
      </el-form-item>
      <el-form-item label="课程类型" prop="course_type">
        <UiRadioGroup v-model="drawerForm.course_type">
          <el-radio value="video">视频课程</el-radio>
          <el-radio value="illustrated">图文课程（含 PPT）</el-radio>
        </UiRadioGroup>
        <div class="cc-hint">公开目录按类型分流。一门课的章节内容必须同质：视频课不能放图文章节，反之亦然。</div>
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
      <el-form-item label="积分兑换价" prop="points_price">
        <el-input-number v-model="drawerForm.points_price" :min="0" :step="1" controls-position="right" />
        <span class="cc-hours-unit">留空 = 免费</span>
      </el-form-item>
      <el-form-item label="证书模板">
        <UiSelect v-model="drawerForm.certificate_template_id" clearable placeholder="不关联" style="width: 100%" :options="(certificateTemplates).map((t) => ({ label: t.name, value: t.id }))" />
      </el-form-item>
      <el-form-item label="前置课程">
        <UiSelect v-model="drawerForm.prerequisite_course_ids" multiple filterable collapse-tags placeholder="选择需先完成的课程" style="width: 100%" :options="(courseOptions).map((c) => ({ label: c.name, value: c.course_id, disabled: c.course_id === drawerForm.course_id }))" />
      </el-form-item>
      <el-form-item label="课程简介">
        <UiInput v-model="drawerForm.description" type="textarea" :rows="2" :maxlength="500" />
      </el-form-item>
      <el-form-item label="上架状态">
        <UiRadioGroup v-model="drawerForm.status">
          <el-radio :value="1">上架</el-radio>
          <el-radio :value="0">下架</el-radio>
        </UiRadioGroup>
      </el-form-item>
      <el-form-item label="热门">
        <UiCheckbox v-model="drawerForm.is_hot">热门</UiCheckbox>
      </el-form-item>
      <el-form-item label="精品">
        <UiCheckbox v-model="drawerForm.is_featured">精品</UiCheckbox>
      </el-form-item>
    </el-form>

    <div class="cc-chapters" v-if="drawerForm.course_id">
      <div class="cc-chapters-head">
        章节管理
        <UiButton variant="primary" plain size="small" @click="openChapterDialog()">新增章节</UiButton>
      </div>
      <div v-for="(ch, i) in drawerChapters" :key="ch.chapter_id" class="cc-chapter-row">
        <span class="cc-chapter-idx">{{ i + 1 }}</span>
        <span class="cc-chapter-title">{{ ch.title }}</span>
        <UiButton link size="small" @click="openChapterDialog(ch)">编辑</UiButton>
        <UiButton link size="small" @click="moveChapter(ch, -1)">上移</UiButton>
        <UiButton link size="small" @click="moveChapter(ch, 1)">下移</UiButton>
        <el-popconfirm title="确定删除该章节？" @confirm="handleDeleteChapter(ch)">
          <template #reference>
            <UiButton variant="danger" link size="small">删除</UiButton>
          </template>
        </el-popconfirm>
      </div>
      <div v-if="drawerChapters.length === 0" class="cc-chapters-empty">暂无章节</div>
    </div>

    <template #footer>
      <UiButton @click="drawerVisible = false">取消</UiButton>
      <UiButton variant="primary" :loading="submitting" @click="submitCourse">保存</UiButton>
    </template>
  </el-drawer>

  <!-- 章节对话框 -->
  <UiDialog v-model="chapterDialogVisible" :title="chapterForm.chapter_id ? '编辑章节' : '新增章节'" width="520px" destroy-on-close confirm-text="保存" :confirm-loading="submitting" @confirm="submitChapter">
    <el-form ref="chapterFormRef" :model="chapterForm" :rules="chapterRules" label-width="90px">
      <el-form-item label="章节标题" prop="title">
        <UiInput v-model="chapterForm.title" placeholder="章节标题" :maxlength="100" />
      </el-form-item>
      <el-form-item label="时长(分钟)">
        <el-input-number v-model="chapterForm.duration" :min="0" :max="9999" style="width: 100%" />
      </el-form-item>
    </el-form>
  </UiDialog>
</template>

<style scoped>
.cc-hours {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cc-hours-unit {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.cc-hint {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  line-height: 1.5;
  margin-top: 4px;
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
